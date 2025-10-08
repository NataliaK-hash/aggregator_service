package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"aggregator-service/app/src/domain"

	"github.com/stretchr/testify/assert"
)

type stubPacketMaxReader struct {
	byIDResult   domain.PacketMax
	byIDErr      error
	rangeResults []domain.PacketMax
	rangeErr     error
}

func (s *stubPacketMaxReader) PacketMaxByID(ctx context.Context, packetID string) (domain.PacketMax, error) {
	return s.byIDResult, s.byIDErr
}

func (s *stubPacketMaxReader) PacketMaxInRange(ctx context.Context, from, to time.Time) ([]domain.PacketMax, error) {
	return s.rangeResults, s.rangeErr
}

func newTestAggregator(repo *stubPacketMaxReader) *Aggregator {
	return NewAggregator(repo)
}

func newPacket(id string, value float64, ts time.Time) domain.PacketMax {
	return domain.PacketMax{PacketID: id, SourceID: "source", Value: value, Timestamp: ts}
}

func TestNewAggregator(t *testing.T) {
	repo := &stubPacketMaxReader{}
	agg := newTestAggregator(repo)

	assert.NotNil(t, agg)
	assert.Equal(t, repo, agg.repo)
}

func TestAggregatorMaxByPacketIDSuccess(t *testing.T) {
	now := time.Now().UTC()
	packet := newPacket("packet", 42.5, now)
	agg := newTestAggregator(&stubPacketMaxReader{byIDResult: packet})

	result, err := agg.MaxByPacketID(context.Background(), "packet")

	assert.NoError(t, err)
	assert.Equal(t, toResult(packet), result)
}

func TestAggregatorMaxByPacketIDError(t *testing.T) {
	expected := errors.New("boom")
	agg := newTestAggregator(&stubPacketMaxReader{byIDErr: expected})

	result, err := agg.MaxByPacketID(context.Background(), "packet")

	assert.ErrorIs(t, err, expected)
	assert.Equal(t, domain.AggregatorResult{}, result)
}

func TestAggregatorMaxInRangeSuccess(t *testing.T) {
	now := time.Now().UTC()
	packet := newPacket("packet", 7, now)
	agg := newTestAggregator(&stubPacketMaxReader{rangeResults: []domain.PacketMax{packet}})

	results, err := agg.MaxInRange(context.Background(), now.Add(-time.Hour), now)

	assert.NoError(t, err)
	assert.Equal(t, []domain.AggregatorResult{toResult(packet)}, results)
}

func TestAggregatorMaxInRangeError(t *testing.T) {
	expected := errors.New("boom")
	agg := newTestAggregator(&stubPacketMaxReader{rangeErr: expected})

	results, err := agg.MaxInRange(context.Background(), time.Now(), time.Now())

	assert.ErrorIs(t, err, expected)
	assert.Nil(t, results)
}

func TestToResult(t *testing.T) {
	now := time.Now().UTC()
	packet := newPacket("packet", 11.2, now)

	result := toResult(packet)

	assert.Equal(t, domain.AggregatorResult{
		PacketID:  "packet",
		SourceID:  "source",
		Value:     11.2,
		Timestamp: now,
	}, result)
}
