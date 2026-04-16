package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vigilafrica/api/internal/handlers"
)

// TestHandleHealth validates F-001 acceptance criteria using the real handler.
func TestHandleHealth(t *testing.T) {
	h := handlers.NewHealthHandler(version, nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	// Criterion: returns HTTP 200
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Criterion: Content-Type is application/json
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	// Criterion: body matches {"status":"ok","version":"<semver>"}
	var resp handlers.HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status field 'ok', got %q", resp.Status)
	}

	if resp.Version != version {
		t.Errorf("expected version %q, got %q", version, resp.Version)
	}
}

