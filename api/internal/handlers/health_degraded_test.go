package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"vigilafrica/api/internal/models"
)

// fix-gdacs-degraded-run-status: a GDACS outage must be visible on /health,
// but must NOT make /ready fail — an upstream data provider being down does
// not make this API unable to serve requests.

func healthStatus(t *testing.T, h http.Handler, path string) (int, string) {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return w.Code, body.Status
}

func runWith(code string, s models.IngestionRunStatus) *models.IngestionRun {
	return &models.IngestionRun{CountryCode: code, StartedAt: time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC), Status: s}
}

func TestHealthReportsDegradedRun(t *testing.T) {
	repo := &healthTestRepo{
		last: runWith("GH", models.RunStatusSuccess),
		byCountry: map[string]*models.IngestionRun{
			"NG": runWith("NG", models.RunStatusDegraded),
			"GH": runWith("GH", models.RunStatusSuccess),
		},
	}

	code, status := healthStatus(t, NewHealthHandler("test", repo), "/health")
	if code != http.StatusOK || status != "degraded" {
		t.Errorf("/health = %d %q, want 200 \"degraded\" — a GDACS outage must not read as ok", code, status)
	}

	code, status = healthStatus(t, NewReadinessHandler("test", repo), "/ready")
	if code != http.StatusOK {
		t.Errorf("/ready = %d for a degraded-only state, want 200 — an upstream outage is not unreadiness", code)
	}
	if status != "degraded" {
		t.Errorf("/ready body status = %q, want \"degraded\" (still reported, just not 503)", status)
	}
}

func TestReadinessStillFailsOnRealFailureAlongsideDegraded(t *testing.T) {
	repo := &healthTestRepo{
		last: runWith("NG", models.RunStatusDegraded),
		byCountry: map[string]*models.IngestionRun{
			"NG": runWith("NG", models.RunStatusDegraded),
			"GH": runWith("GH", models.RunStatusFailure),
		},
	}
	if code, _ := healthStatus(t, NewReadinessHandler("test", repo), "/ready"); code != http.StatusServiceUnavailable {
		t.Errorf("/ready = %d with a failed country, want 503 — degraded must not mask a real failure", code)
	}
}

func TestHealthOkWhenAllSucceed(t *testing.T) {
	repo := &healthTestRepo{
		last:      runWith("NG", models.RunStatusSuccess),
		byCountry: map[string]*models.IngestionRun{"NG": runWith("NG", models.RunStatusSuccess)},
	}
	if code, status := healthStatus(t, NewHealthHandler("test", repo), "/health"); code != http.StatusOK || status != "ok" {
		t.Errorf("/health = %d %q, want 200 \"ok\"", code, status)
	}
}
