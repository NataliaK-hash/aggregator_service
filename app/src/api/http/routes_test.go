package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	chi "aggregator-service/app/src/api/chi"
	"aggregator-service/app/src/domain"
	"aggregator-service/app/src/infra"
	"aggregator-service/app/src/shared/constants"
	"github.com/stretchr/testify/assert"
)

type stubAggregatorService struct {
	maxByIDResult    domain.AggregatorResult
	maxByIDErr       error
	maxInRangeResult []domain.AggregatorResult
	maxInRangeErr    error

	lastID   string
	lastFrom time.Time
	lastTo   time.Time
}

func (s *stubAggregatorService) MaxByPacketID(ctx context.Context, packetID string) (domain.AggregatorResult, error) {
	s.lastID = packetID
	return s.maxByIDResult, s.maxByIDErr
}

func (s *stubAggregatorService) MaxInRange(ctx context.Context, from, to time.Time) ([]domain.AggregatorResult, error) {
	s.lastFrom = from
	s.lastTo = to
	return s.maxInRangeResult, s.maxInRangeErr
}

func TestRegisterRoutesRegistersHealthEndpoints(t *testing.T) {
	t.Log("Шаг 1: регистрируем роуты и проверяем эндпоинты здоровья")
	router := chi.NewRouter()
	service := &stubAggregatorService{}
	h := &handler{service: service, logger: infra.NewLogger(io.Discard, "test")}

	registerRoutes(router, h)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandlerHandleGetMaxRequiresParameters(t *testing.T) {
	t.Log("Шаг 1: выполняем запрос без параметров и ожидаем 400")
	h := &handler{service: &stubAggregatorService{}}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/max", nil)

	h.handleGetMax(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandlerHandleGetMaxRejectsMixedParameters(t *testing.T) {
	t.Log("Шаг 1: отправляем одновременно packet_id и диапазон")
	h := &handler{service: &stubAggregatorService{}}

	req := httptest.NewRequest(http.MethodGet, "/max?packet_id=1&from=2", nil)
	rr := httptest.NewRecorder()

	h.handleGetMax(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleMaxByIDSuccess(t *testing.T) {
	t.Log("Шаг 1: готовим успешный ответ сервиса по ID")
	id := "00000000-0000-0000-0000-000000000000"
	now := time.Now().UTC()
	service := &stubAggregatorService{maxByIDResult: domain.AggregatorResult{PacketID: id, SourceID: "src", Value: 10, Timestamp: now}}
	h := &handler{service: service}

	req := httptest.NewRequest(http.MethodGet, "/max?packet_id="+id, nil)
	rr := httptest.NewRecorder()

	t.Log("Шаг 2: вызываем обработчик и проверяем ответ")
	h.handleGetMax(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, id, service.lastID)
}

func TestHandleMaxByIDValidationError(t *testing.T) {
	t.Log("Шаг 1: проверяем отказ при некорректном идентификаторе")
	h := &handler{service: &stubAggregatorService{}}

	req := httptest.NewRequest(http.MethodGet, "/max?packet_id=invalid", nil)
	rr := httptest.NewRecorder()

	h.handleGetMax(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleMaxByIDServiceError(t *testing.T) {
	t.Log("Шаг 1: сервис возвращает ошибку NotFound для ID")
	id := "00000000-0000-0000-0000-000000000000"
	service := &stubAggregatorService{maxByIDErr: domain.ErrNotFound}
	h := &handler{service: service}

	req := httptest.NewRequest(http.MethodGet, "/max?packet_id="+id, nil)
	rr := httptest.NewRecorder()

	t.Log("Шаг 2: вызываем обработчик и ожидаем 404")
	h.handleGetMax(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleMaxByRangeValidation(t *testing.T) {
	t.Log("Шаг 1: отправляем пустые параметры диапазона")
	h := &handler{service: &stubAggregatorService{}}

	req := httptest.NewRequest(http.MethodGet, "/max?from=&to=", nil)
	rr := httptest.NewRecorder()
	h.handleGetMax(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	t.Log("Шаг 2: отправляем некорректные даты")
	req = httptest.NewRequest(http.MethodGet, "/max?from=invalid&to=invalid", nil)
	rr = httptest.NewRecorder()
	h.handleGetMax(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleMaxByRangeSuccess(t *testing.T) {
	t.Log("Шаг 1: готовим корректный диапазон и ответ сервиса")
	now := time.Now().UTC().Truncate(time.Second)
	from := now.Add(-time.Hour)
	to := now
	result := domain.AggregatorResult{PacketID: constants.GenerateUUID(), SourceID: constants.GenerateUUID(), Value: 5, Timestamp: now}
	service := &stubAggregatorService{maxInRangeResult: []domain.AggregatorResult{result}}
	h := &handler{service: service}

	query := "/max?from=" + from.Format(constants.TimeFormat) + "&to=" + to.Format(constants.TimeFormat)
	req := httptest.NewRequest(http.MethodGet, query, nil)
	rr := httptest.NewRecorder()

	t.Log("Шаг 2: выполняем запрос и проверяем диапазон")
	h.handleGetMax(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, service.lastFrom.Equal(from))
	assert.True(t, service.lastTo.Equal(to))
}

func TestHandleMaxByRangeServiceError(t *testing.T) {
	t.Log("Шаг 1: сервис возвращает ошибку NotFound по диапазону")
	now := time.Now().UTC()
	from := now.Add(-time.Hour)
	service := &stubAggregatorService{maxInRangeErr: domain.ErrNotFound}
	h := &handler{service: service}

	query := "/max?from=" + from.Format(constants.TimeFormat) + "&to=" + now.Format(constants.TimeFormat)
	req := httptest.NewRequest(http.MethodGet, query, nil)
	rr := httptest.NewRecorder()

	t.Log("Шаг 2: проверяем, что обработчик вернул 404")
	h.handleGetMax(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestWriteJSONSetsContentType(t *testing.T) {
	t.Log("Шаг 1: записываем JSON-ответ и проверяем заголовки")
	h := &handler{service: &stubAggregatorService{}}
	rr := httptest.NewRecorder()
	payload := map[string]string{"status": "ok"}

	h.writeJSON(rr, http.StatusAccepted, payload)

	assert.Equal(t, http.StatusAccepted, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var decoded map[string]string
	assert.NoError(t, json.Unmarshal(rr.Body.Bytes(), &decoded))
	assert.Equal(t, payload, decoded)
}

func TestToHTTPResponse(t *testing.T) {
	t.Log("Шаг 1: конвертируем доменную структуру в HTTP-ответ")
	now := time.Now().UTC()
	result := domain.AggregatorResult{PacketID: "id", SourceID: "src", Value: 1, Timestamp: now}

	response := toHTTPResponse(result)
	assert.Equal(t, result.PacketID, response.PacketID)
	assert.Equal(t, result.SourceID, response.SourceID)
	assert.Equal(t, result.Value, response.Value)
	assert.Equal(t, now.UTC().Format(constants.TimeFormat), response.Timestamp)
}

func TestServerRouterAndServeHTTP(t *testing.T) {
	t.Log("Шаг 1: создаём HTTP-сервер и проверяем его роутер")
	service := &stubAggregatorService{}
	server := NewServer(service, infra.NewLogger(io.Discard, "test"))

	assert.NotNil(t, server.Router())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	t.Log("Шаг 2: вызываем ServeHTTP и ожидаем успешный ответ")
	server.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}
