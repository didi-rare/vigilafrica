package ingestor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// GDACS geometry resolution (openspec/changes/fix-eonet-polygon-transposition).
//
// EONET republishes GDACS polygons with latitude and longitude reversed. Verified
// against GDACS for both affected production events: the vertex counts match
// exactly (1311 and 28) and only the axis order differs. GDACS is upstream of
// EONET, so it is the origin rather than a second opinion.
//
// ⚠️ We resolve rather than reverse. Blanket-swapping every polygon is correct
// today and silently corrupts everything the moment EONET repairs the feed;
// asking GDACS keeps working either way.

var gdacsHTTPClient = &http.Client{Timeout: 20 * time.Second}

// gdacsEventDataURL is the GDACS event endpoint. A var, not a const, so tests
// can point it at an httptest server — the same seam pattern as eonetURL.
//
// ⚠️ The host allowlist in parseGDACSReference validates the URL we were GIVEN;
// this constant is the URL we ACTUALLY call. Keeping them separate is the point:
// an upstream value can influence which event is fetched, never which host.
var gdacsEventDataURL = "https://www.gdacs.org/gdacsapi/api/events/geteventdata"

// gdacsEventTypes are the GDACS hazard codes we accept. Restricting the set keeps
// a hostile upstream value out of the URL we build.
var gdacsEventTypeRe = regexp.MustCompile(`^[A-Z]{2}$`)
var gdacsEventIDRe = regexp.MustCompile(`^[0-9]{1,12}$`)

// gdacsReference identifies a GDACS event extracted from a source URL.
type gdacsReference struct {
	EventType string
	EventID   string
}

// parseGDACSReference pulls the event type and id out of a GDACS report URL.
//
// ⚠️ The URL originates in upstream data, so it is never fetched directly. Only
// the two validated components are used, and the request URL is rebuilt from a
// constant base — an upstream value can therefore never redirect the request
// somewhere else.
func parseGDACSReference(sourceURL string) (gdacsReference, bool) {
	parsed, err := url.Parse(strings.TrimSpace(sourceURL))
	if err != nil || parsed.Scheme != "https" {
		return gdacsReference{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "gdacs.org" && !strings.HasSuffix(host, ".gdacs.org") {
		return gdacsReference{}, false
	}

	q := parsed.Query()
	ref := gdacsReference{
		EventType: strings.ToUpper(strings.TrimSpace(q.Get("eventtype"))),
		EventID:   strings.TrimSpace(q.Get("eventid")),
	}
	if !gdacsEventTypeRe.MatchString(ref.EventType) || !gdacsEventIDRe.MatchString(ref.EventID) {
		return gdacsReference{}, false
	}
	return ref, true
}

// gdacsEventData is the subset of the GDACS event payload we rely on. The
// geometry is a Point that GDACS labels "Centroid".
type gdacsEventData struct {
	Geometry struct {
		Type        string    `json:"type"`
		Coordinates []float64 `json:"coordinates"`
	} `json:"geometry"`
}

// resolveGDACSCentroid returns the authoritative [lon, lat] centroid GDACS holds
// for the event named by sourceURL.
//
// Returns ok=false whenever the point cannot be established with confidence. The
// caller must NOT fall back to the EONET geometry in that case — doing so would
// reinstate the transposed coordinates under a network blip.
func resolveGDACSCentroid(ctx context.Context, sourceURL string) (lon, lat float64, ok bool) {
	ref, valid := parseGDACSReference(sourceURL)
	if !valid {
		return 0, 0, false
	}

	reqURL := fmt.Sprintf(
		"%s?eventtype=%s&eventid=%s",
		gdacsEventDataURL, url.QueryEscape(ref.EventType), url.QueryEscape(ref.EventID),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, 0, false
	}

	resp, err := gdacsHTTPClient.Do(req)
	if err != nil {
		return 0, 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, 0, false
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxGDACSResponseBytes))
	if err != nil {
		return 0, 0, false
	}

	var data gdacsEventData
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, 0, false
	}
	if data.Geometry.Type != "Point" || len(data.Geometry.Coordinates) != 2 {
		return 0, 0, false
	}

	lon, lat = data.Geometry.Coordinates[0], data.Geometry.Coordinates[1]

	// GDACS publishes GeoJSON order, verified against its own polygon and country
	// fields. Validate anyway: the whole reason this code exists is an upstream
	// that got the order wrong, so trusting a second upstream unconditionally
	// would repeat the mistake.
	if lon < -180 || lon > 180 || lat < -90 || lat > 90 {
		return 0, 0, false
	}
	return lon, lat, true
}

const maxGDACSResponseBytes = 2 << 20 // 2 MiB
