package handlers

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// This file covers the rate limiter's CLIENT IDENTITY, which
// fix-unify-client-ip-resolution task 5.5 claimed was tested and was not. The
// limiter's resolution was already correct; these are regression guards so a
// future change to the shared resolver cannot silently collapse every caller
// into one bucket while /health and the deploy smoke test stay green.

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

// limiterFor builds a handler that exhausts after `rpm` requests per identity,
// mirroring rateLimitMiddlewareFromEnv but with injected configuration so the
// test does not depend on the environment (§9.5).
func limiterFor(rpm int, proxies ProxyConfig) http.Handler {
	limiter := newIPRateLimiterWithOptions(rpm, defaultRateLimitMaxBuckets, 0)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.bucketFor(proxies.ClientIP(r)).allow() {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func drain(t *testing.T, h http.Handler, remoteAddr string, headers map[string]string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
		req.RemoteAddr = remoteAddr
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d/%d unexpectedly limited (%d)", i+1, n, rec.Code)
		}
	}
}

func status(h http.Handler, remoteAddr string, headers map[string]string) int {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	h.ServeHTTP(rec, req)
	return rec.Code
}

// TestRateLimitBucketsAreDistinctPerForwardedClient is task 5.5.
//
// Two different clients arriving through the SAME trusted proxy must not share
// a bucket. If the resolver ever regressed to keying on the proxy address, one
// noisy client would throttle everyone — invisibly, since the endpoint would
// still return 200 for the first caller.
func TestRateLimitBucketsAreDistinctPerForwardedClient(t *testing.T) {
	const rpm = 3
	proxies := identityTestProxies(t)
	h := limiterFor(rpm, proxies)

	const trustedProxy = "172.18.0.1:40000"
	clientA := map[string]string{"X-Forwarded-For": "41.58.100.7"}
	clientB := map[string]string{"X-Forwarded-For": "102.89.10.4"}

	// Exhaust client A entirely.
	drain(t, h, trustedProxy, clientA, rpm)
	if got := status(h, trustedProxy, clientA); got != http.StatusTooManyRequests {
		t.Fatalf("client A after %d requests = %d, want 429", rpm, got)
	}

	// Client B, same proxy, must be unaffected.
	if got := status(h, trustedProxy, clientB); got != http.StatusOK {
		t.Errorf("client B = %d, want 200 — distinct clients through one proxy must not share a bucket", got)
	}
}

// TestRateLimitIdentityCannotBeForgedByUntrustedPeer is task 5.6.
//
// A caller that is not a trusted proxy must not be able to escape its own
// bucket by rotating X-Forwarded-For. This behaviour predates the change; it is
// asserted here because nothing did so before.
func TestRateLimitIdentityCannotBeForgedByUntrustedPeer(t *testing.T) {
	const rpm = 3
	h := limiterFor(rpm, identityTestProxies(t))

	const untrustedPeer = "203.0.113.9:51000"

	// Exhaust the peer's bucket while claiming to be someone else each time.
	for i, spoof := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		if got := status(h, untrustedPeer, map[string]string{"X-Forwarded-For": spoof}); got != http.StatusOK {
			t.Fatalf("request %d unexpectedly limited (%d)", i+1, got)
		}
	}

	// A fourth attempt with yet another forged identity must still be refused:
	// all four were accounted to the peer, not to the claimed addresses.
	if got := status(h, untrustedPeer, map[string]string{"X-Forwarded-For": "4.4.4.4"}); got != http.StatusTooManyRequests {
		t.Errorf("got %d, want 429 — an untrusted peer must not rotate its way out of its own bucket", got)
	}
}
