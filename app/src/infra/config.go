package infra

import (
	"context"
	"os"
	"strconv"

	"aggregator-service/app/src/infra/utils"
)

type Config struct {
	HTTPPort                string
	GRPCPort                string
	MetricsPort             string
	DatabaseDSN             string
	DatabaseHost            string
	DatabasePort            string
	DatabaseUser            string
	DatabasePassword        string
	DatabaseName            string
	DatabaseBatchSize       int
	DatabaseBatchTimeoutMS  int
	DatabaseBatchBufferSize int
	GeneratorIntervalMillis int
	MeasurementsPerPacket   int
	WorkerCount             int
	PacketBufferSize        int
}

func LoadConfig() Config {
	return Config{
		HTTPPort:                getEnv("HTTP_PORT", "8080"),
		GRPCPort:                getEnv("GRPC_PORT", "50051"),
		MetricsPort:             getEnv("METRICS_PORT", "2112"),
		DatabaseDSN:             os.Getenv("DB_DSN"),
		DatabaseHost:            os.Getenv("DB_HOST"),
		DatabasePort:            os.Getenv("DB_PORT"),
		DatabaseUser:            os.Getenv("DB_USER"),
		DatabasePassword:        os.Getenv("DB_PASSWORD"),
		DatabaseName:            os.Getenv("DB_NAME"),
		DatabaseBatchSize:       getEnvInt("DB_BATCH_SIZE", 32),
		DatabaseBatchTimeoutMS:  getEnvInt("DB_BATCH_TIMEOUT_MS", 250),
		DatabaseBatchBufferSize: getEnvInt("DB_BATCH_BUFFER", 128),
		GeneratorIntervalMillis: getEnvInt("N", 1000),
		MeasurementsPerPacket:   getEnvInt("K", 10),
		WorkerCount:             getEnvInt("M", 4),
		PacketBufferSize:        getEnvInt("PACKET_BUFFER", 100),
	}
}

func LogConfig(ctx context.Context, logger *Logger, cfg Config) {
	logger.Printf(ctx, "HTTP_PORT=%s", cfg.HTTPPort)
	logger.Printf(ctx, "GRPC_PORT=%s", cfg.GRPCPort)
	logger.Printf(ctx, "METRICS_PORT=%s", utils.EmptyFallback(cfg.MetricsPort, "(disabled)"))
	if cfg.DatabaseDSN != "" {
		logger.Printf(ctx, "DB_DSN set (length %d)", len(cfg.DatabaseDSN))
	} else {
		logger.Println(ctx, "DB_DSN not provided")
	}
	logger.Printf(ctx, "DB_HOST=%s", utils.EmptyFallback(cfg.DatabaseHost, "(not set)"))
	logger.Printf(ctx, "DB_PORT=%s", utils.EmptyFallback(cfg.DatabasePort, "(not set)"))
	logger.Printf(ctx, "DB_USER=%s", utils.EmptyFallback(cfg.DatabaseUser, "(not set)"))
	if cfg.DatabasePassword != "" {
		logger.Println(ctx, "DB_PASSWORD set (redacted)")
	} else {
		logger.Println(ctx, "DB_PASSWORD not provided")
	}
	logger.Printf(ctx, "DB_NAME=%s", utils.EmptyFallback(cfg.DatabaseName, "(not set)"))
	logger.Printf(ctx, "DB_BATCH_SIZE=%d", cfg.DatabaseBatchSize)
	logger.Printf(ctx, "DB_BATCH_TIMEOUT_MS=%d", cfg.DatabaseBatchTimeoutMS)
	logger.Printf(ctx, "DB_BATCH_BUFFER=%d", cfg.DatabaseBatchBufferSize)
	logger.Printf(ctx, "GENERATOR_INTERVAL_MS=%d", cfg.GeneratorIntervalMillis)
	logger.Printf(ctx, "MEASUREMENTS_PER_PACKET=%d", cfg.MeasurementsPerPacket)
	logger.Printf(ctx, "WORKER_COUNT=%d", cfg.WorkerCount)
	logger.Printf(ctx, "PACKET_BUFFER=%d", cfg.PacketBufferSize)
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return fallback
}
