package domain

import "time"

// Measurement represents a single numeric observation produced by a source.
type Measurement struct {
	PacketID  string
	SourceID  string
	Value     float64
	Timestamp time.Time
}
