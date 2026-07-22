package handlers

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/models"
)

type listEventsTestRepo struct {
	called            bool
	filters           database.EventFilters
	statesCalled      bool
	lastStatesCountry string
	listErr           error
}

func (r *listEventsTestRepo) UpsertEvent(context.Context, models.Event, string) error {
	return nil
}

func (r *listEventsTestRepo) ListEvents(ctx context.Context, filters database.EventFilters) ([]models.Event, int, error) {
	r.called = true
	r.filters = filters
	if r.listErr != nil {
		return nil, 0, r.listErr
	}
	return []models.Event{}, 0, nil
}

func (r *listEventsTestRepo) GetEventByID(context.Context, uuid.UUID) (*models.Event, error) {
	return nil, nil
}

func (r *listEventsTestRepo) GetNearbyEvents(context.Context, float64, float64, float64, int) ([]models.Event, error) {
	return nil, nil
}

func (r *listEventsTestRepo) CreateIngestionRun(context.Context, time.Time, string) (int64, error) {
	return 0, nil
}

func (r *listEventsTestRepo) CompleteIngestionRun(context.Context, int64, models.IngestionRunStatus, int, int, *string) error {
	return nil
}

func (r *listEventsTestRepo) GetLastIngestionRun(context.Context) (*models.IngestionRun, error) {
	return nil, nil
}

func (r *listEventsTestRepo) GetLastSuccessfulIngestionRun(context.Context) (*models.IngestionRun, error) {
	return nil, nil
}

func (r *listEventsTestRepo) GetFirstIngestionRun(context.Context) (*models.IngestionRun, error) {
	return nil, nil
}

func (r *listEventsTestRepo) GetLastIngestionRunAllCountries(context.Context) (map[string]*models.IngestionRun, error) {
	return nil, nil
}

func (r *listEventsTestRepo) GetEnrichmentStats(context.Context) ([]database.EnrichmentStat, error) {
	return nil, nil
}

func (r *listEventsTestRepo) GetDistinctStatesByCountry(_ context.Context, country string) ([]string, error) {
	r.statesCalled = true
	r.lastStatesCountry = country
	return nil, nil
}

func (r *listEventsTestRepo) Close() {}

func TestListEventsRejectsNonIntegerPagination(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantError string
	}{
		{
			name:      "limit",
			query:     "limit=abc",
			wantError: "invalid limit: must be an integer",
		},
		{
			name:      "offset",
			query:     "offset=abc",
			wantError: "invalid offset: must be an integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &listEventsTestRepo{}
			handler := NewEventHandler(repo, nil)
			req := httptest.NewRequest(http.MethodGet, "/v1/events?"+tt.query, nil)
			rec := httptest.NewRecorder()

			handler.ListEvents(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}
			if !strings.Contains(rec.Body.String(), tt.wantError) {
				t.Fatalf("expected body to contain %q, got %q", tt.wantError, rec.Body.String())
			}
			if repo.called {
				t.Fatal("expected invalid pagination to stop before repository access")
			}
		})
	}
}

func TestListEventsUsesValidPagination(t *testing.T) {
	repo := &listEventsTestRepo{}
	handler := NewEventHandler(repo, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/events?limit=25&offset=3", nil)
	rec := httptest.NewRecorder()

	handler.ListEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !repo.called {
		t.Fatal("expected repository to be called for valid pagination")
	}
	if repo.filters.Limit != 25 {
		t.Fatalf("expected limit 25, got %d", repo.filters.Limit)
	}
	if repo.filters.Offset != 3 {
		t.Fatalf("expected offset 3, got %d", repo.filters.Offset)
	}
}

// TestListEventsCountryFilter exercises the country/country_code input
// hardening from fix-api-country-filter. Covers B1-B8 from the spec.
func TestListEventsCountryFilter(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		wantStatus  int
		wantCountry string // expected filters.Country passed to the repo
		wantBody    string // substring expected in response body (for 400s)
	}{
		{name: "B1 canonical name", query: "country=Nigeria", wantStatus: http.StatusOK, wantCountry: "Nigeria"},
		{name: "B2 lowercase name", query: "country=nigeria", wantStatus: http.StatusOK, wantCountry: "Nigeria"},
		{name: "B3 ISO code", query: "country_code=NG", wantStatus: http.StatusOK, wantCountry: "Nigeria"},
		{name: "B4 lowercase code", query: "country_code=ng", wantStatus: http.StatusOK, wantCountry: "Nigeria"},
		{name: "B5 both — code wins", query: "country=Ghana&country_code=NG", wantStatus: http.StatusOK, wantCountry: "Nigeria"},
		{name: "B6 unknown code", query: "country_code=XX", wantStatus: http.StatusBadRequest, wantBody: "unknown country"},
		{name: "B7 unknown name", query: "country=Atlantis", wantStatus: http.StatusBadRequest, wantBody: "unknown country"},
		{name: "B8 no params", query: "", wantStatus: http.StatusOK, wantCountry: ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			repo := &listEventsTestRepo{}
			handler := NewEventHandler(repo, nil)
			req := httptest.NewRequest(http.MethodGet, "/v1/events?"+tt.query, nil)
			rec := httptest.NewRecorder()

			handler.ListEvents(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantStatus == http.StatusBadRequest {
				if !strings.Contains(rec.Body.String(), tt.wantBody) {
					t.Errorf("body = %q, want substring %q", rec.Body.String(), tt.wantBody)
				}
				if repo.called {
					t.Error("expected repository not to be called when input is rejected")
				}
				return
			}
			if !repo.called {
				t.Fatal("expected repository to be called on a 200 path")
			}
			if repo.filters.Country != tt.wantCountry {
				t.Errorf("filters.Country = %q, want %q", repo.filters.Country, tt.wantCountry)
			}
		})
	}
}

func TestListEventsLogsRepositoryFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	repo := &listEventsTestRepo{listErr: errors.New("connection refused")}
	handler := NewEventHandler(repo, logger)

	req := httptest.NewRequest(http.MethodGet, "/v1/events?category=floods", nil)
	rec := httptest.NewRecorder()

	handler.ListEvents(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	// The client must not see the internal cause (§4.5).
	if strings.Contains(rec.Body.String(), "connection refused") {
		t.Errorf("response body leaked the internal error: %q", rec.Body.String())
	}
	// ...but it must reach the log, or the 500 is invisible in production.
	logged := buf.String()
	if !strings.Contains(logged, "list events failed") {
		t.Errorf("expected log to contain %q, got %q", "list events failed", logged)
	}
	if !strings.Contains(logged, "connection refused") {
		t.Errorf("expected log to contain the underlying error, got %q", logged)
	}
}
