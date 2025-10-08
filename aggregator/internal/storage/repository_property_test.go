package storage

import (
	"context"
	"database/sql"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"testing/quick"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	testcontainers "github.com/testcontainers/testcontainers-go"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"

	"aggregator/internal/domain"
	"aggregator/internal/storage/postgres"
)

type packetSample domain.PacketMax

func (packetSample) Generate(r *rand.Rand, _ int) reflect.Value {
	offset := r.Intn(27) - 13
	zone := time.FixedZone("prop", offset*3600)
	ts := time.Unix(r.Int63n(1_000_000), int64(r.Intn(1_000_000_000))).In(zone)

	sample := packetSample(domain.PacketMax{
		ID:        uuid.NewString(),
		Timestamp: ts,
		MaxValue:  int(r.Int31()),
	})

	return reflect.ValueOf(sample)
}

func TestPostgresRepositoryPropertyBased(t *testing.T) {
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
	migrationPath := filepath.Join(filepath.Dir(file), "postgres", "migrations", "0001_init.up.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err = db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	repo, err := postgres.NewRepository(db, postgres.WithBatchSize(4), postgres.WithFlushInterval(25*time.Millisecond))
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

	config := &quick.Config{MaxCount: 20, Rand: rand.New(rand.NewSource(time.Now().UnixNano()))}

	property := func(sample packetSample) bool {
		packet := domain.PacketMax(sample)
		packet.Timestamp = packet.Timestamp.UTC()

		saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := repo.Save(saveCtx, []domain.PacketMax{packet}); err != nil {
			t.Logf("save error: %v", err)
			return false
		}

		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			stored, err := repo.GetByID(context.Background(), packet.ID)
			if err != nil {
				t.Logf("get error: %v", err)
				return false
			}
			if stored == nil {
				time.Sleep(25 * time.Millisecond)
				continue
			}

			if !stored.Timestamp.Equal(packet.Timestamp) || stored.MaxValue != packet.MaxValue {
				t.Logf("mismatch: expected %+v got %+v", packet, stored)
				return false
			}

			return true
		}

		t.Logf("timeout waiting for packet %s", packet.ID)
		return false
	}

	if err := quick.Check(property, config); err != nil {
		t.Fatalf("property check failed: %v", err)
	}
}
