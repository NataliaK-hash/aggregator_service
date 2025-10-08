package integration

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"aggregator-service/app/src/database"
	"aggregator-service/app/src/domain"
	"aggregator-service/app/src/shared/constants"
)

func TestRepositoryBatchFlushBySize(t *testing.T) {
	t.Log("Шаг 1: создаём репозиторий с маленьким батчем")
	ctx := context.Background()
	runner := newStubRunner(200 * time.Millisecond)

	repo, err := database.New(ctx, database.Config{
		DSN:          "postgres://user:pass@localhost:5432/test?sslmode=disable",
		Runner:       runner,
		BatchSize:    3,
		BufferSize:   3,
		BatchTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}
	t.Cleanup(func() {
		_ = repo.Close()
	})

	t.Log("Шаг 2: добавляем записи до заполнения батча")
	start := time.Now()
	for i := 0; i < 3; i++ {
		measurement := domain.PacketMax{
			PacketID:  constants.GenerateUUID(),
			SourceID:  constants.GenerateUUID(),
			Value:     float64(i),
			Timestamp: time.Now().UTC(),
		}
		if err := repo.Add(ctx, measurement); err != nil {
			t.Fatalf("enqueue measurement %d: %v", i, err)
		}
	}
	if elapsed := time.Since(start); elapsed >= runner.delay/2 {
		t.Fatalf("expected enqueue to be significantly faster than runner delay, took %s", elapsed)
	}

	t.Log("Шаг 3: ожидаем, что батч будет сброшен по размеру")
	waitForRunnerCalls(t, runner, 3, time.Second)
}

func TestRepositoryBatchFlushByTimeout(t *testing.T) {
	t.Log("Шаг 1: инициализируем репозиторий с таймаутом батча")
	ctx := context.Background()
	const timeout = 120 * time.Millisecond
	runner := newStubRunner(10 * time.Millisecond)

	repo, err := database.New(ctx, database.Config{
		DSN:          "postgres://user:pass@localhost:5432/test?sslmode=disable",
		Runner:       runner,
		BatchSize:    5,
		BufferSize:   5,
		BatchTimeout: timeout,
	})
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}
	t.Cleanup(func() {
		_ = repo.Close()
	})

	t.Log("Шаг 2: добавляем одну запись и проверяем скорость очереди")
	enqueueStart := time.Now()
	measurement := domain.PacketMax{
		PacketID:  constants.GenerateUUID(),
		SourceID:  constants.GenerateUUID(),
		Value:     1,
		Timestamp: time.Now().UTC(),
	}
	if err := repo.Add(ctx, measurement); err != nil {
		t.Fatalf("enqueue measurement: %v", err)
	}
	if elapsed := time.Since(enqueueStart); elapsed > 30*time.Millisecond {
		t.Fatalf("expected enqueue to be quick, took %s", elapsed)
	}

	t.Log("Шаг 3: убеждаемся, что сброс произошёл по таймауту")
	waitForRunnerCalls(t, runner, 1, time.Second)
	callTimes := runner.CallTimes()
	if len(callTimes) == 0 {
		t.Fatal("expected at least one runner call")
	}
	if diff := callTimes[0].Sub(enqueueStart); diff < timeout-30*time.Millisecond {
		t.Fatalf("expected flush after ~%s, got %s", timeout, diff)
	}
}

type stubRunner struct {
	mu         sync.Mutex
	statements []string
	times      []time.Time
	delay      time.Duration
}

func newStubRunner(delay time.Duration) *stubRunner {
	return &stubRunner{delay: delay}
}

func (s *stubRunner) Exec(ctx context.Context, _ string, _ string, statement string, _ ...any) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	trimmed := strings.TrimSpace(statement)

	s.mu.Lock()
	s.statements = append(s.statements, trimmed)
	s.times = append(s.times, time.Now())
	s.mu.Unlock()

	if s.delay > 0 {
		time.Sleep(s.delay)
	}

	upper := strings.ToUpper(trimmed)
	switch {
	case strings.HasPrefix(upper, "INSERT"):
		return "INSERT 0 1", nil
	case strings.HasPrefix(upper, "UPDATE"):
		return "UPDATE 1", nil
	default:
		return "", nil
	}
}

func (s *stubRunner) Close() error { return nil }

func (s *stubRunner) CallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.statements)
}

func (s *stubRunner) CallTimes() []time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]time.Time, len(s.times))
	copy(out, s.times)
	return out
}

func waitForRunnerCalls(t *testing.T, runner *stubRunner, expected int, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for {
		if runner.CallCount() >= expected {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected %d calls, got %d", expected, runner.CallCount())
		}
		time.Sleep(10 * time.Millisecond)
	}
}
