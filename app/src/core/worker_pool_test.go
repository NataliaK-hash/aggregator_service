package core

import (
	"aggregator-service/app/src/domain"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingRepo struct {
	mu      sync.Mutex
	records []domain.PacketMax
	err     error
}

func (r *recordingRepo) Add(_ context.Context, packetMax domain.PacketMax) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	r.records = append(r.records, packetMax)
	return nil
}

func (r *recordingRepo) calls() []domain.PacketMax {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.PacketMax(nil), r.records...)
}

func newTestRepo() *recordingRepo {
	return &recordingRepo{}
}

func newTestPool(workers int, repo *recordingRepo, logger *stubLogger) *WorkerPool {
	return NewWorkerPool(workers, repo, logger)
}

// ------------------
// Тесты
// ------------------

func TestNewWorkerPoolNormalizesWorkerCount(t *testing.T) {
	repo := newTestRepo()
	pool := NewWorkerPool(-1, repo, &stubLogger{})

	assert.Equal(t, 0, pool.workerCount)
	assert.Equal(t, repo, pool.repo)
}

func TestWorkerPoolRunWithZeroWorkersDrainsChannel(t *testing.T) {
	repo := newTestRepo()
	pool := newTestPool(0, repo, &stubLogger{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	packets := make(chan domain.DataPacket, 2)
	packets <- domain.DataPacket{ID: "one"}
	packets <- domain.DataPacket{ID: "two"}
	close(packets)

	done := make(chan struct{})
	go func() {
		pool.Run(ctx, packets)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("worker pool did not finish")
	}

	assert.Empty(t, repo.calls())
}

func TestWorkerPoolRunProcessesPackets(t *testing.T) {
	repo := newTestRepo()
	logger := &stubLogger{}
	pool := newTestPool(2, repo, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	packets := make(chan domain.DataPacket)

	go func() { pool.Run(ctx, packets) }()

	measurements := []domain.Measurement{{PacketID: "packet", SourceID: "source", Value: 10, Timestamp: time.Now()}}
	packets <- domain.DataPacket{ID: "packet", Measurements: measurements}
	close(packets)

	require.Eventually(t, func() bool {
		return len(repo.calls()) == 1
	}, time.Second, 10*time.Millisecond)
}

func TestProcessPacketChoosesMaxMeasurement(t *testing.T) {
	repo := newTestRepo()
	logger := &stubLogger{}
	pool := newTestPool(1, repo, logger)

	now := time.Now().UTC()
	packet := domain.DataPacket{ID: "packet", Measurements: []domain.Measurement{
		{PacketID: "packet", SourceID: "s1", Value: 1, Timestamp: now.Add(-time.Minute)},
		{PacketID: "packet", SourceID: "s2", Value: 5, Timestamp: now},
		{PacketID: "packet", SourceID: "s3", Value: 5, Timestamp: now.Add(time.Minute)},
	}}

	pool.processPacket(context.Background(), packet)

	calls := repo.calls()
	require.Len(t, calls, 1)
	assert.Equal(t, "s3", calls[0].SourceID)
	assert.Equal(t, 5.0, calls[0].Value)
	assert.Equal(t, packet.ID, calls[0].PacketID)
}

func TestProcessPacketSkipsEmptyMeasurements(t *testing.T) {
	repo := newTestRepo()
	pool := newTestPool(1, repo, &stubLogger{})

	pool.processPacket(context.Background(), domain.DataPacket{ID: "packet"})

	assert.Empty(t, repo.calls())
}

func TestProcessPacketStopsOnContextError(t *testing.T) {
	repo := newTestRepo()
	logger := &stubLogger{}
	pool := newTestPool(1, repo, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	packet := domain.DataPacket{ID: "packet", Measurements: []domain.Measurement{{PacketID: "packet", SourceID: "s1", Value: 1}}}
	pool.processPacket(ctx, packet)

	assert.Empty(t, repo.calls())
}

func TestProcessPacketLogsOnAddError(t *testing.T) {
	repo := &recordingRepo{err: errors.New("failure")}
	logger := &stubLogger{}
	pool := newTestPool(1, repo, logger)

	packet := domain.DataPacket{ID: "packet", Measurements: []domain.Measurement{{PacketID: "packet", SourceID: "s1", Value: 1}}}
	pool.processPacket(context.Background(), packet)

	assert.Empty(t, repo.calls())
	assert.NotEmpty(t, logger.messages())
}

func TestWorkerLoopHandlesClosedChannel(t *testing.T) {
	repo := newTestRepo()
	pool := newTestPool(1, repo, &stubLogger{})

	packets := make(chan domain.DataPacket)
	close(packets)

	pool.workerLoop(context.Background(), packets)
}

func TestWorkerLoopLogsOnContextCancel(t *testing.T) {
	repo := newTestRepo()
	logger := &stubLogger{}
	pool := newTestPool(1, repo, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	packets := make(chan domain.DataPacket)
	pool.workerLoop(ctx, packets)

	assert.NotEmpty(t, logger.messages())
}

func TestDrainUntilClosed(t *testing.T) {
	pool := newTestPool(0, newTestRepo(), &stubLogger{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	packets := make(chan domain.DataPacket, 2)
	packets <- domain.DataPacket{ID: "a"}
	close(packets)

	pool.drainUntilClosed(ctx, packets)
}

func TestWorkerPoolLog(t *testing.T) {
	logger := &stubLogger{}
	pool := newTestPool(1, newTestRepo(), logger)

	pool.log(context.Background(), "message %d", 1)

	assert.Contains(t, logger.messages()[0], "message 1")
}

func TestWorkerPoolLogNilLogger(t *testing.T) {
	pool := &WorkerPool{}
	assert.NotPanics(t, func() {
		pool.log(context.Background(), "ignored")
	})
}
