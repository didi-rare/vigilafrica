package ingestor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestParseGDACSReferenceRejectsForeignHosts is the SSRF guard.
//
// The source URL arrives inside upstream data, so it is attacker-influenced in
// the same sense any third-party feed is. We never fetch it; we extract two
// validated components and rebuild the request against a constant host.
func TestParseGDACSReferenceRejectsForeignHosts(t *testing.T) {
	rejected := []string{
		"https://evil.example.com/report.aspx?eventtype=FL&eventid=1104078",
		"https://gdacs.org.evil.example.com/report.aspx?eventtype=FL&eventid=1104078",
		"http://www.gdacs.org/report.aspx?eventtype=FL&eventid=1104078", // not https
		"https://www.gdacs.org/report.aspx?eventtype=FL",                // no id
		"https://www.gdacs.org/report.aspx?eventid=1104078",             // no type
		"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=abc",    // non-numeric id
		"https://www.gdacs.org/report.aspx?eventtype=FLOOD&eventid=1",   // not a 2-letter code
		"https://www.gdacs.org/report.aspx?eventtype=../&eventid=1",
		// ⚠️ A bare [A-Z]{2} pattern accepts this. The allowlist must not.
		"https://www.gdacs.org/report.aspx?eventtype=ZZ&eventid=1",
		"",
		"not a url at all",
	}
	for _, u := range rejected {
		if _, ok := parseGDACSReference(u); ok {
			t.Errorf("expected %q to be rejected", u)
		}
	}
}

func TestParseGDACSReferenceAcceptsRealSourceURLs(t *testing.T) {
	cases := map[string]gdacsReference{
		"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1104078": {EventType: "FL", EventID: "1104078"},
		"https://gdacs.org/report.aspx?eventtype=EQ&eventid=42":          {EventType: "EQ", EventID: "42"},
		"https://www.gdacs.org/report.aspx?eventtype=fl&eventid=1104105": {EventType: "FL", EventID: "1104105"},
	}
	for u, want := range cases {
		got, ok := parseGDACSReference(u)
		if !ok {
			t.Fatalf("expected %q to parse", u)
		}
		if got != want {
			t.Errorf("%q -> %+v, want %+v", u, got, want)
		}
	}
}

// TestGDACSClientDoesNotFollowCrossHostRedirects closes the gap between "we
// validated the URL we were given" and "we controlled the host we called".
// Go's client follows redirects by default, so without CheckRedirect the pinned
// origin could be left mid-request and the allowlist would be decorative.
func TestGDACSClientDoesNotFollowCrossHostRedirects(t *testing.T) {
	reachedElsewhere := false
	elsewhere := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		reachedElsewhere = true
	}))
	defer elsewhere.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL, http.StatusFound)
	}))
	defer redirector.Close()
	swapGDACSURLs(t, redirector.URL, redirector.URL)

	if _, ok := ResolveGDACSPolygon(context.Background(),
		"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1", ring5); ok {
		t.Error("a redirected response must not be treated as a successful resolution")
	}
	if reachedElsewhere {
		t.Error("the client followed a cross-host redirect off the pinned origin")
	}
}

// episodeServers builds the two GDACS endpoints. episodes maps episode id to the
// raw coordinates JSON its Poly_Affected feature should carry.
func episodeServers(t *testing.T, count int, episodes map[int]string) (func(), *int) {
	t.Helper()
	calls := 0

	eventData := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		list := strings.Repeat(`{"details":"x"},`, count)
		if count > 0 {
			list = list[:len(list)-1]
		}
		_, _ = w.Write([]byte(`{"properties":{"episodeid":` + itoa(count) + `,"episodes":[` + list + `]}}`))
	}))
	geometry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		ep := r.URL.Query().Get("episodeid")
		coords, ok := episodes[atoi(ep)]
		if !ok {
			_, _ = w.Write([]byte(`{"features":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"features":[{"properties":{"Class":"Poly_Affected","episodeid":` + ep + `},"geometry":{"type":"Polygon","coordinates":` + coords + `}}]}`))
	}))

	swapGDACSURLs(t, eventData.URL, geometry.URL)
	t.Cleanup(func() {
		eventData.Close()
		geometry.Close()
	})
	return func() {}, &calls
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	if s == "" {
		return -1
	}
	return n
}

func TestResolveGDACSPolygon(t *testing.T) {
	// A 4-vertex ring and a 5-vertex ring, both in correct [lon, lat] order.
	const fourVertex = `[[[1.0,2.0],[1.1,2.0],[1.1,2.1],[1.0,2.0]]]`
	const fiveVertex = `[[[9.216,4.377],[9.216,4.828],[9.569,4.828],[9.569,4.377],[9.216,4.377]]]`

	t.Run("matches the episode by vertex count, not the current episode", func(t *testing.T) {
		// ⚠️ The heart of the fix. Episode 3 is "current"; EONET published the
		// 5-vertex ring that belongs to episode 2. Taking the current episode
		// would relocate the flood rather than correct it — for the real 1104105
		// that is a ~700km move into a different part of Nigeria.
		cleanup, _ := episodeServers(t, 3, map[int]string{
			1: fourVertex,
			2: fiveVertex,
			3: fourVertex,
		})
		defer cleanup()

		got, ok := ResolveGDACSPolygon(context.Background(),
			"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1104105", ring5)
		if !ok {
			t.Fatal("expected the 5-vertex episode to be found")
		}
		if got.EpisodeID != 2 {
			t.Errorf("matched episode %d, want 2", got.EpisodeID)
		}
		if got.Vertices != 5 {
			t.Errorf("vertices = %d, want 5", got.Vertices)
		}
		if !strings.Contains(got.GeoJSON, "9.216") {
			t.Errorf("returned the wrong ring: %s", got.GeoJSON)
		}
		// Centroid must sit inside the ring, in Cameroon rather than the Kwara
		// transposition.
		if got.Lon < 9.0 || got.Lon > 10.0 || got.Lat < 4.0 || got.Lat > 5.0 {
			t.Errorf("centroid lon=%v lat=%v is outside the ring", got.Lon, got.Lat)
		}
	})

	t.Run("fails when no episode matches, rather than guessing", func(t *testing.T) {
		cleanup, _ := episodeServers(t, 2, map[int]string{1: fourVertex, 2: fourVertex})
		defer cleanup()

		if _, ok := ResolveGDACSPolygon(context.Background(),
			"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1", ring5); ok {
			t.Error("no episode has 5 vertices; resolution must fail rather than substitute one")
		}
	})

	t.Run("refuses an episode whose coordinates are themselves impossible", func(t *testing.T) {
		// Do not trust a second upstream unconditionally — the whole defect is an
		// upstream that got axis order wrong.
		cleanup, _ := episodeServers(t, 1, map[int]string{
			1: `[[[36.5,136.9],[36.6,136.9],[36.6,137.0],[36.5,136.9]]]`,
		})
		defer cleanup()

		if _, ok := ResolveGDACSPolygon(context.Background(),
			"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1", ring4); ok {
			t.Error("latitude 136.9 is impossible and must be refused")
		}
	})

	t.Run("fails closed on transport and shape problems", func(t *testing.T) {
		for name, handler := range map[string]http.HandlerFunc{
			"500":            func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) },
			"not json":       func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<html>")) },
			"no episodes":    func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"properties":{}}`)) },
			"empty response": func(w http.ResponseWriter, _ *http.Request) {},
		} {
			t.Run(name, func(t *testing.T) {
				srv := httptest.NewServer(handler)
				defer srv.Close()
				swapGDACSURLs(t, srv.URL, srv.URL)

				if _, ok := ResolveGDACSPolygon(context.Background(),
					"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1", ring5); ok {
					t.Errorf("%s: expected failure, got success", name)
				}
			})
		}
	})

	t.Run("a non-GDACS source URL never triggers a request", func(t *testing.T) {
		called := false
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
		defer srv.Close()
		swapGDACSURLs(t, srv.URL, srv.URL)

		if _, ok := ResolveGDACSPolygon(context.Background(), "https://evil.example.com/x?eventtype=FL&eventid=1", ring5); ok {
			t.Error("expected rejection")
		}
		if called {
			t.Error("a foreign source URL must not reach the network at all")
		}
	})

	t.Run("respects context cancellation between episodes", func(t *testing.T) {
		cleanup, _ := episodeServers(t, 5, map[int]string{5: fiveVertex})
		defer cleanup()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, ok := ResolveGDACSPolygon(ctx,
			"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1", ring5); ok {
			t.Error("a cancelled context must not resolve")
		}
	})
}

// TestGDACSBudgetCachesAndBounds pins the two properties that stop a slow
// upstream turning one run into an unbounded one.
func TestGDACSBudgetCachesAndBounds(t *testing.T) {
	const ring = `[[[1.0,2.0],[1.1,2.0],[1.1,2.1],[1.0,2.0]]]`

	t.Run("a repeated lookup is served from cache", func(t *testing.T) {
		cleanup, calls := episodeServers(t, 1, map[int]string{1: ring})
		defer cleanup()

		b := NewRunBudget()
		url := "https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1"
		if _, ok := b.resolve(context.Background(), url, ring4); !ok {
			t.Fatal("first resolve should succeed")
		}
		after := *calls
		if _, ok := b.resolve(context.Background(), url, ring4); !ok {
			t.Fatal("second resolve should succeed from cache")
		}
		if *calls != after {
			t.Errorf("cache miss: calls went %d -> %d", after, *calls)
		}
	})

	t.Run("a failure is cached too, so a dead upstream is asked once", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(500)
		}))
		defer srv.Close()
		swapGDACSURLs(t, srv.URL, srv.URL)

		b := NewRunBudget()
		url := "https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1"
		b.resolve(context.Background(), url, ring4)
		spent := maxGDACSRequestsPerRun - b.requestsLeft
		b.resolve(context.Background(), url, ring4)
		if got := maxGDACSRequestsPerRun - b.requestsLeft; got != spent {
			t.Errorf("a cached failure still spent budget: %d -> %d", spent, got)
		}
	})

	t.Run("an exhausted budget fails closed", func(t *testing.T) {
		b := NewRunBudget()
		b.requestsLeft = 0
		if _, ok := b.resolve(context.Background(),
			"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=999", ring4); ok {
			t.Error("an exhausted budget must not resolve")
		}
	})
}

func TestEnvelopesOverlap(t *testing.T) {
	ng := [4]float64{2.0, 4.0, 15.0, 14.0}

	// ⚠️ The case that a centroid test gets wrong: a polygon straddling the
	// border belongs to the country it reaches, even when its centre does not.
	straddling := [4]float64{14.5, 13.5, 16.0, 15.0}
	if !envelopesOverlap(ng, straddling) {
		t.Error("a polygon overlapping the country box must be kept")
	}
	if withinBBox(ng, 15.25, 14.25) {
		t.Error("precondition: that polygon's centroid is outside the box, which is the point")
	}

	if envelopesOverlap(ng, [4]float64{-84.6, 30.0, -84.5, 30.1}) {
		t.Error("a Florida polygon must not intersect the Nigeria box")
	}
	if !envelopesOverlap(ng, ng) {
		t.Error("a box intersects itself")
	}
}

// swapGDACSURLs points both GDACS endpoints at test servers for the duration of
// the test. Two URLs because episode matching needs the event endpoint (for the
// episode count) and the geometry endpoint (for each episode's polygon).
//
// Registered with t.Cleanup rather than returning a restore func for the caller
// to defer (developers-go.md §9.10): cleanup that the test cannot forget to run.
func swapGDACSURLs(t *testing.T, eventData, geometry string) {
	t.Helper()
	prevEvent, prevGeom := gdacsEventDataURL, gdacsGeometryURL
	gdacsEventDataURL, gdacsGeometryURL = eventData, geometry
	t.Cleanup(func() { gdacsEventDataURL, gdacsGeometryURL = prevEvent, prevGeom })
}

// Shared EONET-side fixtures. ring5 is the transposed form of the corrected
// 5-vertex ring used by the episode tests; ring4 matches fourVertex.
var (
	ring4 = [][2]float64{{2.0, 1.0}, {2.0, 1.1}, {2.1, 1.1}, {2.0, 1.0}}
	ring5 = [][2]float64{{4.377, 9.216}, {4.828, 9.216}, {4.828, 9.569}, {4.377, 9.569}, {4.377, 9.216}}
)

// TestResolveGDACSPolygonRejectsCoincidentalVertexMatch closes the gap Sol
// identified: a matching vertex count is not proof of the same footprint.
//
// Here the only episode has the right number of vertices but sits somewhere else
// entirely. Substituting it would relocate the disaster — the precise failure
// that taking the event-level centroid would have caused for 1104105.
func TestResolveGDACSPolygonRejectsCoincidentalVertexMatch(t *testing.T) {
	cleanup, _ := episodeServers(t, 1, map[int]string{
		1: `[[[100.0,20.0],[100.1,20.0],[100.1,20.1],[100.0,20.1],[100.0,20.0]]]`,
	})
	defer cleanup()

	if _, ok := ResolveGDACSPolygon(context.Background(),
		"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1", ring5); ok {
		t.Error("an unrelated polygon with the same vertex count must be refused")
	}
}

// TestPositionsCorrespond pins geometry identity, not envelope similarity.
func TestPositionsCorrespond(t *testing.T) {
	eonet := [][2]float64{{4.377, 9.216}, {4.828, 9.216}, {4.828, 9.569}, {4.377, 9.569}, {4.377, 9.216}}
	gdacsSwapped := [][2]float64{{9.216, 4.377}, {9.216, 4.828}, {9.569, 4.828}, {9.569, 4.377}, {9.216, 4.377}}

	t.Run("recognises the transposed publication", func(t *testing.T) {
		transposed, ok := positionsCorrespond(eonet, gdacsSwapped)
		if !ok || !transposed {
			t.Errorf("expected a transposed match, got transposed=%v ok=%v", transposed, ok)
		}
	})

	t.Run("recognises an already-correct feed", func(t *testing.T) {
		// What a repaired EONET would look like. Accepting this is why the fix
		// survives upstream being corrected, where a blanket swap would not.
		transposed, ok := positionsCorrespond(eonet, eonet)
		if !ok || transposed {
			t.Errorf("expected a direct match, got transposed=%v ok=%v", transposed, ok)
		}
	})

	// ⚠️ THE CASE BOUNDING BOXES GET WRONG. Same vertex count, same envelope,
	// different shape — the previous extent-only check accepted this, which is how
	// a different episode could still be substituted.
	t.Run("rejects a different shape with an identical envelope", func(t *testing.T) {
		sameBoxDifferentShape := [][2]float64{
			{9.216, 4.377}, {9.569, 4.377}, {9.216, 4.828}, {9.569, 4.828}, {9.216, 4.377},
		}
		if _, ok := positionsCorrespond(eonet, sameBoxDifferentShape); ok {
			t.Error("an envelope match is not a geometry match and must be refused")
		}
	})

	t.Run("rejects differing lengths and empty input", func(t *testing.T) {
		if _, ok := positionsCorrespond(eonet, gdacsSwapped[:3]); ok {
			t.Error("different vertex counts must not correspond")
		}
		if _, ok := positionsCorrespond(nil, gdacsSwapped); ok {
			t.Error("empty input must not correspond")
		}
	})

	t.Run("tolerance absorbs GDACS rounding but not a real difference", func(t *testing.T) {
		// GDACS publishes 4dp; observed production deltas were ~5e-5.
		rounded := make([][2]float64, len(gdacsSwapped))
		for i, c := range gdacsSwapped {
			rounded[i] = [2]float64{c[0] + 5e-5, c[1] - 5e-5}
		}
		if _, ok := positionsCorrespond(eonet, rounded); !ok {
			t.Error("4-decimal rounding must still correspond")
		}

		shifted := make([][2]float64, len(gdacsSwapped))
		for i, c := range gdacsSwapped {
			shifted[i] = [2]float64{c[0] + 0.01, c[1]}
		}
		if _, ok := positionsCorrespond(eonet, shifted); ok {
			t.Error("a ~1km shift must NOT correspond")
		}
	})
}

// TestRepresentativePointIsInsideConcavePolygon pins the property the arithmetic
// mean did not have.
func TestRepresentativePointIsInsideConcavePolygon(t *testing.T) {
	// A "U" shape. The mean of its boundary vertices falls in the gap between the
	// arms — outside the polygon — which would put the map marker where the flood
	// is not.
	u := [][2]float64{
		{0, 0}, {3, 0}, {3, 3}, {2, 3}, {2, 1}, {1, 1}, {1, 3}, {0, 3}, {0, 0},
	}
	lon, lat := representativePoint(u)
	if !pointInPolygon(lon, lat, u) {
		t.Errorf("representative point (%v, %v) is outside its own polygon", lon, lat)
	}

	// Demonstrate the defect it replaces, so the test documents WHY.
	var mlon, mlat float64
	for _, c := range u {
		mlon += c[0]
		mlat += c[1]
	}
	mlon /= float64(len(u))
	mlat /= float64(len(u))
	if pointInPolygon(mlon, mlat, u) {
		t.Log("note: the arithmetic mean happens to be inside for this shape")
	}
}

// TestGDACSBudgetRespectsWallClockDeadline pins the control that protects the
// scheduler lock. A call cap alone permits ~13 minutes of work against a
// 5-minute lease; the wall-clock budget is what keeps the added work inside the
// existing headroom.
func TestGDACSBudgetRespectsWallClockDeadline(t *testing.T) {
	cleanup, calls := episodeServers(t, 1, map[int]string{
		1: `[[[2.0,1.0],[2.0,1.1],[2.1,1.1],[2.0,1.0]]]`,
	})
	defer cleanup()

	b := NewRunBudget()
	b.deadline = time.Now().Add(-time.Second) // already expired
	before := *calls

	if _, ok := b.resolve(context.Background(),
		"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1", ring4); ok {
		t.Error("an expired run budget must not resolve")
	}
	if *calls != before {
		t.Errorf("an expired budget still made %d network calls", *calls-before)
	}
	if b.requestsLeft != maxGDACSRequestsPerRun {
		t.Errorf("an expired budget consumed request allowance: remaining=%d", b.requestsLeft)
	}
}

// pointInPolygon is a ray-cast containment test, used only to verify that the
// representative point really lies inside its polygon.
func pointInPolygon(x, y float64, poly [][2]float64) bool {
	inside := false
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		xi, yi := poly[i][0], poly[i][1]
		xj, yj := poly[j][0], poly[j][1]
		if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
			inside = !inside
		}
	}
	return inside
}

// TestRepresentativePointWithFlattenedHoles records a claim that was CHECKED and
// did not hold.
//
// Round-3 review asserted that flattening an exterior ring together with a hole
// creates an artificial connector edge whose crossings make the widest interval
// the hole itself, returning a marker off the surface. Run against the real
// function, five hole configurations — including the exact one cited — all return
// on-surface points, because the connector edges add crossings in pairs and the
// even-odd parity survives.
//
// ⚠️ This does NOT prove the function is correct for every possible ring; it
// records that the specific counterexample is wrong. Both real GDACS
// Poly_Affected polygons are single-ring (1311 and 28 vertices), so holes do not
// arise in the data we actually ingest.
func TestRepresentativePointWithFlattenedHoles(t *testing.T) {
	outer := [][2]float64{{-10, -10}, {10, -10}, {10, 10}, {-10, 10}, {-10, -10}}
	cases := map[string][][2]float64{
		"hole starting (-9,1)":  {{-9, 1}, {-9, -9}, {9, -9}, {9, 9}, {-9, 9}, {-9, 1}},
		"hole starting (-9,-9)": {{-9, -9}, {9, -9}, {9, 9}, {-9, 9}, {-9, -9}},
		"hole starting (9,9)":   {{9, 9}, {-9, 9}, {-9, -9}, {9, -9}, {9, 9}},
		"hole starting (9,-9)":  {{9, -9}, {9, 9}, {-9, 9}, {-9, -9}, {9, -9}},
	}
	for name, hole := range cases {
		t.Run(name, func(t *testing.T) {
			flat := append(append([][2]float64{}, outer...), hole...)
			lon, lat := representativePoint(flat)
			if lon > -9 && lon < 9 && lat > -9 && lat < 9 {
				t.Errorf("marker (%.3f, %.3f) landed in the hole", lon, lat)
			}
		})
	}
}

// TestResolveGDACSPolygonDeduplicatesIdenticalEpisodes covers a round-3 finding:
// "exactly one" counted matching FEATURE RECORDS, so GDACS serving the same ring
// under two episodes reported ambiguity where there was none, and refused to
// resolve an event whose geometry was perfectly clear.
func TestResolveGDACSPolygonDeduplicatesIdenticalEpisodes(t *testing.T) {
	const sameRing = `[[[9.216,4.377],[9.216,4.828],[9.569,4.828],[9.569,4.377],[9.216,4.377]]]`

	t.Run("the same ring under two episodes resolves, not ambiguous", func(t *testing.T) {
		cleanup, _ := episodeServers(t, 2, map[int]string{1: sameRing, 2: sameRing})
		defer cleanup()

		got, ok := ResolveGDACSPolygon(context.Background(),
			"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1", ring5)
		if !ok {
			t.Fatal("identical geometry under two episodes is not ambiguous and must resolve")
		}
		// Deterministic: lowest episode id wins, not map iteration order.
		if got.EpisodeID != 1 {
			t.Errorf("episode = %d, want the lowest (1)", got.EpisodeID)
		}
	})

	t.Run("two DIFFERENT corresponding rings are still refused", func(t *testing.T) {
		// Genuine ambiguity: both correspond to the EONET ring within tolerance but
		// are not the same geometry. Picking either would be a guess.
		const shifted = `[[[9.2161,4.3771],[9.2161,4.8281],[9.5691,4.8281],[9.5691,4.3771],[9.2161,4.3771]]]`
		cleanup, _ := episodeServers(t, 2, map[int]string{1: sameRing, 2: shifted})
		defer cleanup()

		if _, ok := ResolveGDACSPolygon(context.Background(),
			"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1", ring5); ok {
			t.Error("two distinct corresponding geometries must be refused, not guessed between")
		}
	})
}

// TestRunBudgetCountsHTTPRequestsNotResolutions covers a round-3 finding: the cap
// counted resolutions, but one resolution issues one event request plus up to
// maxGDACSEpisodes geometry requests — so a nominal 40 permitted ~840.
func TestRunBudgetCountsHTTPRequestsNotResolutions(t *testing.T) {
	cleanup, calls := episodeServers(t, 5, map[int]string{})
	defer cleanup()

	b := NewRunBudget()
	b.requestsLeft = 3 // event request + 2 episode requests, then exhausted

	b.resolve(context.Background(), "https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1", ring5)

	if *calls != 3 {
		t.Errorf("made %d HTTP requests, want exactly the 3 the budget allowed", *calls)
	}
	if b.requestsLeft != 0 {
		t.Errorf("requestsLeft = %d, want 0", b.requestsLeft)
	}
}
