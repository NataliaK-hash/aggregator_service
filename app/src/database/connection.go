package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"aggregator-service/app/src/domain"
	"aggregator-service/app/src/infra"
)

// Connect opens a SQL database handle using the provided configuration.
// It validates the connection by pinging the database before returning the handle.
func Connect(cfg *Config) (*sql.DB, error) {
	if cfg == nil {
		return nil, errors.New("db: config is required")
	}
	if cfg.DSN == "" {
		return nil, errors.New("db: DSN is required")
	}

	db, err := sql.Open("postgres", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("db: open connection: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	return db, nil
}

// ShouldCheckDatabase determines if connectivity should be validated based on the config.
func ShouldCheckDatabase(cfg infra.Config) bool {
	if cfg.DatabaseDSN != "" {
		return true
	}
	return cfg.DatabaseHost != ""
}

// WaitForDatabase probes the configured host/port until it becomes reachable or context cancellation.
func WaitForDatabase(ctx context.Context, cfg infra.Config, logger *infra.Logger) error {
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

		if logger != nil {
			logger.Printf(ctx, "database check attempt %d failed: %v", attempt, err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	return fmt.Errorf("database not reachable at %s", address)
}

// SetupRepository initialises the Postgres-backed repository and cleanup routine.
func SetupRepository(ctx context.Context, cfg infra.Config, logger *infra.Logger) (domain.PacketMaxRepository, func(), error) {
	dsn, err := BuildDatabaseDSN(cfg)
	if err != nil {
		return nil, nil, err
	}

	if parsed, parseErr := url.Parse(dsn); parseErr == nil {
		host := parsed.Hostname()
		dbName := strings.TrimPrefix(parsed.Path, "/")
		user := parsed.User.Username()
		if logger != nil {
			logger.Printf(ctx, "connected to DSN host=%s db=%s user=%s", host, dbName, user)
		}
	} else {
		if logger != nil {
			logger.Printf(ctx, "failed to parse DSN for logging: %v", parseErr)
		}
	}

	migrationsDir := ResolveMigrationsDir()
	runner := NewSQLRunner()
	defer runner.Close()
	if err := ApplyMigrations(ctx, runner, dsn, migrationsDir, logger); err != nil {
		return nil, nil, err
	}

	repo, err := New(ctx, Config{
		DSN:          dsn,
		Logger:       logger,
		BatchSize:    cfg.DatabaseBatchSize,
		BatchTimeout: time.Duration(cfg.DatabaseBatchTimeoutMS) * time.Millisecond,
		BufferSize:   cfg.DatabaseBatchBufferSize,
	})
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		if err := repo.Close(); err != nil {
			if logger != nil {
				logger.Printf(ctx, "failed to close repository: %v", err)
			}
		}
	}

	return repo, cleanup, nil
}

// BuildDatabaseDSN constructs a DSN from discrete configuration values when not provided explicitly.
func BuildDatabaseDSN(cfg infra.Config) (string, error) {
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
