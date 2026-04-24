package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"vigilafrica/api/internal/alert"
	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/geoip"
	"vigilafrica/api/internal/handlers"
	"vigilafrica/api/internal/ingestor"
)

// version is injected at build time via:
// go build -ldflags "-X main.version=0.5.0" ./cmd/server/
var version = "0.7.0"

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

	// ── Middleware ────────────────────────────────────────────────────────────
	cache := handlers.NewResponseCache()

	// ── Handlers ──────────────────────────────────────────────────────────────
	healthHandler := handlers.NewHealthHandler(version, repo)
	eventHandler := handlers.NewEventHandler(repo)
	enrichmentStatsHandler := handlers.NewEnrichmentStatsHandler(repo)

	// ── Router ────────────────────────────────────────────────────────────────
	// v1 sub-mux: all /v1/* routes go through rate limiting.
	// /health and local API docs are registered on the root mux so they are never rate-limited —
	// uptime monitors and local manual testing must always be able to reach them.
	v1Mux := http.NewServeMux()
	v1Mux.Handle("GET /v1/events",
		cache.CacheMiddleware(http.HandlerFunc(eventHandler.ListEvents)),
	)
	v1Mux.HandleFunc("GET /v1/events/{id}", eventHandler.GetEventByID)
	v1Mux.HandleFunc("GET /v1/context", handlers.GetContext(repo, geoReader))
	v1Mux.Handle("GET /v1/enrichment-stats", enrichmentStatsHandler)
	v1Mux.HandleFunc("GET /v1/states", handlers.StatesHandler(repo))

	mux := http.NewServeMux()
	mux.Handle("GET /health", healthHandler)                       // exempt from rate limit
	mux.Handle("GET /openapi.yaml", handlers.OpenAPISpecHandler()) // local docs/testing
	mux.Handle("GET /docs", handlers.SwaggerUIHandler())           // local docs/testing
	mux.Handle("GET /docs/", handlers.SwaggerUIHandler())          // local docs/testing
	mux.Handle("/v1/", handlers.RateLimitMiddleware(v1Mux))        // rate-limited v1 routes

	// Global middleware chain: CORS wraps everything (health + v1 + docs)
	globalHandler := handlers.CORSMiddleware(mux)

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
		ToEmail:      os.Getenv("ALERT_EMAIL_TO"),
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
