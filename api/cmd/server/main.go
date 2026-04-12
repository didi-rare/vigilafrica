package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

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

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(version)

	// F-001: Health endpoint
	// Spec: GET /health → {"status":"ok","version":"<semver>"}
	mux.Handle("GET /health", healthHandler)

	addr := fmt.Sprintf(":%s", port)
	slog.Info("VigilAfrica API starting", "addr", addr, "version", version)

	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server failed to start", "err", err)
		os.Exit(1)
	}
}

