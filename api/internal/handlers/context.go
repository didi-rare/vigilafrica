package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"
	"strconv"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/geoip"
	"vigilafrica/api/internal/models"
)

// LocationSource tells the caller how the returned location was determined, so
// a degraded IP lookup is distinguishable from a deliberate query. Without it
// the fallback is invisible — the same failure mode feature-events-pagination
// removed for truncated result sets.
type LocationSource string

const (
	// LocationSourceExplicit — the caller supplied lat/lng.
	LocationSourceExplicit LocationSource = "explicit"
	// LocationSourceDevOverride — a DEV_* configuration override was in effect.
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

// GeoLookup is the GeoIP surface this handler needs. *geoip.Reader satisfies
// it. Declared as an interface so handler behaviour is testable without an
// mmdb file (§9).
type GeoLookup interface {
	Lookup(ipStr string) (*geoip.Location, error)
}

// ContextConfig is the configuration GetContext needs, read once at startup
// (§2.6) and injected rather than pulled from the environment per request.
type ContextConfig struct {
	// Proxy decides which peers may set forwarded headers.
	Proxy ProxyConfig
	// DevOverrideIP, when non-empty, is looked up instead of the request's IP.
	DevOverrideIP string
	// DevForceLagos short-circuits to a hardcoded Lagos location.
	DevForceLagos bool
}

// LoadContextConfig reads the context handler's configuration from the
// environment. Call this ONCE from main and inject the result (§2.6).
func LoadContextConfig(logger *slog.Logger) ContextConfig {
	if logger == nil {
		logger = slog.Default()
	}
	cfg := ContextConfig{
		Proxy:         LoadProxyConfig(),
		DevOverrideIP: os.Getenv("DEV_OVERRIDE_IP"),
		DevForceLagos: os.Getenv("DEV_FORCE_LAGOS") == "true",
	}
	if cfg.DevOverrideIP != "" || cfg.DevForceLagos {
		// These exist for local development. If they are ever set in a deployed
		// environment they silently replace real geolocation, so make that
		// visible in the logs at startup rather than leaving it to be discovered.
		logger.Warn("context: development location override active",
			"dev_force_lagos", cfg.DevForceLagos,
			"dev_override_ip_set", cfg.DevOverrideIP != "")
	}
	return cfg
}

// ContextHandler serves GET /v1/context. Dependencies are held on the struct
// and supplied by NewContextHandler in main (§6.3).
type ContextHandler struct {
	repo database.Repository
	geo  GeoLookup
	cfg  ContextConfig
	log  *slog.Logger
}

// NewContextHandler constructs the handler.
//
// ⚠️ geo may legitimately be nil — the server starts without a GeoIP database
// and degrades to a null location. Pass an untyped nil, NOT a nil *geoip.Reader
// stored in the interface: a typed nil is a non-nil interface value and would
// dereference on the first lookup. main guards this explicitly.
func NewContextHandler(repo database.Repository, geo GeoLookup, cfg ContextConfig, logger *slog.Logger) *ContextHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ContextHandler{repo: repo, geo: geo, cfg: cfg, log: logger}
}

// locationPlan is the decision about where the caller is, made before any
// database or mmdb work. At most one of Explicit or LookupIP is set.
type locationPlan struct {
	Explicit *geoip.Location
	LookupIP string
	Source   LocationSource
}

// resolveLocationPlan decides which location the response should describe.
//
// Precedence is fixed and deliberate (fix-unify-client-ip-resolution §0):
//
//	1. explicit lat/lng    -> the caller said exactly where they mean
//	2. DevForceLagos       -> hardcoded, suppresses any lookup
//	3. DevOverrideIP       -> look that address up instead of the request's
//	4. otherwise           -> look up the trusted-proxy-aware client IP
//
// Explicit outranks the DEV_* overrides because those exist to substitute for
// unhelpful IP geolocation on a developer machine, not to override a caller who
// stated their intent. If DEV_* won, the documented parameter would silently
// stop working wherever the variable happened to be set.
//
// This is the single seam where a request becomes a location. Keeping it one
// function is what stops a second, unguarded IP path reappearing.
func (h *ContextHandler) resolveLocationPlan(r *http.Request) (locationPlan, error) {
	explicit, ok, err := parseExplicitLocation(r)
	if err != nil {
		return locationPlan{}, err
	}
	if ok {
		return locationPlan{Explicit: explicit, Source: LocationSourceExplicit}, nil
	}

	if h.cfg.DevForceLagos {
		return locationPlan{
			Explicit: &geoip.Location{Country: "Nigeria", State: "Lagos", Lat: 6.5244, Lng: 3.3792},
			Source:   LocationSourceDevOverride,
		}, nil
	}

	if h.cfg.DevOverrideIP != "" {
		return locationPlan{LookupIP: h.cfg.DevOverrideIP, Source: LocationSourceDevOverride}, nil
	}

	// Forwarded headers are honoured ONLY from a configured trusted proxy. This
	// handler previously used its own unguarded helper, which believed
	// X-Forwarded-For from anybody; that helper is deleted.
	return locationPlan{LookupIP: h.cfg.Proxy.ClientIP(r), Source: LocationSourceIP}, nil
}

// parseExplicitLocation reads the optional lat/lng query parameters.
//
// Returns ok=false with a nil error when neither is present — absence is not an
// error. A malformed, non-finite, or out-of-range value IS an error naming the
// offending parameter, and the caller must surface it as 400 (§6.4): falling
// back to IP would make a typo indistinguishable from a working query.
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

	lat, err := parseCoordinate(q.Get("lat"), "lat", 90)
	if err != nil {
		return nil, false, err
	}
	lng, err := parseCoordinate(q.Get("lng"), "lng", 180)
	if err != nil {
		return nil, false, err
	}

	// Country and State are deliberately left empty: no point-to-admin-name
	// resolver exists in this service, and inventing one here would be new
	// capability. The coordinates are all the nearby-events query needs.
	return &geoip.Location{Lat: lat, Lng: lng}, true, nil
}

// parseCoordinate parses one coordinate and bounds it to ±limit.
//
// ⚠️ The non-finite check is not defensive padding. strconv.ParseFloat accepts
// "NaN" and "Inf" without error, and EVERY comparison against NaN is false — so
// a plain `v < -limit || v > limit` range check lets NaN straight through. It
// would then reach json.Marshal, which cannot encode NaN, and the failure would
// land AFTER WriteHeader(200) had already committed: the caller gets a 200 with
// a truncated body instead of the documented 400.
func parseCoordinate(raw, name string, limit float64) (float64, error) {
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: must be a number between %g and %g", name, -limit, limit)
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("invalid %s: must be a finite number between %g and %g", name, -limit, limit)
	}
	if v < -limit || v > limit {
		return 0, fmt.Errorf("invalid %s: must be between %g and %g", name, -limit, limit)
	}
	return v, nil
}

// GetContext returns the caller's location and nearby events.
//
// Location comes from an explicit lat/lng when supplied, otherwise from the
// client IP resolved under the trusted-proxy policy. See resolveLocationPlan
// for the full precedence.
func (h *ContextHandler) GetContext(w http.ResponseWriter, r *http.Request) {
	plan, err := h.resolveLocationPlan(r)
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
	case h.geo != nil && plan.LookupIP != "":
		// A failed lookup is not an error: the endpoint degrades to a null
		// location with HTTP 200 rather than failing the request.
		if loc, lookupErr := h.geo.Lookup(plan.LookupIP); lookupErr == nil {
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

	events, err := h.repo.GetNearbyEvents(r.Context(), centerLat, centerLng, 200.0, 5) // 200km radius, max 5 events
	if err == nil {
		resp.NearbyEvents = events
	}

	// ⚠️ Marshal BEFORE committing the status. Encoding straight to the
	// ResponseWriter writes the 200 first, so any marshal failure leaves a 200
	// with a truncated body — exactly how the NaN defect manifested.
	//
	// An earlier revision "fixed" that by rejecting non-finite user input and
	// then asserting in a comment that every float here was finite. That claim
	// was FALSE: GeoLookup results and NearbyEvents carry floats this handler
	// never validates, so malformed dependency or database data could still
	// produce a truncated 200. Buffering removes the class, not just the one
	// input that happened to be noticed.
	body, marshalErr := json.Marshal(resp)
	if marshalErr != nil {
		h.log.Error("context: failed to encode response", "err", marshalErr)
		respondWithError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// best-effort — the status is framed, so only the body can be lost and a
	// failure here is network-level (§4.7).
	if _, writeErr := w.Write(body); writeErr != nil {
		h.log.Warn("context: failed to write response body", "err", writeErr)
	}
}
