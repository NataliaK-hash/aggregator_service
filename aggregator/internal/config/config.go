package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config содержит параметры конфигурации приложения, загружаемые из переменных окружения.
type Config struct {
	GeneratorPeriodMs    int
	GeneratorPayloadSize int
	WorkerPoolSize       int
	DbDriver             string
	DbDsn                string
	HttpPort             int
	GrpcPort             int
	LogLevel             string
}

// Load считывает значения конфигурации из переменных окружения и подставляет значения по умолчанию при их отсутствии.
func Load() (*Config, error) {
	generatorPeriod, err := getEnvInt(EnvGeneratorPeriodMs, DefaultGeneratorPeriodMs)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", EnvGeneratorPeriodMs, err)
	}

	payloadSize, err := getEnvInt(EnvGeneratorPayloadSize, DefaultGeneratorPayloadSize)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", EnvGeneratorPayloadSize, err)
	}

	workerPoolSize, err := getEnvInt(EnvWorkerPoolSize, DefaultWorkerPoolSize)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", EnvWorkerPoolSize, err)
	}

	httpPort, err := getEnvInt(EnvHTTPPort, DefaultHTTPPort)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", EnvHTTPPort, err)
	}

	grpcPort, err := getEnvInt(EnvGRPCPort, DefaultGRPCPort)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", EnvGRPCPort, err)
	}

	cfg := &Config{
		GeneratorPeriodMs:    generatorPeriod,
		GeneratorPayloadSize: payloadSize,
		WorkerPoolSize:       workerPoolSize,
		DbDriver:             getEnvString(EnvDbDriver, DefaultDbDriver),
		DbDsn:                getEnvString(EnvDbDsn, DefaultDbDsn),
		HttpPort:             httpPort,
		GrpcPort:             grpcPort,
		LogLevel:             normalizeLogLevel(getEnvString(EnvLogLevel, DefaultLogLevel)),
	}

	return cfg, nil
}

// getEnvString возвращает строковое значение переменной окружения или значение по умолчанию.
func getEnvString(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnvInt возвращает целочисленное значение переменной окружения или значение по умолчанию.
func getEnvInt(key string, defaultValue int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}

	return parsed, nil
}

// normalizeLogLevel приводит текстовый уровень логирования к поддерживаемому значению.
func normalizeLogLevel(level string) string {
	switch level {
	case "debug", "info", "warn", "error":
		return level
	case "warning":
		return "warn"
	default:
		return DefaultLogLevel
	}
}
