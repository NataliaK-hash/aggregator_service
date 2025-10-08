package domain

import "time"

type Measurement struct {
	SourceID  string
	Value     float64
	Timestamp time.Time
}
