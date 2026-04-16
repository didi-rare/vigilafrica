package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/geoip"
	"vigilafrica/api/internal/handlers"
	"vigilafrica/api/internal/ingestor"
)

// version is injected at build time via:
// go build -ldflags "-X main.version=0.5.0" ./cmd/server/
var version = "0.5.0"

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
	alertCfg, _ := ingestor.LoadAlertConfig()
	ingestor.StartScheduler(ctx, repo, alertCfg)
	ingestor.StartStalenessWatchdog(ctx, repo, alertCfg)

	// ── Middleware ────────────────────────────────────────────────────────────
	cache := handlers.NewResponseCache()

	// ── Handlers ──────────────────────────────────────────────────────────────
	healthHandler := handlers.NewHealthHandler(version, repo)
	eventHandler := handlers.NewEventHandler(repo)

	// ── Router ────────────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// /health — exempt from rate limit and cache (monitoring must always respond)
	mux.Handle("GET /health", healthHandler)

	// /v1/events — rate limited + cached
	mux.Handle("GET /v1/events",
		cache.CacheMiddleware(http.HandlerFunc(eventHandler.ListEvents)),
	)
	mux.HandleFunc("GET /v1/events/{id}", eventHandler.GetEventByID)

	// /v1/context
	mux.HandleFunc("GET /v1/context", handlers.GetContext(repo, geoReader))

	// Global middleware chain: CORS → rate limit → router
	var globalHandler http.Handler = mux
	globalHandler = handlers.RateLimitMiddleware(globalHandler)
	globalHandler = handlers.CORSMiddleware(globalHandler)

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
