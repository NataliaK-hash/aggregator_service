package errors

import "errors"

var (
	// ErrInvalidUUID is returned when a UUID fails validation.
	ErrInvalidUUID = errors.New("invalid uuid")
	// ErrInternal represents an unexpected application error.
	ErrInternal = errors.New("internal error")
)
