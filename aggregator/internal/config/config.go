package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Generator contains configuration for the data packet generator.
type Generator struct {
	PayloadLen int
	Interval   time.Duration
}

// Config содержит параметры конфигурации приложения, загружаемые из переменных окружения.
type Config struct {
	Generator      Generator
	WorkerPoolSize int
	DbDriver       string
	DbDsn          string
	HttpPort       int
	GrpcPort       int
	LogLevel       string
}

// Load считывает значения конфигурации из переменных окружения и подставляет значения по умолчанию при их отсутствии.
func Load() (*Config, error) {
	payloadLen, err := getEnvInt(EnvGeneratorPayloadLen, DefaultGeneratorPayloadLen)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", EnvGeneratorPayloadLen, err)
	}

	interval, err := getEnvDuration(EnvGeneratorInterval, DefaultGeneratorInterval)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", EnvGeneratorInterval, err)
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
		Generator: Generator{
			PayloadLen: payloadLen,
			Interval:   interval,
		},
		WorkerPoolSize: workerPoolSize,
		DbDriver:       getEnvString(EnvDbDriver, DefaultDbDriver),
		DbDsn:          getEnvString(EnvDbDsn, DefaultDbDsn),
		HttpPort:       httpPort,
		GrpcPort:       grpcPort,
		LogLevel:       normalizeLogLevel(getEnvString(EnvLogLevel, DefaultLogLevel)),
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

func getEnvDuration(key string, defaultValue time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue, nil
	}

	parsed, err := time.ParseDuration(value)
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
