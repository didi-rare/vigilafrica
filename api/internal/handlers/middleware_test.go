package handlers

import (
	"bytes"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCORSMiddlewareRejectsDisallowedOrigin(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "https://staging.vigilafrica.org")

	called := false
	handler := CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/v1/events", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("expected no Access-Control-Allow-Origin header, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if called {
		t.Fatal("expected disallowed preflight to stop before the next handler")
	}
}

func TestClientIPIgnoresSpoofedForwardedForFromUntrustedPeer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	req.Header.Set("X-Forwarded-For", "198.51.100.99")

	_, network, err := net.ParseCIDR("127.0.0.1/8")
	if err != nil {
		t.Fatal(err)
	}

	if got := clientIPWithTrustedProxies(req, []*net.IPNet{network}); got != "203.0.113.10" {
		t.Fatalf("expected untrusted peer remote IP, got %q", got)
	}
}

func TestClientIPUsesForwardedForFromTrustedPeer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "198.51.100.99, 127.0.0.1")

	_, network, err := net.ParseCIDR("127.0.0.1/8")
	if err != nil {
		t.Fatal(err)
	}

	if got := clientIPWithTrustedProxies(req, []*net.IPNet{network}); got != "198.51.100.99" {
		t.Fatalf("expected trusted forwarded client IP, got %q", got)
	}
}

func TestIPRateLimiterEvictsBuckets(t *testing.T) {
	limiter := newIPRateLimiterWithOptions(60, 2, 10*time.Millisecond)
	limiter.bucketFor("192.0.2.1")
	limiter.bucketFor("192.0.2.2")
	limiter.bucketFor("192.0.2.3")

	if got := len(limiter.buckets); got != 2 {
		t.Fatalf("expected max bucket count 2, got %d", got)
	}

	time.Sleep(15 * time.Millisecond)
	limiter.bucketFor("192.0.2.4")

	if got := len(limiter.buckets); got != 1 {
		t.Fatalf("expected expired buckets to be evicted, got %d", got)
	}
}

func TestResponseCacheBoundsEntriesAndNormalizesQuery(t *testing.T) {
	t.Setenv("CACHE_MAX_ENTRIES", "2")
	t.Setenv("CACHE_MAX_QUERY_BYTES", "64")
	cache := NewResponseCache()
	calls := 0
	handler := cache.CacheMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[1]}`))
	}))

	for _, path := range []string{
		"/v1/events?state=Lagos&country=Nigeria",
		"/v1/events?country=Nigeria&state=Lagos",
		"/v1/events?country=Ghana",
		"/v1/events?country=Benin",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 for %s, got %d", path, rec.Code)
		}
	}

	if calls != 3 {
		t.Fatalf("expected normalized query cache hit to avoid one call, got %d calls", calls)
	}
	if got := len(cache.entries); got != 2 {
		t.Fatalf("expected cache to retain max 2 entries, got %d", got)
	}
}

func TestResponseCacheRejectsOversizedQuery(t *testing.T) {
	t.Setenv("CACHE_MAX_QUERY_BYTES", "8")
	cache := NewResponseCache()
	handler := cache.CacheMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/events?country=Nigeria", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestURITooLong {
		t.Fatalf("expected status %d, got %d", http.StatusRequestURITooLong, rec.Code)
	}
}

func TestCORSMiddlewareAllowsConfiguredOriginAndNoOriginRequests(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "https://staging.vigilafrica.org")

	handler := CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	allowedReq := httptest.NewRequest(http.MethodOptions, "/v1/events", nil)
	allowedReq.Header.Set("Origin", "https://staging.vigilafrica.org")
	allowedRec := httptest.NewRecorder()

	handler.ServeHTTP(allowedRec, allowedReq)

	if allowedRec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, allowedRec.Code)
	}
	if got := allowedRec.Header().Get("Access-Control-Allow-Origin"); got != "https://staging.vigilafrica.org" {
		t.Fatalf("expected configured origin header, got %q", got)
	}

	noOriginReq := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	noOriginRec := httptest.NewRecorder()

	handler.ServeHTTP(noOriginRec, noOriginReq)

	if noOriginRec.Code != http.StatusOK {
		t.Fatalf("expected status %d for no-Origin request, got %d", http.StatusOK, noOriginRec.Code)
	}
	if got := noOriginRec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no Access-Control-Allow-Origin for no-Origin request, got %q", got)
	}
}

func TestRecoveryMiddlewareConvertsPanicToLogged500(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := recoveryMiddlewareWithLogger(logger, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	rec := httptest.NewRecorder()

	// The whole point: this must not propagate out of ServeHTTP.
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	// The client gets the sanitised envelope, never the panic value (§4.5).
	if strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("response leaked the panic value: %q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "internal server error") {
		t.Errorf("body = %q, want the standard error envelope", rec.Body.String())
	}

	logged := buf.String()
	for _, want := range []string{"panic recovered in handler", "boom", "/v1/events"} {
		if !strings.Contains(logged, want) {
			t.Errorf("expected log to contain %q, got %q", want, logged)
		}
	}
	if !strings.Contains(logged, "stack") {
		t.Error("expected the recovery log to carry a stack trace")
	}
}

func TestRecoveryMiddlewarePreservesAlreadyWrittenResponse(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := recoveryMiddlewareWithLogger(logger, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":`))
		panic("mid-stream failure")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/events", nil))

	// The status line was already on the wire — it must not be rewritten to 500.
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (already-framed response must be left alone)", rec.Code, http.StatusOK)
	}
	if strings.Contains(rec.Body.String(), "internal server error") {
		t.Errorf("recovery appended an error envelope to a partial body: %q", rec.Body.String())
	}
	if !strings.Contains(buf.String(), "panic recovered in handler") {
		t.Errorf("panic must still be logged even when no 500 can be sent, got %q", buf.String())
	}
}

func TestRecoveryMiddlewareRepanicsErrAbortHandler(t *testing.T) {
	handler := recoveryMiddlewareWithLogger(slog.Default(), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected http.ErrAbortHandler to propagate so net/http can handle it")
		}
		if recovered != http.ErrAbortHandler {
			t.Fatalf("recovered = %v, want http.ErrAbortHandler", recovered)
		}
	}()

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/events", nil))
}
