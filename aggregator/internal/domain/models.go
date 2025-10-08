package domain

import (
	"time"

	"github.com/google/uuid"
)

// DataPacket represents a unit of data produced by a generator and consumed by workers.
type DataPacket struct {
	ID        uuid.UUID
	Timestamp time.Time
	Payload   []int64
}

// PacketMax описывает агрегированный результат обработки пакета, содержащий максимум из полезной нагрузки.
type PacketMax struct {
	ID        string
	Timestamp time.Time
	MaxValue  int
}
