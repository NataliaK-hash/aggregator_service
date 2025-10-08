package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"time"

	"aggregator-service-project/internal/domain"
	"aggregator-service-project/internal/infrastructure/repository/postgres"
)

func shouldCheckDatabase(cfg config) bool {
	if cfg.DatabaseDSN != "" {
		return true
	}
	return cfg.DatabaseHost != ""
}

func waitForDatabase(ctx context.Context, cfg config, logger *log.Logger) error {
	host := cfg.DatabaseHost
	port := cfg.DatabasePort

	if (host == "" || port == "") && cfg.DatabaseDSN != "" {
		parsed, err := url.Parse(cfg.DatabaseDSN)
		if err != nil {
			return fmt.Errorf("invalid DB_DSN: %w", err)
		}
		if host == "" {
			host = parsed.Hostname()
		}
		if port == "" {
			port = parsed.Port()
		}
	}

	if host == "" {
		return nil
	}
	if port == "" {
		port = "5432"
	}

	address := net.JoinHostPort(host, port)
	dialer := &net.Dialer{Timeout: 3 * time.Second}

	const maxAttempts = 5
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		conn, err := dialer.DialContext(ctx, "tcp", address)
		if err == nil {
			_ = conn.Close()
			return nil
		}

		logger.Printf("database check attempt %d failed: %v", attempt, err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	return fmt.Errorf("database not reachable at %s", address)
}

func setupPostgresRepository(ctx context.Context, cfg config, logger *log.Logger) (domain.MeasurementRepository, func(), error) {
	dsn, err := buildDatabaseDSN(cfg)
	if err != nil {
		return nil, nil, err
	}

	repo, err := postgres.New(ctx, postgres.Config{DSN: dsn})
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		if err := repo.Close(); err != nil {
			logger.Printf("failed to close repository: %v", err)
		}
	}

	return repo, cleanup, nil
}

func buildDatabaseDSN(cfg config) (string, error) {
	if cfg.DatabaseDSN != "" {
		return cfg.DatabaseDSN, nil
	}

	if cfg.DatabaseHost == "" {
		return "", errors.New("database host is required when DSN is not provided")
	}
	if cfg.DatabaseUser == "" {
		return "", errors.New("database user is required when DSN is not provided")
	}
	if cfg.DatabaseName == "" {
		return "", errors.New("database name is required when DSN is not provided")
	}

	port := cfg.DatabasePort
	if port == "" {
		port = "5432"
	}

	connectionURL := &url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(cfg.DatabaseHost, port),
		Path:   "/" + cfg.DatabaseName,
		User:   url.UserPassword(cfg.DatabaseUser, cfg.DatabasePassword),
	}

	query := connectionURL.Query()
	if query.Get("sslmode") == "" {
		query.Set("sslmode", "disable")
	}
	connectionURL.RawQuery = query.Encode()

	return connectionURL.String(), nil
}
