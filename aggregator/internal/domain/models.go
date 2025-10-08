package domain

import (
	"time"

	"github.com/google/uuid"
)

type DataPacket struct {
	ID        uuid.UUID
	Timestamp time.Time
	Payload   []int64
}

type PacketMax struct {
	ID        string
	Timestamp time.Time
	MaxValue  int
}
