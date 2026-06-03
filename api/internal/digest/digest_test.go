package digest

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/models"
)

type fakeLister struct {
	got    database.EventFilters
	events []models.Event
}

func (f *fakeLister) ListEvents(_ context.Context, filters database.EventFilters) ([]models.Event, int, error) {
	f.got = filters
	return f.events, len(f.events), nil
}

func ptr[T any](v T) *T { return &v }

func floodEvent(title, country, state string) models.Event {
	d := time.Date(2026, 6, 2, 4, 0, 0, 0, time.UTC)
	return models.Event{
		ID:          uuid.New(),
		Title:       title,
		Category:    models.CategoryFloods,
		CountryName: &country,
		StateName:   &state,
		EventDate:   &d,
		SourceURL:   ptr("https://example.test/" + title),
	}
}

func TestBuildTodayDigestFiltersFloodsForUTCDay(t *testing.T) {
	lister := &fakeLister{}
	now := time.Date(2026, 6, 2, 15, 30, 0, 0, time.UTC)

	if _, err := BuildTodayDigest(context.Background(), lister, now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if lister.got.Category != string(models.CategoryFloods) {
		t.Errorf("category = %q, want floods", lister.got.Category)
	}
	wantFrom := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	if lister.got.DateFrom == nil || !lister.got.DateFrom.Equal(wantFrom) {
		t.Errorf("DateFrom = %v, want %v", lister.got.DateFrom, wantFrom)
	}
	if lister.got.DateTo == nil || !lister.got.DateTo.Equal(wantFrom.Add(24*time.Hour)) {
		t.Errorf("DateTo = %v, want %v", lister.got.DateTo, wantFrom.Add(24*time.Hour))
	}
	if lister.got.Limit != maxDigestEvents {
		t.Errorf("Limit = %d, want %d", lister.got.Limit, maxDigestEvents)
	}
}

func TestBuildTodayDigestGroupsAndSortsAlphabetically(t *testing.T) {
	lister := &fakeLister{events: []models.Event{
		floodEvent("Oyo flood", "Nigeria", "Oyo"),
		floodEvent("Accra flood", "Ghana", "Greater Accra"),
		floodEvent("Benue flood", "Nigeria", "Benue"),
	}}
	now := time.Date(2026, 6, 2, 6, 0, 0, 0, time.UTC)

	d, err := BuildTodayDigest(context.Background(), lister, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if d.Date != "2026-06-02" {
		t.Errorf("date = %q, want 2026-06-02", d.Date)
	}
	if d.Total != 3 {
		t.Errorf("total = %d, want 3", d.Total)
	}
	if len(d.ByCountry) != 2 {
		t.Fatalf("country groups = %d, want 2", len(d.ByCountry))
	}
	// Countries sorted alphabetically: Ghana before Nigeria.
	if d.ByCountry[0].CountryName != "Ghana" || d.ByCountry[1].CountryName != "Nigeria" {
		t.Fatalf("country order = [%s, %s], want [Ghana, Nigeria]", d.ByCountry[0].CountryName, d.ByCountry[1].CountryName)
	}
	// Nigeria states sorted: Benue before Oyo.
	ng := d.ByCountry[1].States
	if len(ng) != 2 || ng[0].StateName != "Benue" || ng[1].StateName != "Oyo" {
		t.Fatalf("nigeria state order wrong: %+v", ng)
	}
	if ng[0].Events[0].Title != "Benue flood" {
		t.Errorf("benue event title = %q", ng[0].Events[0].Title)
	}
}

func TestBuildTodayDigestNilLocationsGroupedUnknown(t *testing.T) {
	lister := &fakeLister{events: []models.Event{
		{ID: uuid.New(), Title: "Unlocated flood", Category: models.CategoryFloods},
	}}
	now := time.Date(2026, 6, 2, 6, 0, 0, 0, time.UTC)

	d, err := BuildTodayDigest(context.Background(), lister, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.ByCountry) != 1 || d.ByCountry[0].CountryName != unknownGroup {
		t.Fatalf("expected single Unknown country group, got %+v", d.ByCountry)
	}
	if d.ByCountry[0].States[0].StateName != unknownGroup {
		t.Errorf("expected Unknown state, got %q", d.ByCountry[0].States[0].StateName)
	}
}

func TestBuildTodayDigestEmptyDay(t *testing.T) {
	lister := &fakeLister{events: nil}
	now := time.Date(2026, 6, 2, 6, 0, 0, 0, time.UTC)

	d, err := BuildTodayDigest(context.Background(), lister, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Total != 0 {
		t.Errorf("total = %d, want 0", d.Total)
	}
	if len(d.ByCountry) != 0 {
		t.Errorf("by_country = %d groups, want 0", len(d.ByCountry))
	}
	if d.Date != "2026-06-02" {
		t.Errorf("date = %q, want 2026-06-02 even on an empty day", d.Date)
	}
}
