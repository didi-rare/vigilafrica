package handlers

import (
	"context"
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
	called  bool
	filters database.EventFilters
}

func (r *listEventsTestRepo) UpsertEvent(context.Context, models.Event, string) error {
	return nil
}

func (r *listEventsTestRepo) ListEvents(ctx context.Context, filters database.EventFilters) ([]models.Event, int, error) {
	r.called = true
	r.filters = filters
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

func (r *listEventsTestRepo) GetDistinctStatesByCountry(context.Context, string) ([]string, error) {
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
			handler := NewEventHandler(repo)
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
	handler := NewEventHandler(repo)
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
