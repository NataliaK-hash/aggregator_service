package domain

import "errors"

// ErrNotFound is returned when no measurement satisfies the provided filters.
var ErrNotFound = errors.New("measurement not found")
