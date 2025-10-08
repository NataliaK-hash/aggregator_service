package tests

import (
	"aggregator/internal/app"
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"aggregator/internal/domain"
)

// TestWorkerPoolCalculatesMax проверяет вычисление максимума полезной нагрузки и корректность результатов.
func TestWorkerPoolCalculatesMax(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	input := make(chan domain.DataPacket)
	pool := app.NewWorkerPool(2)
	pool.Start(ctx, input)

	packets := []domain.DataPacket{
		{
			ID:        uuid.New(),
			Timestamp: time.Unix(1, 0).UTC(),
			Payload:   []int64{1, 5, 3, 4},
		},
		{
			ID:        uuid.New(),
			Timestamp: time.Unix(2, 0).UTC(),
			Payload:   []int64{-10, -5, -7},
		},
		{
			ID:        uuid.New(),
			Timestamp: time.Unix(3, 0).UTC(),
			Payload:   []int64{42},
		},
	}

	expected := make(map[string]int, len(packets))
	for _, packet := range packets {
		expected[packet.ID.String()] = int(testMax(packet.Payload))
	}

	go func() {
		defer close(input)
		for _, packet := range packets {
			input <- packet
		}
	}()

	results := make(map[string]domain.PacketMax, len(packets))
	collectorDone := make(chan struct{})
	go func() {
		defer close(collectorDone)
		for result := range pool.Results() {
			results[result.ID] = result
		}
	}()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := pool.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("unexpected shutdown error: %v", err)
	}

	<-collectorDone

	if len(results) != len(expected) {
		t.Fatalf("unexpected number of results: got %d want %d", len(results), len(expected))
	}

	for id, maxValue := range expected {
		result, ok := results[id]
		if !ok {
			t.Fatalf("result for packet %s not found", id)
		}
		if result.MaxValue != maxValue {
			t.Errorf("max value mismatch for %s: got %d want %d", id, result.MaxValue, maxValue)
		}
	}
}

// TestWorkerPoolShutdownProcessesAllPackets убеждается, что при остановке обрабатываются все ожидающие пакеты.
func TestWorkerPoolShutdownProcessesAllPackets(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	input := make(chan domain.DataPacket, 16)
	pool := app.NewWorkerPool(4)
	pool.Start(ctx, input)

	const totalPackets = 200

	go func() {
		defer close(input)
		for i := 0; i < totalPackets; i++ {
			packet := domain.DataPacket{
				ID:        uuid.New(),
				Timestamp: time.Now().UTC(),
				Payload:   []int64{int64(i), int64(totalPackets - i)},
			}
			input <- packet
			if i == totalPackets/4 {
				cancel()
			}
		}
	}()

	var processed int64
	collectorDone := make(chan struct{})
	go func() {
		defer close(collectorDone)
		for range pool.Results() {
			atomic.AddInt64(&processed, 1)
		}
	}()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := pool.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("unexpected shutdown error: %v", err)
	}

	<-collectorDone

	if processed != totalPackets {
		t.Fatalf("not all packets processed: got %d want %d", processed, totalPackets)
	}
}

// testMax находит максимальный элемент в срезе для проверки результатов.
func testMax(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	maxValue := values[0]
	for i := 1; i < len(values); i++ {
		if values[i] > maxValue {
			maxValue = values[i]
		}
	}
	return maxValue
}
