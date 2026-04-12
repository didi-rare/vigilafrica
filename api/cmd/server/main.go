package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
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

	// F-001: Health endpoint
	// Spec: GET /health → {"status":"ok","version":"<semver>"}
	// Contract: api-contract.md §2 — stateless, < 100ms, no DB dependency
	mux.HandleFunc("GET /health", handleHealth)

	addr := fmt.Sprintf(":%s", port)
	slog.Info("VigilAfrica API starting", "addr", addr, "version", version)

	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server failed to start", "err", err)
		os.Exit(1)
	}
}

// healthResponse is the response body for GET /health.
// Schema locked in api-contract.md §2.
type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// handleHealth implements F-001.
// Acceptance criteria:
//   - Returns HTTP 200
//   - Body: {"status":"ok","version":"<semver>"}
//   - No database dependency
//   - Response < 100ms
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(healthResponse{
		Status:  "ok",
		Version: version,
	}); err != nil {
		slog.Error("failed to encode health response", "err", err)
	}
}
