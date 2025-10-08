package core

import "context"

type Logger interface {
	Printf(ctx context.Context, format string, v ...any)
	Println(ctx context.Context, v ...any)
}
