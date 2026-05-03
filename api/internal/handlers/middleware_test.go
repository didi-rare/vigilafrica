package handlers

import (
	"net"
	"net/http"
	"net/http/httptest"
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
