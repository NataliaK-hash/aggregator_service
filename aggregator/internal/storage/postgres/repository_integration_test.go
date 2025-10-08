package postgres_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	testcontainers "github.com/testcontainers/testcontainers-go"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"

	"aggregator/internal/domain"
	"aggregator/internal/storage/postgres"
)

// TestPostgresRepositoryIntegration проверяет корректность базовых операций на реальной базе PostgreSQL.
func TestPostgresRepositoryIntegration(t *testing.T) {
	ctx := context.Background()

	container, err := postgrescontainer.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgrescontainer.WithDatabase("testdb"),
		postgrescontainer.WithUsername("postgres"),
		postgrescontainer.WithPassword("postgres"),
	)
	if err != nil {
		t.Fatalf("run container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Fatalf("terminate container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	_, file, _, _ := runtime.Caller(0)
	migrationPath := filepath.Join(filepath.Dir(file), "migrations", "0001_init.up.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err = db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	repo, err := postgres.NewRepository(db, postgres.WithBatchSize(1), postgres.WithFlushInterval(20*time.Millisecond))
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := repo.Close(closeCtx); err != nil {
			t.Fatalf("close repository: %v", err)
		}
	})

	first := domain.PacketMax{ID: uuid.NewString(), Timestamp: time.Now().UTC().Truncate(time.Millisecond), MaxValue: 15}
	second := domain.PacketMax{ID: uuid.NewString(), Timestamp: first.Timestamp.Add(2 * time.Second), MaxValue: 42}

	if err := repo.Save(ctx, []domain.PacketMax{first, second}); err != nil {
		t.Fatalf("save: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		saved, err := repo.GetByID(ctx, first.ID)
		if err != nil {
			t.Fatalf("get by id: %v", err)
		}
		if saved != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for first packet")
		}
		time.Sleep(50 * time.Millisecond)
	}

	savedFirst, err := repo.GetByID(ctx, first.ID)
	if err != nil {
		t.Fatalf("get first: %v", err)
	}
	if savedFirst == nil {
		t.Fatalf("first not found")
	}
	if savedFirst.MaxValue != first.MaxValue {
		t.Fatalf("first max mismatch: got %d want %d", savedFirst.MaxValue, first.MaxValue)
	}
	if !savedFirst.Timestamp.Equal(first.Timestamp) {
		t.Fatalf("first timestamp mismatch: %s vs %s", savedFirst.Timestamp, first.Timestamp)
	}

	var savedSecond *domain.PacketMax
	deadline = time.Now().Add(5 * time.Second)
	for {
		saved, err := repo.GetByID(ctx, second.ID)
		if err != nil {
			t.Fatalf("get second: %v", err)
		}
		if saved != nil {
			savedSecond = saved
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for second packet")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if savedSecond.MaxValue != second.MaxValue {
		t.Fatalf("second max mismatch: got %d want %d", savedSecond.MaxValue, second.MaxValue)
	}

	windowStart := first.Timestamp.Add(-time.Second)
	windowEnd := second.Timestamp.Add(time.Second)
	results, err := repo.GetByTimeRange(ctx, windowStart, windowEnd)
	if err != nil {
		t.Fatalf("get range: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("unexpected results len: %d", len(results))
	}
	if results[0].ID != first.ID {
		t.Fatalf("first id mismatch: %s vs %s", results[0].ID, first.ID)
	}
	if results[1].ID != second.ID {
		t.Fatalf("second id mismatch: %s vs %s", results[1].ID, second.ID)
	}
}
