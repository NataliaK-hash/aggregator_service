package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"aggregator-service/app/src/infra"
)

const defaultMigrationsDir = "app/resources/db/migrations"

// ResolveMigrationsDir returns the directory containing SQL migrations.
// It respects the MIGRATIONS_DIR environment variable when present and
// falls back to the default migrations path bundled with the application.
func ResolveMigrationsDir() string {
	if dir := strings.TrimSpace(os.Getenv("MIGRATIONS_DIR")); dir != "" {
		return dir
	}
	return defaultMigrationsDir
}

// ApplyMigrations executes the SQL migration files located in the provided
// directory using the supplied CommandRunner. Migrations are executed in
// lexical order to preserve dependencies between files.
func ApplyMigrations(ctx context.Context, runner CommandRunner, dsn, dir string, logger *infra.Logger) error {
	if strings.TrimSpace(dir) == "" {
		return errors.New("migrations directory is not specified")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations directory %q: %w", dir, err)
	}

	migrationFiles := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		migrationFiles = append(migrationFiles, entry)
	}

	sort.Slice(migrationFiles, func(i, j int) bool {
		return migrationFiles[i].Name() < migrationFiles[j].Name()
	})

	if len(migrationFiles) == 0 {
		if logger != nil {
			logger.Printf(ctx, "no migrations found in %s", dir)
		}
		return nil
	}

	for _, file := range migrationFiles {
		path := filepath.Join(dir, file.Name())

		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read migration %q: %w", file.Name(), readErr)
		}

		statements := strings.TrimSpace(string(contents))
		if statements == "" {
			if logger != nil {
				logger.Printf(ctx, "skipping empty migration %s", file.Name())
			}
			continue
		}

		if logger != nil {
			logger.Printf(ctx, "applying migration %s", file.Name())
		}

		if _, execErr := runner.Exec(ctx, dsn, "", statements); execErr != nil {
			return fmt.Errorf("apply migration %q: %w", file.Name(), execErr)
		}

		if logger != nil {
			logger.Printf(ctx, "migration %s applied", file.Name())
		}
	}

	if logger != nil {
		logger.Println(ctx, "migrations applied successfully")
	}

	return nil
}
