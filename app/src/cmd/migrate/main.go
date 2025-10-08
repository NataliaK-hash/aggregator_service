package main

import (
	_ "aggregator-service/app/src/infra/utils/autoload"
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aggregator-service/app/src/database"
	"aggregator-service/app/src/infra"
)

func main() {
	migrationsDir := flag.String("dir", database.ResolveMigrationsDir(), "directory with SQL migration files")
	flag.Parse()

	cfg, logger := initEnvironment()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	checkDatabaseConnection(ctx, cfg, logger)
	runMigrations(ctx, cfg, logger, *migrationsDir)
}

// ----------------------------
// Вспомогательные функции
// ----------------------------

// initEnvironment загружает конфигурацию и логгер.
func initEnvironment() (infra.Config, *infra.Logger) {
	cfg := infra.LoadConfig()
	logger := infra.NewLogger(os.Stdout, "migrate")
	return cfg, logger
}

// checkDatabaseConnection выполняет проверку соединения с БД.
func checkDatabaseConnection(ctx context.Context, cfg infra.Config, logger *infra.Logger) {
	if !database.ShouldCheckDatabase(cfg) {
		return
	}
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := database.WaitForDatabase(waitCtx, cfg, logger); err != nil {
		logger.Fatalf(ctx, "database connectivity check failed: %v", err)
	}
}

// runMigrations строит DSN, создаёт runner и применяет миграции.
func runMigrations(ctx context.Context, cfg infra.Config, logger *infra.Logger, migrationsDir string) {
	dsn, err := database.BuildDatabaseDSN(cfg)
	if err != nil {
		logger.Fatalf(ctx, "failed to build database DSN: %v", err)
	}

	runner := database.NewSQLRunner()
	defer runner.Close()

	if err := database.ApplyMigrations(ctx, runner, dsn, migrationsDir, logger); err != nil {
		logger.Fatalf(ctx, "migrate: %v", err)
	}
}
