package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	chi "aggregator-service/app/src/api/chi"
	"aggregator-service/app/src/domain"
	"aggregator-service/app/src/infra"
	"aggregator-service/app/src/shared/constants"
)

const (
	queryPacketID = "packet_id"
	queryFrom     = "from"
	queryTo       = "to"
)

// handler contains the HTTP handlers and shared dependencies for the REST API.
type handler struct {
	service domain.AggregatorService
	logger  *infra.Logger
}

func registerRoutes(router *chi.Mux, h *handler) {
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		if h.logger != nil {
			h.logger.Println(r.Context(), "health check OK")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	router.Get("/max", h.handleGetMax)
}

type maxResponse struct {
	PacketID  string  `json:"packet_id"`
	SourceID  string  `json:"source_id"`
	Value     float64 `json:"value"`
	Timestamp string  `json:"timestamp"`
}

type errorResponse struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}

func (h *handler) handleGetMax(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()

	idParam := params.Get(queryPacketID)
	fromParam := params.Get(queryFrom)
	toParam := params.Get(queryTo)

	switch {
	case idParam != "" && (fromParam != "" || toParam != ""):
		h.writeError(w, http.StatusBadRequest, "provide either packet_id or time range")
		return
	case idParam != "":
		h.handleMaxByID(w, r, idParam)
	case fromParam != "" || toParam != "":
		h.handleMaxByRange(w, r, fromParam, toParam)
	default:
		h.writeError(w, http.StatusBadRequest, "missing required query parameters")
	}
}

func (h *handler) handleMaxByID(w http.ResponseWriter, r *http.Request, idParam string) {
	id, err := constants.ParseUUID(idParam)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid packet_id format")
		return
	}

	result, err := h.service.MaxByPacketID(r.Context(), id)
	if err != nil {
		h.respondServiceError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, toHTTPResponse(result))
}

func (h *handler) handleMaxByRange(w http.ResponseWriter, r *http.Request, fromParam, toParam string) {
	if fromParam == "" || toParam == "" {
		h.writeError(w, http.StatusBadRequest, "both from and to parameters are required")
		return
	}

	from, err := time.Parse(constants.TimeFormat, fromParam)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid from timestamp")
		return
	}

	to, err := time.Parse(constants.TimeFormat, toParam)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid to timestamp")
		return
	}

	if from.After(to) {
		h.writeError(w, http.StatusBadRequest, "from must be before to")
		return
	}

	results, err := h.service.MaxInRange(r.Context(), from, to)
	if err != nil {
		h.respondServiceError(w, err)
		return
	}

	payload := make([]maxResponse, len(results))
	for i, result := range results {
		payload[i] = toHTTPResponse(result)
	}

	h.writeJSON(w, http.StatusOK, payload)
}

func (h *handler) respondServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		h.writeError(w, http.StatusNotFound, "measurement not found")
	default:
		h.writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func (h *handler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, errorResponse{Error: message, Code: status})
}

func (h *handler) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func toHTTPResponse(result domain.AggregatorResult) maxResponse {
	return maxResponse{
		PacketID:  result.PacketID,
		SourceID:  result.SourceID,
		Value:     result.Value,
		Timestamp: result.Timestamp.UTC().Format(constants.TimeFormat),
	}
}
