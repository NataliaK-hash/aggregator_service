package domain

import "time"

// DataPacket represents a unit of data produced by a generator and consumed by workers.
type DataPacket struct {
        ID        string
        Payload   []byte
        CreatedAt time.Time
}

// PacketMax describes limits or statistics associated with processed packets.
type PacketMax struct {
        ID        string
        Timestamp time.Time
        MaxValue  int
}
