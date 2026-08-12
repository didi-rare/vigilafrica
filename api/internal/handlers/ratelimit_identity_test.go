package handlers

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Covers the rate limiter's CLIENT IDENTITY, which fix-unify-client-ip-resolution
// task 5.5 originally claimed was tested and was not.
//
// ⚠️ These tests drive rateLimitMiddlewareWithConfig — the SAME function
// production uses, differing only in where the numbers come from. An earlier
// revision built an equivalent handler inline instead; independent review caught
// that it would still pass if the production middleware keyed on the proxy, the
// peer, or a constant. That is the identical defect class this change exists to
// remove, one layer up: a test that reimplements the wiring cannot detect the
// wiring being wrong.

func identityTestProxies(t *testing.T) ProxyConfig {
	t.Helper()
	var out []*net.IPNet
	for _, c := range []string{"127.0.0.1/8", "172.16.0.0/12"} {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			t.Fatalf("bad CIDR %q: %v", c, err)
		}
		out = append(out, n)
	}
	return ProxyConfig{Trusted: out}
}

// productionLimiter builds the real middleware with injected configuration.
func productionLimiter(t *testing.T, rpm int) http.Handler {
	t.Helper()
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	return rateLimitMiddlewareWithConfig(ok, rpm, defaultRateLimitMaxBuckets, time.Hour, identityTestProxies(t))
}

func callWith(h http.Handler, remoteAddr string, headers map[string]string) int {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	h.ServeHTTP(rec, req)
	return rec.Code
}

func drain(t *testing.T, h http.Handler, remoteAddr string, headers map[string]string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if got := callWith(h, remoteAddr, headers); got != http.StatusOK {
			t.Fatalf("request %d/%d unexpectedly limited (%d)", i+1, n, got)
		}
	}
}

// TestRateLimitBucketsAreDistinctPerForwardedClient is task 5.5.
//
// Two clients arriving through the SAME trusted proxy must not share a bucket.
// If resolution regressed to keying on the proxy address, one noisy client would
// throttle everyone — invisibly, since the first caller still gets 200.
func TestRateLimitBucketsAreDistinctPerForwardedClient(t *testing.T) {
	const rpm = 3
	h := productionLimiter(t, rpm)

	const trustedProxy = "172.18.0.1:40000"
	clientA := map[string]string{"X-Forwarded-For": "41.58.100.7"}
	clientB := map[string]string{"X-Forwarded-For": "102.89.10.4"}

	drain(t, h, trustedProxy, clientA, rpm)
	if got := callWith(h, trustedProxy, clientA); got != http.StatusTooManyRequests {
		t.Fatalf("client A after %d requests = %d, want 429", rpm, got)
	}

	if got := callWith(h, trustedProxy, clientB); got != http.StatusOK {
		t.Errorf("client B = %d, want 200 — distinct clients through one proxy must not share a bucket", got)
	}
}

// TestRateLimitIdentityCannotBeForgedByUntrustedPeer is task 5.6.
//
// A caller that is not a trusted proxy must not escape its own bucket by
// rotating X-Forwarded-For.
func TestRateLimitIdentityCannotBeForgedByUntrustedPeer(t *testing.T) {
	const rpm = 3
	h := productionLimiter(t, rpm)

	const untrustedPeer = "203.0.113.9:51000"

	for i, spoof := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		if got := callWith(h, untrustedPeer, map[string]string{"X-Forwarded-For": spoof}); got != http.StatusOK {
			t.Fatalf("request %d unexpectedly limited (%d)", i+1, got)
		}
	}

	if got := callWith(h, untrustedPeer, map[string]string{"X-Forwarded-For": "4.4.4.4"}); got != http.StatusTooManyRequests {
		t.Errorf("got %d, want 429 — an untrusted peer must not rotate its way out of its own bucket", got)
	}
}

// TestRateLimitMiddlewareRejectsWithJSON pins the 429 contract, so a change to
// the limiter's response shape cannot pass unnoticed.
func TestRateLimitMiddlewareRejectsWithJSON(t *testing.T) {
	h := productionLimiter(t, 1)
	const peer = "203.0.113.50:1"

	if got := callWith(h, peer, nil); got != http.StatusOK {
		t.Fatalf("first request = %d, want 200", got)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	req.RemoteAddr = peer
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second request = %d, want 429", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if ra := rec.Header().Get("Retry-After"); ra == "" {
		t.Error("Retry-After header missing on 429")
	}
}
