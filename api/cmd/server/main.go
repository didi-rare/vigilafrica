package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/handlers"
)

// version is injected at build time via:
// go build -ldflags "-X main.version=0.1.0" ./cmd/server/
var version = "0.1.0"

func main() {
	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		slog.Error("DATABASE_URL is not set")
		os.Exit(1)
	}

	ctx := context.Background()
	repo, err := database.NewRepository(ctx, dbURL)
	if err != nil {
		slog.Error("database initialization failed", "err", err)
		os.Exit(1)
	}
	defer repo.Close()

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(version)
	eventHandler := handlers.NewEventHandler(repo)

	// F-001: Health endpoint
	// Spec: GET /health → {"status":"ok","version":"<semver>"}
	mux.Handle("GET /health", healthHandler)

	// F-006, F-007: Events endpoints
	mux.HandleFunc("GET /v1/events", eventHandler.ListEvents)
	mux.HandleFunc("GET /v1/events/{id}", eventHandler.GetEventByID)

	addr := fmt.Sprintf(":%s", port)
	slog.Info("VigilAfrica API starting", "addr", addr, "version", version)

	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server failed to start", "err", err)
		os.Exit(1)
	}
}

