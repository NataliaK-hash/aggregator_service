package domain

import "errors"

var (
	ErrInvalidPacket     = errors.New("invalid data packet")
	ErrProcessingTimeout = errors.New("processing timeout")
)
