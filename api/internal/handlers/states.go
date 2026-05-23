package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"vigilafrica/api/internal/database"
)

// StatesHandler handles GET /v1/states?country=… or ?country_code=….
// Returns distinct state names for a given country (or all states if neither
// param is set). Used by the frontend country+state filter to dynamically
// populate the state dropdown. See fix-api-country-filter for the input
// contract.
func StatesHandler(repo database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		canonical, _, err := resolveCountry(r.URL.Query())
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		states, err := repo.GetDistinctStatesByCountry(r.Context(), canonical)
		if err != nil {
			slog.Error("states: query failed", "country", canonical, "err", err)
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
