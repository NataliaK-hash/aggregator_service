package config

import "time"

const (
	// ENV keys
	EnvGeneratorPayloadLen = "GEN_K"
	EnvGeneratorInterval   = "GEN_N"
	EnvWorkerPoolSize      = "WORKER_POOL_SIZE"
	EnvDbDriver            = "DB_DRIVER"
	EnvDbDsn               = "DB_DSN"
	EnvHTTPPort            = "HTTP_PORT"
	EnvGRPCPort            = "GRPC_PORT"
	EnvLogLevel            = "LOG_LEVEL"

	// Defaults
	DefaultGeneratorPayloadLen = 8
	DefaultGeneratorInterval   = 10 * time.Millisecond
	DefaultWorkerPoolSize      = 4
	DefaultDbDriver            = "postgres"
	DefaultDbDsn               = "postgres://user:pass@localhost:5432/aggregator"
	DefaultHTTPPort            = 8080
	DefaultGRPCPort            = 50051
	DefaultLogLevel            = "info"
)
