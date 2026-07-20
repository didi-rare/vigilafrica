package ingestor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/models"
)

// failOnceRoundTripper returns failWith on the first RoundTrip call, then
// delegates to base for subsequent calls. Used to drive the eonetHTTPClient
// injection seam for tests that need a transport-layer failure on the first
// attempt — something httptest.Server can't reproduce directly.
type failOnceRoundTripper struct {
	base     http.RoundTripper
	count    int32
	failWith error
}

func (rt *failOnceRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if atomic.AddInt32(&rt.count, 1) == 1 {
		return nil, rt.failWith
	}
	return rt.base.RoundTrip(req)
}

// installHTTPClient replaces eonetHTTPClient with c and returns a restore fn.
// Mirrors installTestServer / installInstantSleep for the new injection seam
// added in chore-eonet-retry-backoff.
func installHTTPClient(t *testing.T, c *http.Client) func() {
	t.Helper()
	orig := eonetHTTPClient
	eonetHTTPClient = c
	return func() { eonetHTTPClient = orig }
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// mockRepo satisfies database.Repository without a real DB.
// Declared once here; all tests in this package share it.
type mockRepo struct{}

func (m *mockRepo) UpsertEvent(ctx context.Context, e models.Event, geoJSON string) error {
	return nil
}
func (m *mockRepo) ListEvents(ctx context.Context, filters database.EventFilters) ([]models.Event, int, error) {
	return nil, 0, nil
}
func (m *mockRepo) GetEventByID(ctx context.Context, id uuid.UUID) (*models.Event, error) {
	return nil, nil
}
func (m *mockRepo) GetNearbyEvents(ctx context.Context, lat, lng float64, radiusKm float64, limit int) ([]models.Event, error) {
	return nil, nil
}
func (m *mockRepo) CreateIngestionRun(ctx context.Context, startedAt time.Time, countryCode string) (int64, error) {
	return 1, nil
}
func (m *mockRepo) CompleteIngestionRun(ctx context.Context, id int64, status models.IngestionRunStatus, fetched, stored int, errMsg *string) error {
	return nil
}
func (m *mockRepo) GetLastIngestionRun(ctx context.Context) (*models.IngestionRun, error) {
	return nil, nil
}
func (m *mockRepo) GetLastSuccessfulIngestionRun(ctx context.Context) (*models.IngestionRun, error) {
	return nil, nil
}
func (m *mockRepo) GetFirstIngestionRun(ctx context.Context) (*models.IngestionRun, error) {
	return nil, nil
}
func (m *mockRepo) GetLastIngestionRunAllCountries(ctx context.Context) (map[string]*models.IngestionRun, error) {
	return nil, nil
}
func (m *mockRepo) GetEnrichmentStats(ctx context.Context) ([]database.EnrichmentStat, error) {
	return nil, nil
}
func (m *mockRepo) GetDistinctStatesByCountry(ctx context.Context, country string) ([]string, error) {
	return nil, nil
}
func (m *mockRepo) Close() {}

// recordingRepo is a mockRepo that remembers which events were upserted, so a
// test can assert *which* events survived the ingest loop rather than only how
// many. Embedding *mockRepo promotes the rest of the database.Repository
// surface; only UpsertEvent is overridden.
// Construct as: &recordingRepo{mockRepo: &mockRepo{}}
type recordingRepo struct {
	*mockRepo
	upserts []models.Event
}

func (r *recordingRepo) UpsertEvent(ctx context.Context, e models.Event, geoJSON string) error {
	r.upserts = append(r.upserts, e)
	return nil
}

// sourceIDs returns the source IDs recorded so far, in upsert order.
func (r *recordingRepo) sourceIDs() []string {
	ids := make([]string, 0, len(r.upserts))
	for _, e := range r.upserts {
		ids = append(ids, e.SourceID)
	}
	return ids
}

// testCountry is the standard CountryConfig used in ingestor unit tests.
var testCountry = CountryConfig{Code: "NG", Name: "Nigeria", BBox: [4]float64{2.0, 4.0, 15.0, 14.0}}

// okBody is a minimal valid EONET JSON response that the normalizer accepts.
// The point sits inside testCountry's bbox so the containment guard in
// runIngest stores it — keep it that way, or tests asserting EventsStored break.
const okBody = `{
	"events": [
		{
			"id": "EONET_123",
			"title": "Wildfire",
			"categories": [{"id": "wildfires"}],
			"geometry": [{"magnitudeValue": null, "magnitudeUnit": null, "date": "2023-01-01T00:00:00Z", "type": "Point", "coordinates": [8.0, 9.0]}]
		}
	]
}`

// installInstantSleep replaces eonetSleepFn with a no-op so tests don't wait
// on real wall-clock time (openspec-review §9.11).
// Returns a restore function — call it in defer.
func installInstantSleep(t *testing.T) func() {
	t.Helper()
	orig := eonetSleepFn
	eonetSleepFn = func(ctx context.Context, d time.Duration) error {
		// Still honour cancellation even in fast tests.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	return func() { eonetSleepFn = orig }
}

// installTestServer points eonetURL at srv and returns a restore function.
func installTestServer(t *testing.T, srv *httptest.Server) func() {
	t.Helper()
	orig := eonetURL
	eonetURL = srv.URL
	return func() { eonetURL = orig }
}

// emptyEventsBody is a well-formed EONET response carrying no events.
const emptyEventsBody = `{"events": []}`

// closedQueryStub answers the status=closed request with an empty event list
// and returns WITHOUT invoking h.
//
// Since fix-eonet-closed-events-ingest, runIngest issues two requests per
// country (status=open, then status=closed&days=30). Every retry, backoff and
// error test in this file describes the behaviour of a SINGLE fetch, and their
// request counters and event-count assertions were written against the open
// request. Short-circuiting the closed request before h runs means those
// counters never see it, so each test keeps its original meaning rather than
// being rewritten to accommodate a second request it does not care about.
//
// The union behaviour itself is covered separately by
// TestRunIngest_QueriesOpenAndClosed and TestRunIngest_IngestsClosedFloodEvent.
func closedQueryStub(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("status") == "closed" {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, emptyEventsBody)
			return
		}
		h(w, r)
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestRunIngest_429_ThenSuccess verifies the happy-path retry: the ingestor
// receives a 429 with retry_after=1, waits (instant in tests), then succeeds.
func TestRunIngest_429_ThenSuccess(t *testing.T) {
	defer installInstantSleep(t)()

	var requestCount int32
	srv := httptest.NewServer(closedQueryStub(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&requestCount, 1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"retry_after": 1}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, okBody)
	}))
	defer srv.Close()
	defer installTestServer(t, srv)()

	result, err := runIngest(context.Background(), &mockRepo{}, testCountry)
	if err != nil {
		t.Fatalf("expected success after retry, got err: %v", err)
	}
	if result.EventsFetched != 1 {
		t.Errorf("expected 1 event fetched, got %d", result.EventsFetched)
	}
	if atomic.LoadInt32(&requestCount) != 2 {
		t.Errorf("expected 2 HTTP requests (1 rate-limited + 1 success), got %d", requestCount)
	}
}

// TestRunIngest_503_ThenSuccess verifies the same recovery path for a
// 503 Service Unavailable response (identical code path to 429).
func TestRunIngest_503_ThenSuccess(t *testing.T) {
	defer installInstantSleep(t)()

	var requestCount int32
	srv := httptest.NewServer(closedQueryStub(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&requestCount, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			// No retry_after body — exercises the exponential fallback branch.
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, okBody)
	}))
	defer srv.Close()
	defer installTestServer(t, srv)()

	result, err := runIngest(context.Background(), &mockRepo{}, testCountry)
	if err != nil {
		t.Fatalf("expected success after 503 retry, got err: %v", err)
	}
	if result.EventsFetched != 1 {
		t.Errorf("expected 1 event fetched, got %d", result.EventsFetched)
	}
}

// TestRunIngest_ExhaustRetries verifies that after maxRetries (3) attempts the
// ingestor gives up and returns an error rather than looping forever.
func TestRunIngest_ExhaustRetries(t *testing.T) {
	defer installInstantSleep(t)()

	var requestCount int32
	srv := httptest.NewServer(closedQueryStub(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"retry_after": 1}`)
	}))
	defer srv.Close()
	defer installTestServer(t, srv)()

	_, err := runIngest(context.Background(), &mockRepo{}, testCountry)
	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}

	// Loop runs attempt 0, 1, 2, 3 — maxRetries(3)+1 = 4 total requests.
	if got := atomic.LoadInt32(&requestCount); got != 4 {
		t.Errorf("expected 4 total requests (attempt 0-3), got %d", got)
	}
}

// TestRunIngest_MissingRetryAfter_ExponentialFallback confirms the ingestor
// falls back to exponential backoff when the 429 body is empty / unparseable.
func TestRunIngest_MissingRetryAfter_ExponentialFallback(t *testing.T) {
	var sleepDurations []time.Duration
	orig := eonetSleepFn
	eonetSleepFn = func(ctx context.Context, d time.Duration) error {
		sleepDurations = append(sleepDurations, d)
		return nil
	}
	defer func() { eonetSleepFn = orig }()

	var requestCount int32
	srv := httptest.NewServer(closedQueryStub(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&requestCount, 1) <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			// Deliberately empty body — forces exponential fallback.
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, okBody)
	}))
	defer srv.Close()
	defer installTestServer(t, srv)()

	_, err := runIngest(context.Background(), &mockRepo{}, testCountry)
	if err != nil {
		t.Fatalf("expected success after exponential retries, got err: %v", err)
	}

	// Attempt 0 → 5*(1<<0) = 5s; attempt 1 → 5*(1<<1) = 10s.
	wantDurations := []time.Duration{5 * time.Second, 10 * time.Second}
	if len(sleepDurations) != len(wantDurations) {
		t.Fatalf("expected %d sleep calls, got %d: %v", len(wantDurations), len(sleepDurations), sleepDurations)
	}
	for i, want := range wantDurations {
		if sleepDurations[i] != want {
			t.Errorf("sleep[%d]: want %v, got %v", i, want, sleepDurations[i])
		}
	}
}

// TestRunIngest_ContextCancelledDuringSleep verifies that context cancellation
// during the retry sleep is propagated back as an error immediately.
// The context is cancelled while sleepFn is blocking, so the ingestor unwinds
// without completing further retries.
func TestRunIngest_ContextCancelledDuringSleep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Override sleepFn to cancel the context on the first sleep call and then
	// block on ctx.Done() — this exercises the real cancellation path inside
	// eonetSleepFn without any real wall-clock wait.
	orig := eonetSleepFn
	eonetSleepFn = func(c context.Context, d time.Duration) error {
		cancel() // trigger cancellation mid-sleep
		<-c.Done()
		return c.Err()
	}
	defer func() { eonetSleepFn = orig }()

	srv := httptest.NewServer(closedQueryStub(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"retry_after": 60}`) // large value — we must not actually wait
	}))
	defer srv.Close()
	defer installTestServer(t, srv)()

	_, err := runIngest(ctx, &mockRepo{}, testCountry)
	if err == nil {
		t.Fatal("expected an error due to context cancellation, got nil")
	}
	// The error returned by eonetSleepFn is ctx.Err() = context.Canceled.
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestRunIngest_RejectsExcessiveRetryAfter(t *testing.T) {
	defer installInstantSleep(t)()

	srv := httptest.NewServer(closedQueryStub(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"retry_after": 86400}`)
	}))
	defer srv.Close()
	defer installTestServer(t, srv)()

	_, err := runIngest(context.Background(), &mockRepo{}, testCountry)
	if err == nil {
		t.Fatal("expected excessive retry_after to fail")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Fatalf("expected bounded retry_after error, got %v", err)
	}
}

func TestRunIngest_RejectsOversizedEONETResponse(t *testing.T) {
	srv := httptest.NewServer(closedQueryStub(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Repeat("x", maxEONETResponseBytes+1))
	}))
	defer srv.Close()
	defer installTestServer(t, srv)()

	_, err := runIngest(context.Background(), &mockRepo{}, testCountry)
	if err == nil {
		t.Fatal("expected oversized response to fail")
	}
	if !strings.Contains(err.Error(), "response body exceeds maximum") {
		t.Fatalf("expected oversized response error, got %v", err)
	}
}

func TestRunIngest_RejectsOversizedRawEventPayload(t *testing.T) {
	largeTitle := strings.Repeat("x", maxEONETEventRawPayloadBytes)
	body := fmt.Sprintf(`{
		"events": [
			{
				"id": "EONET_HUGE",
				"title": %q,
				"categories": [{"id": "wildfires"}],
				"geometry": [{"magnitudeValue": null, "magnitudeUnit": null, "date": "2023-01-01T00:00:00Z", "type": "Point", "coordinates": [0.0, 0.0]}]
			}
		]
	}`, largeTitle)

	srv := httptest.NewServer(closedQueryStub(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, body)
	}))
	defer srv.Close()
	defer installTestServer(t, srv)()

	_, err := runIngest(context.Background(), &mockRepo{}, testCountry)
	if err == nil {
		t.Fatal("expected oversized raw event payload to fail")
	}
	if !strings.Contains(err.Error(), "raw payload exceeds maximum") {
		t.Fatalf("expected raw payload limit error, got %v", err)
	}
}

// TestRunIngest_NonRetryable4xx confirms that 4xx status codes (other than
// 429) are surfaced as errors without retrying. 5xx codes now follow the
// transient retry path (see TestRunIngest_5xx_*) and 429 keeps its own
// dynamic-backoff branch (TestRunIngest_429_ThenSuccess).
func TestRunIngest_NonRetryable4xx(t *testing.T) {
	defer installInstantSleep(t)()

	tests := []struct {
		name   string
		status int
	}{
		{"unauthorized", http.StatusUnauthorized},
		{"forbidden", http.StatusForbidden},
		{"not found", http.StatusNotFound},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var requestCount int32
			srv := httptest.NewServer(closedQueryStub(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&requestCount, 1)
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			origURL := eonetURL
			eonetURL = srv.URL
			defer func() { eonetURL = origURL }()

			_, err := runIngest(context.Background(), &mockRepo{}, testCountry)
			if err == nil {
				t.Errorf("status %d: expected error, got nil", tt.status)
			}
			// Must fail on the first attempt — no retries for 4xx (non-429).
			if got := atomic.LoadInt32(&requestCount); got != 1 {
				t.Errorf("status %d: expected exactly 1 request (no retry), got %d", tt.status, got)
			}
		})
	}
}

// TestRunIngest_5xx_ThenSuccess verifies the transient retry path for a 5xx
// response other than 503 (which has its own retry_after-aware branch).
// First request 500, second 200 → ingestion succeeds.
func TestRunIngest_5xx_ThenSuccess(t *testing.T) {
	defer installInstantSleep(t)()

	var requestCount int32
	srv := httptest.NewServer(closedQueryStub(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&requestCount, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, okBody)
	}))
	defer srv.Close()
	defer installTestServer(t, srv)()

	result, err := runIngest(context.Background(), &mockRepo{}, testCountry)
	if err != nil {
		t.Fatalf("expected success after 5xx retry, got err: %v", err)
	}
	if result.EventsFetched != 1 {
		t.Errorf("expected 1 event fetched, got %d", result.EventsFetched)
	}
	if got := atomic.LoadInt32(&requestCount); got != 2 {
		t.Errorf("expected 2 requests (1 server-error + 1 success), got %d", got)
	}
}

// TestRunIngest_5xx_ExhaustsTransientRetries verifies that maxTransientRetries
// bounds the 5xx retry loop. Always 500 → maxTransientRetries+1 total attempts
// then error.
func TestRunIngest_5xx_ExhaustsTransientRetries(t *testing.T) {
	defer installInstantSleep(t)()

	var requestCount int32
	srv := httptest.NewServer(closedQueryStub(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	defer installTestServer(t, srv)()

	_, err := runIngest(context.Background(), &mockRepo{}, testCountry)
	if err == nil {
		t.Fatal("expected error after exhausting transient retries, got nil")
	}
	if !strings.Contains(err.Error(), "transient retries") {
		t.Errorf("expected transient-retries error, got: %v", err)
	}
	// 1 initial attempt + maxTransientRetries (=2) = 3 total requests
	wantRequests := int32(maxTransientRetries + 1)
	if got := atomic.LoadInt32(&requestCount); got != wantRequests {
		t.Errorf("expected %d requests (1 initial + %d retries), got %d", wantRequests, maxTransientRetries, got)
	}
}

// TestRunIngest_NetworkError_ThenSuccess verifies the transient retry path
// for a network-level error (no HTTP response). Uses the eonetHTTPClient
// injection seam (chore-eonet-retry-backoff D5) with a failOnceRoundTripper
// so the first request fails at the transport layer before reaching the
// server, and the second succeeds normally.
func TestRunIngest_NetworkError_ThenSuccess(t *testing.T) {
	defer installInstantSleep(t)()

	var serverHits int32
	srv := httptest.NewServer(closedQueryStub(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&serverHits, 1)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, okBody)
	}))
	defer srv.Close()
	defer installTestServer(t, srv)()

	rt := &failOnceRoundTripper{
		base:     http.DefaultTransport,
		failWith: errors.New("simulated network timeout"),
	}
	defer installHTTPClient(t, &http.Client{Timeout: 30 * time.Second, Transport: rt})()

	result, err := runIngest(context.Background(), &mockRepo{}, testCountry)
	if err != nil {
		t.Fatalf("expected success after network-error retry, got err: %v", err)
	}
	if result.EventsFetched != 1 {
		t.Errorf("expected 1 event fetched, got %d", result.EventsFetched)
	}
	// rt.count is transport-level, so unlike serverHits it also sees the
	// status=closed request that closedQueryStub answers without invoking the
	// handler: open-fails, open-retry-succeeds, then closed. The behaviour under
	// test — one network failure retried exactly once — is the first two.
	if got := atomic.LoadInt32(&rt.count); got != 3 {
		t.Errorf("expected 3 RoundTrip calls (open: 1 fail + 1 success, then closed), got %d", got)
	}
	if got := atomic.LoadInt32(&serverHits); got != 1 {
		t.Errorf("expected 1 server hit (transport-layer fail doesn't reach server), got %d", got)
	}
}

// TestRunIngest_RateLimit_RealSleep is the original integration-style test
// that validates the actual wall-clock timing of the retry_after + 5s buffer.
// NOTE: this test sleeps ~6 real seconds (retry_after=1 + 5s buffer). This is
// intentional — it validates the production sleep path, not the injected mock.
// It is retained for confidence but is slow; run individually with:
//
//	go test -run TestRunIngest_RateLimit_RealSleep ./internal/ingestor/
func TestRunIngest_RateLimit_RealSleep(t *testing.T) {
	var requestCount int32
	srv := httptest.NewServer(closedQueryStub(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&requestCount, 1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"retry_after": 1}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, okBody)
	}))
	defer srv.Close()
	defer installTestServer(t, srv)()

	start := time.Now()
	result, err := runIngest(context.Background(), &mockRepo{}, testCountry)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected success after retry, got err: %v", err)
	}
	if result.EventsFetched != 1 {
		t.Errorf("expected 1 event fetched, got %d", result.EventsFetched)
	}
	// Must have honoured retry_after(1) + 5s buffer = 6s minimum.
	if elapsed < 6*time.Second {
		t.Errorf("expected ≥6s sleep for retry_after=1 + 5s buffer; took %v", elapsed)
	}
}

// ---------------------------------------------------------------------------
// Bounding-box containment guard (fix-ingest-bbox-validation)
// ---------------------------------------------------------------------------

func TestWithinBBox(t *testing.T) {
	nigeria := [4]float64{2.0, 4.0, 15.0, 14.0}
	ghana := [4]float64{-3.5, 4.5, 1.2, 11.2}

	tests := []struct {
		name string
		bbox [4]float64
		lon  float64
		lat  float64
		want bool
	}{
		{"inside nigeria", nigeria, 8.0, 9.0, true},
		// Nigeria's bbox legitimately overlaps neighbouring countries. Real
		// coordinates from production data — these must NOT be rejected: the
		// guard drops out-of-box events, not out-of-country ones.
		{"cameroon border event inside nigeria bbox", nigeria, 12.765, 5.992, true},
		{"benin border event inside nigeria bbox", nigeria, 2.686, 11.781, true},
		{"west of nigeria", nigeria, 0.0, 9.0, false},
		{"east of nigeria", nigeria, 16.0, 9.0, false},
		{"south of nigeria", nigeria, 8.0, 3.0, false},
		{"north of nigeria", nigeria, 8.0, 20.0, false},
		{"florida is far outside nigeria", nigeria, -84.5657, 30.0531, false},
		{"south-west corner is inclusive", nigeria, 2.0, 4.0, true},
		{"north-east corner is inclusive", nigeria, 15.0, 14.0, true},
		{"inside ghana with negative longitude", ghana, -1.5, 7.0, true},
		{"west of ghana with negative longitude", ghana, -4.0, 7.0, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := withinBBox(tt.bbox, tt.lon, tt.lat); got != tt.want {
				t.Errorf("withinBBox(%v, %v, %v) = %v, want %v", tt.bbox, tt.lon, tt.lat, got, tt.want)
			}
		})
	}
}

// TestRunIngest_SkipsEventOutsideCountryBBox pins the containment guard: EONET's
// server-side bbox filter is a hint, not a guarantee. It has been observed
// returning EONET_20263 (a Wakulla, Florida wildfire) against the Nigeria bbox,
// which then reached production with an empty country_name. Such an event must
// never be upserted.
func TestRunIngest_SkipsEventOutsideCountryBBox(t *testing.T) {
	const body = `{
		"events": [
			{
				"id": "EONET_NG_IN",
				"title": "Wildfire in Nigeria",
				"categories": [{"id": "wildfires"}],
				"geometry": [{"date": "2026-07-18T00:00:00Z", "type": "Point", "coordinates": [8.0, 9.0]}]
			},
			{
				"id": "EONET_20263",
				"title": "340 Wildfire, Wakulla, Florida",
				"categories": [{"id": "wildfires"}],
				"geometry": [{"date": "2026-07-18T00:00:00Z", "type": "Point", "coordinates": [-84.5657, 30.0531]}]
			}
		]
	}`

	srv := httptest.NewServer(closedQueryStub(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, body)
	}))
	defer srv.Close()
	defer installTestServer(t, srv)()

	repo := &recordingRepo{mockRepo: &mockRepo{}}
	result, err := runIngest(context.Background(), repo, testCountry)
	if err != nil {
		t.Fatalf("runIngest returned err: %v", err)
	}

	if result.EventsFetched != 2 {
		t.Errorf("expected 2 events fetched, got %d", result.EventsFetched)
	}
	if result.EventsStored != 1 {
		t.Errorf("expected 1 event stored, got %d", result.EventsStored)
	}
	if result.EventsSkippedBBox != 1 {
		t.Errorf("expected 1 event skipped for bbox, got %d", result.EventsSkippedBBox)
	}
	if got := repo.sourceIDs(); len(got) != 1 || got[0] != "EONET_NG_IN" {
		t.Errorf("expected only EONET_NG_IN to be upserted, got %v", got)
	}
}

// TestRunIngest_StoresEventWithUnverifiableGeometry pins the deliberate decision
// that events whose containment cannot be checked are stored rather than
// dropped. The normalizer resolves lon/lat only for Point geometry and leaves
// them nil for Polygon, so a polygon is unverifiable — we do not discard data we
// cannot verify, even when the ring lies outside the country bbox.
func TestRunIngest_StoresEventWithUnverifiableGeometry(t *testing.T) {
	const body = `{
		"events": [
			{
				"id": "EONET_POLY",
				"title": "Flood polygon",
				"categories": [{"id": "floods"}],
				"geometry": [{"date": "2026-07-18T00:00:00Z", "type": "Polygon", "coordinates": [[[-84.6,30.0],[-84.5,30.0],[-84.5,30.1],[-84.6,30.1],[-84.6,30.0]]]}]
			}
		]
	}`

	srv := httptest.NewServer(closedQueryStub(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, body)
	}))
	defer srv.Close()
	defer installTestServer(t, srv)()

	repo := &recordingRepo{mockRepo: &mockRepo{}}
	result, err := runIngest(context.Background(), repo, testCountry)
	if err != nil {
		t.Fatalf("runIngest returned err: %v", err)
	}

	if result.EventsStored != 1 {
		t.Errorf("expected the polygon event to be stored, got %d stored", result.EventsStored)
	}
	if result.EventsSkippedBBox != 0 {
		t.Errorf("polygon must not count as a bbox skip, got %d", result.EventsSkippedBBox)
	}
	if result.EventsUnverifiedGeom != 1 {
		t.Errorf("expected 1 unverified-geometry event counted, got %d", result.EventsUnverifiedGeom)
	}
	if got := repo.sourceIDs(); len(got) != 1 || got[0] != "EONET_POLY" {
		t.Errorf("expected EONET_POLY to be upserted, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// fix-eonet-closed-events-ingest — open/closed union
// ---------------------------------------------------------------------------

// closedFloodBody mirrors the real EONET_20881 record — the Lagos flood of
// 2026-06-30 that the invalid status=open,closed query silently dropped.
// Its point is inside testCountry's bbox, so the containment guard from
// fix-ingest-bbox-validation stores rather than skips it.
const closedFloodBody = `{
	"events": [
		{
			"id": "EONET_20881",
			"title": "Flood in Nigeria 1103997",
			"closed": "2026-07-02T00:00:00Z",
			"categories": [{"id": "floods"}],
			"geometry": [{"date": "2026-06-30T20:00:00Z", "type": "Point", "coordinates": [3.3941795, 6.4550575]}]
		}
	]
}`

// TestRunIngest_QueriesOpenAndClosed pins the outbound query strings.
//
// This is the regression guard for the root cause: the ingestor previously sent
// status=open,closed, which is not a valid EONET v3 value. EONET answered 200
// and silently degraded to open-only, so floods — which close within ~48h —
// were never ingested. No test asserted the query string, which is exactly how
// that survived to production.
func TestRunIngest_QueriesOpenAndClosed(t *testing.T) {
	var mu sync.Mutex
	var gotURLs []string

	// Deliberately NOT wrapped in closedQueryStub — this test inspects both requests.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		func() {
			mu.Lock() // §7.7 — paired with defer
			defer mu.Unlock()
			gotURLs = append(gotURLs, r.URL.String())
		}()
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, emptyEventsBody)
	}))
	defer srv.Close()
	defer installTestServer(t, srv)()

	if _, err := runIngest(context.Background(), &mockRepo{}, testCountry); err != nil {
		t.Fatalf("runIngest failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(gotURLs) != 2 {
		t.Fatalf("expected exactly 2 requests (open + closed), got %d: %v", len(gotURLs), gotURLs)
	}

	for _, raw := range gotURLs {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("unparseable request URL %q: %v", raw, err)
		}
		q := u.Query()

		// The exact bug. EONET accepts only open|closed|all.
		if got := q.Get("status"); got == "open,closed" {
			t.Errorf("request used the invalid status value %q — EONET degrades this to open-only", got)
		}
		if got := q.Get("category"); got != "floods,wildfires" {
			t.Errorf("category = %q, want floods,wildfires", got)
		}
	}

	openQ, err := url.Parse(gotURLs[0])
	if err != nil {
		t.Fatalf("unparseable open URL: %v", err)
	}
	if got := openQ.Query().Get("status"); got != "open" {
		t.Errorf("first request status = %q, want open", got)
	}
	// Load-bearing: EONET wildfires stay open for months (the oldest open
	// Nigeria wildfire is dated 2024-10-24). days= filters on event date, so
	// windowing the open query would silently drop them.
	if got := openQ.Query().Get("days"); got != "" {
		t.Errorf("open request must NOT be windowed, got days=%q — this drops long-burning wildfires", got)
	}

	closedQ, err := url.Parse(gotURLs[1])
	if err != nil {
		t.Fatalf("unparseable closed URL: %v", err)
	}
	if got := closedQ.Query().Get("status"); got != "closed" {
		t.Errorf("second request status = %q, want closed", got)
	}
	if got, want := closedQ.Query().Get("days"), strconv.Itoa(closedEventWindowDays); got != want {
		t.Errorf("closed request days = %q, want %q", got, want)
	}
}

// TestRunIngest_IngestsClosedFloodEvent proves the fix end-to-end: a flood that
// is already closed upstream is fetched, normalised to StatusClosed, and stored.
// Before the fix this event was unreachable regardless of ingestion cadence.
func TestRunIngest_IngestsClosedFloodEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.URL.Query().Get("status") == "closed" {
			fmt.Fprint(w, closedFloodBody)
			return
		}
		fmt.Fprint(w, emptyEventsBody)
	}))
	defer srv.Close()
	defer installTestServer(t, srv)()

	repo := &recordingRepo{mockRepo: &mockRepo{}}
	result, err := runIngest(context.Background(), repo, testCountry)
	if err != nil {
		t.Fatalf("runIngest failed: %v", err)
	}

	if len(repo.upserts) != 1 {
		t.Fatalf("expected 1 stored event, got %d", len(repo.upserts))
	}
	got := repo.upserts[0]

	if got.SourceID != "EONET_20881" {
		t.Errorf("source_id = %q, want EONET_20881", got.SourceID)
	}
	if got.Category != models.CategoryFloods {
		t.Errorf("category = %q, want floods", got.Category)
	}
	if got.Status != models.StatusClosed {
		t.Errorf("status = %q, want closed — the closed field must drive status so the UI cannot present a stale flood as active", got.Status)
	}
	if result.EventsStored != 1 {
		t.Errorf("EventsStored = %d, want 1", result.EventsStored)
	}
	// Counters accumulate across both responses; the open one carried nothing.
	if result.EventsFetched != 1 {
		t.Errorf("EventsFetched = %d, want 1", result.EventsFetched)
	}
	if result.EventsSkippedBBox != 0 {
		t.Errorf("Lagos is inside the Nigeria bbox and must not be skipped, got %d", result.EventsSkippedBBox)
	}
}
