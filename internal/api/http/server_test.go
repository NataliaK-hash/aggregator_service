package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpapi "aggregator-service-project/internal/api/http"
	"aggregator-service-project/internal/domain"
	"aggregator-service-project/internal/pkg/uuid"
)

type stubService struct {
	resultByID    domain.AggregatorResult
	resultByRange domain.AggregatorResult
	errByID       error
	errByRange    error

	capturedID   string
	capturedFrom time.Time
	capturedTo   time.Time
}

func (s *stubService) MaxBySource(_ context.Context, id string) (domain.AggregatorResult, error) {
	s.capturedID = id
	if s.errByID != nil {
		return domain.AggregatorResult{}, s.errByID
	}
	return s.resultByID, nil
}

func (s *stubService) MaxInRange(_ context.Context, from, to time.Time) (domain.AggregatorResult, error) {
	s.capturedFrom = from
	s.capturedTo = to
	if s.errByRange != nil {
		return domain.AggregatorResult{}, s.errByRange
	}
	return s.resultByRange, nil
}

func TestGetMaxByID_Success(t *testing.T) {
	t.Parallel()

	id := uuid.NewString()
	timestamp := time.Now().UTC().Truncate(time.Millisecond)
	service := &stubService{resultByID: domain.AggregatorResult{SourceID: id, Value: 42.5, Timestamp: timestamp}}

	req := httptest.NewRequest(http.MethodGet, "/max?id="+id, nil)
	recorder := httptest.NewRecorder()

	httpapi.NewServer(service).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var response struct {
		ID        string  `json:"id"`
		Value     float64 `json:"value"`
		Timestamp string  `json:"timestamp"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ID != id {
		t.Fatalf("unexpected id: %s", response.ID)
	}
	if response.Value != 42.5 {
		t.Fatalf("unexpected value: %f", response.Value)
	}
	if response.Timestamp != timestamp.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected timestamp: %s", response.Timestamp)
	}
	if service.capturedID != id {
		t.Fatalf("service received id %s", service.capturedID)
	}
}

func TestGetMaxByRange_Success(t *testing.T) {
	t.Parallel()

	from := time.Now().Add(-time.Hour).UTC().Truncate(time.Millisecond)
	to := time.Now().UTC().Truncate(time.Millisecond)
	id := uuid.NewString()
	service := &stubService{resultByRange: domain.AggregatorResult{SourceID: id, Value: 99.9, Timestamp: to}}

	url := "/max?from=" + from.Format(time.RFC3339Nano) + "&to=" + to.Format(time.RFC3339Nano)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	recorder := httptest.NewRecorder()

	httpapi.NewServer(service).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var response struct {
		ID        string  `json:"id"`
		Value     float64 `json:"value"`
		Timestamp string  `json:"timestamp"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ID != id {
		t.Fatalf("unexpected id: %s", response.ID)
	}
	if response.Value != 99.9 {
		t.Fatalf("unexpected value: %f", response.Value)
	}
	if response.Timestamp != to.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected timestamp: %s", response.Timestamp)
	}
	if !service.capturedFrom.Equal(from) || !service.capturedTo.Equal(to) {
		t.Fatalf("service captured unexpected range: %s - %s", service.capturedFrom, service.capturedTo)
	}
}

func TestGetMax_InvalidUUID(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/max?id=invalid", nil)
	recorder := httptest.NewRecorder()

	httpapi.NewServer(&stubService{}).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestGetMax_InvalidRange(t *testing.T) {
	t.Parallel()

	to := time.Now().UTC().Format(time.RFC3339Nano)
	from := time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)
	req := httptest.NewRequest(http.MethodGet, "/max?from="+from+"&to="+to, nil)
	recorder := httptest.NewRecorder()

	httpapi.NewServer(&stubService{}).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestGetMax_NotFound(t *testing.T) {
	t.Parallel()

	id := uuid.NewString()
	service := &stubService{errByID: domain.ErrNotFound}

	req := httptest.NewRequest(http.MethodGet, "/max?id="+id, nil)
	recorder := httptest.NewRecorder()

	httpapi.NewServer(service).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}
}
