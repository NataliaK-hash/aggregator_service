package postgres_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"aggregator-service-project/internal/domain"
	"aggregator-service-project/internal/infrastructure/repository/postgres"
	"aggregator-service-project/internal/pkg/uuid"
)

type fakeRunner struct {
	mu      sync.Mutex
	outputs map[string]string
	queries []string
	closed  bool
	err     error
}

func (f *fakeRunner) Exec(_ context.Context, _ string, _ string, sql string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, sql)
	if f.err != nil {
		return "", f.err
	}
	if output, ok := f.outputs[sql]; ok {
		return output, nil
	}
	return "", nil
}

func (f *fakeRunner) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func TestNewEnsuresSchema(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{outputs: map[string]string{}}
	cfg := postgres.Config{DSN: "postgres://user:pass@localhost:5432/db?sslmode=disable", Runner: runner}

	repo, err := postgres.New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	runner.mu.Lock()
	defer runner.mu.Unlock()

	if len(runner.queries) != 3 {
		t.Fatalf("expected 3 schema statements, got %d", len(runner.queries))
	}
	if !strings.Contains(runner.queries[0], "CREATE TABLE") {
		t.Fatalf("expected create table statement, got %s", runner.queries[0])
	}
}

func TestAddPersistsMeasurement(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{outputs: map[string]string{}}
	repo, err := postgres.New(context.Background(), postgres.Config{DSN: "postgres://user@localhost/db", Runner: runner})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	measurement := domain.Measurement{
		PacketID:  uuid.NewString(),
		SourceID:  uuid.NewString(),
		Value:     12.34,
		Timestamp: time.Now().UTC().Truncate(time.Second),
	}

	if err := repo.Add(context.Background(), measurement); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	runner.mu.Lock()
	defer runner.mu.Unlock()

	if len(runner.queries) == 0 {
		t.Fatal("expected query to be executed")
	}
	if !strings.Contains(runner.queries[len(runner.queries)-1], "INSERT INTO packet_max") {
		t.Fatalf("unexpected insert statement: %s", runner.queries[len(runner.queries)-1])
	}
}

func TestMaxBySourceParsesResult(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{outputs: map[string]string{}}
	repo, err := postgres.New(context.Background(), postgres.Config{DSN: "postgres://user@localhost/db", Runner: runner})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	packetID := uuid.NewString()
	sourceID := uuid.NewString()
	timestamp := time.Now().UTC().Truncate(time.Second)
	runner.outputs["SELECT packet_id::text, source_id::text, value, ts AT TIME ZONE 'UTC' FROM packet_max WHERE source_id = '"+sourceID+"'::uuid ORDER BY value DESC, ts DESC LIMIT 1"] = strings.Join([]string{packetID, sourceID, "55.5", timestamp.Format(time.RFC3339Nano)}, ",") + "\n"

	result, err := repo.MaxBySource(context.Background(), sourceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.PacketID != packetID || result.SourceID != sourceID || result.Value != 55.5 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestMaxBySourceNotFound(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{outputs: map[string]string{}}
	repo, err := postgres.New(context.Background(), postgres.Config{DSN: "postgres://user@localhost/db", Runner: runner})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	_, err = repo.MaxBySource(context.Background(), uuid.NewString())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMaxInRangeParsesResult(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{outputs: map[string]string{}}
	repo, err := postgres.New(context.Background(), postgres.Config{DSN: "postgres://user@localhost/db", Runner: runner})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	packetID := uuid.NewString()
	sourceID := uuid.NewString()
	timestamp := time.Now().UTC().Truncate(time.Second)
	query := "SELECT packet_id::text, source_id::text, value, ts AT TIME ZONE 'UTC' FROM packet_max WHERE ts BETWEEN '" +
		timestamp.Add(-time.Minute).Format(time.RFC3339Nano) +
		"'::timestamptz AND '" + timestamp.Format(time.RFC3339Nano) + "'::timestamptz ORDER BY value DESC, ts DESC LIMIT 1"
	runner.outputs[query] = strings.Join([]string{packetID, sourceID, "77.7", timestamp.Format(time.RFC3339Nano)}, ",") + "\n"

	result, err := repo.MaxInRange(context.Background(), timestamp.Add(-time.Minute), timestamp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.PacketID != packetID || result.SourceID != sourceID || result.Value != 77.7 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestCloseDelegatesToRunner(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{outputs: map[string]string{}}
	repo, err := postgres.New(context.Background(), postgres.Config{DSN: "postgres://user@localhost/db", Runner: runner})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := repo.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !runner.closed {
		t.Fatal("expected runner to be closed")
	}
}
