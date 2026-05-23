package ingestor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

// testCountry is the standard CountryConfig used in ingestor unit tests.
var testCountry = CountryConfig{Code: "NG", Name: "Nigeria", BBox: [4]float64{2.0, 4.0, 15.0, 14.0}}

// okBody is a minimal valid EONET JSON response that the normalizer accepts.
const okBody = `{
	"events": [
		{
			"id": "EONET_123",
			"title": "Wildfire",
			"categories": [{"id": "wildfires"}],
			"geometry": [{"magnitudeValue": null, "magnitudeUnit": null, "date": "2023-01-01T00:00:00Z", "type": "Point", "coordinates": [0.0, 0.0]}]
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

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestRunIngest_429_ThenSuccess verifies the happy-path retry: the ingestor
// receives a 429 with retry_after=1, waits (instant in tests), then succeeds.
func TestRunIngest_429_ThenSuccess(t *testing.T) {
	defer installInstantSleep(t)()

	var requestCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	// First request fails at the RoundTripper before hitting the server,
	// second request reaches the server normally.
	if got := atomic.LoadInt32(&rt.count); got != 2 {
		t.Errorf("expected 2 RoundTrip calls (1 fail + 1 success), got %d", got)
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
