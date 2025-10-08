package errors

import "errors"

var (
	ErrInvalidUUID = errors.New("invalid uuid")
	ErrInternal    = errors.New("internal error")
)
