package main

import (
	"log"
	"os"
	"strconv"
)

type config struct {
	HTTPPort                string
	GRPCPort                string
	DatabaseDSN             string
	DatabaseHost            string
	DatabasePort            string
	DatabaseUser            string
	DatabasePassword        string
	DatabaseName            string
	GeneratorIntervalMillis int
	MeasurementsPerPacket   int
	WorkerCount             int
	PacketBufferSize        int
}

func loadConfig() config {
	return config{
		HTTPPort:                getEnv("HTTP_PORT", "8080"),
		GRPCPort:                getEnv("GRPC_PORT", "50051"),
		DatabaseDSN:             os.Getenv("DB_DSN"),
		DatabaseHost:            os.Getenv("DB_HOST"),
		DatabasePort:            os.Getenv("DB_PORT"),
		DatabaseUser:            os.Getenv("DB_USER"),
		DatabasePassword:        os.Getenv("DB_PASSWORD"),
		DatabaseName:            os.Getenv("DB_NAME"),
		GeneratorIntervalMillis: getEnvInt("N", 1000),
		MeasurementsPerPacket:   getEnvInt("K", 10),
		WorkerCount:             getEnvInt("M", 4),
		PacketBufferSize:        getEnvInt("PACKET_BUFFER", 100),
	}
}

func logConfig(logger *log.Logger, cfg config) {
	logger.Printf("HTTP_PORT=%s", cfg.HTTPPort)
	logger.Printf("GRPC_PORT=%s", cfg.GRPCPort)
	if cfg.DatabaseDSN != "" {
		logger.Printf("DB_DSN set (length %d)", len(cfg.DatabaseDSN))
	} else {
		logger.Println("DB_DSN not provided")
	}
	logger.Printf("DB_HOST=%s", emptyFallback(cfg.DatabaseHost))
	logger.Printf("DB_PORT=%s", emptyFallback(cfg.DatabasePort))
	logger.Printf("DB_USER=%s", emptyFallback(cfg.DatabaseUser))
	if cfg.DatabasePassword != "" {
		logger.Println("DB_PASSWORD set (redacted)")
	} else {
		logger.Println("DB_PASSWORD not provided")
	}
	logger.Printf("DB_NAME=%s", emptyFallback(cfg.DatabaseName))
	logger.Printf("GENERATOR_INTERVAL_MS=%d", cfg.GeneratorIntervalMillis)
	logger.Printf("MEASUREMENTS_PER_PACKET=%d", cfg.MeasurementsPerPacket)
	logger.Printf("WORKER_COUNT=%d", cfg.WorkerCount)
	logger.Printf("PACKET_BUFFER=%d", cfg.PacketBufferSize)
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
		log.Printf("invalid %s=%q, falling back to %d", key, value, fallback)
	}
	return fallback
}

func emptyFallback(value string) string {
	if value == "" {
		return "(not set)"
	}
	return value
}
