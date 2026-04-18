package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"vigilafrica/api/internal/database"
)

// StatesHandler handles GET /v1/states?country=.
// Returns distinct state names for a given country (or all states if country omitted).
// Used by the frontend country+state filter to dynamically populate the state dropdown.
func StatesHandler(repo database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		country := r.URL.Query().Get("country")

		states, err := repo.GetDistinctStatesByCountry(r.Context(), country)
		if err != nil {
			slog.Error("states: query failed", "country", country, "err", err)
			respondWithError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{"states": states}); err != nil {
			slog.Error("states: failed to encode response", "err", err)
		}
	}
}
