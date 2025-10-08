package tests

import (
	"aggregator/internal/generator"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"aggregator/internal/domain"
)

func TestRandomSourceUUIDUniquenessAndPayloadLength(t *testing.T) {
	const (
		k      = 8
		count  = 20
		buffer = 64
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	source := generator.NewRandomSource(generator.Config{
		PayloadLen: k,
		Interval:   10 * time.Millisecond,
		BufferSize: buffer,
	})
	packets := source.Start(ctx)

	seen := make(map[uuid.UUID]struct{}, count)
	deadline := time.After(time.Second)

	for i := 0; i < count; i++ {
		select {
		case packet, ok := <-packets:
			if !ok {
				t.Fatalf("channel closed before receiving %d packets", count)
			}

			if len(packet.Payload) != k {
				t.Fatalf("expected payload length %d, got %d", k, len(packet.Payload))
			}

			if _, exists := seen[packet.ID]; exists {
				t.Fatalf("duplicate packet ID detected: %s", packet.ID)
			}
			seen[packet.ID] = struct{}{}
		case <-deadline:
			t.Fatalf("timeout waiting for packets")
		}
	}
}

func TestRandomSourceTiming(t *testing.T) {
	const (
		k      = 4
		count  = 15
		buffer = 64
	)

	interval := 15 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	source := generator.NewRandomSource(generator.Config{
		PayloadLen: k,
		Interval:   interval,
		BufferSize: buffer,
	})
	packets := source.Start(ctx)

	collected := make([]domain.DataPacket, 0, count)
	deadline := time.After(2 * time.Second)

	for len(collected) < count {
		select {
		case packet, ok := <-packets:
			if !ok {
				t.Fatalf("channel closed before collecting %d packets", count)
			}
			collected = append(collected, packet)
		case <-deadline:
			t.Fatalf("timeout collecting packets")
		}
	}

	cancel()

	if len(collected) < 2 {
		t.Fatalf("need at least two packets to compute intervals")
	}

	var sum time.Duration
	for i := 1; i < len(collected); i++ {
		delta := collected[i].Timestamp.Sub(collected[i-1].Timestamp)
		if delta < 0 {
			delta = 0
		}
		sum += delta
	}

	avg := sum / time.Duration(len(collected)-1)
	minExpected := interval * 7 / 10
	maxExpected := interval * 3

	if avg < minExpected {
		t.Fatalf("average interval %s is shorter than expected minimum %s", avg, minExpected)
	}

	if avg > maxExpected {
		t.Fatalf("average interval %s exceeds expected maximum %s", avg, maxExpected)
	}
}

func TestRandomSourceCancellationClosesChannel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	source := generator.NewRandomSource(generator.Config{
		PayloadLen: 4,
		Interval:   5 * time.Millisecond,
		BufferSize: 4,
	})
	packets := source.Start(ctx)

	deadline := time.After(time.Second)
	for i := 0; i < 2; i++ {
		select {
		case _, ok := <-packets:
			if !ok {
				t.Fatalf("channel closed before receiving initial packets")
			}
		case <-deadline:
			t.Fatalf("timeout waiting for initial packets")
		}
	}

	cancel()

	closeDeadline := time.After(200 * time.Millisecond)
	for {
		select {
		case _, ok := <-packets:
			if !ok {
				return
			}
		case <-closeDeadline:
			t.Fatalf("channel did not close after cancellation")
		}
	}
}
