package timestamppb

import "time"

// Timestamp is a minimal representation of google.protobuf.Timestamp used in tests.
type Timestamp struct {
	Seconds int64
	Nanos   int32
}

// New constructs a Timestamp from the provided time value.
func New(t time.Time) *Timestamp {
	t = t.UTC()
	return &Timestamp{
		Seconds: t.Unix(),
		Nanos:   int32(t.Nanosecond()),
	}
}

// AsTime converts the timestamp into time.Time in UTC.
func (x *Timestamp) AsTime() time.Time {
	if x == nil {
		return time.Time{}
	}
	return time.Unix(x.Seconds, int64(x.Nanos)).UTC()
}

// IsValid reports whether the timestamp contains a valid value.
func (x *Timestamp) IsValid() bool {
	if x == nil {
		return false
	}
	// Reuse the same validation as the official implementation: seconds in [min, max] and nanos in [0, 999999999].
	if x.Nanos < 0 || x.Nanos >= int32(time.Second) {
		return false
	}
	const (
		minSeconds = -62135596800 // 0001-01-01T00:00:00Z
		maxSeconds = 253402300799 // 9999-12-31T23:59:59Z
	)
	return x.Seconds >= minSeconds && x.Seconds <= maxSeconds
}
