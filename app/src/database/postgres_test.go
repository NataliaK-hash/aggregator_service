package database

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"aggregator-service/app/src/domain"
	"aggregator-service/app/src/infra"
	"aggregator-service/app/src/shared/constants"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type execResponse struct {
	tag string
	err error
}

type fakeRunner struct {
	mu        sync.Mutex
	responses []execResponse
	calls     []execCall
	closeErr  error
}

type execCall struct {
	statement string
	args      []any
}

func (r *fakeRunner) Exec(ctx context.Context, dsn, password, sql string, args ...any) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, execCall{statement: sql, args: append([]any(nil), args...)})
	if len(r.responses) == 0 {
		return "", nil
	}
	resp := r.responses[0]
	r.responses = r.responses[1:]
	return resp.tag, resp.err
}

func (r *fakeRunner) Close() error { return r.closeErr }

func (r *fakeRunner) setResponses(responses ...execResponse) {
	r.mu.Lock()
	r.responses = responses
	r.mu.Unlock()
}

func (r *fakeRunner) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func (r *fakeRunner) lastCall() execCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.calls) == 0 {
		return execCall{}
	}
	return r.calls[len(r.calls)-1]
}

func newTestRepository(t *testing.T, runner *fakeRunner) *Repository {
	t.Helper()
	cfg := Config{
		DSN:          "postgres://user:pass@localhost:5432/db?sslmode=disable",
		Runner:       runner,
		BatchSize:    1,
		BatchTimeout: time.Millisecond,
		BufferSize:   1,
		Logger:       infra.NewLogger(io.Discard, "test"),
	}
	repo, err := New(context.Background(), cfg)
	require.NoError(t, err)
	return repo
}

func TestNewValidatesConfig(t *testing.T) {
	_, err := New(context.Background(), Config{})
	assert.Error(t, err)
}

func TestNewInitialisesDefaults(t *testing.T) {
	runner := &fakeRunner{}
	repo, err := New(context.Background(), Config{DSN: "postgres://user:pass@localhost:5432/db", Runner: runner})
	require.NoError(t, err)
	defer repo.Close()

	assert.Equal(t, 1, repo.batchSize)
	assert.Equal(t, repo.batchSize, cap(repo.buffer))
}

func TestCloseIsIdempotent(t *testing.T) {
	runner := &fakeRunner{}
	repo := newTestRepository(t, runner)

	assert.NoError(t, repo.Close())
	assert.NoError(t, repo.Close())
}

func TestAddValidatesPacketMax(t *testing.T) {
	repo := newTestRepository(t, &fakeRunner{})
	defer repo.Close()

	err := repo.Add(context.Background(), domain.PacketMax{})
	assert.Error(t, err)
}

func TestAddReturnsErrorWhenClosed(t *testing.T) {
	repo := newTestRepository(t, &fakeRunner{})
	require.NoError(t, repo.Close())

	err := repo.Add(context.Background(), domain.PacketMax{PacketID: constants.GenerateUUID(), SourceID: constants.GenerateUUID()})
	assert.Error(t, err)
}

func TestAddRespectsContextCancellation(t *testing.T) {
	repo := newTestRepository(t, &fakeRunner{})
	defer repo.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := repo.Add(ctx, domain.PacketMax{PacketID: constants.GenerateUUID(), SourceID: constants.GenerateUUID()})
	assert.Error(t, err)
}

func TestAddEnqueuesPacket(t *testing.T) {
	runner := &fakeRunner{responses: []execResponse{{tag: "INSERT 0 1"}}}
	repo := newTestRepository(t, runner)
	defer repo.Close()

	packet := domain.PacketMax{PacketID: constants.GenerateUUID(), SourceID: constants.GenerateUUID(), Value: 1, Timestamp: time.Now()}
	require.NoError(t, repo.Add(context.Background(), packet))
	require.Eventually(t, func() bool { return runner.callCount() > 0 }, time.Second, 10*time.Millisecond)
}

func TestProcessBatchNoMeasurements(t *testing.T) {
	runner := &fakeRunner{}
	repo := newTestRepository(t, runner)
	defer repo.Close()

	repo.processBatch(nil)
	assert.Equal(t, 0, runner.callCount())
}

func TestWritePacketMaxInsertSuccess(t *testing.T) {
	runner := &fakeRunner{}
	runner.setResponses(execResponse{tag: "INSERT 0 1"})
	repo := newTestRepository(t, runner)
	defer repo.Close()

	packet := domain.PacketMax{PacketID: constants.GenerateUUID(), SourceID: constants.GenerateUUID(), Value: 1, Timestamp: time.Now()}
	err := repo.writePacketMax(context.Background(), packet)
	assert.NoError(t, err)
}

type uniqueViolationError struct{}

func (uniqueViolationError) Error() string    { return "duplicate key value violates unique constraint" }
func (uniqueViolationError) SQLState() string { return "23505" }

func TestWritePacketMaxHandlesUniqueViolationPairUpdate(t *testing.T) {
	runner := &fakeRunner{}
	runner.setResponses(
		execResponse{err: uniqueViolationError{}},
		execResponse{tag: "UPDATE 1"},
	)
	repo := newTestRepository(t, runner)
	defer repo.Close()

	packet := domain.PacketMax{PacketID: constants.GenerateUUID(), SourceID: constants.GenerateUUID(), Value: 1, Timestamp: time.Now()}
	err := repo.writePacketMax(context.Background(), packet)
	assert.NoError(t, err)
}

func TestWritePacketMaxFallbackUpdate(t *testing.T) {
	runner := &fakeRunner{}
	runner.setResponses(
		execResponse{err: uniqueViolationError{}},
		execResponse{tag: "UPDATE 0"},
		execResponse{tag: "UPDATE 1"},
	)
	repo := newTestRepository(t, runner)
	defer repo.Close()

	packet := domain.PacketMax{PacketID: constants.GenerateUUID(), SourceID: constants.GenerateUUID(), Value: 1, Timestamp: time.Now()}
	err := repo.writePacketMax(context.Background(), packet)
	assert.NoError(t, err)
}

func TestWritePacketMaxFallbackError(t *testing.T) {
	runner := &fakeRunner{}
	runner.setResponses(
		execResponse{err: uniqueViolationError{}},
		execResponse{tag: "UPDATE 0"},
		execResponse{err: errors.New("update failed")},
	)
	repo := newTestRepository(t, runner)
	defer repo.Close()

	packet := domain.PacketMax{PacketID: constants.GenerateUUID(), SourceID: constants.GenerateUUID(), Value: 1, Timestamp: time.Now()}
	err := repo.writePacketMax(context.Background(), packet)
	assert.Error(t, err)
}

func TestValidatePacketMax(t *testing.T) {
	assert.Error(t, validatePacketMax(domain.PacketMax{}))
	valid := domain.PacketMax{PacketID: constants.GenerateUUID(), SourceID: constants.GenerateUUID()}
	assert.NoError(t, validatePacketMax(valid))
}

func TestExecStatementReturnsError(t *testing.T) {
	runner := &fakeRunner{}
	runner.setResponses(execResponse{err: errors.New("failed")})
	repo := newTestRepository(t, runner)
	defer repo.Close()

	packet := domain.PacketMax{PacketID: constants.GenerateUUID(), SourceID: constants.GenerateUUID(), Value: 1, Timestamp: time.Now()}
	_, err := repo.execStatement(context.Background(), "SQL", packet, time.Now())
	assert.Error(t, err)
}

func TestParseRowsAffected(t *testing.T) {
	count, err := parseRowsAffected("UPDATE 2")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)

	count, err = parseRowsAffected("INSERT 0 1")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	_, err = parseRowsAffected("UNKNOWN")
	assert.Error(t, err)
}

func TestIsUniqueViolation(t *testing.T) {
	assert.True(t, isUniqueViolation(uniqueViolationError{}))
	assert.True(t, isUniqueViolation(errors.New("duplicate key value")))
	assert.False(t, isUniqueViolation(nil))
}

func TestPacketMaxByIDSuccess(t *testing.T) {
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	runner := &fakeRunner{responses: []execResponse{{tag: fmt.Sprintf("packet,source,1.5,%s\n", timestamp)}}}
	repo := newTestRepository(t, runner)
	defer repo.Close()

	result, err := repo.PacketMaxByID(context.Background(), constants.GenerateUUID())
	assert.NoError(t, err)
	assert.Equal(t, "packet", result.PacketID)
	assert.Equal(t, 1.5, result.Value)
}

func TestPacketMaxByIDNotFound(t *testing.T) {
	runner := &fakeRunner{responses: []execResponse{{tag: ""}}}
	repo := newTestRepository(t, runner)
	defer repo.Close()

	_, err := repo.PacketMaxByID(context.Background(), constants.GenerateUUID())
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestPacketMaxInRangeSuccess(t *testing.T) {
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	response := fmt.Sprintf("packet,source,2.5,%s\n", timestamp)
	runner := &fakeRunner{responses: []execResponse{{tag: response}}}
	repo := newTestRepository(t, runner)
	defer repo.Close()

	results, err := repo.PacketMaxInRange(context.Background(), time.Now().Add(-time.Hour), time.Now())
	assert.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestPacketMaxInRangeNotFound(t *testing.T) {
	runner := &fakeRunner{responses: []execResponse{{tag: ""}}}
	repo := newTestRepository(t, runner)
	defer repo.Close()

	_, err := repo.PacketMaxInRange(context.Background(), time.Now().Add(-time.Hour), time.Now())
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestParsePacketMaxList(t *testing.T) {
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	output := fmt.Sprintf("id,source,1.5,%s\n", timestamp)
	results, err := parsePacketMaxList(output)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestParseFloat(t *testing.T) {
	value, err := parseFloat(" 1.23 ")
	assert.NoError(t, err)
	assert.InEpsilon(t, 1.23, value, 1e-9)

	_, err = parseFloat("invalid")
	assert.Error(t, err)
}
