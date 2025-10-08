package generator_test

import (
	"context"
	"io"
	"log"
	"math/rand"
	"testing"
	"time"

	"aggregator-service-project/internal/application/generator"
	"aggregator-service-project/internal/domain"
)

func TestGeneratorProducesPackets(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.New(io.Discard, "", 0)
	cfg := generator.Config{
		Interval:   5 * time.Millisecond,
		PacketSize: 3,
		RandSource: rand.NewSource(1),
	}

	packets := make(chan domain.DataPacket, 4)
	gen := generator.New(cfg, logger)

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

func TestGeneratorStopsOnContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	logger := log.New(io.Discard, "", 0)
	cfg := generator.Config{Interval: 5 * time.Millisecond, PacketSize: 1, RandSource: rand.NewSource(1)}

	packets := make(chan domain.DataPacket)
	gen := generator.New(cfg, logger)

	done := make(chan struct{})
	go func() {
		gen.Run(ctx, packets)
		close(done)
	}()

	// Allow one packet to be produced.
	select {
	case <-packets:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected packet to be produced before cancellation")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("generator did not stop after cancellation")
	}

	if _, ok := <-packets; ok {
		t.Fatal("expected channel to be closed after generator stops")
	}
}
