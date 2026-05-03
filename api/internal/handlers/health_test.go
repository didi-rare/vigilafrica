package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/models"
)

type healthTestRepo struct {
	last      *models.IngestionRun
	byCountry map[string]*models.IngestionRun
}

func (r *healthTestRepo) UpsertEvent(context.Context, models.Event, string) error { return nil }
func (r *healthTestRepo) ListEvents(context.Context, database.EventFilters) ([]models.Event, int, error) {
	return nil, 0, nil
}
func (r *healthTestRepo) GetEventByID(context.Context, uuid.UUID) (*models.Event, error) {
	return nil, nil
}
func (r *healthTestRepo) GetNearbyEvents(context.Context, float64, float64, float64, int) ([]models.Event, error) {
	return nil, nil
}
func (r *healthTestRepo) CreateIngestionRun(context.Context, time.Time, string) (int64, error) {
	return 0, nil
}
func (r *healthTestRepo) CompleteIngestionRun(context.Context, int64, models.IngestionRunStatus, int, int, *string) error {
	return nil
}
func (r *healthTestRepo) GetLastIngestionRun(context.Context) (*models.IngestionRun, error) {
	return r.last, nil
}
func (r *healthTestRepo) GetLastSuccessfulIngestionRun(context.Context) (*models.IngestionRun, error) {
	return nil, nil
}
func (r *healthTestRepo) GetFirstIngestionRun(context.Context) (*models.IngestionRun, error) {
	return nil, nil
}
func (r *healthTestRepo) GetLastIngestionRunAllCountries(context.Context) (map[string]*models.IngestionRun, error) {
	return r.byCountry, nil
}
func (r *healthTestRepo) GetEnrichmentStats(context.Context) ([]database.EnrichmentStat, error) {
	return nil, nil
}
func (r *healthTestRepo) GetDistinctStatesByCountry(context.Context, string) ([]string, error) {
	return nil, nil
}
func (r *healthTestRepo) Close() {}

func TestHealthHandler_ServeHTTP(t *testing.T) {
	version := "1.2.3"
	handler := NewHealthHandler(version, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check status code
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Check content type
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type %q, got %q", "application/json", ct)
	}

	// Check body
	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status %q, got %q", "ok", resp.Status)
	}

	if resp.Version != version {
		t.Errorf("expected version %q, got %q", version, resp.Version)
	}
}

func TestHealthHandlerRedactsIngestionErrors(t *testing.T) {
	errorMessage := "dial tcp 10.0.0.5:443: connection refused"
	startedAt := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	repo := &healthTestRepo{
		last: &models.IngestionRun{
			CountryCode: "NG",
			StartedAt:   startedAt,
			Status:      models.RunStatusFailure,
			Error:       &errorMessage,
		},
		byCountry: map[string]*models.IngestionRun{
			"NG": &models.IngestionRun{
				CountryCode: "NG",
				StartedAt:   startedAt,
				Status:      models.RunStatusFailure,
				Error:       &errorMessage,
			},
		},
	}
	handler := NewHealthHandler("test", repo)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected /health to stay 200, got %d", w.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "degraded" {
		t.Fatalf("expected degraded status, got %q", resp.Status)
	}
	if resp.LastIngestion == nil {
		t.Fatal("expected last ingestion metadata")
	}
	if resp.LastIngestion.Error != nil {
		t.Fatalf("expected public health error redacted, got %q", *resp.LastIngestion.Error)
	}
	if resp.LastIngestion.StartedAt != nil {
		t.Fatalf("expected public health started_at redacted, got %v", resp.LastIngestion.StartedAt)
	}
	if resp.LastIngestion.EventsFetched != nil {
		t.Fatalf("expected public health events_fetched redacted, got %d", *resp.LastIngestion.EventsFetched)
	}
	if resp.LastIngestion.EventsStored != nil {
		t.Fatalf("expected public health events_stored redacted, got %d", *resp.LastIngestion.EventsStored)
	}
	if resp.LastIngestionByCountry["NG"].Error != nil {
		t.Fatalf("expected per-country public health error redacted, got %q", *resp.LastIngestionByCountry["NG"].Error)
	}
}

func TestReadinessHandlerFailsWhenDegraded(t *testing.T) {
	startedAt := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	repo := &healthTestRepo{
		last: &models.IngestionRun{
			CountryCode: "NG",
			StartedAt:   startedAt,
			Status:      models.RunStatusFailure,
		},
	}
	handler := NewReadinessHandler("test", repo)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected /ready status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
}
