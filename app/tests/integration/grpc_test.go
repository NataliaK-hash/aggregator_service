package integration

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	grpcapi "aggregator-service/app/src/api/grpc"
	pb "aggregator-service/app/src/api/grpc/pb"
	httpapi "aggregator-service/app/src/api/http"
	"aggregator-service/app/src/domain"
	"aggregator-service/app/src/infra"
	"aggregator-service/app/src/shared/constants"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const bufConnSize = 1024 * 1024

func startGRPCClient(t *testing.T, service domain.AggregatorService) (pb.AggregatorServiceClient, func()) {
	t.Helper()

	listener := bufconn.Listen(bufConnSize)
	server := grpcapi.NewServer(service, infra.NewLogger(io.Discard, "test-grpc"))

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Logf("gRPC server exited: %v", err)
		}
	}()

	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		return listener.Dial()
	}

	conn, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(dialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to dial bufconn: %v", err)
	}

	cleanup := func() {
		_ = conn.Close()
		server.Stop()
		_ = listener.Close()
	}

	return pb.NewAggregatorServiceClient(conn), cleanup
}

func performHTTPMax(t *testing.T, service domain.AggregatorService, url string) (int, []byte) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, url, nil)
	recorder := httptest.NewRecorder()

	httpapi.NewServer(service, infra.NewLogger(io.Discard, "test-http")).ServeHTTP(recorder, req)

	return recorder.Code, recorder.Body.Bytes()
}

func TestGRPCMaxByIDParity(t *testing.T) {
	t.Parallel()

	id := "123e4567-e89b-12d3-a456-426614174000"
	timestamp := time.Now().UTC().Truncate(time.Millisecond)
	service := &stubService{resultByID: domain.AggregatorResult{PacketID: id, SourceID: "source", Value: 42.5, Timestamp: timestamp}}

	client, cleanup := startGRPCClient(t, service)
	defer cleanup()

	ctx := context.Background()
	grpcResp, err := client.GetMaxByID(ctx, &pb.GetByIDRequest{Id: id})
	if err != nil {
		t.Fatalf("gRPC request failed: %v", err)
	}

	statusCode, body := performHTTPMax(t, service, "/max?packet_id="+id)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected HTTP status: %d", statusCode)
	}

	var httpResp struct {
		PacketID  string  `json:"packet_id"`
		SourceID  string  `json:"source_id"`
		Value     float64 `json:"value"`
		Timestamp string  `json:"timestamp"`
	}
	if err := json.Unmarshal(body, &httpResp); err != nil {
		t.Fatalf("failed to decode HTTP response: %v", err)
	}

	if grpcResp.GetId() != httpResp.PacketID {
		t.Fatalf("packet_id mismatch: grpc=%s http=%s", grpcResp.GetId(), httpResp.PacketID)
	}

	if httpResp.SourceID != "source" {
		t.Fatalf("unexpected source id in HTTP response: %s", httpResp.SourceID)
	}

	if grpcResp.GetMaxValue() != httpResp.Value {
		t.Fatalf("value mismatch: grpc=%f http=%f", grpcResp.GetMaxValue(), httpResp.Value)
	}

	if got := grpcResp.GetTimestamp().AsTime().UTC().Format(constants.TimeFormat); got != httpResp.Timestamp {
		t.Fatalf("timestamp mismatch: grpc=%s http=%s", got, httpResp.Timestamp)
	}
}

func TestGRPCMaxByTimeRangeParity(t *testing.T) {
	t.Parallel()

	from := time.Now().Add(-time.Hour).UTC().Truncate(time.Millisecond)
	to := time.Now().UTC().Truncate(time.Millisecond)
	id := "123e4567-e89b-12d3-a456-426614174000"
	service := &stubService{rangeResults: []domain.AggregatorResult{{PacketID: id, SourceID: "source", Value: 84.2, Timestamp: to}}}

	client, cleanup := startGRPCClient(t, service)
	defer cleanup()

	ctx := context.Background()
	grpcResp, err := client.GetMaxByTimeRange(ctx, &pb.GetByTimeRangeRequest{From: timestamppb.New(from), To: timestamppb.New(to)})
	if err != nil {
		t.Fatalf("gRPC request failed: %v", err)
	}

	if len(grpcResp.GetResults()) != 1 {
		t.Fatalf("expected one result, got %d", len(grpcResp.GetResults()))
	}
	result := grpcResp.GetResults()[0]

	query := "/max?from=" + from.Format(constants.TimeFormat) + "&to=" + to.Format(constants.TimeFormat)
	statusCode, body := performHTTPMax(t, service, query)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected HTTP status: %d", statusCode)
	}

	var httpResp []struct {
		PacketID  string  `json:"packet_id"`
		SourceID  string  `json:"source_id"`
		Value     float64 `json:"value"`
		Timestamp string  `json:"timestamp"`
	}
	if err := json.Unmarshal(body, &httpResp); err != nil {
		t.Fatalf("failed to decode HTTP response: %v", err)
	}

	if len(httpResp) != 1 {
		t.Fatalf("expected one HTTP result, got %d", len(httpResp))
	}

	if result.GetId() != httpResp[0].PacketID {
		t.Fatalf("packet_id mismatch: grpc=%s http=%s", result.GetId(), httpResp[0].PacketID)
	}

	if httpResp[0].SourceID != "source" {
		t.Fatalf("unexpected source id in HTTP response: %s", httpResp[0].SourceID)
	}

	if result.GetMaxValue() != httpResp[0].Value {
		t.Fatalf("value mismatch: grpc=%f http=%f", result.GetMaxValue(), httpResp[0].Value)
	}

	if got := result.GetTimestamp().AsTime().UTC().Format(constants.TimeFormat); got != httpResp[0].Timestamp {
		t.Fatalf("timestamp mismatch: grpc=%s http=%s", got, httpResp[0].Timestamp)
	}
}

func TestGRPCMaxByTimeRangeInvalidTimestamp(t *testing.T) {
	t.Parallel()

	service := &stubService{}
	client, cleanup := startGRPCClient(t, service)
	defer cleanup()

	invalid := &timestamppb.Timestamp{Seconds: 1, Nanos: 1_000_000_000}
	_, err := client.GetMaxByTimeRange(context.Background(), &pb.GetByTimeRangeRequest{
		From: invalid,
		To:   timestamppb.Now(),
	})

	if err == nil {
		t.Fatalf("expected validation error")
	}

	st := status.Convert(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %s", st.Code())
	}
}

func TestGRPCValidationErrorParity(t *testing.T) {
	t.Parallel()

	service := &stubService{}
	client, cleanup := startGRPCClient(t, service)
	defer cleanup()

	_, err := client.GetMaxByID(context.Background(), &pb.GetByIDRequest{Id: "invalid"})
	if err == nil {
		t.Fatalf("expected validation error")
	}

	st := status.Convert(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %s", st.Code())
	}

	statusCode, body := performHTTPMax(t, service, "/max?packet_id=invalid")
	if statusCode != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400, got %d", statusCode)
	}

	var httpErr struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &httpErr); err != nil {
		t.Fatalf("failed to decode HTTP error: %v", err)
	}

	if st.Message() != httpErr.Error {
		t.Fatalf("error message mismatch: grpc=%s http=%s", st.Message(), httpErr.Error)
	}
}

func TestGRPCNotFoundParity(t *testing.T) {
	t.Parallel()

	service := &stubService{errByID: domain.ErrNotFound}
	client, cleanup := startGRPCClient(t, service)
	defer cleanup()

	_, err := client.GetMaxByID(context.Background(), &pb.GetByIDRequest{Id: "123e4567-e89b-12d3-a456-426614174000"})
	if err == nil {
		t.Fatalf("expected not found error")
	}

	st := status.Convert(err)
	if st.Code() != codes.NotFound {
		t.Fatalf("expected NotFound, got %s", st.Code())
	}

	statusCode, body := performHTTPMax(t, service, "/max?packet_id=123e4567-e89b-12d3-a456-426614174000")
	if statusCode != http.StatusNotFound {
		t.Fatalf("expected HTTP 404, got %d", statusCode)
	}

	var httpErr struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &httpErr); err != nil {
		t.Fatalf("failed to decode HTTP error: %v", err)
	}

	if st.Message() != httpErr.Error {
		t.Fatalf("error message mismatch: grpc=%s http=%s", st.Message(), httpErr.Error)
	}
}
