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

	cfg := infra.LoadConfig()
	logger := infra.NewLogger(os.Stdout, "migrate")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if database.ShouldCheckDatabase(cfg) {
		waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		if err := database.WaitForDatabase(waitCtx, cfg, logger); err != nil {
			cancel()
			logger.Fatalf(ctx, "database connectivity check failed: %v", err)
		}
		cancel()
	}

	dsn, err := database.BuildDatabaseDSN(cfg)
	if err != nil {
		logger.Fatalf(ctx, "failed to build database DSN: %v", err)
	}

	runner := database.NewSQLRunner()
	defer runner.Close()

	if err := database.ApplyMigrations(ctx, runner, dsn, *migrationsDir, logger); err != nil {
		logger.Fatalf(ctx, "migrate: %v", err)
	}
}
