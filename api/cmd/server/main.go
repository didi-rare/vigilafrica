package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"vigilafrica/api/internal/alert"
	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/digest"
	"vigilafrica/api/internal/geoip"
	"vigilafrica/api/internal/handlers"
	"vigilafrica/api/internal/ingestor"
)

// version is injected at build time via:
// go build -ldflags "-X main.version=1.1.1" ./cmd/server/
//
// The source-level fallback should match the most recent shipped release so
// that an accidentally ldflag-less build doesn't silently report a wildly
// stale version. Bump alongside each tagged release.
var version = "1.1.1"

func main() {
	// ── Structured JSON logging ───────────────────────────────────────────────
	logLevel := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})))

	// ── Shutdown context (SIGTERM / SIGINT) ───────────────────────────────────
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// ── Database ──────────────────────────────────────────────────────────────
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		slog.Error("DATABASE_URL is not set")
		os.Exit(1)
	}

	repo, err := database.NewRepository(ctx, dbURL)
	if err != nil {
		slog.Error("database initialization failed", "err", err)
		os.Exit(1)
	}
	defer repo.Close()

	// ── GeoIP ─────────────────────────────────────────────────────────────────
	var geoReader *geoip.Reader
	geoIPPath := os.Getenv("GEOIP_DB_PATH")
	if geoIPPath == "" {
		geoIPPath = "/usr/share/GeoIP/GeoLite2-City.mmdb"
	}
	if r, geoErr := geoip.NewReader(geoIPPath); geoErr != nil {
		slog.Warn("GeoIP reader failed — context localization will degrade gracefully", "err", geoErr)
	} else {
		geoReader = r
		defer geoReader.Close()
	}

	// ── Alert config + scheduler + watchdog ───────────────────────────────────
	alertClient := alert.NewClient(loadAlertConfigFromEnv(), slog.Default().With("component", "alert"))
	ingestor.StartScheduler(ctx, repo, alertClient)
	alert.StartStalenessWatchdog(ctx, repo, alertClient, loadWatchdogConfigFromEnv(), slog.Default().With("component", "watchdog"))

	// ── Daily flood digest (feature-daily-flood-digest) ───────────────────────
	// A second Resend client scoped to DIGEST_TO recipients so the digest and
	// the operational alerts have independent recipient lists. No-op when
	// DIGEST_TO is unset.
	digestMailer := alert.NewClient(loadDigestConfigFromEnv(), slog.Default().With("component", "digest"))
	digest.StartDigestScheduler(ctx, repo, digestMailer, loadDigestSchedulerConfigFromEnv(), slog.Default().With("component", "digest"))

	// ── Middleware ────────────────────────────────────────────────────────────
	cache := handlers.NewResponseCache()

	// ── Handlers ──────────────────────────────────────────────────────────────
	healthHandler := handlers.NewHealthHandler(version, repo)
	eventHandler := handlers.NewEventHandler(repo, slog.Default().With("component", "events"))
	enrichmentStatsHandler := handlers.NewEnrichmentStatsHandler(repo)
	digestHandler := handlers.NewDigestHandler(repo, slog.Default().With("component", "digest"))

	// ── Router ────────────────────────────────────────────────────────────────
	// v1 sub-mux: all /v1/* routes go through rate limiting.
	// /health, /live, /ready, and docs are also covered by a lighter global limiter.
	v1Mux := http.NewServeMux()
	v1Mux.Handle("GET /v1/events",
		cache.CacheMiddleware(http.HandlerFunc(eventHandler.ListEvents)),
	)
	v1Mux.HandleFunc("GET /v1/events/{id}", eventHandler.GetEventByID)
	v1Mux.HandleFunc("GET /v1/context", handlers.GetContext(repo, geoReader))
	v1Mux.Handle("GET /v1/enrichment-stats", enrichmentStatsHandler)
	v1Mux.HandleFunc("GET /v1/states", handlers.StatesHandler(repo))
	v1Mux.HandleFunc("GET /v1/digest/today.json", digestHandler.GetTodayDigest)

	mux := http.NewServeMux()
	mux.Handle("GET /live", handlers.LiveHandler(version))
	mux.Handle("GET /health", healthHandler)
	mux.Handle("GET /ready", handlers.NewReadinessHandler(version, repo))
	mux.Handle("GET /openapi.yaml", handlers.OpenAPISpecHandler()) // local docs/testing
	mux.Handle("GET /docs", handlers.SwaggerUIHandler())           // local docs/testing
	mux.Handle("GET /docs/", handlers.SwaggerUIHandler())          // local docs/testing
	mux.Handle("/v1/", handlers.RateLimitMiddleware(v1Mux))        // rate-limited v1 routes

	// Global middleware chain, outermost first: panic recovery, security headers,
	// CORS, and a light public limiter wrap everything. Recovery is outermost so
	// it also catches panics raised inside the other middleware; the security
	// headers it wraps are already set on the ResponseWriter by the time a
	// recovered 500 is written, so they still apply. See developers-go.md §6.7.
	globalHandler := handlers.RecoveryMiddleware(
		handlers.SecurityHeadersMiddleware(
			handlers.CORSMiddleware(
				handlers.GlobalRateLimitMiddleware(mux),
			),
		),
	)

	// ── HTTP server ───────────────────────────────────────────────────────────
	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      globalHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("VigilAfrica API starting", "addr", srv.Addr, "version", version)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// Block until shutdown signal
	<-ctx.Done()
	slog.Info("shutdown signal received — draining requests")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown error", "err", err)
	}
	slog.Info("server stopped cleanly")
}

func loadAlertConfigFromEnv() alert.Config {
	return alert.Config{
		ResendAPIKey: os.Getenv("RESEND_API_KEY"),
		FromEmail:    envOrDefault("ALERT_FROM_EMAIL", "VigilAfrica Alerts <alerts@vigilafrica.org>"),
		ToEmails:     alert.ParseRecipients(envOrDefaultTrimmed("ALERTS_TO", os.Getenv("ALERT_EMAIL_TO"))),
		Environment:  envOrDefaultTrimmed("APP_ENV", "unknown"),
	}
}

// loadDigestConfigFromEnv builds the Resend config for the daily digest. It
// reuses RESEND_API_KEY but has its own recipient list (DIGEST_TO) and From
// address (DIGEST_FROM), so the digest and operational alerts stay independent.
// Empty DIGEST_TO leaves the client disabled (no-op) — local/CI never send.
func loadDigestConfigFromEnv() alert.Config {
	return alert.Config{
		ResendAPIKey: os.Getenv("RESEND_API_KEY"),
		FromEmail:    envOrDefault("DIGEST_FROM", "VigilAfrica Digest <digest@vigilafrica.org>"),
		ToEmails:     alert.ParseRecipients(os.Getenv("DIGEST_TO")),
		Environment:  envOrDefaultTrimmed("APP_ENV", "unknown"),
	}
}

func loadDigestSchedulerConfigFromEnv() digest.SchedulerConfig {
	return digest.SchedulerConfig{
		Hour:        envHourOfDay("DIGEST_SCHEDULE_HOUR", 6),
		Environment: envOrDefaultTrimmed("APP_ENV", "unknown"),
	}
}

func loadWatchdogConfigFromEnv() alert.WatchdogConfig {
	return alert.WatchdogConfig{
		CheckInterval:      time.Duration(envPositiveInt("ALERT_STALENESS_CHECK_INTERVAL_MIN", 15)) * time.Minute,
		StalenessThreshold: time.Duration(envPositiveInt("ALERT_STALENESS_THRESHOLD_HOURS", 2)) * time.Hour,
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envOrDefaultTrimmed(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

// envHourOfDay parses a UTC hour-of-day (0–23) env var, falling back on unset
// or out-of-range values.
func envHourOfDay(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 || parsed > 23 {
		slog.Warn("invalid hour-of-day env var; using default", "key", key, "value", value, "default", fallback)
		return fallback
	}
	return parsed
}

func envPositiveInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		slog.Warn("invalid positive integer env var; using default", "key", key, "value", value, "default", fallback)
		return fallback
	}
	return parsed
}
