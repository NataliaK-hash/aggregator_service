package app

import (
	"context"
	"sync"
	"time"

	"aggregator/internal/domain"
)

// WorkerPool реализует асинхронный пул воркеров, которые вычисляют максимум значений в пакете данных.
type WorkerPool struct {
	workerCount int
	results     chan domain.PacketMax
	pool        *sync.Pool
	wg          sync.WaitGroup
	startOnce   sync.Once
	finishOnce  sync.Once
	done        chan struct{}
}

// NewWorkerPool создаёт пул воркеров указанного размера с подготовленным пулом переиспользуемых структур PacketMax.
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

// Start запускает воркеры, которые читают входящие пакеты и публикуют результаты в канал результатов.
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

// Results возвращает канал с рассчитанными значениями максимумов пакетов.
func (wp *WorkerPool) Results() <-chan domain.PacketMax {
	return wp.results
}

// Shutdown ожидает завершения работы воркеров или отмены контекста.
func (wp *WorkerPool) Shutdown(ctx context.Context) error {
	select {
	case <-wp.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// worker обрабатывает входящие пакеты в отдельной горутине.
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
			// После сигнала остановки завершаем обработку, дождались ли закрытия канала пакетов.
			wp.drainPackets(packets)
			return
		}
	}
}

// drainPackets дочитывает оставшиеся пакеты до закрытия канала.
func (wp *WorkerPool) drainPackets(packets <-chan domain.DataPacket) {
	for packet := range packets {
		wp.processPacket(packet)
	}
}

// processPacket извлекает максимум из пакета и публикует результат.
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

// publishResult отправляет рассчитанный максимум в канал результатов.
func (wp *WorkerPool) publishResult(result *domain.PacketMax) {
	wp.results <- *result
}

// computeMax находит максимальное значение в срезе полезной нагрузки.
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
