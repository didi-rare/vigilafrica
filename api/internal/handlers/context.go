package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/geoip"
	"vigilafrica/api/internal/models"
)

// LocationSource tells the caller how the returned location was determined, so
// a degraded IP lookup is distinguishable from a deliberate query
// (fix-unify-client-ip-resolution). Without this the fallback is invisible,
// which is the same failure mode feature-events-pagination removed for
// truncated result sets.
type LocationSource string

const (
	// LocationSourceExplicit — the caller supplied lat/lng.
	LocationSourceExplicit LocationSource = "explicit"
	// LocationSourceDevOverride — a DEV_* environment override was in effect.
	LocationSourceDevOverride LocationSource = "dev_override"
	// LocationSourceIP — derived from the resolved client IP.
	LocationSourceIP LocationSource = "ip"
	// LocationSourceUnavailable — no location could be determined; Location is null.
	LocationSourceUnavailable LocationSource = "unavailable"
)

// ContextResponse represents the resolved context for the caller.
type ContextResponse struct {
	Location       *geoip.Location `json:"location"`
	LocationSource LocationSource  `json:"location_source"`
	NearbyEvents   []models.Event  `json:"nearby_events"`
}

// locationPlan is the decision about where the caller is, made before any
// database or mmdb work. Exactly one of Explicit or LookupIP is set.
type locationPlan struct {
	// Explicit is a location already known without a lookup.
	Explicit *geoip.Location
	// LookupIP is an address to resolve through the GeoIP reader.
	LookupIP string
	Source   LocationSource
}

// resolveLocationPlan decides which location the response should describe.
//
// Precedence is fixed and deliberate (fix-unify-client-ip-resolution §0):
//
//	1. explicit lat/lng    -> the caller said exactly where they mean
//	2. DEV_FORCE_LAGOS     -> hardcoded, suppresses any lookup
//	3. DEV_OVERRIDE_IP     -> look that address up instead of the request's
//	4. otherwise           -> look up the trusted-proxy-aware client IP
//
// Explicit outranks the DEV_* overrides because those exist to substitute for
// unhelpful IP geolocation on a developer machine, not to override a caller
// who stated their intent. If DEV_* won, the documented parameter would
// silently stop working wherever the variable happened to be set.
//
// This is the single seam where the request becomes a location. Keeping it one
// function is what stops a second, unguarded IP path reappearing.
func resolveLocationPlan(r *http.Request) (locationPlan, error) {
	explicit, ok, err := parseExplicitLocation(r)
	if err != nil {
		return locationPlan{}, err
	}
	if ok {
		return locationPlan{Explicit: explicit, Source: LocationSourceExplicit}, nil
	}

	if os.Getenv("DEV_FORCE_LAGOS") == "true" {
		return locationPlan{
			Explicit: &geoip.Location{Country: "Nigeria", State: "Lagos", Lat: 6.5244, Lng: 3.3792},
			Source:   LocationSourceDevOverride,
		}, nil
	}

	if devIP := os.Getenv("DEV_OVERRIDE_IP"); devIP != "" {
		return locationPlan{LookupIP: devIP, Source: LocationSourceDevOverride}, nil
	}

	// clientIP honours forwarded headers ONLY from a configured trusted proxy.
	// This handler previously used its own unguarded helper, which believed
	// X-Forwarded-For from anybody; that helper is deleted.
	return locationPlan{LookupIP: clientIP(r), Source: LocationSourceIP}, nil
}

// parseExplicitLocation reads the optional lat/lng query parameters.
//
// Returns ok=false with a nil error when neither is present — absence is not an
// error. A malformed or out-of-range value IS an error naming the offending
// parameter, and the caller must surface it as a 400: falling back to IP would
// make a typo indistinguishable from a working query.
func parseExplicitLocation(r *http.Request) (*geoip.Location, bool, error) {
	q := r.URL.Query()
	hasLat, hasLng := q.Has("lat"), q.Has("lng")

	switch {
	case !hasLat && !hasLng:
		return nil, false, nil
	case hasLat && !hasLng:
		return nil, false, fmt.Errorf("invalid location: lng is required when lat is supplied")
	case !hasLat && hasLng:
		return nil, false, fmt.Errorf("invalid location: lat is required when lng is supplied")
	}

	lat, err := strconv.ParseFloat(q.Get("lat"), 64)
	if err != nil {
		return nil, false, fmt.Errorf("invalid lat: must be a number between -90 and 90")
	}
	lng, err := strconv.ParseFloat(q.Get("lng"), 64)
	if err != nil {
		return nil, false, fmt.Errorf("invalid lng: must be a number between -180 and 180")
	}
	if lat < -90 || lat > 90 {
		return nil, false, fmt.Errorf("invalid lat: must be between -90 and 90")
	}
	if lng < -180 || lng > 180 {
		return nil, false, fmt.Errorf("invalid lng: must be between -180 and 180")
	}

	// Country and State are deliberately left empty: no point-to-admin-name
	// resolver exists in this service, and inventing one here would be new
	// capability. The coordinates are all the nearby-events query needs.
	return &geoip.Location{Lat: lat, Lng: lng}, true, nil
}

// GetContext returns the caller's location and nearby events.
//
// Location comes from an explicit lat/lng when supplied, otherwise from the
// client IP resolved under the trusted-proxy policy. See resolveLocationPlan
// for the full precedence.
func GetContext(db database.Repository, geo *geoip.Reader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		plan, err := resolveLocationPlan(r)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		resp := ContextResponse{
			Location:       nil,
			LocationSource: LocationSourceUnavailable,
			NearbyEvents:   make([]models.Event, 0),
		}

		switch {
		case plan.Explicit != nil:
			resp.Location = plan.Explicit
			resp.LocationSource = plan.Source
		case geo != nil && plan.LookupIP != "":
			// A failed lookup is not an error: the endpoint degrades to a null
			// location with HTTP 200 rather than failing the request.
			if loc, lookupErr := geo.Lookup(plan.LookupIP); lookupErr == nil {
				resp.Location = loc
				resp.LocationSource = plan.Source
			}
		}

		// Defaults for local testing when the IP is localhost or lookup fails.
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
		// best-effort — status code already framed; encode errors are network-level (§4.7).
		_ = json.NewEncoder(w).Encode(resp)
	}
}
