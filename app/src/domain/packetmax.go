package domain

import "time"

type PacketMax struct {
	PacketID  string
	SourceID  string
	Value     float64
	Timestamp time.Time
}
