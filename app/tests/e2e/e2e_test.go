package e2e

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"aggregator-service/app/src/infra/utils/dotenv"
	"aggregator-service/app/src/shared/constants"
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

type e2eConfig struct {
	BaseURL      string
	MetricsURL   string
	PGDSN        string
	PGDSNSource  string
	HTTPClient   *http.Client
	Timeout      time.Duration
	PollInterval time.Duration
}

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

func NewStepper(t *testing.T) *Stepper {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("E2E_OUTPUT")))
	if mode != "ci" {
		mode = "pretty"
	}
	verbose := mode == "pretty" || testing.Verbose()
	return &Stepper{verbose: verbose, mode: mode}
}

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
		t.Logf("STEP[%d] %s: started", step.idx, step.Name)
	}
	return step
}

func (st *Step) Info(t *testing.T, format string, args ...any) {
	if st == nil || st.stepper == nil {
		return
	}
	if !st.stepper.verbose {
		return
	}
	t.Logf("STEP[%d] %s: %s", st.idx, st.Name, fmt.Sprintf(format, args...))
}

func (st *Step) Success(t *testing.T, format string, args ...any) {
	if st == nil || st.stepper == nil {
		return
	}
	st.Done = time.Now()
	st.Status = "OK"
	if st.stepper.verbose {
		msg := "success"
		if format != "" {
			msg = fmt.Sprintf(format, args...)
		}
		t.Logf("STEP[%d] %s: %s", st.idx, st.Name, msg)
	}
}

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
		t.Errorf("STEP[%d] %s: %v", st.idx, st.Name, err)
	} else {
		t.Errorf("STEP[%d] %s: failed", st.idx, st.Name)
	}
}

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

	t.Log("================= E2E SUMMARY =================")
	t.Logf("Result: %-4s     Duration: %s", result, formatDuration(totalDuration))
	t.Logf("Base URL: %s       Metrics: %s", cfg.BaseURL, cfg.MetricsURL)
	maskedDSN := maskDSN(cfg.PGDSN)
	t.Logf("Postgres DSN: %s (source=%s)", maskedDSN, cfg.PGDSNSource)
	t.Log("Steps:")
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
	t.Log("Metrics:")
	t.Logf("  http_requests_total:    %.0f -> %.0f  (Δ=%.0f)", httpBefore, httpAfter, httpAfter-httpBefore)
	t.Logf("  aggregator_packets_total: %.0f -> %.0f  (Δ=%.0f)", packetsBefore, packetsAfter, packetsAfter-packetsBefore)
	t.Log("DB:")
	t.Logf("  packet_max rows:        %d -> %d  (Δ=%d)", rowsBefore, rowsAfter, rowsAfter-rowsBefore)
	t.Log("===============================================")
}

func TestE2E(t *testing.T) {
	if strings.TrimSpace(os.Getenv("RUN_E2E")) == "" {
		t.Skip("skipping integration test; set RUN_E2E=1 to enable")
	}
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
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
			t.Log("DIAGNOSTICS (on failure)")
			t.Log("- last scraped metrics sample (truncated to 500 chars):")
			t.Logf("  %s", truncateString(metricsSample, maxLoggedBodyLength))
			t.Logf("- last observed row count: %d", lastRowCount)
			t.Logf("- last /healthz body: %s", truncateString(lastHealthBody, maxLoggedBodyLength))
			t.Fatalf("E2E flow failed")
		}
	})

	// Step 1: Load config
	step := sp.Begin(t, "Load config")
	loadedCfg, err := loadConfig()
	if err != nil {
		step.Fail(t, err)
		return
	}
	cfg = loadedCfg
	step.Info(t, "BASE_URL=%s", cfg.BaseURL)
	step.Info(t, "METRICS_URL=%s", cfg.MetricsURL)
	step.Info(t, "PG_DSN(source=%s)=%s", cfg.PGDSNSource, maskDSN(cfg.PGDSN))
	step.Info(t, "timeout=%s poll_interval=%s", cfg.Timeout, cfg.PollInterval)
	step.Info(t, "http_client_timeout=%s", requestTimeout)
	step.Success(t, "configuration loaded")

	if reason := skipReasonForUnavailableService(cfg); reason != "" {
		skipped = true
		t.Skip(reason)
	}

	// Step 2: Health check
	step = sp.Begin(t, "Health check /healthz")
	status, body, err := callHealthz(cfg)
	lastHealthBody = string(body)
	if err != nil {
		step.Fail(t, err)
		return
	}
	if status != http.StatusOK {
		step.Fail(t, fmt.Errorf("unexpected status: %d body=%s", status, truncateString(string(body), maxLoggedBodyLength)))
		return
	}
	var healthPayload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &healthPayload); err != nil {
		step.Fail(t, fmt.Errorf("decode healthz response: %w (body=%s)", err, truncateString(string(body), maxLoggedBodyLength)))
		return
	}
	if strings.ToLower(strings.TrimSpace(healthPayload.Status)) != "ok" {
		step.Fail(t, fmt.Errorf("unexpected health status: %q", healthPayload.Status))
		return
	}
	step.Success(t, "GET /healthz returned status ok")

	// Step 3: Read initial metrics
	step = sp.Begin(t, "Read initial metrics")
	metrics, err := fetchMetricsWithTimeout(cfg)
	if err != nil {
		step.Fail(t, err)
		return
	}
	metricsSample = metrics
	httpValue, httpFound, err := parseMetricValue(metrics, "http_requests_total")
	if err != nil {
		step.Fail(t, err)
		return
	}
	if !httpFound {
		step.Info(t, "http_requests_total missing in metrics; assuming 0")
	}
	packetsValue, packetsFound, err := parseMetricValue(metrics, "aggregator_packets_total")
	if err != nil {
		step.Fail(t, err)
		return
	}
	if !packetsFound {
		step.Info(t, "aggregator_packets_total missing in metrics; assuming 0")
	}
	before["http_requests_total"] = httpValue
	before["aggregator_packets_total"] = packetsValue
	histogramExposed := metricExists(metrics, "aggregator_db_write_latency_seconds")
	if histogramExposed {
		step.Info(t, "aggregator_db_write_latency_seconds exposed: yes")
	} else {
		step.Info(t, "aggregator_db_write_latency_seconds exposed: no")
	}
	step.Info(t, "http_requests_total=%.0f aggregator_packets_total=%.0f", httpValue, packetsValue)
	step.Success(t, "initial metrics captured")

	// Step 4: Connect Postgres
	step = sp.Begin(t, "Connect Postgres")
	db, err := connectToDatabase(cfg)
	if err != nil {
		step.Fail(t, err)
		return
	}
	dbConn = db
	count, err := queryRowCount(cfg, dbConn)
	if err != nil {
		step.Fail(t, err)
		return
	}
	rowsBefore = count
	lastRowCount = count
	step.Info(t, "packet_max rows=%d", count)
	step.Success(t, "postgres connected and baseline rows read")

	// Step 5: Call /max valid
	step = sp.Begin(t, "Call /max valid")
	now := time.Now().UTC()
	from := now.Add(-5 * time.Minute)
	to := now.Add(1 * time.Minute)
	maxResp, err := callMaxValid(cfg, from, to)
	if err != nil {
		step.Fail(t, err)
		return
	}
	if maxResp.PacketID == "" {
		step.Fail(t, errors.New("empty packet_id in /max response"))
		return
	}
	if _, err := constants.ParseUUID(maxResp.PacketID); err != nil {
		step.Fail(t, fmt.Errorf("invalid packet_id format: %w", err))
		return
	}
	if maxResp.SourceID == "" {
		step.Fail(t, errors.New("empty source_id in /max response"))
		return
	}
	if _, err := constants.ParseUUID(maxResp.SourceID); err != nil {
		step.Fail(t, fmt.Errorf("invalid source_id format: %w", err))
		return
	}
	if math.IsNaN(maxResp.Value) || math.IsInf(maxResp.Value, 0) {
		step.Fail(t, fmt.Errorf("invalid value: %f", maxResp.Value))
		return
	}
	ts, err := time.Parse(constants.TimeFormat, maxResp.Timestamp)
	if err != nil {
		step.Fail(t, fmt.Errorf("invalid timestamp: %w", err))
		return
	}
	if ts.Before(from) || ts.After(to) {
		step.Fail(t, fmt.Errorf("timestamp %s outside requested range %s - %s", maxResp.Timestamp, from.Format(constants.TimeFormat), to.Format(constants.TimeFormat)))
		return
	}
	step.Info(t, "packet_id=%s source_id=%s value=%.2f timestamp=%s", maxResp.PacketID, maxResp.SourceID, maxResp.Value, maxResp.Timestamp)
	step.Success(t, "valid /max request succeeded")

	// Step 6: Call /max invalid
	step = sp.Begin(t, "Call /max invalid")
	errResp, err := callMaxInvalid(cfg)
	if err != nil {
		step.Fail(t, err)
		return
	}
	if errResp.Code != http.StatusBadRequest {
		step.Fail(t, fmt.Errorf("unexpected error code: %d", errResp.Code))
		return
	}
	if strings.TrimSpace(errResp.Error) == "" {
		step.Fail(t, errors.New("empty error message for invalid request"))
		return
	}
	step.Info(t, "error=%q code=%d", errResp.Error, errResp.Code)
	step.Success(t, "invalid /max request returned 400")

	// Step 7: Wait metrics delta
	step = sp.Begin(t, "Wait metrics delta")
	targetHTTP := before["http_requests_total"] + 2
	targetPackets := before["aggregator_packets_total"] + 1
	metrics, httpValue, packetsValue, err = waitForMetricsDelta(cfg, targetHTTP, targetPackets, step, t)
	if metrics != "" {
		metricsSample = metrics
	}
	after["http_requests_total"] = httpValue
	after["aggregator_packets_total"] = packetsValue
	if err != nil {
		step.Fail(t, err)
		return
	}
	histogramExposed = metricExists(metrics, "aggregator_db_write_latency_seconds")
	if histogramExposed {
		step.Info(t, "aggregator_db_write_latency_seconds exposed: yes")
	}
	batchSizeMetric := metricExists(metrics, "aggregator_db_batch_size")
	batchWaitMetric := metricExists(metrics, "aggregator_db_batch_wait_seconds")
	if !batchSizeMetric || !batchWaitMetric {
		step.Fail(t, fmt.Errorf("batch metrics missing size=%t wait=%t", batchSizeMetric, batchWaitMetric))
		return
	}
	step.Info(t, "batch metrics exposed: size=%t wait=%t", batchSizeMetric, batchWaitMetric)
	step.Success(t, "metrics reached targets http=%.0f packets=%.0f", httpValue, packetsValue)

	// Step 8: Wait DB row increase
	step = sp.Begin(t, "Wait DB row increase")
	count, err = waitForRowIncrease(cfg, dbConn, rowsBefore)
	if err != nil {
		rowsAfter = count
		lastRowCount = count
		step.Fail(t, err)
	} else {
		rowsAfter = count
		lastRowCount = count
		step.Info(t, "rows increased to %d", count)
		step.Success(t, "packet_max row count increased")
	}

	// Step 9: Inspect recent rows
	step = sp.Begin(t, "Inspect recent rows")
	rowsSummary, err := inspectRecentRows(cfg, dbConn)
	if err != nil {
		step.Fail(t, err)
		return
	}
	lastRowCount = rowsAfter
	step.Info(t, "%s", rowsSummary)
	step.Success(t, "recent packet_max rows inspected")
}

func loadConfig() (e2eConfig, error) {
	if err := dotenv.Load(); err != nil {
		return e2eConfig{}, fmt.Errorf("load dotenv: %w", err)
	}
	baseURL := strings.TrimRight(getEnv("BASE_URL", defaultBaseURL), "/")
	metricsURL := strings.TrimSpace(getEnv("METRICS_URL", defaultMetricsURL))
	pgDSN := strings.TrimSpace(os.Getenv("PG_DSN"))
	pgDSNSource := "PG_DSN"
	if pgDSN == "" {
		pgDSN = strings.TrimSpace(os.Getenv("DB_DSN"))
		if pgDSN != "" {
			pgDSNSource = "DB_DSN"
		} else {
			pgDSN = defaultPGDSN
			pgDSNSource = "default"
		}
	}

	timeout := time.Duration(defaultTimeoutSeconds) * time.Second
	if value := strings.TrimSpace(os.Getenv("E2E_TIMEOUT_SECONDS")); value != "" {
		secs, err := strconv.Atoi(value)
		if err != nil {
			return e2eConfig{}, fmt.Errorf("parse E2E_TIMEOUT_SECONDS=%q: %w", value, err)
		}
		timeout = time.Duration(secs) * time.Second
	}

	pollInterval := time.Duration(defaultPollIntervalMS) * time.Millisecond
	if value := strings.TrimSpace(os.Getenv("E2E_POLL_INTERVAL_MS")); value != "" {
		ms, err := strconv.Atoi(value)
		if err != nil {
			return e2eConfig{}, fmt.Errorf("parse E2E_POLL_INTERVAL_MS=%q: %w", value, err)
		}
		pollInterval = time.Duration(ms) * time.Millisecond
	}

	return e2eConfig{
		BaseURL:      baseURL,
		MetricsURL:   metricsURL,
		PGDSN:        pgDSN,
		PGDSNSource:  pgDSNSource,
		HTTPClient:   &http.Client{Timeout: requestTimeout},
		Timeout:      timeout,
		PollInterval: pollInterval,
	}, nil
}

func skipReasonForUnavailableService(cfg e2eConfig) string {
	if strings.TrimSpace(os.Getenv("E2E_FORCE")) != "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	status, _, err := doGET(ctx, cfg.HTTPClient, cfg.BaseURL+"/healthz")
	if err == nil && status == http.StatusOK {
		return ""
	}
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "connection refused") {
			return fmt.Sprintf("skipping E2E tests: service not reachable at %s/healthz (%v)", cfg.BaseURL, err)
		}
	}
	return ""
}

func callHealthz(cfg e2eConfig) (int, []byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	return doGET(ctx, cfg.HTTPClient, cfg.BaseURL+"/healthz")
}

func fetchMetricsWithTimeout(cfg e2eConfig) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	return fetchMetrics(ctx, cfg.HTTPClient, cfg.MetricsURL)
}

func connectToDatabase(cfg e2eConfig) (*sql.DB, error) {
	db, err := sql.Open("aggregator", cfg.PGDSN)
	if err != nil {
		return nil, fmt.Errorf("open aggregator: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}

func queryRowCount(cfg e2eConfig, db *sql.DB) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	return countRows(ctx, db)
}

func callMaxValid(cfg e2eConfig, from, to time.Time) (MaxResponse, error) {
	params := url.Values{}
	params.Set("from", from.Format(constants.TimeFormat))
	params.Set("to", to.Format(constants.TimeFormat))
	endpoint := cfg.BaseURL + "/max?" + params.Encode()
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	status, body, err := doGET(ctx, cfg.HTTPClient, endpoint)
	if err != nil {
		return MaxResponse{}, fmt.Errorf("call /max valid: %w", err)
	}
	if status != http.StatusOK {
		return MaxResponse{}, fmt.Errorf("unexpected status %d body=%s", status, truncateString(string(body), maxLoggedBodyLength))
	}
	var payload []MaxResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return MaxResponse{}, fmt.Errorf("decode /max response: %w (body=%s)", err, truncateString(string(body), maxLoggedBodyLength))
	}
	if len(payload) == 0 {
		return MaxResponse{}, errors.New("empty response payload")
	}
	return payload[0], nil
}

func callMaxInvalid(cfg e2eConfig) (ErrorResponse, error) {
	endpoint := cfg.BaseURL + "/max?packet_id=invalid"
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	status, body, err := doGET(ctx, cfg.HTTPClient, endpoint)
	if err != nil {
		return ErrorResponse{}, fmt.Errorf("call /max invalid: %w", err)
	}
	if status != http.StatusBadRequest {
		return ErrorResponse{}, fmt.Errorf("expected 400 status, got %d body=%s", status, truncateString(string(body), maxLoggedBodyLength))
	}
	var payload ErrorResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return ErrorResponse{}, fmt.Errorf("decode error response: %w (body=%s)", err, truncateString(string(body), maxLoggedBodyLength))
	}
	return payload, nil
}

func waitForMetricsDelta(cfg e2eConfig, targetHTTP, targetPackets float64, step *Step, t *testing.T) (string, float64, float64, error) {
	deadline := time.Now().Add(cfg.Timeout)
	var (
		lastMetrics          string
		lastHTTP             float64
		lastPackets          float64
		missingHTTPLogged    bool
		missingPacketsLogged bool
	)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		metrics, err := fetchMetrics(ctx, cfg.HTTPClient, cfg.MetricsURL)
		cancel()
		if err != nil {
			return metrics, lastHTTP, lastPackets, fmt.Errorf("fetch metrics: %w", err)
		}
		httpValue, foundHTTP, err := parseMetricValue(metrics, "http_requests_total")
		if err != nil {
			return metrics, lastHTTP, lastPackets, err
		}
		if !foundHTTP && step != nil && !missingHTTPLogged {
			step.Info(t, "http_requests_total missing in metrics; assuming 0")
			missingHTTPLogged = true
		}
		packetsValue, foundPackets, err := parseMetricValue(metrics, "aggregator_packets_total")
		if err != nil {
			return metrics, httpValue, packetsValue, err
		}
		if !foundPackets && step != nil && !missingPacketsLogged {
			step.Info(t, "aggregator_packets_total missing in metrics; assuming 0")
			missingPacketsLogged = true
		}
		lastMetrics = metrics
		lastHTTP = httpValue
		lastPackets = packetsValue
		if step != nil {
			step.Info(t, "waiting metrics http>=%.0f packets>=%.0f (now http=%.0f, packets=%.0f)", targetHTTP, targetPackets, httpValue, packetsValue)
		}
		if httpValue >= targetHTTP && packetsValue >= targetPackets {
			return metrics, httpValue, packetsValue, nil
		}
		time.Sleep(cfg.PollInterval)
	}
	return lastMetrics, lastHTTP, lastPackets, fmt.Errorf("metrics did not reach expected values: want http>=%.0f packets>=%.0f (last http=%.0f packets=%.0f)", targetHTTP, targetPackets, lastHTTP, lastPackets)
}

func waitForRowIncrease(cfg e2eConfig, db *sql.DB, baseline int64) (int64, error) {
	deadline := time.Now().Add(cfg.Timeout)
	var lastCount int64
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		count, err := countRows(ctx, db)
		cancel()
		if err != nil {
			return 0, fmt.Errorf("query packet_max count: %w", err)
		}
		lastCount = count
		if count > baseline {
			return count, nil
		}
		time.Sleep(cfg.PollInterval)
	}
	return lastCount, fmt.Errorf("packet_max row count did not increase above %d (last=%d)", baseline, lastCount)
}

func inspectRecentRows(cfg e2eConfig, db *sql.DB) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	rows, err := db.QueryContext(ctx, "SELECT packet_id::text, source_id::text, value, ts AT TIME ZONE 'UTC' FROM public.packet_max ORDER BY ts DESC LIMIT $1", recentRowsLimit)
	if err != nil {
		return "", fmt.Errorf("query recent rows: %w", err)
	}
	defer rows.Close()

	type row struct {
		PacketID  string
		SourceID  string
		Value     float64
		Timestamp time.Time
	}
	var result []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.PacketID, &r.SourceID, &r.Value, &r.Timestamp); err != nil {
			return "", fmt.Errorf("scan row: %w", err)
		}
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate rows: %w", err)
	}

	if len(result) == 0 {
		return "no rows returned", nil
	}

	var builder strings.Builder
	builder.WriteString("[")
	for i, r := range result {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(fmt.Sprintf("{packet:%s source:%s ts:%s value:%.2f}", r.PacketID, r.SourceID, r.Timestamp.Format(time.RFC3339), r.Value))
	}
	builder.WriteString("]")
	return truncateString(builder.String(), maxLoggedBodyLength), nil
}

func fetchMetrics(ctx context.Context, client *http.Client, metricsURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metricsURL, nil)
	if err != nil {
		return "", fmt.Errorf("create metrics request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("perform metrics request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("metrics status %d: %s", resp.StatusCode, truncateString(string(body), maxLoggedBodyLength))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read metrics body: %w", err)
	}
	return string(data), nil
}

func parseMetricValue(metrics, metricName string) (float64, bool, error) {
	scanner := bufio.NewScanner(strings.NewReader(metrics))
	var total float64
	var found bool
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, metricName) {
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			valueStr := fields[len(fields)-1]
			value, err := strconv.ParseFloat(valueStr, 64)
			if err != nil {
				return 0, true, fmt.Errorf("parse metric %s value %q: %w", metricName, valueStr, err)
			}
			total += value
			found = true
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, found, fmt.Errorf("scan metrics: %w", err)
	}
	return total, found, nil
}

func metricExists(metrics, prefix string) bool {
	scanner := bufio.NewScanner(strings.NewReader(metrics))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func countRows(ctx context.Context, db *sql.DB) (int64, error) {
	var count int64
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM public.packet_max").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func doGET(ctx context.Context, client *http.Client, target string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("create request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read body: %w", err)
	}
	return resp.StatusCode, body, nil
}

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

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return d.String()
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}
