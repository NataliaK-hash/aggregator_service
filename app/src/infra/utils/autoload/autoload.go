package autoload

import (
	"context"
	"os"

	"aggregator-service/app/src/infra"
	"aggregator-service/app/src/infra/utils/dotenv"
)

var logger = infra.NewLogger(os.Stdout, "autoload")

func init() {
	if err := dotenv.Load(); err != nil {
		logger.Printf(context.Background(), "dotenv autoload: %v", err)
	}
}
