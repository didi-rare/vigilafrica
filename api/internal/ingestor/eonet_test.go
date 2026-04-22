package ingestor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/models"
)

type mockRepo struct{}

func (m *mockRepo) UpsertEvent(ctx context.Context, e models.Event, geoJSON string) error { return nil }
func (m *mockRepo) ListEvents(ctx context.Context, filters database.EventFilters) ([]models.Event, int, error) { return nil, 0, nil }
func (m *mockRepo) GetEventByID(ctx context.Context, id uuid.UUID) (*models.Event, error) { return nil, nil }
func (m *mockRepo) GetNearbyEvents(ctx context.Context, lat, lng float64, radiusKm float64, limit int) ([]models.Event, error) { return nil, nil }
func (m *mockRepo) CreateIngestionRun(ctx context.Context, startedAt time.Time, countryCode string) (int64, error) { return 1, nil }
func (m *mockRepo) CompleteIngestionRun(ctx context.Context, id int64, status models.IngestionRunStatus, fetched, stored int, errMsg *string) error { return nil }
func (m *mockRepo) GetLastIngestionRun(ctx context.Context) (*models.IngestionRun, error) { return nil, nil }
func (m *mockRepo) GetLastSuccessfulIngestionRun(ctx context.Context) (*models.IngestionRun, error) { return nil, nil }
func (m *mockRepo) GetFirstIngestionRun(ctx context.Context) (*models.IngestionRun, error) { return nil, nil }
func (m *mockRepo) GetLastIngestionRunAllCountries(ctx context.Context) (map[string]*models.IngestionRun, error) { return nil, nil }
func (m *mockRepo) GetEnrichmentStats(ctx context.Context) ([]database.EnrichmentStat, error) { return nil, nil }
func (m *mockRepo) GetDistinctStatesByCountry(ctx context.Context, country string) ([]string, error) { return nil, nil }
func (m *mockRepo) Close() {}

func TestRunIngest_RateLimit(t *testing.T) {
	// Start an httptest server that returns 429 on the first request with retry_after = 1,
	// and 200 OK on the second request.
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"retry_after": 1}`)) // 1 second
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"events": [
				{
					"id": "EONET_123",
					"title": "Wildfire",
					"categories": [{"id": "wildfires"}],
					"geometry": [{"magnitudeValue": null, "magnitudeUnit": null, "date": "2023-01-01T00:00:00Z", "type": "Point", "coordinates": [0.0, 0.0]}]
				}
			]
		}`))
	}))
	defer server.Close()

	// Override eonetURL for the test
	originalURL := eonetURL
	eonetURL = server.URL
	defer func() { eonetURL = originalURL }()

	repo := &mockRepo{}
	country := CountryConfig{Code: "NG", Name: "Nigeria", BBox: [4]float64{2.0, 4.0, 15.0, 14.0}}

	start := time.Now()
	result, err := runIngest(context.Background(), repo, country)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("expected successful ingestion, got err: %v", err)
	}
	if result.EventsFetched != 1 {
		t.Errorf("expected 1 event fetched, got %d", result.EventsFetched)
	}

	// Should have slept for at least retry_after(1) + 5 = 6 seconds.
	// Allow some wiggle room for processing time.
	if duration < 6*time.Second {
		t.Errorf("expected to sleep for at least 6s due to retry_after=1 + 5s buffer, but took %v", duration)
	}
}
