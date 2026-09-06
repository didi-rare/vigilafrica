//go:build integration

package database_test

import (
	"context"
	"fmt"
	"testing"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/models"
)

// TestEnrichmentTrigger_ADM0Fallback exercises the enrichment trigger after
// 000012 added the ADM0 country fallback. Neighbour-country border events
// (ingested via the NG/GH bbox overhang but outside every loaded ADM1 state)
// must be labelled by country with a NULL state; NG/GH events must still resolve
// to their state via ADM1; and points outside all boundaries must stay NULL.
//
// Coordinates are the real staging values (Cameroon/Benin/Niger spillover, the
// Lagos flood EONET_20881, a Borno event near the eastern border).
func TestEnrichmentTrigger_ADM0Fallback(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		sourceID    string
		lon, lat    float64
		wantCountry string // "" = expect NULL
		wantState   string // "" = expect NULL
	}{
		{"cameroon border event -> country only", "ENR_CM", 11.601622, 5.707452, "Cameroon", ""},
		{"benin border event -> country only", "ENR_BJ", 2.685864, 11.781485, "Benin", ""},
		{"niger border event -> country only", "ENR_NE", 12.542507, 13.871897, "Niger", ""},
		{"lagos event -> state wins via ADM1", "ENR_LAG", 3.3941795, 6.4550575, "Nigeria", "Lagos"},
		{"borno event near border -> NG state, not mislabelled to a neighbour", "ENR_BORNO", 14.376242, 11.775278, "Nigeria", "Borno"},
		{"gulf of guinea -> outside all boundaries", "ENR_OCEAN", 0.0, 0.0, "", ""},

		// The exact values migration 000015 writes for the two rows EONET stored
		// with reversed coordinates. These assert the post-migration labelling
		// rather than assuming the trigger does the right thing with them.
		//
		// ⚠️ 1104078 must come out as Cameroon, NOT Nigeria. Its true point still
		// falls inside the Nigeria ingestion bbox because that box overhangs the
		// border, so the row legitimately stays in the dataset — but it is a
		// Cameroonian flood and must be labelled as one. It was previously served
		// as "Kwara, Nigeria", which is the defect this whole change exists to fix.
		{"gdacs 1104078 corrected -> Cameroon, not Kwara", "ENR_TRANS_CM", 9.414, 4.6027, "Cameroon", ""},
		// 1104105's GDACS centroid resolves to Delta State in the Niger Delta.
		// Previously served as Adamawa — ~700km north-east — because EONET's
		// polygon was transposed. Unlike 1104078 this one matches an ADM1 polygon,
		// so a state is expected rather than the ADM0 fallback.
		{"gdacs 1104105 corrected -> Delta, not Adamawa", "ENR_TRANS_NG", 6.0998, 5.5897, "Nigeria", "Delta"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			geoJSON := fmt.Sprintf(`{"type":"Point","coordinates":[%f,%f]}`, tt.lon, tt.lat)
			ev := models.Event{
				SourceID:  tt.sourceID,
				Source:    "eonet",
				Title:     tt.name,
				Category:  models.CategoryWildfires,
				Status:    models.StatusOpen,
				GeomType:  ptrStr("Point"),
				Longitude: ptrF64(tt.lon),
				Latitude:  ptrF64(tt.lat),
			}
			if err := testRepo.UpsertEvent(ctx, ev, geoJSON); err != nil {
				t.Fatalf("upsert failed: %v", err)
			}

			got := findEventBySourceID(t, ctx, tt.sourceID)
			assertOptString(t, "country_name", got.CountryName, tt.wantCountry)
			assertOptString(t, "state_name", got.StateName, tt.wantState)
		})
	}
}

// findEventBySourceID reads back a single event by its source_id, failing the
// test if it is not found.
func findEventBySourceID(t *testing.T, ctx context.Context, sourceID string) models.Event {
	t.Helper()
	events, _, err := testRepo.ListEvents(ctx, database.EventFilters{Limit: 500})
	if err != nil {
		t.Fatalf("list events failed: %v", err)
	}
	for i := range events {
		if events[i].SourceID == sourceID {
			return events[i]
		}
	}
	t.Fatalf("event %q not found after upsert", sourceID)
	return models.Event{}
}

// assertOptString checks a nullable string column: want=="" expects NULL,
// otherwise expects a non-nil pointer equal to want.
func assertOptString(t *testing.T, field string, got *string, want string) {
	t.Helper()
	if want == "" {
		if got != nil {
			t.Errorf("%s = %q, want NULL", field, *got)
		}
		return
	}
	if got == nil {
		t.Errorf("%s = NULL, want %q", field, want)
		return
	}
	if *got != want {
		t.Errorf("%s = %q, want %q", field, *got, want)
	}
}
