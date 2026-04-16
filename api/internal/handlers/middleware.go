package handlers

import (
	"bytes"
	"encoding/json"
	"log/slog"
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

// rateLimiter implements a simple global token-bucket rate limiter.
// Tokens refill at rpm/minute. All requests share a single bucket (global limit).
type rateLimiter struct {
	mu       sync.Mutex
	tokens   float64
	maxToken float64
	refillPS float64 // tokens per second
	lastTime time.Time
}

func newRateLimiter(rpm int) *rateLimiter {
	if rpm <= 0 {
		rpm = 60
	}
	rps := float64(rpm) / 60.0
	return &rateLimiter{
		tokens:   float64(rpm),
		maxToken: float64(rpm),
		refillPS: rps,
		lastTime: time.Now(),
	}
}

func (rl *rateLimiter) allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastTime).Seconds()
	rl.lastTime = now

	rl.tokens += elapsed * rl.refillPS
	if rl.tokens > rl.maxToken {
		rl.tokens = rl.maxToken
	}

	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}
	return false
}

// RateLimitMiddleware wraps a handler with a token-bucket rate limiter.
// The limit is read from RATE_LIMIT_RPM (default: 60 requests/min).
// Returns HTTP 429 when the bucket is empty.
func RateLimitMiddleware(next http.Handler) http.Handler {
	rpm := 60
	if v := os.Getenv("RATE_LIMIT_RPM"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			rpm = n
		}
	}
	limiter := newRateLimiter(rpm)
	slog.Info("rate limiter: initialised", "rpm", rpm)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allow() {
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
// TTL is read from EVENTS_CACHE_TTL_MINUTES (default: 10 minutes).
func NewResponseCache() *ResponseCache {
	ttlMin := 10
	if v := os.Getenv("EVENTS_CACHE_TTL_MINUTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			ttlMin = n
		}
	}
	ttl := time.Duration(ttlMin) * time.Minute
	slog.Info("response cache: initialised", "ttl_minutes", ttlMin)
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
