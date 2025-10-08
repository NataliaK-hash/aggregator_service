package unit

import (
	"context"
	"io"
	"math/rand"
	"sync"
	"testing"
	"time"

	"aggregator-service/app/src/core"
	"aggregator-service/app/src/domain"
	"aggregator-service/app/src/infra"
)

func TestGeneratorProducesPackets(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := infra.NewLogger(io.Discard, "test-core")
	cfg := core.GeneratorConfig{
		Interval:   5 * time.Millisecond,
		PacketSize: 3,
		RandSource: rand.NewSource(1),
	}

	packets := make(chan domain.DataPacket, 4)
	gen := core.NewGenerator(cfg, logger)

	go gen.Run(ctx, packets)

	var received []domain.DataPacket
	timeout := time.After(200 * time.Millisecond)
	for len(received) < 2 {
		select {
		case packet, ok := <-packets:
			if !ok {
				t.Fatalf("channel closed before receiving packets")
			}
			received = append(received, packet)
		case <-timeout:
			t.Fatalf("timeout waiting for packets")
		}
	}

	cancel()

	for _, packet := range received {
		if packet.ID == "" {
			t.Fatal("packet ID should not be empty")
		}
		if len(packet.Measurements) != cfg.PacketSize {
			t.Fatalf("expected %d measurements, got %d", cfg.PacketSize, len(packet.Measurements))
		}
		for _, measurement := range packet.Measurements {
			if measurement.PacketID != packet.ID {
				t.Fatalf("packet id mismatch: %s vs %s", measurement.PacketID, packet.ID)
			}
			if measurement.SourceID == "" {
				t.Fatal("source id should not be empty")
			}
			if measurement.Timestamp.IsZero() {
				t.Fatal("timestamp should be set")
			}
		}
	}
}

func TestWorkerPoolProcessesPackets(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo := &recordingRepo{}
	logger := infra.NewLogger(io.Discard, "test-core")
	pool := core.NewWorkerPool(2, repo, logger)

	packets := make(chan domain.DataPacket)
	done := make(chan struct{})

	go func() {
		pool.Run(ctx, packets)
		close(done)
	}()

	expected := 2
	repo.wg = &sync.WaitGroup{}
	repo.wg.Add(expected)

	for i := 0; i < 2; i++ {
		packet := domain.DataPacket{ID: "packet"}
		packet.Measurements = []domain.Measurement{{PacketID: packet.ID, SourceID: "s1", Value: 1, Timestamp: time.Now()}, {PacketID: packet.ID, SourceID: "s2", Value: 2, Timestamp: time.Now().Add(time.Millisecond)}}
		packets <- packet
	}
	close(packets)

	waitWithTimeout(t, repo.wg, 200*time.Millisecond)

	cancel()
	waitForDone(t, done)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.packetMaxes) != expected {
		t.Fatalf("expected %d packet maxima, got %d", expected, len(repo.packetMaxes))
	}
}

func TestWorkerPoolContinuesAfterError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo := &recordingRepo{errOnce: true, wg: &sync.WaitGroup{}}
	repo.wg.Add(2)

	logger := infra.NewLogger(io.Discard, "test-core")
	pool := core.NewWorkerPool(1, repo, logger)

	packets := make(chan domain.DataPacket, 1)
	done := make(chan struct{})
	go func() {
		pool.Run(ctx, packets)
		close(done)
	}()

	first := domain.DataPacket{ID: "packet1", Measurements: []domain.Measurement{{PacketID: "packet1", SourceID: "s1", Value: 1, Timestamp: time.Now()}}}
	second := domain.DataPacket{ID: "packet2", Measurements: []domain.Measurement{{PacketID: "packet2", SourceID: "s2", Value: 2, Timestamp: time.Now()}}}
	packets <- first
	packets <- second
	close(packets)

	waitWithTimeout(t, repo.wg, 200*time.Millisecond)
	cancel()
	waitForDone(t, done)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.packetMaxes) != 1 {
		t.Fatalf("expected 1 successful packet max, got %d", len(repo.packetMaxes))
	}
}

type recordingRepo struct {
	mu          sync.Mutex
	packetMaxes []domain.PacketMax
	wg          *sync.WaitGroup
	errOnce     bool
}

func (r *recordingRepo) Add(_ context.Context, packetMax domain.PacketMax) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.wg != nil {
		defer r.wg.Done()
	}

	if r.errOnce {
		r.errOnce = false
		return context.Canceled
	}

	r.packetMaxes = append(r.packetMaxes, packetMax)
	return nil
}

func waitWithTimeout(t *testing.T, wg *sync.WaitGroup, timeout time.Duration) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatalf("timeout waiting for waitgroup")
	}
}

func waitForDone(t *testing.T, done <-chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("pool did not stop in time")
	}
}
