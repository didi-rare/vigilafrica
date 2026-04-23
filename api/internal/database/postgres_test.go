//go:build integration

package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/models"
)

// TestUpsertEvent verifies idempotent insert and status update on conflict.
func TestUpsertEvent(t *testing.T) {
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	sourceURL := "https://eonet.gsfc.nasa.gov/api/v3/events/EONET_TEST_UPSERT"
	geoJSON := `{"type":"Point","coordinates":[3.3792,6.5244]}`

	event := models.Event{
		SourceID:  "TEST_UPSERT_001",
		Source:    "eonet",
		Title:     "Test Flood — Lagos",
		Category:  models.CategoryFloods,
		Status:    models.StatusOpen,
		GeomType:  ptrStr("Point"),
		Latitude:  ptrF64(6.5244),
		Longitude: ptrF64(3.3792),
		EventDate: &now,
		SourceURL: &sourceURL,
	}

	// First insert — must succeed.
	if err := testRepo.UpsertEvent(ctx, event, geoJSON); err != nil {
		t.Fatalf("initial insert failed: %v", err)
	}

	// Second upsert with changed status — must not create a duplicate.
	event.Status = models.StatusClosed
	if err := testRepo.UpsertEvent(ctx, event, geoJSON); err != nil {
		t.Fatalf("upsert (update) failed: %v", err)
	}

	// Verify: status reflected and no duplicate rows.
	events, total, err := testRepo.ListEvents(ctx, database.EventFilters{Limit: 200})
	if err != nil {
		t.Fatalf("list events failed: %v", err)
	}

	var count int
	var found *models.Event
	for i := range events {
		if events[i].SourceID == "TEST_UPSERT_001" {
			count++
			found = &events[i]
		}
	}

	if count != 1 {
		t.Errorf("expected exactly 1 row for source_id TEST_UPSERT_001, got %d (total events: %d)", count, total)
	}
	if found != nil && found.Status != models.StatusClosed {
		t.Errorf("expected status %q after upsert, got %q", models.StatusClosed, found.Status)
	}
}

// TestGetNearbyEvents verifies PostGIS distance filtering for events within and outside a radius.
func TestGetNearbyEvents(t *testing.T) {
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	sourceURL := "https://eonet.gsfc.nasa.gov/api/v3/events/EONET_TEST_NEARBY"

	// Insert a flood event at Lagos, Nigeria (lat=6.5244, lng=3.3792).
	lagosEvent := models.Event{
		SourceID:  "TEST_NEARBY_LAGOS_001",
		Source:    "eonet",
		Title:     "Test Flood — Lagos (nearby)",
		Category:  models.CategoryFloods,
		Status:    models.StatusOpen,
		GeomType:  ptrStr("Point"),
		Latitude:  ptrF64(6.5244),
		Longitude: ptrF64(3.3792),
		EventDate: &now,
		SourceURL: &sourceURL,
	}
	lagosGeoJSON := `{"type":"Point","coordinates":[3.3792,6.5244]}`

	if err := testRepo.UpsertEvent(ctx, lagosEvent, lagosGeoJSON); err != nil {
		t.Fatalf("failed to insert Lagos event: %v", err)
	}

	t.Run("finds event within radius", func(t *testing.T) {
		// Query from central Lagos with a 100 km radius — must include the Lagos event.
		results, err := testRepo.GetNearbyEvents(ctx, 6.5244, 3.3792, 100, 10)
		if err != nil {
			t.Fatalf("GetNearbyEvents failed: %v", err)
		}

		var found bool
		for _, e := range results {
			if e.SourceID == "TEST_NEARBY_LAGOS_001" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected Lagos event in nearby results (100 km radius), got %d results", len(results))
		}
	})

	t.Run("excludes event outside radius", func(t *testing.T) {
		// Query from Cape Town (lat=-33.9249, lng=18.4241) with 100 km — must not include Lagos.
		results, err := testRepo.GetNearbyEvents(ctx, -33.9249, 18.4241, 100, 10)
		if err != nil {
			t.Fatalf("GetNearbyEvents from Cape Town failed: %v", err)
		}

		for _, e := range results {
			if e.SourceID == "TEST_NEARBY_LAGOS_001" {
				t.Error("Lagos event incorrectly returned in Cape Town 100 km radius query")
			}
		}
	})
}

// TestGetEventByID verifies fetching a single event by UUID.
func TestGetEventByID(t *testing.T) {
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	sourceURL := "https://eonet.gsfc.nasa.gov/api/v3/events/EONET_TEST_BYID"
	geoJSON := `{"type":"Point","coordinates":[7.4898,9.0579]}`

	event := models.Event{
		SourceID:  "TEST_BYID_001",
		Source:    "eonet",
		Title:     "Test Wildfire — Abuja",
		Category:  models.CategoryWildfires,
		Status:    models.StatusOpen,
		GeomType:  ptrStr("Point"),
		Latitude:  ptrF64(9.0579),
		Longitude: ptrF64(7.4898),
		EventDate: &now,
		SourceURL: &sourceURL,
	}

	if err := testRepo.UpsertEvent(ctx, event, geoJSON); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	// Retrieve via list to get the auto-generated UUID.
	all, _, err := testRepo.ListEvents(ctx, database.EventFilters{Limit: 200})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	var eventUUID uuid.UUID
	var found bool
	for _, e := range all {
		if e.SourceID == "TEST_BYID_001" {
			eventUUID = e.ID
			found = true
			break
		}
	}
	if !found {
		t.Fatal("TEST_BYID_001 not found in list")
	}

	// Fetch by UUID — must return the exact same event.
	fetched, err := testRepo.GetEventByID(ctx, eventUUID)
	if err != nil {
		t.Fatalf("GetEventByID failed: %v", err)
	}
	if fetched.SourceID != "TEST_BYID_001" {
		t.Errorf("expected source_id TEST_BYID_001, got %q", fetched.SourceID)
	}
	if fetched.Category != models.CategoryWildfires {
		t.Errorf("expected category wildfires, got %q", fetched.Category)
	}
}

// TestListEventsFilters verifies that category, status, and pagination filters work correctly.
func TestListEventsFilters(t *testing.T) {
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	url1 := "https://eonet.gsfc.nasa.gov/api/v3/events/EONET_FILTER_F"
	url2 := "https://eonet.gsfc.nasa.gov/api/v3/events/EONET_FILTER_W"

	flood := models.Event{
		SourceID:  "TEST_FILTER_FLOOD_001",
		Source:    "eonet",
		Title:     "Test Flood — filter test",
		Category:  models.CategoryFloods,
		Status:    models.StatusOpen,
		GeomType:  ptrStr("Point"),
		Latitude:  ptrF64(6.0),
		Longitude: ptrF64(3.0),
		EventDate: &now,
		SourceURL: &url1,
	}
	wildfire := models.Event{
		SourceID:  "TEST_FILTER_WILD_001",
		Source:    "eonet",
		Title:     "Test Wildfire — filter test",
		Category:  models.CategoryWildfires,
		Status:    models.StatusClosed,
		GeomType:  ptrStr("Point"),
		Latitude:  ptrF64(9.0),
		Longitude: ptrF64(7.0),
		EventDate: &now,
		SourceURL: &url2,
	}

	for _, e := range []struct {
		event   models.Event
		geoJSON string
	}{
		{flood, `{"type":"Point","coordinates":[3.0,6.0]}`},
		{wildfire, `{"type":"Point","coordinates":[7.0,9.0]}`},
	} {
		if err := testRepo.UpsertEvent(ctx, e.event, e.geoJSON); err != nil {
			t.Fatalf("insert %s failed: %v", e.event.SourceID, err)
		}
	}

	t.Run("filter by category=floods", func(t *testing.T) {
		events, _, err := testRepo.ListEvents(ctx, database.EventFilters{Category: "floods", Limit: 50})
		if err != nil {
			t.Fatalf("ListEvents failed: %v", err)
		}
		for _, e := range events {
			if e.Category != models.CategoryFloods {
				t.Errorf("expected only floods, got %q (source_id=%s)", e.Category, e.SourceID)
			}
		}
	})

	t.Run("filter by status=closed", func(t *testing.T) {
		events, _, err := testRepo.ListEvents(ctx, database.EventFilters{Status: "closed", Limit: 50})
		if err != nil {
			t.Fatalf("ListEvents failed: %v", err)
		}
		for _, e := range events {
			if e.Status != models.StatusClosed {
				t.Errorf("expected only closed events, got %q (source_id=%s)", e.Status, e.SourceID)
			}
		}
	})

	t.Run("pagination limit=1", func(t *testing.T) {
		events, total, err := testRepo.ListEvents(ctx, database.EventFilters{Limit: 1, Offset: 0})
		if err != nil {
			t.Fatalf("ListEvents failed: %v", err)
		}
		if len(events) > 1 {
			t.Errorf("expected at most 1 event with limit=1, got %d (total=%d)", len(events), total)
		}
	})
}

// TestCreateAndCompleteIngestionRun verifies the full ingestion run lifecycle.
func TestCreateAndCompleteIngestionRun(t *testing.T) {
	ctx := context.Background()
	startedAt := time.Now().UTC().Truncate(time.Second)

	// Create a run — must return a valid ID.
	runID, err := testRepo.CreateIngestionRun(ctx, startedAt, "NG")
	if err != nil {
		t.Fatalf("CreateIngestionRun failed: %v", err)
	}
	if runID <= 0 {
		t.Fatalf("expected positive run ID, got %d", runID)
	}

	// Verify status is "running" immediately after creation.
	latest, err := testRepo.GetLastIngestionRun(ctx)
	if err != nil {
		t.Fatalf("GetLastIngestionRun failed: %v", err)
	}
	if latest == nil {
		t.Fatal("expected a run record, got nil")
	}
	if latest.Status != models.RunStatusRunning {
		t.Errorf("expected status %q after create, got %q", models.RunStatusRunning, latest.Status)
	}

	// Complete the run with success.
	if err := testRepo.CompleteIngestionRun(ctx, runID, models.RunStatusSuccess, 12, 10, nil); err != nil {
		t.Fatalf("CompleteIngestionRun failed: %v", err)
	}

	// Verify final state.
	completed, err := testRepo.GetLastSuccessfulIngestionRun(ctx)
	if err != nil {
		t.Fatalf("GetLastSuccessfulIngestionRun failed: %v", err)
	}
	if completed == nil {
		t.Fatal("expected a successful run record, got nil")
	}
	if completed.ID != runID {
		t.Errorf("expected run ID %d, got %d", runID, completed.ID)
	}
	if completed.Status != models.RunStatusSuccess {
		t.Errorf("expected status %q, got %q", models.RunStatusSuccess, completed.Status)
	}
	if completed.EventsFetched != 12 {
		t.Errorf("expected EventsFetched=12, got %d", completed.EventsFetched)
	}
	if completed.EventsStored != 10 {
		t.Errorf("expected EventsStored=10, got %d", completed.EventsStored)
	}
	if completed.CompletedAt == nil {
		t.Error("expected CompletedAt to be set after completion")
	}
}

// TestIngestionRunHelpers covers GetFirstIngestionRun and GetLastIngestionRunAllCountries.
func TestIngestionRunHelpers(t *testing.T) {
	ctx := context.Background()
	startedAt := time.Now().UTC().Truncate(time.Second)

	// Insert one run for GH to ensure multi-country results.
	ghRunID, err := testRepo.CreateIngestionRun(ctx, startedAt, "GH")
	if err != nil {
		t.Fatalf("CreateIngestionRun GH failed: %v", err)
	}
	if err := testRepo.CompleteIngestionRun(ctx, ghRunID, models.RunStatusSuccess, 5, 5, nil); err != nil {
		t.Fatalf("CompleteIngestionRun GH failed: %v", err)
	}

	t.Run("GetFirstIngestionRun returns non-nil", func(t *testing.T) {
		first, err := testRepo.GetFirstIngestionRun(ctx)
		if err != nil {
			t.Fatalf("GetFirstIngestionRun failed: %v", err)
		}
		if first == nil {
			t.Fatal("expected a run record, got nil")
		}
	})

	t.Run("GetLastIngestionRunAllCountries returns map with NG and GH", func(t *testing.T) {
		m, err := testRepo.GetLastIngestionRunAllCountries(ctx)
		if err != nil {
			t.Fatalf("GetLastIngestionRunAllCountries failed: %v", err)
		}
		if _, ok := m["GH"]; !ok {
			t.Errorf("expected GH entry in country map, got keys: %v", func() []string {
				keys := make([]string, 0, len(m))
				for k := range m {
					keys = append(keys, k)
				}
				return keys
			}())
		}
	})
}

// TestEnrichmentAndStates covers GetEnrichmentStats and GetDistinctStatesByCountry.
func TestEnrichmentAndStates(t *testing.T) {
	ctx := context.Background()

	t.Run("GetEnrichmentStats returns a slice without error", func(t *testing.T) {
		stats, err := testRepo.GetEnrichmentStats(ctx)
		if err != nil {
			t.Fatalf("GetEnrichmentStats failed: %v", err)
		}
		// The test DB has events inserted by earlier tests; stats must be a non-nil slice.
		if stats == nil {
			t.Error("expected non-nil stats slice")
		}
	})

	t.Run("GetDistinctStatesByCountry with no country filter", func(t *testing.T) {
		states, err := testRepo.GetDistinctStatesByCountry(ctx, "")
		if err != nil {
			t.Fatalf("GetDistinctStatesByCountry failed: %v", err)
		}
		if states == nil {
			t.Error("expected non-nil states slice")
		}
	})

	t.Run("GetDistinctStatesByCountry with country filter returns no error", func(t *testing.T) {
		_, err := testRepo.GetDistinctStatesByCountry(ctx, "Nigeria")
		if err != nil {
			t.Fatalf("GetDistinctStatesByCountry(Nigeria) failed: %v", err)
		}
	})
}
