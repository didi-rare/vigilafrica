package handlers

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// ─── CORS Middleware ──────────────────────────────────────────────────────────

// CORSMiddleware sets the Access-Control-Allow-Origin header to the value of
// the CORS_ORIGIN environment variable. Defaults to "*" if unset.
// Only the configured origin is allowed in production (ADR-002).
func CORSMiddleware(next http.Handler) http.Handler {
	origin := os.Getenv("CORS_ORIGIN")
	if origin == "" {
		origin = "*"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ─── Rate Limiter ─────────────────────────────────────────────────────────────

// tokenBucket implements a per-client token-bucket rate limiter.
// Tokens refill at rpm/minute.
type tokenBucket struct {
	mu       sync.Mutex
	tokens   float64
	maxToken float64
	refillPS float64
	lastTime time.Time
}

func newTokenBucket(rpm int) *tokenBucket {
	rps := float64(rpm) / 60.0
	return &tokenBucket{
		tokens:   float64(rpm),
		maxToken: float64(rpm),
		refillPS: rps,
		lastTime: time.Now(),
	}
}

func (tb *tokenBucket) allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastTime).Seconds()
	tb.lastTime = now

	tb.tokens += elapsed * tb.refillPS
	if tb.tokens > tb.maxToken {
		tb.tokens = tb.maxToken
	}

	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

// ipRateLimiter holds a per-IP token bucket. Buckets are created lazily on
// first request from a given IP and kept for the lifetime of the process.
type ipRateLimiter struct {
	mu      sync.Mutex
	rpm     int
	buckets map[string]*tokenBucket
}

func newIPRateLimiter(rpm int) *ipRateLimiter {
	if rpm <= 0 {
		rpm = 60
	}
	return &ipRateLimiter{
		rpm:     rpm,
		buckets: make(map[string]*tokenBucket),
	}
}

func (l *ipRateLimiter) bucketFor(ip string) *tokenBucket {
	l.mu.Lock()
	defer l.mu.Unlock()
	tb, ok := l.buckets[ip]
	if !ok {
		tb = newTokenBucket(l.rpm)
		l.buckets[ip] = tb
	}
	return tb
}

// clientIP extracts the client IP from the request. Prefers X-Forwarded-For
// (first hop) when present — required behind reverse proxies like Caddy.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First entry is the originating client
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return trimSpace(xff[:i])
			}
		}
		return trimSpace(xff)
	}
	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return trimSpace(xr)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// RateLimitMiddleware wraps a handler with a per-IP token-bucket rate limiter.
// The limit is read from RATE_LIMIT_RPM (default: 60 requests/min per IP).
// Returns HTTP 429 when a client's bucket is empty.
func RateLimitMiddleware(next http.Handler) http.Handler {
	rpm := 60
	if v := os.Getenv("RATE_LIMIT_RPM"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			rpm = n
		}
	}
	limiter := newIPRateLimiter(rpm)
	slog.Info("rate limiter: initialised", "rpm_per_ip", rpm)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !limiter.bucketFor(ip).allow() {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ─── Response Cache ───────────────────────────────────────────────────────────

// cacheEntry holds a cached response body and its expiry time.
type cacheEntry struct {
	body       []byte
	headers    http.Header
	statusCode int
	expiresAt  time.Time
}

// ResponseCache is a simple in-memory cache keyed by request URL path+query.
type ResponseCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	ttl     time.Duration
}

// NewResponseCache returns a cache with the given TTL.
// TTL is read from CACHE_TTL_SECONDS (default: 300 seconds).
func NewResponseCache() *ResponseCache {
	ttlSec := 300
	if v := os.Getenv("CACHE_TTL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			ttlSec = n
		}
	}
	ttl := time.Duration(ttlSec) * time.Second
	slog.Info("response cache: initialised", "ttl_seconds", ttlSec)
	return &ResponseCache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
	}
}

// CacheMiddleware wraps a handler with response caching for GET requests.
// Cache key is the full request URL (path + query string).
// Only caches successful (2xx) responses.
func (c *ResponseCache) CacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only cache GET requests
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		key := r.URL.RequestURI()

		// Check cache
		c.mu.RLock()
		entry, ok := c.entries[key]
		c.mu.RUnlock()

		if ok && time.Now().Before(entry.expiresAt) {
			// Cache hit — copy stored headers then write body
			for k, vals := range entry.headers {
				for _, v := range vals {
					w.Header().Set(k, v)
				}
			}
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(entry.statusCode)
			w.Write(entry.body) //nolint:errcheck
			return
		}

		// Cache miss — capture the response
		rec := &responseRecorder{
			ResponseWriter: w,
			body:           &bytes.Buffer{},
			statusCode:     http.StatusOK,
		}
		next.ServeHTTP(rec, r)

		// Only cache 2xx responses
		if rec.statusCode >= 200 && rec.statusCode < 300 {
			// Snapshot headers that were actually set on the recorder
			snapHeaders := rec.ResponseWriter.Header().Clone()
			c.mu.Lock()
			c.entries[key] = &cacheEntry{
				body:       rec.body.Bytes(),
				headers:    snapHeaders,
				statusCode: rec.statusCode,
				expiresAt:  time.Now().Add(c.ttl),
			}
			c.mu.Unlock()
		}
	})
}

// responseRecorder captures the status code and body written by the next handler.
type responseRecorder struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
	written    bool
}

func (r *responseRecorder) WriteHeader(code int) {
	if !r.written {
		r.statusCode = code
		r.written = true
		r.ResponseWriter.WriteHeader(code)
	}
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b) //nolint:errcheck
	return r.ResponseWriter.Write(b)
}
