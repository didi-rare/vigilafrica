package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// HealthResponse is the response body for GET /health.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// HealthHandler encapsulates the health check logic.
type HealthHandler struct {
	Version string
}

// NewHealthHandler creates a new instance of HealthHandler.
func NewHealthHandler(version string) *HealthHandler {
	return &HealthHandler{Version: version}
}

// ServeHTTP implements the http.Handler interface for the health check.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := HealthResponse{
		Status:  "ok",
		Version: h.Version,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode health response", "err", err)
	}
}
