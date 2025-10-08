package tests

import (
	"aggregator/internal/app"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"aggregator/internal/domain"
)

func BenchmarkWorkerPoolThroughput(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	input := make(chan domain.DataPacket, 1024)
	pool := app.NewWorkerPool(8)
	pool.Start(ctx, input)

	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		for range pool.Results() {
		}
	}()

	payload := make([]int64, 64)
	for i := range payload {
		payload[i] = int64(i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		packet := domain.DataPacket{
			ID:        uuid.New(),
			Timestamp: time.Now().UTC(),
			Payload:   payload,
		}
		input <- packet
	}
	b.StopTimer()

	close(input)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := pool.Shutdown(shutdownCtx); err != nil {
		b.Fatalf("unexpected shutdown error: %v", err)
	}

	<-drainDone
}
