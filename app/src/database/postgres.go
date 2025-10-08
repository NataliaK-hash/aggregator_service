package database

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"aggregator-service/app/src/domain"
	"aggregator-service/app/src/infra"
	"aggregator-service/app/src/shared/constants"
)

// Config contains the configuration required to connect to a Postgres database.
type Config struct {
	DSN    string
	Runner CommandRunner
	Logger *infra.Logger
	// BatchSize determines how many measurements are flushed together.
	BatchSize int
	// BatchTimeout specifies how long to wait before flushing a partial batch.
	BatchTimeout time.Duration
	// BufferSize controls the capacity of the inbound measurement queue.
	BufferSize int
}

// CommandRunner executes SQL commands against Postgres.
type CommandRunner interface {
	Exec(ctx context.Context, dsn, password, sql string, args ...any) (string, error)
	Close() error
}

// Repository implements the aggregator and worker repository contracts backed by Postgres.
type Repository struct {
	dsn      string
	password string

	runner CommandRunner
	logger *infra.Logger

	batchSize    int
	batchTimeout time.Duration
	buffer       chan domain.PacketMax
	stopCh       chan struct{}
	wg           sync.WaitGroup

	mu     sync.RWMutex
	closed bool

	closeOnce sync.Once
}

const (
	insertMeasurementSQL = `
INSERT INTO public.packet_max (packet_id, source_id, value, ts)
VALUES ($1, $2, $3, $4)
`
	updateMeasurementByPairSQL = `
UPDATE public.packet_max
SET source_id = CASE WHEN $3 > value THEN $2 ELSE source_id END,
    value     = GREATEST(value, $3),
    ts        = $4
WHERE packet_id = $1 AND source_id = $2
`
	updateMeasurementByPacketSQL = `
UPDATE public.packet_max
SET source_id = CASE WHEN $3 > value THEN $2 ELSE source_id END,
    value     = GREATEST(value, $3),
    ts        = $4
WHERE packet_id = $1
`
)

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
		runner = NewSQLRunner()
	}

	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 1
	}

	bufferSize := cfg.BufferSize
	if bufferSize <= 0 {
		bufferSize = batchSize
	}

	batchTimeout := cfg.BatchTimeout
	if batchTimeout < 0 {
		batchTimeout = 0
	}

	repo := &Repository{
		dsn:          cfg.DSN,
		password:     password,
		runner:       runner,
		logger:       cfg.Logger,
		batchSize:    batchSize,
		batchTimeout: batchTimeout,
		buffer:       make(chan domain.PacketMax, bufferSize),
		stopCh:       make(chan struct{}),
	}

	repo.wg.Add(1)
	go repo.run()

	return repo, nil
}

// Close releases resources held by the repository.
func (r *Repository) Close() error {
	r.mu.Lock()
	alreadyClosed := r.closed
	if !r.closed {
		r.closed = true
		close(r.stopCh)
	}
	r.mu.Unlock()

	if !alreadyClosed {
		r.wg.Wait()
	}

	var err error
	r.closeOnce.Do(func() {
		err = r.runner.Close()
	})
	return err
}

// Add stores a packet maximum in the repository.
func (r *Repository) Add(ctx context.Context, packetMax domain.PacketMax) error {
	if err := validatePacketMax(packetMax); err != nil {
		return err
	}

	r.mu.RLock()
	closed := r.closed
	stopCh := r.stopCh
	buffer := r.buffer
	r.mu.RUnlock()

	if closed {
		return errors.New("postgres repository: repository closed")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-stopCh:
		return errors.New("postgres repository: repository closed")
	case buffer <- packetMax:
		return nil
	}
}

func (r *Repository) run() {
	defer r.wg.Done()

	batch := make([]domain.PacketMax, 0, r.batchSize)
	var batchStart time.Time
	var timer *time.Timer

	activateTimer := func() {
		if r.batchTimeout <= 0 {
			return
		}
		if timer == nil {
			timer = time.NewTimer(r.batchTimeout)
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(r.batchTimeout)
	}

	deactivateTimer := func() {
		if timer == nil {
			return
		}
		t := timer
		if !t.Stop() {
			select {
			case <-t.C:
			default:
			}
		}
		timer = nil
	}

	flush := func() {
		if len(batch) == 0 {
			return
		}
		wait := time.Since(batchStart)
		r.processBatch(batch, wait)
		batch = batch[:0]
		deactivateTimer()
	}

	appendToBatch := func(packetMax domain.PacketMax) {
		batch = append(batch, packetMax)
		if len(batch) == 1 {
			batchStart = time.Now()
			activateTimer()
		}
		if len(batch) >= r.batchSize {
			flush()
		}
	}

	for {
		var timeout <-chan time.Time
		if timer != nil {
			timeout = timer.C
		}

		select {
		case <-r.stopCh:
			for {
				select {
				case packetMax := <-r.buffer:
					appendToBatch(packetMax)
				default:
					flush()
					return
				}
			}
		case packetMax := <-r.buffer:
			appendToBatch(packetMax)
		case <-timeout:
			flush()
		}
	}
}

func (r *Repository) processBatch(batch []domain.PacketMax, wait time.Duration) {
	if len(batch) == 0 {
		return
	}

	ctx := context.Background()
	for _, packetMax := range batch {
		if err := r.writePacketMax(ctx, packetMax); err != nil {
			if r.logger != nil {
				r.logger.Printf(ctx, "postgres repository: batch write failed packet=%s source=%s: %v", packetMax.PacketID, packetMax.SourceID, err)
			}
		}
	}

	infra.ObserveDBBatchSize(len(batch))
	infra.ObserveDBBatchWait(wait)
}

func (r *Repository) writePacketMax(ctx context.Context, packetMax domain.PacketMax) error {
	timestamp := packetMax.Timestamp.UTC()

	insertTag, err := r.execStatement(ctx, insertMeasurementSQL, packetMax, timestamp)
	if err == nil {
		if _, err := parseRowsAffected(insertTag); err != nil {
			infra.IncErrors()
			infra.IncDBWriteErrors()
			if r.logger != nil {
				r.logger.Printf(ctx, "postgres repository: parse insert result failed packet=%s source=%s tag=%q: %v", packetMax.PacketID, packetMax.SourceID, insertTag, err)
			}
			return fmt.Errorf("postgres repository: parse insert result: %w", err)
		}

		infra.IncDBWrites()
		infra.IncPackets()
		return nil
	}

	if !isUniqueViolation(err) {
		infra.IncErrors()
		infra.IncDBWriteErrors()
		if r.logger != nil {
			r.logger.Printf(ctx, "postgres repository: insert failed packet=%s source=%s value=%v ts=%s: %v", packetMax.PacketID, packetMax.SourceID, packetMax.Value, timestamp.Format(time.RFC3339Nano), err)
		}
		return fmt.Errorf("postgres repository: insert packet max: %w", err)
	}

	updateTag, err := r.execStatement(ctx, updateMeasurementByPairSQL, packetMax, timestamp)
	if err != nil {
		return fmt.Errorf("postgres repository: update packet max by pair: %w", err)
	}

	affected, err := parseRowsAffected(updateTag)
	if err != nil {
		infra.IncErrors()
		infra.IncDBWriteErrors()
		if r.logger != nil {
			r.logger.Printf(ctx, "postgres repository: parse update pair result failed packet=%s source=%s tag=%q: %v", packetMax.PacketID, packetMax.SourceID, updateTag, err)
		}
		return fmt.Errorf("postgres repository: parse update pair result: %w", err)
	}
	if affected > 0 {
		infra.IncDBWrites()
		infra.IncPackets()
		return nil
	}

	fallbackTag, err := r.execStatement(ctx, updateMeasurementByPacketSQL, packetMax, timestamp)
	if err != nil {
		return fmt.Errorf("postgres repository: update packet max by packet: %w", err)
	}

	fallbackAffected, err := parseRowsAffected(fallbackTag)
	if err != nil {
		infra.IncErrors()
		infra.IncDBWriteErrors()
		if r.logger != nil {
			r.logger.Printf(ctx, "postgres repository: parse update packet result failed packet=%s source=%s tag=%q: %v", packetMax.PacketID, packetMax.SourceID, fallbackTag, err)
		}
		return fmt.Errorf("postgres repository: parse update packet result: %w", err)
	}
	if fallbackAffected == 0 {
		infra.IncErrors()
		infra.IncDBWriteErrors()
		if r.logger != nil {
			r.logger.Printf(ctx, "postgres repository: update packet affected no rows packet=%s source=%s", packetMax.PacketID, packetMax.SourceID)
		}
		return errors.New("postgres repository: update affected 0 rows after unique violation")
	}

	infra.IncDBWrites()
	infra.IncPackets()
	return nil
}

func validatePacketMax(packetMax domain.PacketMax) error {
	if packetMax.PacketID == "" {
		infra.IncErrors()
		return errors.New("postgres repository: packet id is required")
	}
	if _, err := constants.ParseUUID(packetMax.PacketID); err != nil {
		infra.IncErrors()
		return fmt.Errorf("postgres repository: invalid packet id: %w", err)
	}
	if packetMax.SourceID == "" {
		infra.IncErrors()
		return errors.New("postgres repository: source id is required")
	}
	if _, err := constants.ParseUUID(packetMax.SourceID); err != nil {
		infra.IncErrors()
		return fmt.Errorf("postgres repository: invalid source id: %w", err)
	}
	return nil
}

func (r *Repository) execStatement(ctx context.Context, statement string, packetMax domain.PacketMax, timestamp time.Time) (string, error) {
	start := time.Now()
	tag, err := r.runner.Exec(ctx, r.dsn, r.password, statement, packetMax.PacketID, packetMax.SourceID, packetMax.Value, timestamp)
	duration := time.Since(start)
	infra.ObserveDBWrite(duration)
	if err != nil {
		if !isUniqueViolation(err) {
			infra.IncErrors()
			infra.IncDBWriteErrors()
			if r.logger != nil {
				r.logger.Printf(ctx, "postgres repository: exec failed packet=%s source=%s value=%v ts=%s statement=%q: %v", packetMax.PacketID, packetMax.SourceID, packetMax.Value, timestamp.Format(time.RFC3339Nano), strings.TrimSpace(statement), err)
			}
		}
		return "", err
	}
	return tag, nil
}

func parseRowsAffected(tag string) (int64, error) {
	fields := strings.Fields(strings.TrimSpace(tag))
	if len(fields) == 0 {
		return 0, nil
	}

	switch strings.ToUpper(fields[0]) {
	case "UPDATE", "DELETE":
		if len(fields) < 2 {
			return 0, fmt.Errorf("unexpected command tag %q", tag)
		}
		count, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse rows affected: %w", err)
		}
		return count, nil
	case "INSERT":
		if len(fields) < 3 {
			return 0, fmt.Errorf("unexpected command tag %q", tag)
		}
		count, err := strconv.ParseInt(fields[len(fields)-1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse rows affected: %w", err)
		}
		return count, nil
	default:
		return 0, fmt.Errorf("unsupported command tag %q", tag)
	}
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}

	type sqlState interface {
		SQLState() string
	}

	var state sqlState
	if errors.As(err, &state) {
		return state.SQLState() == "23505"
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate key value") || strings.Contains(message, "unique constraint")
}

// PacketMaxByID returns the persisted maximum for the provided packet identifier.
func (r *Repository) PacketMaxByID(ctx context.Context, packetID string) (domain.PacketMax, error) {
	if _, err := constants.ParseUUID(packetID); err != nil {
		return domain.PacketMax{}, fmt.Errorf("postgres repository: invalid packet id: %w", err)
	}

	statement := fmt.Sprintf(
		"SELECT packet_id::text, source_id::text, value, ts AT TIME ZONE 'UTC' FROM public.packet_max WHERE packet_id = '%s'::uuid LIMIT 1",
		packetID,
	)

	output, err := r.runner.Exec(ctx, r.dsn, r.password, statement)
	if err != nil {
		return domain.PacketMax{}, fmt.Errorf("postgres repository: packet max by id: %w", err)
	}

	packetMaxes, err := parsePacketMaxList(output)
	if err != nil {
		return domain.PacketMax{}, fmt.Errorf("postgres repository: packet max by id parse: %w", err)
	}
	if len(packetMaxes) == 0 {
		return domain.PacketMax{}, domain.ErrNotFound
	}

	return packetMaxes[0], nil
}

// PacketMaxInRange returns the maxima for all packets recorded within the provided time range ordered by timestamp.
func (r *Repository) PacketMaxInRange(ctx context.Context, from, to time.Time) ([]domain.PacketMax, error) {
	statement := fmt.Sprintf(
		"SELECT packet_id::text, source_id::text, value, ts AT TIME ZONE 'UTC' FROM public.packet_max WHERE ts BETWEEN '%s'::timestamptz AND '%s'::timestamptz ORDER BY ts ASC",
		from.UTC().Format(time.RFC3339Nano),
		to.UTC().Format(time.RFC3339Nano),
	)

	output, err := r.runner.Exec(ctx, r.dsn, r.password, statement)
	if err != nil {
		return nil, fmt.Errorf("postgres repository: packet max in range: %w", err)
	}

	packetMaxes, err := parsePacketMaxList(output)
	if err != nil {
		return nil, fmt.Errorf("postgres repository: packet max in range parse: %w", err)
	}
	if len(packetMaxes) == 0 {
		return nil, domain.ErrNotFound
	}

	return packetMaxes, nil
}

func parsePacketMaxList(output string) ([]domain.PacketMax, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil, nil
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.TrimLeadingSpace = true

	var results []domain.PacketMax
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse csv: %w", err)
		}

		if len(record) < 4 {
			return nil, fmt.Errorf("unexpected column count: %d", len(record))
		}

		value, err := parseFloat(record[2])
		if err != nil {
			return nil, err
		}

		timestamp, err := time.Parse(time.RFC3339Nano, record[3])
		if err != nil {
			return nil, fmt.Errorf("parse timestamp: %w", err)
		}

		results = append(results, domain.PacketMax{
			PacketID:  record[0],
			SourceID:  record[1],
			Value:     value,
			Timestamp: timestamp,
		})
	}

	return results, nil
}

func parseFloat(input string) (float64, error) {
	value, err := strconv.ParseFloat(strings.TrimSpace(input), 64)
	if err != nil {
		return 0, fmt.Errorf("parse float: %w", err)
	}
	return value, nil
}

var _ domain.PacketMaxRepository = (*Repository)(nil)
