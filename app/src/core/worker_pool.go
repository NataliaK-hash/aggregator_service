package core

import (
	"context"
	"sync"

	"aggregator-service/app/src/domain"
)

// WorkerPool consumes data packets and stores their measurements using the repository.
type WorkerPool struct {
	repo        domain.PacketMaxWriter
	workerCount int
	logger      Logger
}

// NewWorkerPool creates a pool with the provided repository and worker count.
func NewWorkerPool(workerCount int, repo domain.PacketMaxWriter, logger Logger) *WorkerPool {
	if workerCount < 0 {
		workerCount = 0
	}
	return &WorkerPool{repo: repo, workerCount: workerCount, logger: logger}
}

// Run starts the worker pool and blocks until the context is cancelled or the packets channel is closed.
func (p *WorkerPool) Run(ctx context.Context, packets <-chan domain.DataPacket) {
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

func (p *WorkerPool) workerLoop(ctx context.Context, packets <-chan domain.DataPacket) {
	for {
		select {
		case <-ctx.Done():
			p.log(ctx, "worker: context cancelled: %v", ctx.Err())
			return
		case packet, ok := <-packets:
			if !ok {
				return
			}
			p.processPacket(ctx, packet)
		}
	}
}

func (p *WorkerPool) processPacket(ctx context.Context, packet domain.DataPacket) {
	if len(packet.Measurements) == 0 {
		return
	}

	var (
		maxMeasurement domain.Measurement
		found          bool
	)
	for _, measurement := range packet.Measurements {
		if ctx.Err() != nil {
			p.log(ctx, "worker: aborting packet %s due to context: %v", packet.ID, ctx.Err())
			return
		}

		if !found || measurement.Value > maxMeasurement.Value || (measurement.Value == maxMeasurement.Value && measurement.Timestamp.After(maxMeasurement.Timestamp)) {
			maxMeasurement = measurement
			found = true
		}
	}

	if !found {
		return
	}

	packetMax := domain.PacketMax{
		PacketID:  maxMeasurement.PacketID,
		SourceID:  maxMeasurement.SourceID,
		Value:     maxMeasurement.Value,
		Timestamp: maxMeasurement.Timestamp,
	}

	if err := p.repo.Add(ctx, packetMax); err != nil {
		p.log(ctx, "worker: failed to store packet=%s source=%s: %v", packet.ID, packetMax.SourceID, err)
		return
	}
	p.log(ctx, "worker: stored packet=%s source=%s", packet.ID, packetMax.SourceID)
}

func (p *WorkerPool) drainUntilClosed(ctx context.Context, packets <-chan domain.DataPacket) {
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

func (p *WorkerPool) log(ctx context.Context, format string, v ...any) {
	if p.logger == nil {
		return
	}
	p.logger.Printf(ctx, format, v...)
}

var _ domain.WorkerPool = (*WorkerPool)(nil)
