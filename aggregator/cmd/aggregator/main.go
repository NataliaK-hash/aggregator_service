package main

import (
	"context"
	"fmt"
	"os"

	"aggregator/internal/app"
)

func main() {
	ctx := context.Background()

	application, err := app.InitializeApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize application: %v\n", err)
		os.Exit(1)
	}

	if err := application.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "application terminated with error: %v\n", err)
		os.Exit(1)
	}
}
