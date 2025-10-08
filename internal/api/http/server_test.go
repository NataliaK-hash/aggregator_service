package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aggregator/internal/api/grpc"
	httpapi "aggregator/internal/api/http"
	"aggregator/internal/application/aggregator"
	"aggregator/internal/pkg/uuid"
)

type stubService struct {
	resultByID    aggregator.Result
	resultByRange aggregator.Result
	errByID       error
	errByRange    error

	calledMaxByID    bool
	calledMaxByRange bool
	capturedID       string
	capturedFrom     time.Time
	capturedTo       time.Time
}

func (s *stubService) MaxBySource(_ context.Context, id string) (aggregator.Result, error) {
	s.calledMaxByID = true
	s.capturedID = id
	if s.errByID != nil {
		return aggregator.Result{}, s.errByID
	}
	return s.resultByID, nil
}

func (s *stubService) MaxInRange(_ context.Context, from, to time.Time) (aggregator.Result, error) {
	s.calledMaxByRange = true
	s.capturedFrom = from
	s.capturedTo = to
	if s.errByRange != nil {
		return aggregator.Result{}, s.errByRange
	}
	return s.resultByRange, nil
}

func TestGetMaxByID(t *testing.T) {
	t.Parallel()

	id := uuid.NewString()
	timestamp := time.Now().UTC().Truncate(time.Millisecond)
	service := &stubService{
		resultByID: aggregator.Result{
			SourceID:  id,
			Value:     42.5,
			Timestamp: timestamp,
		},
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/max?id="+id, nil)

	srv := httpapi.NewServer(service)
	srv.ServeHTTP(recorder, req)

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

	if !service.calledMaxByID {
		t.Fatalf("expected service MaxBySource to be called")
	}
	if service.capturedID != id {
		t.Fatalf("expected id %s, got %s", id, service.capturedID)
	}
	if response.ID != id {
		t.Fatalf("expected response id %s, got %s", id, response.ID)
	}
	if response.Value != 42.5 {
		t.Fatalf("expected value 42.5, got %f", response.Value)
	}
	expectedTimestamp := timestamp.Format(time.RFC3339Nano)
	if response.Timestamp != expectedTimestamp {
		t.Fatalf("expected timestamp %s, got %s", expectedTimestamp, response.Timestamp)
	}
}

func TestGetMaxByRange(t *testing.T) {
	t.Parallel()

	from := time.Now().Add(-time.Hour).UTC().Truncate(time.Millisecond)
	to := time.Now().UTC().Truncate(time.Millisecond)
	id := uuid.NewString()
	service := &stubService{
		resultByRange: aggregator.Result{
			SourceID:  id,
			Value:     99.9,
			Timestamp: to,
		},
	}

	recorder := httptest.NewRecorder()
	url := "/max?from=" + from.Format(time.RFC3339Nano) + "&to=" + to.Format(time.RFC3339Nano)
	req := httptest.NewRequest(http.MethodGet, url, nil)

	srv := httpapi.NewServer(service)
	srv.ServeHTTP(recorder, req)

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

	if !service.calledMaxByRange {
		t.Fatalf("expected MaxInRange to be called")
	}
	if !service.capturedFrom.Equal(from) || !service.capturedTo.Equal(to) {
		t.Fatalf("expected captured range %s - %s, got %s - %s", from, to, service.capturedFrom, service.capturedTo)
	}
	if response.ID != id {
		t.Fatalf("expected id %s, got %s", id, response.ID)
	}
	if response.Value != 99.9 {
		t.Fatalf("expected value 99.9, got %f", response.Value)
	}
	expectedTimestamp := to.Format(time.RFC3339Nano)
	if response.Timestamp != expectedTimestamp {
		t.Fatalf("expected timestamp %s, got %s", expectedTimestamp, response.Timestamp)
	}
}

func TestGetMaxByID_InvalidUUID(t *testing.T) {
	t.Parallel()

	service := &stubService{}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/max?id=not-a-uuid", nil)

	srv := httpapi.NewServer(service)
	srv.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
	if service.calledMaxByID {
		t.Fatalf("expected service MaxBySource not to be called")
	}

	var response struct {
		Error string `json:"error"`
		Code  int    `json:"code"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Error != "invalid id format" {
		t.Fatalf("unexpected error message: %s", response.Error)
	}
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unexpected error code: %d", response.Code)
	}
}

func TestGetMaxByRange_InvalidRange(t *testing.T) {
	t.Parallel()

	service := &stubService{}
	recorder := httptest.NewRecorder()
	to := time.Now().UTC().Format(time.RFC3339Nano)
	from := time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)
	url := "/max?from=" + from + "&to=" + to
	req := httptest.NewRequest(http.MethodGet, url, nil)

	srv := httpapi.NewServer(service)
	srv.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
	if service.calledMaxByRange {
		t.Fatalf("expected service MaxInRange not to be called")
	}

	var response struct {
		Error string `json:"error"`
		Code  int    `json:"code"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Error != "from must be before to" {
		t.Fatalf("unexpected error message: %s", response.Error)
	}
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unexpected error code: %d", response.Code)
	}
}

func TestRESTMatchesGRPCResponses(t *testing.T) {
	t.Parallel()

	id := uuid.NewString()
	from := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Millisecond)
	to := time.Now().UTC().Truncate(time.Millisecond)
	result := aggregator.Result{
		SourceID:  id,
		Value:     123.45,
		Timestamp: to,
	}

	restService := &stubService{
		resultByID:    result,
		resultByRange: result,
	}
	restServer := httpapi.NewServer(restService)

	reqByID := httptest.NewRequest(http.MethodGet, "/max?id="+id, nil)
	recorderByID := httptest.NewRecorder()
	restServer.ServeHTTP(recorderByID, reqByID)

	if recorderByID.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorderByID.Code)
	}

	var restByID struct {
		ID        string  `json:"id"`
		Value     float64 `json:"value"`
		Timestamp string  `json:"timestamp"`
	}
	if err := json.NewDecoder(recorderByID.Body).Decode(&restByID); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	grpcService := &stubService{
		resultByID:    result,
		resultByRange: result,
	}
	grpcServer := grpcapi.NewServer(grpcService)

	grpcByID, err := grpcServer.MaxByID(context.Background(), &grpcapi.MaxByIDRequest{Id: id})
	if err != nil {
		t.Fatalf("gRPC MaxByID returned error: %v", err)
	}

	if grpcByID.Id != restByID.ID {
		t.Fatalf("id mismatch: rest=%s grpc=%s", restByID.ID, grpcByID.Id)
	}
	if grpcByID.Value != restByID.Value {
		t.Fatalf("value mismatch: rest=%f grpc=%f", restByID.Value, grpcByID.Value)
	}
	if grpcByID.Timestamp != restByID.Timestamp {
		t.Fatalf("timestamp mismatch: rest=%s grpc=%s", restByID.Timestamp, grpcByID.Timestamp)
	}

	reqByRange := httptest.NewRequest(http.MethodGet, "/max?from="+from.Format(time.RFC3339Nano)+"&to="+to.Format(time.RFC3339Nano), nil)
	recorderByRange := httptest.NewRecorder()
	restServer.ServeHTTP(recorderByRange, reqByRange)
	if recorderByRange.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorderByRange.Code)
	}

	var restByRange struct {
		ID        string  `json:"id"`
		Value     float64 `json:"value"`
		Timestamp string  `json:"timestamp"`
	}
	if err := json.NewDecoder(recorderByRange.Body).Decode(&restByRange); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	grpcByRange, err := grpcServer.MaxByRange(context.Background(), &grpcapi.MaxByRangeRequest{
		From: from.Format(time.RFC3339Nano),
		To:   to.Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatalf("gRPC MaxByRange returned error: %v", err)
	}

	if grpcByRange.Id != restByRange.ID {
		t.Fatalf("id mismatch: rest=%s grpc=%s", restByRange.ID, grpcByRange.Id)
	}
	if grpcByRange.Value != restByRange.Value {
		t.Fatalf("value mismatch: rest=%f grpc=%f", restByRange.Value, grpcByRange.Value)
	}
	if grpcByRange.Timestamp != restByRange.Timestamp {
		t.Fatalf("timestamp mismatch: rest=%s grpc=%s", restByRange.Timestamp, grpcByRange.Timestamp)
	}
}
