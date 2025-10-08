package integration

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aggregator-service/app/src/infra"
)

func TestMetricsEndpoint(t *testing.T) {
	infra.RecordDBBatchFlush(0)
	infra.IncGeneratorPackets()

	server := httptest.NewServer(infra.Handler())
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatalf("failed to GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if !strings.Contains(string(body), "aggregator_") {
		t.Fatalf("expected metrics body to contain aggregator_ prefix, got:\n%s", body)
	}
}
