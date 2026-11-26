package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"vigilafrica/api/internal/models"
)

type fakeBoundaryStatus struct {
	st  *models.BoundaryStatus
	err error
}

func (f fakeBoundaryStatus) BoundaryStatus(context.Context) (*models.BoundaryStatus, error) {
	return f.st, f.err
}

func serveHealth(t *testing.T, h *HealthHandler) (int, map[string]json.RawMessage) {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	var body map[string]json.RawMessage
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return w.Code, body
}

func TestHealthReportsBoundaries(t *testing.T) {
	src := fakeBoundaryStatus{st: &models.BoundaryStatus{Loaded: 1, Failed: []string{"GH"}, ManifestSHA256: "abc"}}
	code, body := serveHealth(t, NewHealthHandler("t", &healthTestRepo{}).WithBoundaryStatus(src))
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	var got models.BoundaryStatus
	if err := json.Unmarshal(body["boundaries"], &got); err != nil {
		t.Fatalf("boundaries: %v (%s)", err, body["boundaries"])
	}
	if got.Loaded != 1 || len(got.Failed) != 1 || got.Failed[0] != "GH" || got.ManifestSHA256 != "abc" {
		t.Errorf("boundaries = %+v", got)
	}
}

// Spec §2: a failed country does not affect /ready.
func TestReadinessIgnoresFailedBoundaries(t *testing.T) {
	src := fakeBoundaryStatus{st: &models.BoundaryStatus{Loaded: 0, Failed: []string{"NG", "GH"}, ManifestSHA256: "abc"}}
	code, _ := serveHealth(t, NewReadinessHandler("t", &healthTestRepo{}).WithBoundaryStatus(src))
	if code != http.StatusOK {
		t.Errorf("/ready = %d with failed boundaries, want 200", code)
	}
}

// The key follows the null-handling rule: always present, null when unreadable.
func TestHealthBoundariesNullWhenUnreadable(t *testing.T) {
	for name, h := range map[string]*HealthHandler{
		"no source":   NewHealthHandler("t", nil),
		"query error": NewReadinessHandler("t", &healthTestRepo{}).WithBoundaryStatus(fakeBoundaryStatus{err: errors.New("db down")}),
	} {
		code, body := serveHealth(t, h)
		raw, ok := body["boundaries"]
		if !ok || string(raw) != "null" {
			t.Errorf("%s: boundaries = %s (present=%v), want null", name, raw, ok)
		}
		if code != http.StatusOK {
			t.Errorf("%s: status %d, want 200", name, code)
		}
	}
}
