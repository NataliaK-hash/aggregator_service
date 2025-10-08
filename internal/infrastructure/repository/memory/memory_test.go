package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"aggregator-service-project/internal/domain"
	"aggregator-service-project/internal/infrastructure/repository/memory"
	"aggregator-service-project/internal/pkg/uuid"
)

func TestRepositoryAddAndMaxBySource(t *testing.T) {
	t.Parallel()

	repo := memory.New()
	ctx := context.Background()

	packetID := uuid.NewString()
	sourceID := uuid.NewString()
	measurement := domain.Measurement{PacketID: packetID, SourceID: sourceID, Value: 42.5, Timestamp: time.Now().UTC()}
	if err := repo.Add(ctx, measurement); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := repo.MaxBySource(ctx, sourceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Value != measurement.Value || result.SourceID != sourceID {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRepositoryMaxInRange(t *testing.T) {
	t.Parallel()

	repo := memory.New()
	now := time.Now().UTC().Truncate(time.Second)
	measurements := []domain.Measurement{
		{PacketID: "p1", SourceID: "s1", Value: 10, Timestamp: now.Add(-time.Minute)},
		{PacketID: "p2", SourceID: "s2", Value: 55, Timestamp: now.Add(-30 * time.Second)},
		{PacketID: "p3", SourceID: "s3", Value: 25, Timestamp: now.Add(time.Minute)},
	}
	repo.Seed(measurements)

	result, err := repo.MaxInRange(context.Background(), now.Add(-45*time.Second), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.PacketID != "p2" {
		t.Fatalf("expected packet p2, got %s", result.PacketID)
	}
}

func TestRepositoryReturnsErrNotFound(t *testing.T) {
	t.Parallel()

	repo := memory.New()
	_, err := repo.MaxBySource(context.Background(), uuid.NewString())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	_, err = repo.MaxInRange(context.Background(), time.Now(), time.Now().Add(time.Second))
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
