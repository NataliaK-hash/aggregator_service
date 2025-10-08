package integration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpapi "aggregator-service/app/src/api/http"
	"aggregator-service/app/src/domain"
	"aggregator-service/app/src/infra"
	"aggregator-service/app/src/shared/constants"
)

type stubService struct {
	resultByID   domain.AggregatorResult
	rangeResults []domain.AggregatorResult
	errByID      error
	errByRange   error

	capturedID   string
	capturedFrom time.Time
	capturedTo   time.Time
}

func (s *stubService) MaxByPacketID(_ context.Context, id string) (domain.AggregatorResult, error) {
	s.capturedID = id
	if s.errByID != nil {
		return domain.AggregatorResult{}, s.errByID
	}
	return s.resultByID, nil
}

func (s *stubService) MaxInRange(_ context.Context, from, to time.Time) ([]domain.AggregatorResult, error) {
	s.capturedFrom = from
	s.capturedTo = to
	if s.errByRange != nil {
		return nil, s.errByRange
	}
	return s.rangeResults, nil
}

func TestHTTPMaxByID(t *testing.T) {
	t.Parallel()

	id := "123e4567-e89b-12d3-a456-426614174000"
	timestamp := time.Now().UTC().Truncate(time.Millisecond)
	service := &stubService{resultByID: domain.AggregatorResult{PacketID: id, SourceID: "source", Value: 42.5, Timestamp: timestamp}}

	req := httptest.NewRequest(http.MethodGet, "/max?packet_id="+id, nil)
	recorder := httptest.NewRecorder()

	httpapi.NewServer(service, infra.NewLogger(io.Discard, "test-http")).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var response struct {
		PacketID  string  `json:"packet_id"`
		SourceID  string  `json:"source_id"`
		Value     float64 `json:"value"`
		Timestamp string  `json:"timestamp"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.PacketID != id {
		t.Fatalf("unexpected packet_id: %s", response.PacketID)
	}
	if response.SourceID != "source" {
		t.Fatalf("unexpected source_id: %s", response.SourceID)
	}
	if response.Value != 42.5 {
		t.Fatalf("unexpected value: %f", response.Value)
	}
	if response.Timestamp != timestamp.Format(constants.TimeFormat) {
		t.Fatalf("unexpected timestamp: %s", response.Timestamp)
	}
	if service.capturedID != id {
		t.Fatalf("service received id %s", service.capturedID)
	}
}

func TestHTTPMaxByRange(t *testing.T) {
	t.Parallel()

	from := time.Now().Add(-time.Hour).UTC().Truncate(time.Millisecond)
	to := time.Now().UTC().Truncate(time.Millisecond)
	id := "123e4567-e89b-12d3-a456-426614174000"
	service := &stubService{rangeResults: []domain.AggregatorResult{{PacketID: id, SourceID: "source", Value: 99.9, Timestamp: to}}}

	url := "/max?from=" + from.Format(constants.TimeFormat) + "&to=" + to.Format(constants.TimeFormat)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	recorder := httptest.NewRecorder()

	httpapi.NewServer(service, infra.NewLogger(io.Discard, "test-http")).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var response []struct {
		PacketID  string  `json:"packet_id"`
		SourceID  string  `json:"source_id"`
		Value     float64 `json:"value"`
		Timestamp string  `json:"timestamp"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response) != 1 {
		t.Fatalf("expected one response element, got %d", len(response))
	}
	if response[0].PacketID != id {
		t.Fatalf("unexpected packet_id: %s", response[0].PacketID)
	}
	if response[0].SourceID != "source" {
		t.Fatalf("unexpected source_id: %s", response[0].SourceID)
	}
	if response[0].Value != 99.9 {
		t.Fatalf("unexpected value: %f", response[0].Value)
	}
	if response[0].Timestamp != to.Format(constants.TimeFormat) {
		t.Fatalf("unexpected timestamp: %s", response[0].Timestamp)
	}
	if !service.capturedFrom.Equal(from) || !service.capturedTo.Equal(to) {
		t.Fatalf("service captured unexpected range: %s - %s", service.capturedFrom, service.capturedTo)
	}
}

func TestHTTPMaxValidationErrors(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/max?packet_id=invalid", nil)
	recorder := httptest.NewRecorder()

	httpapi.NewServer(&stubService{}, infra.NewLogger(io.Discard, "test-http")).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestHTTPMethodNotAllowed(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/max", nil)
	recorder := httptest.NewRecorder()

	httpapi.NewServer(&stubService{}, infra.NewLogger(io.Discard, "test-http")).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", recorder.Code)
	}

	if allow := recorder.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf("expected Allow header %s, got %s", http.MethodGet, allow)
	}
}
