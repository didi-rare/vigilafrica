package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	
	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/models"
)

type EventHandler struct {
	repo database.Repository
}

func NewEventHandler(repo database.Repository) *EventHandler {
	return &EventHandler{repo: repo}
}

type APIError struct {
	Error string `json:"error"`
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(APIError{Error: message})
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
		if limit, err := strconv.Atoi(limitStr); err == nil {
			if limit < 1 || limit > 200 {
				respondWithError(w, http.StatusBadRequest, "invalid limit: must be between 1 and 200")
				return
			}
			filters.Limit = limit
		}
	}

	filters.Offset = 0
	if offsetStr := query.Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			if offset < 0 {
				respondWithError(w, http.StatusBadRequest, "invalid offset")
				return
			}
			filters.Offset = offset
		}
	}

	events, total, err := h.repo.ListEvents(r.Context(), filters)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Always output an empty array instead of null
	if events == nil {
		events = make([]models.Event, 0)
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
	json.NewEncoder(w).Encode(response)
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
			respondWithError(w, http.StatusNotFound, "event not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(event)
}
