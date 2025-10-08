package autoload

import (
	"log"

	"aggregator-service-project/internal/pkg/dotenv"
)

func init() {
	if err := dotenv.Load(); err != nil {
		log.Printf("dotenv autoload: %v", err)
	}
}
