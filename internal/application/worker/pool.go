package worker

import (
	"context"
	"sync"

	"aggregator-service-project/internal/domain"
)

// Logger defines the logging behaviour required by the worker pool.
type Logger interface {
	Printf(format string, v ...any)
}

// Pool consumes data packets and stores their measurements using the repository.
type Pool struct {
	repo        domain.MeasurementWriter
	workerCount int
	logger      Logger
}

// New creates a pool with the provided repository and worker count.
func New(workerCount int, repo domain.MeasurementWriter, logger Logger) *Pool {
	if workerCount < 0 {
		workerCount = 0
	}
	return &Pool{repo: repo, workerCount: workerCount, logger: logger}
}

// Run starts the worker pool and blocks until the context is cancelled or the
// packets channel is closed.
func (p *Pool) Run(ctx context.Context, packets <-chan domain.DataPacket) {
	if p.workerCount == 0 {
		p.drainUntilClosed(ctx, packets)
		return
	}

	var wg sync.WaitGroup
	wg.Add(p.workerCount)
	for i := 0; i < p.workerCount; i++ {
		go func() {
			defer wg.Done()
			p.workerLoop(ctx, packets)
		}()
	}
	wg.Wait()
}

func (p *Pool) workerLoop(ctx context.Context, packets <-chan domain.DataPacket) {
	for {
		select {
		case <-ctx.Done():
			p.log("worker: context cancelled: %v", ctx.Err())
			return
		case packet, ok := <-packets:
			if !ok {
				return
			}
			p.processPacket(ctx, packet)
		}
	}
}

func (p *Pool) processPacket(ctx context.Context, packet domain.DataPacket) {
	for _, measurement := range packet.Measurements {
		if ctx.Err() != nil {
			p.log("worker: aborting packet %s due to context: %v", packet.ID, ctx.Err())
			return
		}

		if err := p.repo.Add(ctx, measurement); err != nil {
			p.log("worker: failed to store packet=%s source=%s: %v", packet.ID, measurement.SourceID, err)
			continue
		}
		p.log("worker: stored packet=%s source=%s", packet.ID, measurement.SourceID)
	}
}

func (p *Pool) drainUntilClosed(ctx context.Context, packets <-chan domain.DataPacket) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-packets:
			if !ok {
				return
			}
		}
	}
}

func (p *Pool) log(format string, v ...any) {
	if p.logger == nil {
		return
	}
	p.logger.Printf(format, v...)
}

var _ domain.WorkerPool = (*Pool)(nil)
