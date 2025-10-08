package postgres

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"aggregator-service-project/internal/domain"
	"aggregator-service-project/internal/pkg/uuid"
)

const (
	createTableStatement = `
CREATE TABLE IF NOT EXISTS packet_max (
    packet_id UUID NOT NULL,
    source_id UUID NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    ts TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (packet_id, source_id, ts)
)`
	sourceIndexStatement = `
CREATE INDEX IF NOT EXISTS packet_max_source_idx
ON packet_max (source_id, value DESC, ts DESC)`
	timestampIndexStatement = `
CREATE INDEX IF NOT EXISTS packet_max_ts_idx
ON packet_max (ts DESC)`
)

// Config contains the configuration required to connect to a Postgres database.
type Config struct {
	DSN    string
	Runner CommandRunner
}

// CommandRunner executes SQL commands against Postgres.
type CommandRunner interface {
	Exec(ctx context.Context, dsn, password, sql string) (string, error)
	Close() error
}

// Repository implements the aggregator and worker repository contracts backed by Postgres.
type Repository struct {
	dsn      string
	password string

	runner CommandRunner

	closeOnce sync.Once
}

// New creates a repository backed by Postgres using a SQL command runner.
func New(ctx context.Context, cfg Config) (*Repository, error) {
	if cfg.DSN == "" {
		return nil, errors.New("postgres repository: DSN is required")
	}

	parsed, err := url.Parse(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("postgres repository: parse dsn: %w", err)
	}

	password, _ := parsed.User.Password()

	runner := cfg.Runner
	if runner == nil {
		runner = NewPGXRunner()
	}

	repo := &Repository{
		dsn:      cfg.DSN,
		password: password,
		runner:   runner,
	}

	if err := repo.ensureSchema(ctx); err != nil {
		_ = repo.Close()
		return nil, err
	}

	return repo, nil
}

// Close releases resources held by the repository.
func (r *Repository) Close() error {
	var err error
	r.closeOnce.Do(func() {
		err = r.runner.Close()
	})
	return err
}

// Add stores a measurement in the repository.
func (r *Repository) Add(ctx context.Context, measurement domain.Measurement) error {
	if measurement.PacketID == "" {
		return errors.New("postgres repository: packet id is required")
	}
	if _, err := uuid.Parse(measurement.PacketID); err != nil {
		return fmt.Errorf("postgres repository: invalid packet id: %w", err)
	}
	if measurement.SourceID == "" {
		return errors.New("postgres repository: source id is required")
	}
	if _, err := uuid.Parse(measurement.SourceID); err != nil {
		return fmt.Errorf("postgres repository: invalid source id: %w", err)
	}

	timestamp := measurement.Timestamp.UTC().Format(time.RFC3339Nano)
	statement := fmt.Sprintf(
		"INSERT INTO packet_max (packet_id, source_id, value, ts) VALUES ('%s'::uuid, '%s'::uuid, %.12f, '%s'::timestamptz) ON CONFLICT (packet_id, source_id, ts) DO UPDATE SET value = EXCLUDED.value",
		measurement.PacketID,
		measurement.SourceID,
		measurement.Value,
		timestamp,
	)

	if _, err := r.runner.Exec(ctx, r.dsn, r.password, statement); err != nil {
		return fmt.Errorf("postgres repository: add measurement: %w", err)
	}

	return nil
}

// MaxBySource returns the maximum measurement for the provided source identifier.
func (r *Repository) MaxBySource(ctx context.Context, id string) (domain.Measurement, error) {
	if _, err := uuid.Parse(id); err != nil {
		return domain.Measurement{}, fmt.Errorf("postgres repository: invalid source id: %w", err)
	}

	statement := fmt.Sprintf(
		"SELECT packet_id::text, source_id::text, value, ts AT TIME ZONE 'UTC' FROM packet_max WHERE source_id = '%s'::uuid ORDER BY value DESC, ts DESC LIMIT 1",
		id,
	)

	output, err := r.runner.Exec(ctx, r.dsn, r.password, statement)
	if err != nil {
		return domain.Measurement{}, fmt.Errorf("postgres repository: max by source: %w", err)
	}

	measurement, err := parseMeasurement(output)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Measurement{}, err
		}
		return domain.Measurement{}, fmt.Errorf("postgres repository: max by source parse: %w", err)
	}

	return measurement, nil
}

// MaxInRange returns the maximum measurement recorded within the provided time range.
func (r *Repository) MaxInRange(ctx context.Context, from, to time.Time) (domain.Measurement, error) {
	statement := fmt.Sprintf(
		"SELECT packet_id::text, source_id::text, value, ts AT TIME ZONE 'UTC' FROM packet_max WHERE ts BETWEEN '%s'::timestamptz AND '%s'::timestamptz ORDER BY value DESC, ts DESC LIMIT 1",
		from.UTC().Format(time.RFC3339Nano),
		to.UTC().Format(time.RFC3339Nano),
	)

	output, err := r.runner.Exec(ctx, r.dsn, r.password, statement)
	if err != nil {
		return domain.Measurement{}, fmt.Errorf("postgres repository: max in range: %w", err)
	}

	measurement, err := parseMeasurement(output)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Measurement{}, err
		}
		return domain.Measurement{}, fmt.Errorf("postgres repository: max in range parse: %w", err)
	}

	return measurement, nil
}

func (r *Repository) ensureSchema(ctx context.Context) error {
	statements := []string{createTableStatement, sourceIndexStatement, timestampIndexStatement}
	for _, stmt := range statements {
		if _, err := r.runner.Exec(ctx, r.dsn, r.password, stmt); err != nil {
			return fmt.Errorf("postgres repository: ensure schema: %w", err)
		}
	}
	return nil
}

func parseMeasurement(output string) (domain.Measurement, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return domain.Measurement{}, domain.ErrNotFound
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.TrimLeadingSpace = true
	record, err := reader.Read()
	if err != nil {
		return domain.Measurement{}, fmt.Errorf("parse csv: %w", err)
	}

	if len(record) < 4 {
		return domain.Measurement{}, fmt.Errorf("unexpected column count: %d", len(record))
	}

	value, err := parseFloat(record[2])
	if err != nil {
		return domain.Measurement{}, err
	}

	timestamp, err := time.Parse(time.RFC3339Nano, record[3])
	if err != nil {
		return domain.Measurement{}, fmt.Errorf("parse timestamp: %w", err)
	}

	return domain.Measurement{
		PacketID:  record[0],
		SourceID:  record[1],
		Value:     value,
		Timestamp: timestamp,
	}, nil
}

func parseFloat(input string) (float64, error) {
	value, err := strconv.ParseFloat(strings.TrimSpace(input), 64)
	if err != nil {
		return 0, fmt.Errorf("parse float: %w", err)
	}
	return value, nil
}

var _ domain.MeasurementRepository = (*Repository)(nil)
