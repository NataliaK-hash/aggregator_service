package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/wire"
	_ "github.com/jackc/pgx/v5/stdlib"

	"aggregator/internal/config"
	"aggregator/internal/logging"
	ch "aggregator/internal/storage/clickhouse"
	pg "aggregator/internal/storage/postgres"
	rd "aggregator/internal/storage/redis"
)

var ProviderSet = wire.NewSet(ProvideRepository)

func ProvideRepository(cfg *config.Config, logger *logging.Logger) (Repository, error) {
	switch cfg.DbDriver {
	case "postgres", "pgx":
		if cfg.DbDsn == "" {
			return nil, fmt.Errorf("postgres DSN is empty")
		}

		db, err := sql.Open("pgx", cfg.DbDsn)
		if err != nil {
			return nil, fmt.Errorf("open postgres connection: %w", err)
		}
		db.SetMaxIdleConns(5)
		db.SetMaxOpenConns(10)
		db.SetConnMaxLifetime(30 * time.Minute)

		repo, err := pg.NewRepository(db)
		if err != nil {
			_ = db.Close()
			return nil, err
		}
		return repo, nil
	case "clickhouse":
		if logger != nil {
			logger.Warn("используется заглушка ClickHouse репозитория")
		}
		return ch.NewRepository(), nil
	case "redis":
		if logger != nil {
			logger.Warn("используется заглушка Redis репозитория")
		}
		return rd.NewRepository(), nil
	default:
		return nil, fmt.Errorf("unsupported repository driver: %s", cfg.DbDriver)
	}
}
