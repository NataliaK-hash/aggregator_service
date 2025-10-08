package domain

import "time"

type Measurement struct {
	PacketID  string
	SourceID  string
	Value     float64
	Timestamp time.Time
}
