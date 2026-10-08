package ingestor

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vigilafrica/api/internal/models"
)

// Tests for fix-gdacs-degraded-run-status.
//
// The whole change rests on ONE distinction: a GDACS outage (upstream) must
// degrade the run, while a refusal (GDACS answered, nothing matched) must not.
// Getting it wrong in either direction is a real failure — treat refusals as
// outages and one unmatchable flood keeps the system degraded forever; treat
// outages as refusals and nothing changes from the silent behaviour this fixes.

const fourVertexRing = `[[[1.0,2.0],[1.1,2.0],[1.1,2.1],[1.0,2.0]]]`

const testSourceURL = "https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1"

// gdacsStub points both GDACS endpoints at handlers built by the test.
func gdacsStub(t *testing.T, eventData, geometry http.HandlerFunc) {
	t.Helper()
	ev := httptest.NewServer(eventData)
	geo := httptest.NewServer(geometry)
	swapGDACSURLs(t, ev.URL, geo.URL)
	t.Cleanup(func() { ev.Close(); geo.Close() })
}

func status(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) }
}

func body(s string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(s)) }
}

// manyEpisodes: event data declaring n episodes.
func manyEpisodes(n int) http.HandlerFunc {
	list := strings.TrimSuffix(strings.Repeat(`{"details":"x"},`, n), ",")
	return body(`{"properties":{"episodeid":` + itoa(n) + `,"episodes":[` + list + `]}}`)
}

// twoEpisodes: event data declaring two episodes.
var twoEpisodes = body(`{"properties":{"episodeid":2,"episodes":[{"details":"x"},{"details":"x"}]}}`)

func TestResolveFailureClassification(t *testing.T) {
	cases := []struct {
		name      string
		eventData http.HandlerFunc
		geometry  http.HandlerFunc
		source    string
		want      resolveFailure
	}{
		// ── upstream: GDACS could not give a complete answer ───────────────────
		{"event endpoint 503 is an outage", status(http.StatusServiceUnavailable), status(http.StatusOK), testSourceURL, resolveUpstream},
		{"event endpoint 429 is an outage", status(http.StatusTooManyRequests), status(http.StatusOK), testSourceURL, resolveUpstream},
		{"unparseable event body is an outage", body(`<html>maintenance</html>`), status(http.StatusOK), testSourceURL, resolveUpstream},
		{
			// ⚠️ "No episode matched" may only be claimed from a COMPLETE scan.
			// Episode 1 errored; the matching ring may be the one we never saw.
			"no match from a partial episode scan is an outage, not a refusal",
			twoEpisodes,
			func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("episodeid") == "1" {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				_, _ = w.Write([]byte(`{"features":[]}`))
			},
			testSourceURL, resolveUpstream,
		},

		// ── schema-less 200s are NOT answers (independent review, PR #280) ─────
		// A 200 that parses as JSON but lacks the fields an answer must carry is a
		// maintenance page, an error envelope, or a broken proxy — not GDACS saying
		// "no episodes". Classifying these as refusals re-hides the outage.
		{"event body {} is an outage", body(`{}`), status(http.StatusOK), testSourceURL, resolveUpstream},
		{"event body with null properties is an outage", body(`{"properties":null}`), status(http.StatusOK), testSourceURL, resolveUpstream},
		{"event body as a maintenance envelope is an outage", body(`{"status":"maintenance"}`), status(http.StatusOK), testSourceURL, resolveUpstream},
		{"geometry body {} for every episode is an outage", twoEpisodes, body(`{}`), testSourceURL, resolveUpstream},
		{"geometry with null features is an outage", twoEpisodes, body(`{"features":null}`), testSourceURL, resolveUpstream},
		{
			// Scanning stops at maxGDACSEpisodes; the matching ring may lie beyond it.
			"no match within the episode cap is a partial scan, not a refusal",
			manyEpisodes(maxGDACSEpisodes + 5), body(`{"features":[]}`), testSourceURL, resolveUpstream,
		},
		{
			"a match within the episode cap still resolves",
			manyEpisodes(maxGDACSEpisodes + 5),
			func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("episodeid") == "3" {
					_, _ = w.Write([]byte(`{"features":[{"properties":{"Class":"Poly_Affected","episodeid":3},"geometry":{"type":"Polygon","coordinates":` + fourVertexRing + `}}]}`))
					return
				}
				_, _ = w.Write([]byte(`{"features":[]}`))
			},
			testSourceURL, resolveOK,
		},

		// Round 2: the inner fields must be present too.
		{"properties with neither episode field is an outage", body(`{"properties":{}}`), status(http.StatusOK), testSourceURL, resolveUpstream},
		{"properties with null episodes and no episodeid is an outage", body(`{"properties":{"episodes":null}}`), status(http.StatusOK), testSourceURL, resolveUpstream},
		{
			// Round 2: GDACS flood 1104053 declared 34 episodes on 2026-10-03. A
			// real long event must be scanned in full, not truncated into a
			// permanent false outage.
			"a real 34-episode event matching at episode 30 resolves",
			manyEpisodes(34),
			func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("episodeid") == "30" {
					_, _ = w.Write([]byte(`{"features":[{"properties":{"Class":"Poly_Affected","episodeid":30},"geometry":{"type":"Polygon","coordinates":` + fourVertexRing + `}}]}`))
					return
				}
				_, _ = w.Write([]byte(`{"features":[]}`))
			},
			testSourceURL, resolveOK,
		},
		{"a real 34-episode event with no match is a refusal, not an outage", manyEpisodes(34), body(`{"features":[]}`), testSourceURL, resolveUnverifiable},

		// ── unverifiable: GDACS answered, geometry cannot be established ───────
		{"event endpoint 404 means GDACS does not know the event", status(http.StatusNotFound), status(http.StatusOK), testSourceURL, resolveUnverifiable},
		{"complete scan, no matching ring, is a refusal", twoEpisodes, body(`{"features":[]}`), testSourceURL, resolveUnverifiable},
		{
			"an episode GDACS reports 404 for is a complete answer, not a gap",
			twoEpisodes, status(http.StatusNotFound), testSourceURL, resolveUnverifiable,
		},
		{"event with zero episodes is a refusal", body(`{"properties":{"episodeid":0,"episodes":[]}}`), status(http.StatusOK), testSourceURL, resolveUnverifiable},
		{"non-GDACS source reference is a refusal, never a fetch", status(http.StatusOK), status(http.StatusOK), "https://evil.example.com/x?eventtype=FL&eventid=1", resolveUnverifiable},

		// ── ok ─────────────────────────────────────────────────────────────────
		{
			"one episode errors but another matches: resolves, as before",
			twoEpisodes,
			func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("episodeid") == "1" {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				_, _ = w.Write([]byte(`{"features":[{"properties":{"Class":"Poly_Affected","episodeid":2},"geometry":{"type":"Polygon","coordinates":` + fourVertexRing + `}}]}`))
			},
			testSourceURL, resolveOK,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gdacsStub(t, c.eventData, c.geometry)
			_, got := resolveGDACSPolygon(context.Background(), nil, c.source, ring4)
			if got != c.want {
				t.Errorf("failure = %v, want %v", got, c.want)
			}
		})
	}
}

func TestResolveFailureWhenGDACSIsUnreachable(t *testing.T) {
	// A closed server: connection refused, the commonest real outage shape.
	dead := httptest.NewServer(http.NotFoundHandler())
	url := dead.URL
	dead.Close()
	swapGDACSURLs(t, url, url)

	if _, got := resolveGDACSPolygon(context.Background(), nil, testSourceURL, ring4); got != resolveUpstream {
		t.Errorf("unreachable GDACS: failure = %v, want upstream", got)
	}
}

func TestBudgetExhaustionIsUpstream(t *testing.T) {
	// Exhausted budget = coverage we did not even try to get. Not a refusal.
	b := NewRunBudget()
	b.requestsLeft = 0
	if _, got := b.resolveTyped(context.Background(), testSourceURL, ring4); got != resolveUpstream {
		t.Errorf("exhausted budget: failure = %v, want upstream", got)
	}
	// And it must not be cached as a permanent answer.
	if _, cached := b.cache[testSourceURL+"#4"]; cached {
		t.Error("a budget refusal must not be cached")
	}
}

func TestResolveBoolWrapperUnchanged(t *testing.T) {
	// The bool-returning API is kept so existing callers and tests are untouched;
	// it must report ok exactly when the typed result is resolveOK.
	gdacsStub(t, status(http.StatusServiceUnavailable), status(http.StatusOK))
	if _, ok := ResolveGDACSPolygon(context.Background(), testSourceURL, ring4); ok {
		t.Error("an outage must still report ok=false through the bool API")
	}
}

func TestRunOutcome(t *testing.T) {
	cases := []struct {
		name       string
		result     *IngestResult
		err        error
		want       models.IngestionRunStatus
		msgContain string
	}{
		{"ingest error is a failure", &IngestResult{EventsGeomUpstreamFailed: 3}, errors.New("eonet down"), models.RunStatusFailure, "eonet down"},
		{"upstream GDACS failure degrades a completed run", &IngestResult{EventsGeomUnresolved: 2, EventsGeomUpstreamFailed: 2}, nil, models.RunStatusDegraded, "2 polygon event(s)"},
		{
			// ⚠️ The case that must NOT degrade: GDACS answered, nothing matched.
			"refusals alone leave the run successful",
			&IngestResult{EventsGeomUnresolved: 5, EventsGeomUpstreamFailed: 0}, nil, models.RunStatusSuccess, "",
		},
		{"clean run is a success", &IngestResult{}, nil, models.RunStatusSuccess, ""},
		{"nil result without error is a success", nil, nil, models.RunStatusSuccess, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, msg := runOutcome(c.result, c.err)
			if got != c.want {
				t.Fatalf("status = %q, want %q", got, c.want)
			}
			if c.msgContain == "" {
				if msg != nil {
					t.Errorf("expected no error message, got %q", *msg)
				}
				return
			}
			if msg == nil || !strings.Contains(*msg, c.msgContain) {
				t.Errorf("message %v does not contain %q", msg, c.msgContain)
			}
		})
	}
}

func TestDegradedAlertAction(t *testing.T) {
	degraded := &IngestResult{EventsGeomUpstreamFailed: 1}
	healthy := &IngestResult{}
	sent := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	run := func(s models.IngestionRunStatus, alerted bool) *models.IngestionRun {
		r := &models.IngestionRun{Status: s}
		if alerted {
			r.AlertSentAt = &sent
		}
		return r
	}
	cases := []struct {
		name   string
		result *IngestResult
		prev   *models.IngestionRun
		err    error
		want   degradedAction
	}{
		{"not degraded: nothing to do", healthy, run(models.RunStatusDegraded, true), nil, degradedNoAlert},
		{"first degraded run after success sends", degraded, run(models.RunStatusSuccess, false), nil, degradedSend},
		{"first degraded run ever sends", degraded, nil, nil, degradedSend},
		{"failure then degraded sends", degraded, run(models.RunStatusFailure, false), nil, degradedSend},
		{"streak already alerted is carried, not re-sent", degraded, run(models.RunStatusDegraded, true), nil, degradedCarry},
		{
			// ⚠️ The independent-review case: the previous send FAILED, so the
			// streak was never actually alerted. It must retry, not stay silent.
			"previous degraded run whose alert failed retries",
			degraded, run(models.RunStatusDegraded, false), nil, degradedSend,
		},
		{"lookup error errs towards sending", degraded, run(models.RunStatusDegraded, true), errors.New("db down"), degradedSend},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := degradedAlertAction(c.result, c.prev, c.err); got != c.want {
				t.Errorf("degradedAlertAction = %d, want %d", got, c.want)
			}
		})
	}
}

// TestLongEventResolvesWithinProductionBudget (independent review, round 3): the
// other 34-episode tests use an unbounded (nil) budget. This one uses the real
// per-run budget, proving a realistic long event fits. A >=60-episode event
// would not, and would degrade every run — an accepted, recorded limitation
// (the longest NG/GH event in the past year reached 8 episodes).
func TestLongEventResolvesWithinProductionBudget(t *testing.T) {
	gdacsStub(t, manyEpisodes(34), func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("episodeid") == "34" {
			_, _ = w.Write([]byte(`{"features":[{"properties":{"Class":"Poly_Affected","episodeid":34},"geometry":{"type":"Polygon","coordinates":` + fourVertexRing + `}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"features":[]}`))
	})
	if _, got := NewRunBudget().resolveTyped(context.Background(), testSourceURL, ring4); got != resolveOK {
		t.Errorf("34-episode event under the production budget: %v, want ok", got)
	}
}
