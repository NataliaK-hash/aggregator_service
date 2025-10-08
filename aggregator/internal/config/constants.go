package config

const (
	// ENV keys
	EnvGeneratorPeriodMs    = "GENERATOR_PERIOD_MS"
	EnvGeneratorPayloadSize = "GENERATOR_PAYLOAD_SIZE"
	EnvWorkerPoolSize       = "WORKER_POOL_SIZE"
	EnvDbDriver             = "DB_DRIVER"
	EnvDbDsn                = "DB_DSN"
	EnvHTTPPort             = "HTTP_PORT"
	EnvGRPCPort             = "GRPC_PORT"
	EnvLogLevel             = "LOG_LEVEL"

	// Defaults
	DefaultGeneratorPeriodMs    = 1000
	DefaultGeneratorPayloadSize = 256
	DefaultWorkerPoolSize       = 4
	DefaultDbDriver             = "postgres"
	DefaultDbDsn                = "postgres://user:pass@localhost:5432/aggregator"
	DefaultHTTPPort             = 8080
	DefaultGRPCPort             = 50051
	DefaultLogLevel             = "info"
)
