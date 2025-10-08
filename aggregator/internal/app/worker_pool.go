package app

import (
	"context"
	"sync"
	"time"

	"aggregator/internal/domain"
)

type WorkerPool struct {
	workerCount int
	results     chan domain.PacketMax
	pool        *sync.Pool
	wg          sync.WaitGroup
	startOnce   sync.Once
	finishOnce  sync.Once
	done        chan struct{}
}

func NewWorkerPool(workerCount int) *WorkerPool {
	if workerCount <= 0 {
		workerCount = 1
	}

	wp := &WorkerPool{
		workerCount: workerCount,
		results:     make(chan domain.PacketMax, workerCount),
		pool: &sync.Pool{New: func() any {
			return &domain.PacketMax{}
		}},
		done: make(chan struct{}),
	}

	return wp
}

func (wp *WorkerPool) Start(ctx context.Context, packets <-chan domain.DataPacket) {
	wp.startOnce.Do(func() {
		for i := 0; i < wp.workerCount; i++ {
			wp.wg.Add(1)
			go wp.worker(ctx, packets)
		}

		go func() {
			wp.wg.Wait()
			wp.finishOnce.Do(func() {
				close(wp.results)
				close(wp.done)
			})
		}()
	})
}

func (wp *WorkerPool) Results() <-chan domain.PacketMax {
	return wp.results
}

func (wp *WorkerPool) Shutdown(ctx context.Context) error {
	select {
	case <-wp.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (wp *WorkerPool) worker(ctx context.Context, packets <-chan domain.DataPacket) {
	defer wp.wg.Done()

	for {
		select {
		case packet, ok := <-packets:
			if !ok {
				return
			}
			wp.processPacket(packet)
		case <-ctx.Done():
			wp.drainPackets(packets)
			return
		}
	}
}

func (wp *WorkerPool) drainPackets(packets <-chan domain.DataPacket) {
	for packet := range packets {
		wp.processPacket(packet)
	}
}

func (wp *WorkerPool) processPacket(packet domain.DataPacket) {
	result := wp.pool.Get().(*domain.PacketMax)
	result.ID = packet.ID.String()
	result.Timestamp = packet.Timestamp
	result.MaxValue = int(computeMax(packet.Payload))

	wp.publishResult(result)

	result.ID = ""
	result.Timestamp = time.Time{}
	result.MaxValue = 0
	wp.pool.Put(result)
}

func (wp *WorkerPool) publishResult(result *domain.PacketMax) {
	wp.results <- *result
}

func computeMax(payload []int64) int64 {
	if len(payload) == 0 {
		return 0
	}

	maxValue := payload[0]
	for i := 1; i < len(payload); i++ {
		if payload[i] > maxValue {
			maxValue = payload[i]
		}
	}

	return maxValue
}
