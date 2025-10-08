package storage

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"aggregator/internal/domain"
	pg "aggregator/internal/storage/postgres"
)

// TestPostgresRepositorySaveBatch проверяет, что батч сохраняется при достижении размера.
func TestPostgresRepositorySaveBatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	mock.ExpectPing()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO packet_max (id, timestamp, max_value) VALUES ($1,$2,$3),($4,$5,$6) ON CONFLICT (id) DO UPDATE SET timestamp = EXCLUDED.timestamp, max_value = EXCLUDED.max_value")).
		WithArgs("id-1", sqlmock.AnyArg(), 10, "id-2", sqlmock.AnyArg(), 20).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectClose()

	repo, err := pg.NewRepository(db, pg.WithBatchSize(2), pg.WithQueueSize(2), pg.WithFlushInterval(time.Hour))
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	packets := []domain.PacketMax{
		{ID: "id-1", Timestamp: time.Now(), MaxValue: 10},
		{ID: "id-2", Timestamp: time.Now().Add(time.Second), MaxValue: 20},
	}

	ctx := context.Background()
	if err := repo.Save(ctx, packets); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := repo.Close(ctx); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestPostgresRepositoryGetByID проверяет выборку по идентификатору.
func TestPostgresRepositoryGetByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	mock.ExpectPing()
	rows := sqlmock.NewRows([]string{"id", "timestamp", "max_value"}).
		AddRow("id-1", time.Now(), 42)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, timestamp, max_value FROM packet_max WHERE id = $1")).
		WithArgs("id-1").
		WillReturnRows(rows)
	mock.ExpectClose()

	repo, err := pg.NewRepository(db, pg.WithBatchSize(1), pg.WithFlushInterval(time.Hour))
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	result, err := repo.GetByID(context.Background(), "id-1")
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if result == nil {
		t.Fatalf("expected result")
	}
	if result.ID != "id-1" {
		t.Fatalf("unexpected id: %s", result.ID)
	}
	if result.MaxValue != 42 {
		t.Fatalf("unexpected max value: %d", result.MaxValue)
	}

	if err := repo.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestPostgresRepositoryGetByTimeRange проверяет выборку по диапазону времени.
func TestPostgresRepositoryGetByTimeRange(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	now := time.Now()

	mock.ExpectPing()
	rows := sqlmock.NewRows([]string{"id", "timestamp", "max_value"}).
		AddRow("id-1", now, 10).
		AddRow("id-2", now.Add(time.Minute), 20)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, timestamp, max_value FROM packet_max WHERE timestamp >= $1 AND timestamp < $2 ORDER BY timestamp")).
		WithArgs(now, now.Add(2*time.Minute)).
		WillReturnRows(rows)
	mock.ExpectClose()

	repo, err := pg.NewRepository(db, pg.WithBatchSize(1), pg.WithFlushInterval(time.Hour))
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	results, err := repo.GetByTimeRange(context.Background(), now, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("get by range: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("unexpected length: %d", len(results))
	}
	if results[0].ID != "id-1" {
		t.Fatalf("unexpected first id: %s", results[0].ID)
	}
	if results[1].ID != "id-2" {
		t.Fatalf("unexpected second id: %s", results[1].ID)
	}

	if err := repo.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
