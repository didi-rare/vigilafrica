package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHandleHealth validates F-001 acceptance criteria:
// - Returns HTTP 200
// - Body is {"status":"ok","version":"<non-empty>"}
// - Content-Type is application/json
func TestHandleHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handleHealth(rec, req)

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
	var resp healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status field 'ok', got %q", resp.Status)
	}

	if resp.Version == "" {
		t.Error("expected non-empty version field")
	}
}
