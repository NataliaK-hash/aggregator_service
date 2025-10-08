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
	t.Log("Шаг 1: записываем тестовые метрики")
	infra.RecordDBBatchFlush(0)
	infra.IncGeneratorPackets()

	t.Log("Шаг 2: поднимаем HTTP-сервер с обработчиком метрик")
	server := httptest.NewServer(infra.Handler())
	t.Cleanup(server.Close)

	t.Log("Шаг 3: отправляем запрос к эндпоинту /metrics")
	resp, err := http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatalf("failed to GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	t.Log("Шаг 4: проверяем содержимое ответа")
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if !strings.Contains(string(body), "aggregator_") {
		t.Fatalf("expected metrics body to contain aggregator_ prefix, got:\n%s", body)
	}
}
