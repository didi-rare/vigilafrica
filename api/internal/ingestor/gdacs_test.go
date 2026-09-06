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

		b := newGDACSBudget()
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

		b := newGDACSBudget()
		url := "https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1"
		b.resolve(context.Background(), url, ring4)
		spent := maxGDACSCallsPerRun - b.remaining
		b.resolve(context.Background(), url, ring4)
		if got := maxGDACSCallsPerRun - b.remaining; got != spent {
			t.Errorf("a cached failure still spent budget: %d -> %d", spent, got)
		}
	})

	t.Run("an exhausted budget fails closed", func(t *testing.T) {
		b := newGDACSBudget()
		b.remaining = 0
		if _, ok := b.resolve(context.Background(),
			"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=999", ring4); ok {
			t.Error("an exhausted budget must not resolve")
		}
	})
}

func TestBBoxesIntersect(t *testing.T) {
	ng := [4]float64{2.0, 4.0, 15.0, 14.0}

	// ⚠️ The case that a centroid test gets wrong: a polygon straddling the
	// border belongs to the country it reaches, even when its centre does not.
	straddling := [4]float64{14.5, 13.5, 16.0, 15.0}
	if !bboxesIntersect(ng, straddling) {
		t.Error("a polygon overlapping the country box must be kept")
	}
	if withinBBox(ng, 15.25, 14.25) {
		t.Error("precondition: that polygon's centroid is outside the box, which is the point")
	}

	if bboxesIntersect(ng, [4]float64{-84.6, 30.0, -84.5, 30.1}) {
		t.Error("a Florida polygon must not intersect the Nigeria box")
	}
	if !bboxesIntersect(ng, ng) {
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

// TestExtentsCorrespond pins that BOTH orientations are accepted.
//
// The transposed case is what matches today. The direct case is what will match
// if EONET repairs its feed — and accepting it is why this keeps working then,
// where a blanket coordinate swap would silently start corrupting data.
func TestExtentsCorrespond(t *testing.T) {
	eonetTransposed := [4]float64{4.377, 9.216, 4.828, 9.569}
	gdacs := [4]float64{9.2156, 4.377, 9.5688, 4.8284}

	if !extentsCorrespond(eonetTransposed, gdacs) {
		t.Error("the transposed pair must be recognised as the same geometry")
	}
	if !extentsCorrespond(gdacs, gdacs) {
		t.Error("an already-correct feed must also be recognised")
	}
	if extentsCorrespond([4]float64{0, 0, 1, 1}, gdacs) {
		t.Error("unrelated extents must not correspond")
	}
	// Tolerance exists for GDACS's 4-decimal precision, not for sloppiness:
	// ~11m is fine, a whole degree is not.
	if !extentsCorrespond([4]float64{4.3770, 9.2156, 4.8284, 9.5688}, gdacs) {
		t.Error("rounding-level differences must still correspond")
	}
	if extentsCorrespond([4]float64{5.377, 9.216, 5.828, 9.569}, gdacs) {
		t.Error("a one-degree difference must NOT correspond")
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

	b := newGDACSBudget()
	b.deadline = time.Now().Add(-time.Second) // already expired
	before := *calls

	if _, ok := b.resolve(context.Background(),
		"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1", ring4); ok {
		t.Error("an expired run budget must not resolve")
	}
	if *calls != before {
		t.Errorf("an expired budget still made %d network calls", *calls-before)
	}
	if b.remaining != maxGDACSCallsPerRun {
		t.Errorf("an expired budget consumed call allowance: remaining=%d", b.remaining)
	}
}
