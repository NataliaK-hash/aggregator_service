package e2e

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	defaultBaseURL        = "http://localhost:8080"
	defaultMetricsURL     = "http://localhost:2112/metrics"
	defaultPGDSN          = "postgres://aggregator:aggregator@localhost:5432/aggregator?sslmode=disable"
	defaultTimeoutSeconds = 60
	defaultPollIntervalMS = 1000
	requestTimeout        = 10 * time.Second
	recentRowsLimit       = 5
	maxLoggedBodyLength   = 500
)

// Конфигурация E2E теста
type e2eConfig struct {
	BaseURL      string
	MetricsURL   string
	PGDSN        string
	PGDSNSource  string
	HTTPClient   *http.Client
	Timeout      time.Duration
	PollInterval time.Duration
}

// Форматы ответов API
type MaxResponse struct {
	PacketID  string  `json:"packet_id"`
	SourceID  string  `json:"source_id"`
	Value     float64 `json:"value"`
	Timestamp string  `json:"timestamp"`
}

type ErrorResponse struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}

// Шаги выполнения теста
type Step struct {
	idx     int
	Name    string
	Started time.Time
	Done    time.Time
	Status  string
	Err     error
	stepper *Stepper
}

type Stepper struct {
	mu      sync.Mutex
	steps   []*Step
	verbose bool
	mode    string
	failed  bool
}

// Создание степпера
func NewStepper(t *testing.T) *Stepper {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("E2E_OUTPUT")))
	if mode != "ci" {
		mode = "pretty"
	}
	verbose := mode == "pretty" || testing.Verbose()
	return &Stepper{verbose: verbose, mode: mode}
}

// Начало шага
func (s *Stepper) Begin(t *testing.T, name string) *Step {
	s.mu.Lock()
	defer s.mu.Unlock()
	step := &Step{
		idx:     len(s.steps) + 1,
		Name:    name,
		Started: time.Now(),
		Status:  "RUNNING",
		stepper: s,
	}
	s.steps = append(s.steps, step)
	if s.verbose {
		t.Logf("[%d] %s — начат", step.idx, step.Name)
	}
	return step
}

// Лог информации
func (st *Step) Info(t *testing.T, format string, args ...any) {
	if st == nil || st.stepper == nil || !st.stepper.verbose {
		return
	}
	t.Logf("[%d] %s — %s", st.idx, st.Name, fmt.Sprintf(format, args...))
}

// Успешный шаг
func (st *Step) Success(t *testing.T, format string, args ...any) {
	if st == nil || st.stepper == nil {
		return
	}
	st.Done = time.Now()
	st.Status = "OK"
	if st.stepper.verbose {
		msg := "успешно"
		if format != "" {
			msg = fmt.Sprintf(format, args...)
		}
		t.Logf("[%d] %s — %s", st.idx, st.Name, msg)
	}
}

// Ошибка шага
func (st *Step) Fail(t *testing.T, err error) {
	if st == nil || st.stepper == nil {
		return
	}
	st.Done = time.Now()
	st.Status = "FAIL"
	st.Err = err
	st.stepper.mu.Lock()
	st.stepper.failed = true
	st.stepper.mu.Unlock()
	if err != nil {
		t.Errorf("[%d] %s — ошибка: %v", st.idx, st.Name, err)
	} else {
		t.Errorf("[%d] %s — ошибка без описания", st.idx, st.Name)
	}
}

// Проверка ошибок
func (s *Stepper) HasFailure() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failed {
		return true
	}
	for _, st := range s.steps {
		if st.Err != nil {
			return true
		}
	}
	return false
}

// Финальное резюме
func (s *Stepper) Summary(t *testing.T, cfg e2eConfig, before, after map[string]float64, rowsBefore, rowsAfter int64, started time.Time, skipped bool) {
	s.mu.Lock()
	steps := make([]*Step, len(s.steps))
	copy(steps, s.steps)
	s.mu.Unlock()

	totalDuration := time.Since(started)
	result := "PASS"
	if skipped {
		result = "SKIP"
	} else if s.HasFailure() || t.Failed() {
		result = "FAIL"
	}

	t.Log("========== РЕЗЮМЕ E2E-ТЕСТА ==========")
	t.Logf("Результат: %-4s     Время выполнения: %s", result, formatDuration(totalDuration))
	t.Logf("Base URL: %s       Metrics: %s", cfg.BaseURL, cfg.MetricsURL)
	maskedDSN := maskDSN(cfg.PGDSN)
	t.Logf("Postgres DSN: %s (источник=%s)", maskedDSN, cfg.PGDSNSource)
	t.Log("Шаги:")
	for _, st := range steps {
		if st.Done.IsZero() {
			st.Done = time.Now()
		}
		duration := st.Done.Sub(st.Started)
		status := st.Status
		if status == "" {
			status = "PENDING"
		}
		t.Logf(" %d) %-30s %-4s (%s)", st.idx, st.Name, status, formatDuration(duration))
	}

	httpBefore := before["http_requests_total"]
	httpAfter := after["http_requests_total"]
	packetsBefore := before["aggregator_packets_total"]
	packetsAfter := after["aggregator_packets_total"]

	t.Log("")
	t.Log("Метрики:")
	t.Logf("  http_requests_total:    %.0f -> %.0f  (Δ=%.0f)", httpBefore, httpAfter, httpAfter-httpBefore)
	t.Logf("  aggregator_packets_total: %.0f -> %.0f  (Δ=%.0f)", packetsBefore, packetsAfter, packetsAfter-packetsBefore)
	t.Log("База данных:")
	t.Logf("  packet_max rows:        %d -> %d  (Δ=%d)", rowsBefore, rowsAfter, rowsAfter-rowsBefore)
	t.Log("======================================")
}

// Основной тест
func TestE2E(t *testing.T) {
	if strings.TrimSpace(os.Getenv("RUN_E2E")) == "" {
		t.Skip("Пропуск интеграционных тестов (установите RUN_E2E=1 для запуска)")
	}
	if testing.Short() {
		t.Skip("Пропуск E2E в режиме short")
	}

	started := time.Now()
	sp := NewStepper(t)

	var (
		cfg            e2eConfig
		before         = map[string]float64{}
		after          = map[string]float64{}
		rowsBefore     int64
		rowsAfter      int64
		metricsSample  string
		lastRowCount   int64
		lastHealthBody string
		dbConn         *sql.DB
		skipped        bool
	)

	t.Cleanup(func() {
		if dbConn != nil {
			dbConn.Close()
		}
		sp.Summary(t, cfg, before, after, rowsBefore, rowsAfter, started, skipped)
		if !skipped && (sp.HasFailure() || t.Failed()) {
			t.Log("Диагностика при ошибке:")
			t.Log("- последние метрики (усечены до 500 символов):")
			t.Logf("  %s", truncateString(metricsSample, maxLoggedBodyLength))
			t.Logf("- последнее количество строк в БД: %d", lastRowCount)
			t.Logf("- последнее тело /healthz: %s", truncateString(lastHealthBody, maxLoggedBodyLength))
			t.Fatalf("E2E-поток завершился с ошибкой")
		}
	})
}

// Вспомогательные функции

func truncateString(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit]
}

func maskDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return dsn
	}
	if u.User != nil {
		username := u.User.Username()
		if _, hasPassword := u.User.Password(); hasPassword {
			u.User = url.UserPassword(username, "***")
		}
	}
	return u.String()
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return d.String()
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}
