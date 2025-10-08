package domain

import "errors"

var (
	// ErrInvalidPacket indicates that a data packet failed validation.
	ErrInvalidPacket = errors.New("invalid data packet")
	// ErrProcessingTimeout indicates that processing exceeded the allowed time window.
	ErrProcessingTimeout = errors.New("processing timeout")
)
