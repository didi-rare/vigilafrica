package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"vigilafrica/api/internal/database"
)

type EventHandler struct {
	repo database.Repository
	log  *slog.Logger
}

// NewEventHandler builds the handler. A nil logger falls back to slog.Default()
// (mirrors NewDigestHandler), so tests can pass nil.
func NewEventHandler(repo database.Repository, logger *slog.Logger) *EventHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &EventHandler{repo: repo, log: logger}
}

type APIError struct {
	Error string `json:"error"`
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	// best-effort — status code already framed; encode errors are network-level (§4.7).
	_ = json.NewEncoder(w).Encode(APIError{Error: message})
}

func (h *EventHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	filters := database.EventFilters{}

	// Parse Query Params
	query := r.URL.Query()

	if cat := query.Get("category"); cat != "" {
		if cat != "floods" && cat != "wildfires" {
			respondWithError(w, http.StatusBadRequest, "invalid category: valid values: floods, wildfires")
			return
		}
		filters.Category = cat
	}

	canonical, present, err := resolveCountry(query)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	if present {
		filters.Country = canonical
	}

	if state := query.Get("state"); state != "" {
		filters.State = state
	}

	if status := query.Get("status"); status != "" {
		if status != "open" && status != "closed" {
			respondWithError(w, http.StatusBadRequest, "invalid status: valid values: open, closed")
			return
		}
		filters.Status = status
	}

	filters.Limit = 50
	if limitStr := query.Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "invalid limit: must be an integer")
			return
		}
		if limit < 1 || limit > 200 {
			respondWithError(w, http.StatusBadRequest, "invalid limit: must be between 1 and 200")
			return
		}
		filters.Limit = limit
	}

	filters.Offset = 0
	if offsetStr := query.Get("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "invalid offset: must be an integer")
			return
		}
		if offset < 0 {
			respondWithError(w, http.StatusBadRequest, "invalid offset")
			return
		}
		filters.Offset = offset
	}

	// Note: repo.ListEvents guarantees a non-nil slice — no defensive nil-coalesce needed here.
	events, total, err := h.repo.ListEvents(r.Context(), filters)
	if err != nil {
		// Log the real cause (§4.5/§8.6); return a sanitised message to the client.
		h.log.Error("list events failed", "err", err,
			"category", filters.Category, "country", filters.Country, "state", filters.State)
		respondWithError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response := map[string]interface{}{
		"data": events,
		"meta": map[string]interface{}{
			"total":  total,
			"limit":  filters.Limit,
			"offset": filters.Offset,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// best-effort — status code already framed; encode errors are network-level (§4.7).
	_ = json.NewEncoder(w).Encode(response)
}

func (h *EventHandler) GetEventByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		respondWithError(w, http.StatusBadRequest, "missing event id")
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid event id: must be a valid UUID")
		return
	}

	event, err := h.repo.GetEventByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Not an error condition — a 404 is the correct answer, so no log.
			respondWithError(w, http.StatusNotFound, "event not found")
			return
		}
		// Log the real cause (§4.5/§8.6); return a sanitised message to the client.
		h.log.Error("get event by id failed", "err", err, "event_id", id)
		respondWithError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// best-effort — status code already framed; encode errors are network-level (§4.7).
	_ = json.NewEncoder(w).Encode(event)
}
