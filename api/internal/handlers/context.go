package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/geoip"
	"vigilafrica/api/internal/models"
)

// ContextResponse represents the resolved context from the user's IP.
type ContextResponse struct {
	Location     *geoip.Location `json:"location"`
	NearbyEvents []models.Event  `json:"nearby_events"`
}

// GetContext returns the location context based on the client's IP and 
// automatically searches for nearby events in PostgreSQL.
func GetContext(db database.Repository, geo *geoip.Reader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := ContextResponse{
			Location:     nil,
			NearbyEvents: make([]models.Event, 0),
		}

		// Priority 1: Check for development override in ENV
		ip := os.Getenv("DEV_OVERRIDE_IP")
		forceLagos := os.Getenv("DEV_FORCE_LAGOS") == "true"
		
		if forceLagos {
			resp.Location = &geoip.Location{
				Country: "Nigeria",
				State:   "Lagos",
				Lat:     6.5244,
				Lng:     3.3792,
			}
		} else if ip == "" {
			// Priority 2: Extract real IP from request
			ip = extractIP(r)
		}

		if !forceLagos && geo != nil && ip != "" {
			loc, err := geo.Lookup(ip)
			if err == nil {
				resp.Location = loc
			}
		}

		// Defaults for local testing when IP is localhost or lookup fails
		centerLat := 9.0820 // Nigeria center default
		centerLng := 8.6753 // Nigeria center default

		if resp.Location != nil {
			centerLat = resp.Location.Lat
			centerLng = resp.Location.Lng
		}

		events, err := db.GetNearbyEvents(r.Context(), centerLat, centerLng, 200.0, 5) // 200km radius, max 5 events
		if err == nil {
			resp.NearbyEvents = events
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}

// extractIP pulls the client IP from the incoming request, accounting for reverse proxies (Vercel/Caddy).
func extractIP(r *http.Request) string {
	// 1. Check X-Forwarded-For
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0]) // User's original IP is the first one
	}

	// 2. Check X-Real-IP
	xrp := r.Header.Get("X-Real-IP")
	if xrp != "" {
		return strings.TrimSpace(xrp)
	}

	// 3. Fallback to RemoteAddr
	remote := r.RemoteAddr
	if idx := strings.LastIndex(remote, ":"); idx != -1 {
		return remote[:idx]
	}
	return remote
}
