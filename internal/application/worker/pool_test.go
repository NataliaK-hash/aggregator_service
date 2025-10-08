package worker_test

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"testing"
	"time"

	"aggregator-service-project/internal/application/worker"
	"aggregator-service-project/internal/domain"
)

type recordingRepo struct {
	mu           sync.Mutex
	measurements []domain.Measurement
	wg           *sync.WaitGroup
	errOnce      bool
}

func (r *recordingRepo) Add(_ context.Context, measurement domain.Measurement) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.wg != nil {
		defer r.wg.Done()
	}

	if r.errOnce {
		r.errOnce = false
		return errors.New("temporary failure")
	}

	r.measurements = append(r.measurements, measurement)
	return nil
}

func TestPoolProcessesPackets(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo := &recordingRepo{}
	logger := log.New(io.Discard, "", 0)
	pool := worker.New(2, repo, logger)

	packets := make(chan domain.DataPacket)
	done := make(chan struct{})

	go func() {
		pool.Run(ctx, packets)
		close(done)
	}()

	expected := 4
	repo.wg = &sync.WaitGroup{}
	repo.wg.Add(expected)

	for i := 0; i < 2; i++ {
		packet := domain.DataPacket{ID: "packet"}
		packet.Measurements = []domain.Measurement{{SourceID: "s1"}, {SourceID: "s2"}}
		packets <- packet
	}
	close(packets)

	waitWithTimeout(t, repo.wg, 200*time.Millisecond)

	cancel()
	waitForDone(t, done)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.measurements) != expected {
		t.Fatalf("expected %d measurements, got %d", expected, len(repo.measurements))
	}
}

func TestPoolStopsOnContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	logger := log.New(io.Discard, "", 0)
	repo := &recordingRepo{}
	pool := worker.New(1, repo, logger)

	packets := make(chan domain.DataPacket)
	done := make(chan struct{})
	go func() {
		pool.Run(ctx, packets)
		close(done)
	}()

	cancel()
	waitForDone(t, done)
}

func TestPoolContinuesAfterRepositoryError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo := &recordingRepo{errOnce: true, wg: &sync.WaitGroup{}}
	repo.wg.Add(2)

	logger := log.New(io.Discard, "", 0)
	pool := worker.New(1, repo, logger)

	packets := make(chan domain.DataPacket, 1)
	done := make(chan struct{})
	go func() {
		pool.Run(ctx, packets)
		close(done)
	}()

	packet := domain.DataPacket{ID: "packet", Measurements: []domain.Measurement{{SourceID: "s1"}, {SourceID: "s2"}}}
	packets <- packet
	close(packets)

	waitWithTimeout(t, repo.wg, 200*time.Millisecond)
	cancel()
	waitForDone(t, done)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.measurements) != 1 {
		t.Fatalf("expected 1 successful measurement, got %d", len(repo.measurements))
	}
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
