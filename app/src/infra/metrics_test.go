package infra

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

func TestInitMetricsIdempotent(t *testing.T) {
	t.Log("повторно инициализируем метрики без паники")
	assert.NotPanics(t, func() { InitMetrics() })
	assert.NotPanics(t, func() { InitMetrics() })
}

func TestMetricsHandlerServesContent(t *testing.T) {
	t.Log("Швызываем HTTP-обработчик метрик")
	InitMetrics()
	handler := Handler()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	t.Log("проверяем статус и содержимое")
	assert.Equal(t, http.StatusOK, rr.Result().StatusCode)
	assert.Contains(t, rr.Body.String(), "# HELP")
}

func TestStartMetricsServerIsIdempotent(t *testing.T) {
	t.Log("запускаем сервер метрик дважды")
	logger := NewLogger(io.Discard, "metrics")
	StartMetricsServer(logger)
	StartMetricsServer(logger)
}

func TestHTTPMiddlewareRecordsMetrics(t *testing.T) {
	t.Log("измеряем счётчик запросов до вызова")
	InitMetrics()
	beforeRequests := HttpRequestsTotal.Value()

	middleware := HTTPMiddleware(nil)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodGet, "/items", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	t.Log("сравниваем значение счётчика после запроса")
	afterRequests := HttpRequestsTotal.Value()
	assert.Equal(t, beforeRequests+1, afterRequests)
}

func TestHTTPMiddlewareRecordsErrors(t *testing.T) {
	t.Log("измеряем счётчик ошибок до вызова")
	InitMetrics()
	beforeErrors := HttpRequestErrorsTotal.Value()

	middleware := HTTPMiddleware(nil)
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("should not be called")
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, nil)

	t.Log("убеждаемся в росте счётчика ошибок")
	afterErrors := HttpRequestErrorsTotal.Value()
	assert.Equal(t, beforeErrors+1, afterErrors)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGRPCUnaryInterceptorRecordsMetrics(t *testing.T) {
	t.Log("получаем метрики до вызова gRPC-интерсептора")
	InitMetrics()
	interceptor := GRPCUnaryInterceptor()

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		time.Sleep(10 * time.Millisecond)
		return "ok", nil
	}

	before := HttpRequestsTotal.Value()
	t.Log("вызываем интерсептор и проверяем результат")
	resp, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{FullMethod: "/service/Method"}, handler)

	assert.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.Equal(t, before+1, HttpRequestsTotal.Value())
}

func TestRecordDBBatchFlushIncrementsMetrics(t *testing.T) {
	t.Log("вызываем запись события сброса батча")
	InitMetrics()
	before := DbBatchFlushTotal.Value()

	RecordDBBatchFlush(500 * time.Millisecond)

	assert.Equal(t, before+1, DbBatchFlushTotal.Value())
}

func TestIncGeneratorPacketsIncrementsCounter(t *testing.T) {
	t.Log("инкрементируем счётчик пакетов генератора")
	InitMetrics()
	before := PacketsTotal.Value()
	IncGeneratorPackets()
	assert.Equal(t, before+1, PacketsTotal.Value())
}

func TestWorkerStartedAndFinishedAdjustGauge(t *testing.T) {
	t.Log("фиксируем запуск и завершение воркера")
	InitMetrics()
	WorkerStarted()
	assert.Equal(t, 1.0, WorkerPoolActiveGoroutines.Value())
	WorkerFinished()
	assert.Equal(t, 0.0, WorkerPoolActiveGoroutines.Value())
}

func TestStatusRecorder(t *testing.T) {
	t.Log("проверяем, что статус-рекордер сохраняет код ответа")
	recorder := &statusRecorder{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}
	recorder.WriteHeader(http.StatusTeapot)
	assert.Equal(t, http.StatusTeapot, recorder.Status())
}
