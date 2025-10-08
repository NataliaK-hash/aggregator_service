package core

import "context"

// Logger describes minimal logging capabilities required by core components.
type Logger interface {
	Printf(ctx context.Context, format string, v ...any)
	Println(ctx context.Context, v ...any)
}
