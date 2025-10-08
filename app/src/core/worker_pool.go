package core

import (
	"aggregator-service/app/src/domain"
	"aggregator-service/app/src/infra"
	"context"
	"sync"
)

type WorkerPool struct {
	repo        domain.PacketMaxWriter
	workerCount int
	logger      Logger
}

func NewWorkerPool(workerCount int, repo domain.PacketMaxWriter, logger Logger) *WorkerPool {
	if workerCount < 0 {
		workerCount = 0
	}
	return &WorkerPool{repo: repo, workerCount: workerCount, logger: logger}
}

func (p *WorkerPool) Run(ctx context.Context, packets <-chan domain.DataPacket) {
	if p.workerCount == 0 {
		p.drainUntilClosed(ctx, packets)
		return
	}

	var wg sync.WaitGroup
	wg.Add(p.workerCount)

	for i := 0; i < p.workerCount; i++ {
		go func() {
			infra.WorkerStarted()
			defer wg.Done()
			defer infra.WorkerFinished()
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

	maxMeasurement, found := p.findMaxMeasurement(ctx, packet)
	if !found {
		return
	}

	p.storePacketMax(ctx, packet, maxMeasurement)
}

func (p *WorkerPool) findMaxMeasurement(ctx context.Context, packet domain.DataPacket) (domain.Measurement, bool) {
	var (
		maxMeasurement domain.Measurement
		found          bool
	)
	for _, m := range packet.Measurements {
		if ctx.Err() != nil {
			p.log(ctx, "worker: aborting packet %s due to context: %v", packet.ID, ctx.Err())
			return domain.Measurement{}, false
		}
		if !found || m.Value > maxMeasurement.Value ||
			(m.Value == maxMeasurement.Value && m.Timestamp.After(maxMeasurement.Timestamp)) {
			maxMeasurement = m
			found = true
		}
	}
	return maxMeasurement, found
}

func (p *WorkerPool) storePacketMax(ctx context.Context, packet domain.DataPacket, m domain.Measurement) {
	packetMax := domain.PacketMax{
		PacketID:  m.PacketID,
		SourceID:  m.SourceID,
		Value:     m.Value,
		Timestamp: m.Timestamp,
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
	if p.logger != nil {
		p.logger.Printf(ctx, format, v...)
	}
}

var _ domain.WorkerPool = (*WorkerPool)(nil)
