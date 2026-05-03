package handlers

import (
	"bytes"
	"container/list"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	defaultRateLimitRPM              = 60
	defaultGlobalRateLimitRPM        = 300
	defaultRateLimitMaxBuckets       = 10000
	defaultRateLimitBucketTTLSeconds = 900
	defaultCacheTTLSeconds           = 300
	defaultCacheMaxEntries           = 1000
	defaultCacheMaxQueryBytes        = 1024
)

// ─── CORS Middleware ──────────────────────────────────────────────────────────

// CORSMiddleware allows only the configured browser origin. CORS_ORIGIN
// defaults to "*" for local development.
func CORSMiddleware(next http.Handler) http.Handler {
	origin := trimSpace(os.Getenv("CORS_ORIGIN"))
	if origin == "" {
		origin = "*"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestOrigin := r.Header.Get("Origin")
		allowedOrigin, allowed := allowedCORSOrigin(origin, requestOrigin)
		w.Header().Add("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if !allowed {
			respondWithError(w, http.StatusForbidden, "origin not allowed")
			return
		}
		if allowedOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func allowedCORSOrigin(configuredOrigin, requestOrigin string) (string, bool) {
	if configuredOrigin == "*" {
		return "*", true
	}
	if requestOrigin == "" {
		return "", true
	}
	if requestOrigin == configuredOrigin {
		return configuredOrigin, true
	}
	return "", false
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
	lastSeen time.Time
}

func newTokenBucket(rpm int) *tokenBucket {
	rps := float64(rpm) / 60.0
	return &tokenBucket{
		tokens:   float64(rpm),
		maxToken: float64(rpm),
		refillPS: rps,
		lastTime: time.Now(),
		lastSeen: time.Now(),
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
	mu         sync.Mutex
	rpm        int
	maxBuckets int
	bucketTTL  time.Duration
	buckets    map[string]*tokenBucket
}

func newIPRateLimiter(rpm int) *ipRateLimiter {
	return newIPRateLimiterWithOptions(rpm, defaultRateLimitMaxBuckets, time.Duration(defaultRateLimitBucketTTLSeconds)*time.Second)
}

func newIPRateLimiterWithOptions(rpm, maxBuckets int, bucketTTL time.Duration) *ipRateLimiter {
	if rpm <= 0 {
		rpm = defaultRateLimitRPM
	}
	if maxBuckets <= 0 {
		maxBuckets = defaultRateLimitMaxBuckets
	}
	if bucketTTL <= 0 {
		bucketTTL = time.Duration(defaultRateLimitBucketTTLSeconds) * time.Second
	}
	return &ipRateLimiter{
		rpm:        rpm,
		maxBuckets: maxBuckets,
		bucketTTL:  bucketTTL,
		buckets:    make(map[string]*tokenBucket),
	}
}

func (l *ipRateLimiter) bucketFor(ip string) *tokenBucket {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.evictExpiredLocked(now)
	tb, ok := l.buckets[ip]
	if !ok {
		if len(l.buckets) >= l.maxBuckets {
			l.evictOldestLocked()
		}
		tb = newTokenBucket(l.rpm)
		l.buckets[ip] = tb
	}
	tb.lastSeen = now
	return tb
}

func (l *ipRateLimiter) evictExpiredLocked(now time.Time) {
	for ip, bucket := range l.buckets {
		if now.Sub(bucket.lastSeen) > l.bucketTTL {
			delete(l.buckets, ip)
		}
	}
}

func (l *ipRateLimiter) evictOldestLocked() {
	var oldestIP string
	var oldestSeen time.Time
	for ip, bucket := range l.buckets {
		if oldestIP == "" || bucket.lastSeen.Before(oldestSeen) {
			oldestIP = ip
			oldestSeen = bucket.lastSeen
		}
	}
	if oldestIP != "" {
		delete(l.buckets, oldestIP)
	}
}

// clientIP extracts the client IP from the request. Forwarded headers are only
// trusted when the direct peer is a configured trusted proxy.
func clientIP(r *http.Request) string {
	return clientIPWithTrustedProxies(r, trustedProxyCIDRsFromEnv())
}

func clientIPWithTrustedProxies(r *http.Request, trustedProxies []*net.IPNet) string {
	remoteIP := remoteAddrIP(r.RemoteAddr)
	if remoteIP == "" {
		remoteIP = r.RemoteAddr
	}
	if isTrustedProxy(remoteIP, trustedProxies) {
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
	}
	return remoteIP
}

func remoteAddrIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func trustedProxyCIDRsFromEnv() []*net.IPNet {
	config := trimSpace(os.Getenv("TRUSTED_PROXY_CIDRS"))
	if config == "" {
		config = "127.0.0.1/8,::1/128,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
	}
	var networks []*net.IPNet
	for _, part := range splitComma(config) {
		_, network, err := net.ParseCIDR(part)
		if err != nil {
			slog.Warn("rate limiter: ignoring invalid trusted proxy CIDR", "cidr", part)
			continue
		}
		networks = append(networks, network)
	}
	return networks
}

func isTrustedProxy(ipValue string, trustedProxies []*net.IPNet) bool {
	ip := net.ParseIP(ipValue)
	if ip == nil {
		return false
	}
	for _, network := range trustedProxies {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func splitComma(value string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(value); i++ {
		if value[i] == ',' {
			if part := trimSpace(value[start:i]); part != "" {
				parts = append(parts, part)
			}
			start = i + 1
		}
	}
	if part := trimSpace(value[start:]); part != "" {
		parts = append(parts, part)
	}
	return parts
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
	return rateLimitMiddlewareFromEnv(next, "RATE_LIMIT_RPM", defaultRateLimitRPM)
}

// GlobalRateLimitMiddleware applies a lighter limit to public non-v1 endpoints
// such as /health, /ready, /docs, and /openapi.yaml.
func GlobalRateLimitMiddleware(next http.Handler) http.Handler {
	return rateLimitMiddlewareFromEnv(next, "GLOBAL_RATE_LIMIT_RPM", defaultGlobalRateLimitRPM)
}

func rateLimitMiddlewareFromEnv(next http.Handler, rpmEnv string, fallbackRPM int) http.Handler {
	rpm := positiveIntFromEnv(rpmEnv, fallbackRPM)
	maxBuckets := positiveIntFromEnv("RATE_LIMIT_MAX_BUCKETS", defaultRateLimitMaxBuckets)
	bucketTTL := time.Duration(positiveIntFromEnv("RATE_LIMIT_BUCKET_TTL_SECONDS", defaultRateLimitBucketTTLSeconds)) * time.Second
	limiter := newIPRateLimiterWithOptions(rpm, maxBuckets, bucketTTL)
	slog.Info("rate limiter: initialised", "rpm_env", rpmEnv, "rpm_per_ip", rpm, "max_buckets", maxBuckets, "bucket_ttl_seconds", int(bucketTTL.Seconds()))
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

func positiveIntFromEnv(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

// ─── Response Cache ───────────────────────────────────────────────────────────

// cacheEntry holds a cached response body and its expiry time.
type cacheEntry struct {
	body       []byte
	headers    http.Header
	statusCode int
	expiresAt  time.Time
	keyElement *list.Element
}

// ResponseCache is a simple in-memory cache keyed by request URL path+query.
type ResponseCache struct {
	mu            sync.RWMutex
	entries       map[string]*cacheEntry
	order         *list.List
	ttl           time.Duration
	maxEntries    int
	maxQueryBytes int
}

// NewResponseCache returns a cache with the given TTL.
// TTL is read from CACHE_TTL_SECONDS (default: 300 seconds).
func NewResponseCache() *ResponseCache {
	ttlSec := positiveIntFromEnv("CACHE_TTL_SECONDS", defaultCacheTTLSeconds)
	maxEntries := positiveIntFromEnv("CACHE_MAX_ENTRIES", defaultCacheMaxEntries)
	maxQueryBytes := positiveIntFromEnv("CACHE_MAX_QUERY_BYTES", defaultCacheMaxQueryBytes)
	ttl := time.Duration(ttlSec) * time.Second
	slog.Info("response cache: initialised", "ttl_seconds", ttlSec, "max_entries", maxEntries, "max_query_bytes", maxQueryBytes)
	return &ResponseCache{
		entries:       make(map[string]*cacheEntry),
		order:         list.New(),
		ttl:           ttl,
		maxEntries:    maxEntries,
		maxQueryBytes: maxQueryBytes,
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

		if len(r.URL.RawQuery) > c.maxQueryBytes {
			respondWithError(w, http.StatusRequestURITooLong, "query string too long")
			return
		}

		key := normalizedCacheKey(r)

		// Check cache
		c.mu.Lock()
		entry, ok := c.entries[key]
		if ok && time.Now().After(entry.expiresAt) {
			c.removeLocked(key, entry)
			ok = false
		}
		if ok {
			c.order.MoveToFront(entry.keyElement)
		}
		c.mu.Unlock()

		if ok {
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
			if existing, ok := c.entries[key]; ok {
				c.removeLocked(key, existing)
			}
			element := c.order.PushFront(key)
			c.entries[key] = &cacheEntry{
				body:       rec.body.Bytes(),
				headers:    snapHeaders,
				statusCode: rec.statusCode,
				expiresAt:  time.Now().Add(c.ttl),
				keyElement: element,
			}
			c.evictOverflowLocked()
			c.mu.Unlock()
		}
	})
}

func normalizedCacheKey(r *http.Request) string {
	if r.URL.RawQuery == "" {
		return r.URL.Path
	}
	return r.URL.Path + "?" + r.URL.Query().Encode()
}

func (c *ResponseCache) removeLocked(key string, entry *cacheEntry) {
	delete(c.entries, key)
	if entry.keyElement != nil {
		c.order.Remove(entry.keyElement)
	}
}

func (c *ResponseCache) evictOverflowLocked() {
	for len(c.entries) > c.maxEntries {
		back := c.order.Back()
		if back == nil {
			return
		}
		key, ok := back.Value.(string)
		if !ok {
			c.order.Remove(back)
			continue
		}
		if entry, exists := c.entries[key]; exists {
			c.removeLocked(key, entry)
		} else {
			c.order.Remove(back)
		}
	}
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
