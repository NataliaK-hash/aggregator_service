package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"aggregator/internal/application/aggregator"
	"aggregator/internal/pkg/uuid"
)

const (
	queryID   = "id"
	queryFrom = "from"
	queryTo   = "to"
)

// handler contains the HTTP handlers and shared dependencies for the REST API.
type handler struct {
	service aggregator.Service
}

func registerRoutes(router chi.Router, h *handler) {
	router.Get("/max", h.handleGetMax)
}

type maxResponse struct {
	ID        string  `json:"id"`
	Value     float64 `json:"value"`
	Timestamp string  `json:"timestamp"`
}

type errorResponse struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}

func (h *handler) handleGetMax(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()

	idParam := params.Get(queryID)
	fromParam := params.Get(queryFrom)
	toParam := params.Get(queryTo)

	switch {
	case idParam != "" && (fromParam != "" || toParam != ""):
		h.writeError(w, http.StatusBadRequest, "provide either id or time range")
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
	id, err := uuid.Parse(idParam)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid id format")
		return
	}

	result, err := h.service.MaxBySource(r.Context(), id)
	if err != nil {
		h.respondServiceError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, maxResponse{
		ID:        result.SourceID,
		Value:     result.Value,
		Timestamp: result.Timestamp.UTC().Format(time.RFC3339Nano),
	})
}

func (h *handler) handleMaxByRange(w http.ResponseWriter, r *http.Request, fromParam, toParam string) {
	if fromParam == "" || toParam == "" {
		h.writeError(w, http.StatusBadRequest, "both from and to parameters are required")
		return
	}

	from, err := time.Parse(time.RFC3339Nano, fromParam)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid from timestamp")
		return
	}

	to, err := time.Parse(time.RFC3339Nano, toParam)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid to timestamp")
		return
	}

	if from.After(to) {
		h.writeError(w, http.StatusBadRequest, "from must be before to")
		return
	}

	result, err := h.service.MaxInRange(r.Context(), from, to)
	if err != nil {
		h.respondServiceError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, maxResponse{
		ID:        result.SourceID,
		Value:     result.Value,
		Timestamp: result.Timestamp.UTC().Format(time.RFC3339Nano),
	})
}

func (h *handler) respondServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, aggregator.ErrNotFound):
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
