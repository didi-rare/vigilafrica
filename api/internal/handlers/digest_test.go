package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/models"
)

// digestTestRepo embeds the Repository interface (nil) and overrides only
// ListEvents — the single method the digest handler calls. Any other call
// would panic, which is the desired signal that the handler reached past its
// contract.
type digestTestRepo struct {
	database.Repository
	events []models.Event
}

func (r *digestTestRepo) ListEvents(context.Context, database.EventFilters) ([]models.Event, int, error) {
	return r.events, len(r.events), nil
}

func ptr[T any](v T) *T { return &v }

func TestGetTodayDigestReturnsGroupedJSON(t *testing.T) {
	repo := &digestTestRepo{events: []models.Event{
		{ID: uuid.New(), Title: "Makurdi flood", Category: models.CategoryFloods,
			CountryName: ptr("Nigeria"), StateName: ptr("Benue")},
	}}
	handler := NewDigestHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/digest/today.json", nil)
	rec := httptest.NewRecorder()
	handler.GetTodayDigest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}

	var body struct {
		Date      string `json:"date"`
		Total     int    `json:"total"`
		ByCountry []struct {
			CountryName string `json:"country_name"`
			States      []struct {
				StateName string `json:"state_name"`
			} `json:"states"`
		} `json:"by_country"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v (body: %s)", err, rec.Body.String())
	}
	if body.Total != 1 {
		t.Errorf("total = %d, want 1", body.Total)
	}
	if len(body.ByCountry) != 1 || body.ByCountry[0].CountryName != "Nigeria" {
		t.Fatalf("by_country = %+v, want one Nigeria group", body.ByCountry)
	}
	if body.ByCountry[0].States[0].StateName != "Benue" {
		t.Errorf("state = %q, want Benue", body.ByCountry[0].States[0].StateName)
	}
}

func TestGetTodayDigestEmptyIsOK(t *testing.T) {
	handler := NewDigestHandler(&digestTestRepo{events: nil})

	req := httptest.NewRequest(http.MethodGet, "/v1/digest/today.json", nil)
	rec := httptest.NewRecorder()
	handler.GetTodayDigest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for an empty day (never 404/500)", rec.Code)
	}
	var body struct {
		Total     int           `json:"total"`
		ByCountry []interface{} `json:"by_country"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Total != 0 {
		t.Errorf("total = %d, want 0", body.Total)
	}
	if body.ByCountry == nil {
		t.Errorf("by_country should serialize as [], not null")
	}
}
