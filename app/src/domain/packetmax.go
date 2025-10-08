package domain

import "time"

// PacketMax captures the maximum measurement observed within a packet.
type PacketMax struct {
	PacketID  string
	SourceID  string
	Value     float64
	Timestamp time.Time
}
