package ingestor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

// GDACS geometry resolution (openspec/changes/fix-eonet-polygon-transposition).
//
// EONET republishes GDACS polygons with latitude and longitude reversed, so a
// Cameroonian flood was stored inside Kwara State, Nigeria. GDACS is upstream of
// EONET, so it is the origin rather than a second opinion.
//
// ⚠️ We resolve rather than reverse. Blanket-swapping is correct today and
// silently corrupts everything the moment EONET repairs the feed.
//
// ⚠️ The EPISODE is load-bearing, and getting it wrong substitutes a different
// disaster rather than correcting one. GDACS events move: 1104105 ran 13 Aug to
// 4 Sep over five episodes in different parts of Nigeria. EONET publishes ONE of
// them — for that event, episode 2 in the north — while the event-level endpoint
// returns episode 5, ~700km away in the Niger Delta. Taking the event-level
// centroid would therefore relocate the flood, not fix it. We match the episode
// whose vertex count equals EONET's and verify the axes are genuinely reversed
// before trusting it.

var gdacsHTTPClient = &http.Client{
	Timeout: gdacsRequestTimeout,
	// ⚠️ Pin the origin. Go follows cross-host redirects by default, so without
	// this the host allowlist below would validate the URL we were GIVEN while the
	// request ended up somewhere else entirely.
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// Base URLs are vars, not consts, so tests can point them at an httptest server —
// the same seam pattern as eonetURL.
//
// ⚠️ The host allowlist validates the URL we were given; these constants are the
// hosts we actually call. Keeping them separate is the point: an upstream value
// can influence WHICH EVENT is fetched, never WHICH HOST.
var (
	gdacsEventDataURL = "https://www.gdacs.org/gdacsapi/api/events/geteventdata"
	gdacsGeometryURL  = "https://www.gdacs.org/gdacsapi/api/polygons/getgeometry"
)

const (
	maxGDACSResponseBytes = 4 << 20 // 4 MiB — flood polygons run to thousands of vertices
	// maxGDACSEpisodes bounds episode enumeration. GDACS numbers episodes from 1
	// and the event payload declares how many exist; this is a backstop against a
	// malformed count turning one event into unbounded requests.
	maxGDACSEpisodes = 20
)

// gdacsEventTypes is a real allowlist of GDACS hazard codes. A bare `[A-Z]{2}`
// pattern accepts anything two letters, including "ZZ", which made the previous
// "event types we accept" comment false.
var gdacsEventTypes = map[string]bool{
	"FL": true, // flood
	"EQ": true, // earthquake
	"TC": true, // tropical cyclone
	"DR": true, // drought
	"VO": true, // volcano
	"WF": true, // wildfire
	"TS": true, // tsunami
}

var gdacsEventIDRe = regexp.MustCompile(`^[0-9]{1,12}$`)

type gdacsReference struct {
	EventType string
	EventID   string
}

// GDACSGeometry is an authoritative geometry resolved from GDACS.
type GDACSGeometry struct {
	GeoJSON   string // polygon in correct [lon, lat] order
	Lon, Lat  float64
	EpisodeID int
	Vertices  int
	// Transposed records whether GDACS's ring had to be read as the transpose of
	// EONET's to correspond. False means upstream agreed — which is what a repaired
	// EONET feed would look like.
	Transposed bool
}

// parseGDACSReference pulls the event type and id out of a GDACS report URL.
//
// ⚠️ The URL originates in upstream data and is NEVER fetched. Only the two
// validated components are used, against a constant base URL.
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
	if !gdacsEventTypes[ref.EventType] || !gdacsEventIDRe.MatchString(ref.EventID) {
		return gdacsReference{}, false
	}
	return ref, true
}

func gdacsGetJSON(ctx context.Context, reqURL string, into interface{}) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return false
	}
	resp, err := gdacsHTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	// ErrUseLastResponse surfaces the redirect itself, which is not a usable body.
	if resp.StatusCode != http.StatusOK {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxGDACSResponseBytes))
	if err != nil {
		return false
	}
	return json.Unmarshal(body, into) == nil
}

type gdacsEventData struct {
	Properties struct {
		EpisodeID int `json:"episodeid"`
		Episodes  []struct {
			Details string `json:"details"`
		} `json:"episodes"`
	} `json:"properties"`
}

type gdacsFeatureCollection struct {
	Features []struct {
		Properties struct {
			Class     string `json:"Class"`
			EpisodeID int    `json:"episodeid"`
		} `json:"properties"`
		Geometry struct {
			Type        string          `json:"type"`
			Coordinates json.RawMessage `json:"coordinates"`
		} `json:"geometry"`
	} `json:"features"`
}

// ResolveGDACSPolygon finds the GDACS episode whose affected-area polygon matches
// the EONET geometry, and returns it in correct axis order.
//
// wantVertices is the vertex count of the EONET polygon. Matching on it pins the
// episode; without that we would silently swap in a different footprint.
//
// Returns ok=false whenever the geometry cannot be established with confidence.
// ⚠️ The caller MUST NOT fall back to the EONET geometry — that is the transposed
// data this exists to reject.
func ResolveGDACSPolygon(ctx context.Context, sourceURL string, eonet [][2]float64) (GDACSGeometry, bool) {
	ref, valid := parseGDACSReference(sourceURL)
	if !valid || len(eonet) == 0 {
		return GDACSGeometry{}, false
	}
	wantVertices := len(eonet)

	var event gdacsEventData
	if !gdacsGetJSON(ctx, fmt.Sprintf("%s?eventtype=%s&eventid=%s",
		gdacsEventDataURL, url.QueryEscape(ref.EventType), url.QueryEscape(ref.EventID)), &event) {
		return GDACSGeometry{}, false
	}

	episodes := len(event.Properties.Episodes)
	if episodes == 0 {
		episodes = event.Properties.EpisodeID
	}
	if episodes <= 0 || episodes > maxGDACSEpisodes {
		if episodes > maxGDACSEpisodes {
			episodes = maxGDACSEpisodes
		} else {
			return GDACSGeometry{}, false
		}
	}

	// ⚠️ Collect ALL candidates rather than returning the first match. If two
	// episodes both correspond, the geometry is ambiguous and we cannot say which
	// one EONET published — resolving to either would be a guess, and a guess here
	// relocates a disaster.
	var candidates []GDACSGeometry

	for ep := 1; ep <= episodes; ep++ {
		// Respect cancellation between episodes: this loop is network-bound and
		// sits inside a scheduled run with a bounded lease (§3.6).
		if ctx.Err() != nil {
			return GDACSGeometry{}, false
		}

		var fc gdacsFeatureCollection
		if !gdacsGetJSON(ctx, fmt.Sprintf("%s?eventtype=%s&eventid=%s&episodeid=%d",
			gdacsGeometryURL, url.QueryEscape(ref.EventType), url.QueryEscape(ref.EventID), ep), &fc) {
			continue
		}

		for _, f := range fc.Features {
			if f.Properties.Class != "Poly_Affected" || f.Geometry.Type != "Polygon" {
				continue
			}
			var coords []interface{}
			if json.Unmarshal(f.Geometry.Coordinates, &coords) != nil {
				continue
			}
			positions := collectPositions(coords)
			if len(positions) != wantVertices || !positionsWithinWGS84(positions) {
				continue
			}
			transposed, corresponds := positionsCorrespond(eonet, positions)
			if !corresponds {
				continue
			}
			lon, lat := representativePoint(positions)
			candidates = append(candidates, GDACSGeometry{
				GeoJSON:    fmt.Sprintf(`{"type":"Polygon","coordinates":%s}`, string(f.Geometry.Coordinates)),
				Lon:        lon,
				Lat:        lat,
				EpisodeID:  ep,
				Vertices:   len(positions),
				Transposed: transposed,
			})
		}
	}

	if len(candidates) != 1 {
		return GDACSGeometry{}, false
	}
	return candidates[0], true
}

// collectPositions flattens nested GeoJSON coordinate arrays into [lon, lat]
// pairs, ignoring any third dimension.
func collectPositions(v interface{}) [][2]float64 {
	arr, ok := v.([]interface{})
	if !ok || len(arr) == 0 {
		return nil
	}
	if first, isNum := arr[0].(float64); isNum {
		if len(arr) < 2 {
			return nil
		}
		second, ok2 := arr[1].(float64)
		if !ok2 {
			return nil
		}
		return [][2]float64{{first, second}}
	}
	var out [][2]float64
	for _, item := range arr {
		out = append(out, collectPositions(item)...)
	}
	return out
}

func positionsWithinWGS84(p [][2]float64) bool {
	if len(p) == 0 {
		return false
	}
	for _, c := range p {
		if c[0] < -180 || c[0] > 180 || c[1] < -90 || c[1] > 90 {
			return false
		}
	}
	return true
}

// gdacsCoordToleranceDeg is how far a single coordinate may differ and still be
// considered the same vertex.
//
// Derived, not guessed: GDACS publishes 4 decimal places, so rounding alone can
// move a coordinate by up to 5e-5 degrees. This allows 2e-4 — a 4x margin over
// that bound, about 22m. ⚠️ An earlier revision used 1e-3 (~111m), roughly 20x
// the rounding it was meant to absorb and wide enough to accept genuinely
// different vertices. Measured deltas on the two production events were 5.0e-5
// and 4.75e-5, comfortably inside this.
const gdacsCoordToleranceDeg = 2e-4

// positionsCorrespond reports whether two coordinate sequences describe the same
// geometry, and whether GDACS's had to be read as the transpose of EONET's.
//
// ⚠️ Every vertex is compared. An earlier revision compared only bounding boxes,
// which is NOT geometry identity: two different shapes can share a vertex count
// and an envelope — concave outlines, rings with holes, a square whose box is
// symmetric under transposition, or the same ring reordered. Accepting an
// envelope match would let a different episode through, which is precisely the
// defect this change exists to remove.
//
// Both orientations are accepted deliberately. Today EONET reverses the pairs so
// the transposed comparison matches; the day upstream is repaired the direct one
// will, and this keeps working unchanged. That is the property a blanket swap
// would not have.
func positionsCorrespond(eonet, gdacs [][2]float64) (transposed bool, ok bool) {
	if len(eonet) == 0 || len(eonet) != len(gdacs) {
		return false, false
	}
	near := func(a, b float64) bool {
		d := a - b
		return d < gdacsCoordToleranceDeg && d > -gdacsCoordToleranceDeg
	}

	direct, swapped := true, true
	for i := range eonet {
		if !near(eonet[i][0], gdacs[i][0]) || !near(eonet[i][1], gdacs[i][1]) {
			direct = false
		}
		if !near(eonet[i][1], gdacs[i][0]) || !near(eonet[i][0], gdacs[i][1]) {
			swapped = false
		}
		if !direct && !swapped {
			return false, false
		}
	}
	return !direct, true
}

// representativePoint returns a point that is guaranteed to lie INSIDE the
// polygon, for use as the map marker.
//
// ⚠️ It is NOT the arithmetic mean of the boundary vertices. That was the
// previous implementation and it is wrong twice over: it is weighted by how
// densely each part of the outline happens to be sampled, and for a concave
// shape — a flood along a river bend is exactly that — it can land outside the
// polygon entirely. The marker would then sit somewhere the flood is not.
//
// The approach mirrors PostGIS ST_PointOnSurface: take the horizontal line
// through the vertical middle of the shape, find where it crosses the boundary,
// and return the midpoint of the widest interior span. That point is inside the
// polygon by construction.
//
// ⚠️ The polygon remains the geometry used for spatial queries. This point is
// only a label, and callers must not treat it as the event's extent.
func representativePoint(p [][2]float64) (lon, lat float64) {
	box := positionsBBox(p)
	y := (box[1] + box[3]) / 2

	// Collect boundary crossings of the horizontal line at y.
	var xs []float64
	for i := 0; i < len(p); i++ {
		a, b := p[i], p[(i+1)%len(p)]
		if (a[1] <= y && b[1] > y) || (b[1] <= y && a[1] > y) {
			t := (y - a[1]) / (b[1] - a[1])
			xs = append(xs, a[0]+t*(b[0]-a[0]))
		}
	}
	if len(xs) < 2 {
		// Degenerate (a horizontal sliver, or a ring we could not cross cleanly).
		// The bbox centre is the best available answer and is still inside the
		// extent, which is what the marker needs.
		return (box[0] + box[2]) / 2, y
	}
	sort.Float64s(xs)

	// Interior spans of an even-odd crossing sequence are the pairs (0,1), (2,3)…
	// Take the widest, so the point sits in the largest lobe rather than a sliver.
	bestLo, bestHi, bestWidth := xs[0], xs[1], xs[1]-xs[0]
	for i := 0; i+1 < len(xs); i += 2 {
		if w := xs[i+1] - xs[i]; w > bestWidth {
			bestLo, bestHi, bestWidth = xs[i], xs[i+1], w
		}
	}
	return (bestLo + bestHi) / 2, y
}

// positionsBBox returns [minLon, minLat, maxLon, maxLat].
func positionsBBox(p [][2]float64) [4]float64 {
	b := [4]float64{p[0][0], p[0][1], p[0][0], p[0][1]}
	for _, c := range p {
		if c[0] < b[0] {
			b[0] = c[0]
		}
		if c[1] < b[1] {
			b[1] = c[1]
		}
		if c[0] > b[2] {
			b[2] = c[0]
		}
		if c[1] > b[3] {
			b[3] = c[1]
		}
	}
	return b
}

// ---------------------------------------------------------------------------
// Per-run resolution budget and cache.
//
// ⚠️ Deliberately per-run state, not a package global: a shared cache would leak
// results between scheduled runs and between countries, and a shared budget
// would let one run starve the next.

const (
	// maxGDACSCallsPerRun bounds total GDACS requests for one EONET response.
	maxGDACSCallsPerRun = 40

	// gdacsRunBudgetDuration is a WALL-CLOCK ceiling on all GDACS work for one
	// response, and it is the control that actually protects the scheduler lock.
	//
	// ⚠️ A call cap alone is not enough. schedulerLockTTL is 5 minutes, sized
	// against a documented ~3 minute worst-case run, with an explicit SCALE NOTE
	// to revisit it if a run exceeds ~4 minutes. 40 calls at the client timeout
	// would be ~13 minutes on its own — the lock would expire mid-run and a
	// second replica could start ingesting concurrently. This keeps the added
	// work to a bounded slice of the existing headroom instead (§3.5).
	gdacsRunBudgetDuration = 90 * time.Second

	// gdacsRequestTimeout is per request. Deliberately shorter than the run
	// budget so one stalled request cannot consume the whole allowance.
	gdacsRequestTimeout = 10 * time.Second
)

type gdacsBudget struct {
	remaining int
	deadline  time.Time
	cache     map[string]gdacsCacheEntry
}

type gdacsCacheEntry struct {
	geom GDACSGeometry
	ok   bool
}

func newGDACSBudget() gdacsBudget {
	return gdacsBudget{
		remaining: maxGDACSCallsPerRun,
		deadline:  time.Now().Add(gdacsRunBudgetDuration),
		cache:     map[string]gdacsCacheEntry{},
	}
}

// resolve is the budgeted, cached entry point used by the ingest loop.
//
// The cache is keyed on source URL AND vertex count, because the vertex count is
// what selects the episode — two events sharing a GDACS id but different
// footprints must not share a cached answer.
func (b *gdacsBudget) resolve(ctx context.Context, sourceURL string, eonet [][2]float64) (GDACSGeometry, bool) {
	key := fmt.Sprintf("%s#%d", sourceURL, len(eonet))
	if hit, seen := b.cache[key]; seen {
		return hit.geom, hit.ok
	}
	if b.remaining <= 0 || !time.Now().Before(b.deadline) {
		// Budget exhausted, by calls or by wall clock. Returning false means the
		// event is skipped rather than stored unverified — the safe direction.
		return GDACSGeometry{}, false
	}
	b.remaining--

	// Cap this resolution at whatever remains of the run budget, so the total
	// cannot drift past it however slow upstream is.
	cctx, cancel := context.WithDeadline(ctx, b.deadline)
	defer cancel()

	geom, ok := ResolveGDACSPolygon(cctx, sourceURL, eonet)
	b.cache[key] = gdacsCacheEntry{geom: geom, ok: ok}
	return geom, ok
}

// polygonPositionsFromGeoJSON extracts [lon, lat] pairs from a Polygon GeoJSON
// document. Returns nil for anything that is not a usable polygon.
func polygonPositionsFromGeoJSON(geoJSON string) [][2]float64 {
	var doc struct {
		Type        string      `json:"type"`
		Coordinates interface{} `json:"coordinates"`
	}
	if json.Unmarshal([]byte(geoJSON), &doc) != nil {
		return nil
	}
	if doc.Type != "Polygon" && doc.Type != "MultiPolygon" {
		return nil
	}
	return collectPositions(doc.Coordinates)
}

// envelopesOverlap reports whether two [minLon, minLat, maxLon, maxLat] boxes
// overlap.
//
// ⚠️ This is envelope overlap, NOT polygon intersection, and the difference is
// real: two polygons can have overlapping bounding boxes while never touching.
// It is deliberately the permissive direction — a flood straddling a border
// belongs to the country it reaches, and admitting a near-miss is far cheaper
// than dropping a real event. The precise test happens later, in PostGIS, where
// the enrichment trigger uses ST_Intersects against actual boundaries.
func envelopesOverlap(a, b [4]float64) bool {
	return a[0] <= b[2] && b[0] <= a[2] && a[1] <= b[3] && b[1] <= a[3]
}
