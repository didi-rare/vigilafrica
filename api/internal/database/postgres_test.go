//go:build integration

package database_test

import (
	"context"
	"fmt"
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

	// Second upsert with changed canonical fields — must not create a duplicate
	// and must not retain stale upstream-derived state.
	event.Status = models.StatusClosed
	event.Title = "Updated Wildfire — Accra"
	event.Category = models.CategoryWildfires
	event.Latitude = ptrF64(5.6037)
	event.Longitude = ptrF64(-0.1870)
	updatedDate := now.Add(24 * time.Hour)
	event.EventDate = &updatedDate
	updatedSourceURL := "https://eonet.gsfc.nasa.gov/api/v3/events/EONET_TEST_UPSERT_UPDATED"
	event.SourceURL = &updatedSourceURL
	updatedGeoJSON := `{"type":"Point","coordinates":[-0.1870,5.6037]}`
	if err := testRepo.UpsertEvent(ctx, event, updatedGeoJSON); err != nil {
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
	if found != nil && found.Category != models.CategoryWildfires {
		t.Errorf("expected category %q after upsert, got %q", models.CategoryWildfires, found.Category)
	}
	if found != nil && found.Latitude != nil && *found.Latitude != 5.6037 {
		t.Errorf("expected latitude 5.6037 after upsert, got %v", found.Latitude)
	}
	if found != nil && found.Longitude != nil && *found.Longitude != -0.1870 {
		t.Errorf("expected longitude -0.1870 after upsert, got %v", found.Longitude)
	}
	if found != nil && (found.SourceURL == nil || *found.SourceURL != updatedSourceURL) {
		t.Errorf("expected source URL %q after upsert, got %v", updatedSourceURL, found.SourceURL)
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

func TestTryRecordStalenessAlertSuppressesDuplicateReference(t *testing.T) {
	ctx := context.Background()
	referenceTime := time.Now().UTC().Truncate(time.Second)
	recorder, ok := testRepo.(interface {
		TryRecordStalenessAlert(context.Context, time.Time) (bool, error)
	})
	if !ok {
		t.Fatal("test repository does not implement TryRecordStalenessAlert")
	}

	recorded, err := recorder.TryRecordStalenessAlert(ctx, referenceTime)
	if err != nil {
		t.Fatalf("TryRecordStalenessAlert first insert failed: %v", err)
	}
	if !recorded {
		t.Fatal("expected first staleness alert reference to be recorded")
	}

	recorded, err = recorder.TryRecordStalenessAlert(ctx, referenceTime)
	if err != nil {
		t.Fatalf("TryRecordStalenessAlert duplicate insert failed: %v", err)
	}
	if recorded {
		t.Fatal("expected duplicate staleness alert reference to be suppressed")
	}
}

func TestSchedulerLockAllowsOnlyOneHolder(t *testing.T) {
	ctx := context.Background()
	locker, ok := testRepo.(interface {
		TryAcquireSchedulerLock(context.Context, string, string, time.Duration) (bool, error)
		ReleaseSchedulerLock(context.Context, string, string) error
	})
	if !ok {
		t.Fatal("test repository does not implement scheduler locking")
	}

	lockName := "test-scheduler-lock"
	acquired, err := locker.TryAcquireSchedulerLock(ctx, lockName, "holder-a", time.Minute)
	if err != nil {
		t.Fatalf("first TryAcquireSchedulerLock failed: %v", err)
	}
	if !acquired {
		t.Fatal("expected first holder to acquire scheduler lock")
	}
	defer locker.ReleaseSchedulerLock(ctx, lockName, "holder-a")

	acquired, err = locker.TryAcquireSchedulerLock(ctx, lockName, "holder-b", time.Minute)
	if err != nil {
		t.Fatalf("second TryAcquireSchedulerLock failed: %v", err)
	}
	if acquired {
		t.Fatal("expected second holder to be blocked while lock is held")
	}

	if err := locker.ReleaseSchedulerLock(ctx, lockName, "holder-a"); err != nil {
		t.Fatalf("ReleaseSchedulerLock failed: %v", err)
	}
	acquired, err = locker.TryAcquireSchedulerLock(ctx, lockName, "holder-b", time.Minute)
	if err != nil {
		t.Fatalf("third TryAcquireSchedulerLock failed: %v", err)
	}
	if !acquired {
		t.Fatal("expected second holder to acquire lock after release")
	}
	_ = locker.ReleaseSchedulerLock(ctx, lockName, "holder-b")
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

// ── Pagination ordering (feature-events-pagination) ──────────────────────────
//
// The suite shares one database across tests and never truncates, so these tests
// isolate their own rows with a far-future event_date window and the
// DateFrom/DateTo filters rather than by assuming an empty table.

// orderingWindowStart is well past any real EONET event_date, so
// DateFrom/DateTo isolates exactly the rows seeded below.
var orderingWindowStart = time.Date(2999, 1, 1, 0, 0, 0, 0, time.UTC)

// seedTiedEvents inserts n events that ALL share the same event_date, i.e. one
// maximal tie group. Ties are what an ordering without a unique terminator gets
// wrong, and re-upserting a tied row is what an ordering containing ingested_at
// gets wrong. Returns the source IDs in insertion order.
func seedTiedEvents(t *testing.T, ctx context.Context, prefix string, n int, eventDate time.Time) []string {
	t.Helper()
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		sourceID := fmt.Sprintf("%s_%02d", prefix, i)
		url := "https://eonet.gsfc.nasa.gov/api/v3/events/" + sourceID
		e := models.Event{
			SourceID:  sourceID,
			Source:    "eonet",
			Title:     fmt.Sprintf("Ordering fixture %s", sourceID),
			Category:  models.CategoryFloods,
			Status:    models.StatusOpen,
			GeomType:  ptrStr("Point"),
			Latitude:  ptrF64(6.0),
			Longitude: ptrF64(3.0),
			EventDate: &eventDate,
			SourceURL: &url,
		}
		if err := testRepo.UpsertEvent(ctx, e, `{"type":"Point","coordinates":[3.0,6.0]}`); err != nil {
			t.Fatalf("seeding %s failed: %v", sourceID, err)
		}
		// Each upsert is its own implicit transaction, so NOW() differs per row.
		// That is precisely what used to make the order churn.
		ids = append(ids, sourceID)
	}
	return ids
}

// reUpsert re-inserts the given source IDs, which fires
// `ON CONFLICT DO UPDATE SET ingested_at = NOW()` on each one — simulating an
// ingestion tick that touches already-stored events.
func reUpsert(t *testing.T, ctx context.Context, sourceIDs []string, eventDate time.Time) {
	t.Helper()
	for _, sourceID := range sourceIDs {
		url := "https://eonet.gsfc.nasa.gov/api/v3/events/" + sourceID
		e := models.Event{
			SourceID:  sourceID,
			Source:    "eonet",
			Title:     fmt.Sprintf("Ordering fixture %s", sourceID),
			Category:  models.CategoryFloods,
			Status:    models.StatusOpen,
			GeomType:  ptrStr("Point"),
			Latitude:  ptrF64(6.0),
			Longitude: ptrF64(3.0),
			EventDate: &eventDate,
			SourceURL: &url,
		}
		if err := testRepo.UpsertEvent(ctx, e, `{"type":"Point","coordinates":[3.0,6.0]}`); err != nil {
			t.Fatalf("re-upsert of %s failed: %v", sourceID, err)
		}
	}
}

// TestListEventsOrderingIsStableAcrossIdenticalQueries covers task 1.2: events
// tying on event_date must come back in the same order every time.
func TestListEventsOrderingIsStableAcrossIdenticalQueries(t *testing.T) {
	ctx := context.Background()

	eventDate := orderingWindowStart
	windowEnd := eventDate.Add(24 * time.Hour)
	const n = 6
	seedTiedEvents(t, ctx, "TEST_ORDER_STABLE", n, eventDate)

	filters := database.EventFilters{
		DateFrom: &eventDate,
		DateTo:   &windowEnd,
		Limit:    200,
	}

	first, total, err := testRepo.ListEvents(ctx, filters)
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	if total != n {
		t.Fatalf("expected %d seeded events in the isolation window, got %d", n, total)
	}

	want := make([]string, len(first))
	for i, e := range first {
		want[i] = e.SourceID
	}

	// Repeat the identical query. Without a unique terminator in ORDER BY, equal
	// sort keys leave Postgres free to return a different permutation.
	for attempt := 0; attempt < 5; attempt++ {
		got, _, err := testRepo.ListEvents(ctx, filters)
		if err != nil {
			t.Fatalf("ListEvents (attempt %d) failed: %v", attempt, err)
		}
		if len(got) != len(want) {
			t.Fatalf("attempt %d returned %d events, want %d", attempt, len(got), len(want))
		}
		for i := range got {
			if got[i].SourceID != want[i] {
				t.Fatalf("attempt %d: order changed at position %d: got %s, want %s",
					attempt, i, got[i].SourceID, want[i])
			}
		}
	}
}

// TestListEventsPagingIsExactlyOnceDuringReIngestion covers task 1.3 — the case
// the previous ordering failed. Walking by offset while existing events are
// re-upserted must still yield every event exactly once, because ingested_at is
// no longer part of the sort.
func TestListEventsPagingIsExactlyOnceDuringReIngestion(t *testing.T) {
	ctx := context.Background()

	// A distinct day from the stability test, so the two isolation windows do not overlap.
	eventDate := orderingWindowStart.Add(48 * time.Hour)
	windowEnd := eventDate.Add(24 * time.Hour)
	const n = 6
	const pageSize = 2
	sourceIDs := seedTiedEvents(t, ctx, "TEST_ORDER_PAGING", n, eventDate)

	seen := make(map[string]int, n)
	var order []string

	for page := 0; page*pageSize < n; page++ {
		offset := page * pageSize

		// An ingestion tick lands before every page fetch, re-upserting the event
		// with the OLDEST ingested_at (the events were seeded in order, so that is
		// sourceIDs[page]). Under the old `ORDER BY event_date DESC, ingested_at
		// DESC` that row jumps from the BACK of the tie group to the FRONT, pushing
		// every other row down one position: the client then re-sees a row from the
		// previous page and skips the one it displaced. Under the current ordering
		// ingested_at is not a sort key, so nothing moves.
		reUpsert(t, ctx, sourceIDs[page:page+1], eventDate)

		rows, total, err := testRepo.ListEvents(ctx, database.EventFilters{
			DateFrom: &eventDate,
			DateTo:   &windowEnd,
			Limit:    pageSize,
			Offset:   offset,
		})
		if err != nil {
			t.Fatalf("ListEvents at offset %d failed: %v", offset, err)
		}
		if total != n {
			t.Fatalf("expected total %d at offset %d, got %d", n, offset, total)
		}
		if len(rows) != pageSize {
			t.Fatalf("expected %d events at offset %d, got %d", pageSize, offset, len(rows))
		}
		for _, e := range rows {
			seen[e.SourceID]++
			order = append(order, e.SourceID)
		}
	}

	if len(order) != n {
		t.Fatalf("walked %d rows across all pages, want %d", len(order), n)
	}
	for _, sourceID := range sourceIDs {
		switch seen[sourceID] {
		case 1:
			// exactly once — correct
		case 0:
			t.Errorf("event %s was SKIPPED while paging during re-ingestion (walk: %v)", sourceID, order)
		default:
			t.Errorf("event %s appeared %d times while paging during re-ingestion (walk: %v)",
				sourceID, seen[sourceID], order)
		}
	}
}

// TestListEventsOffsetBeyondTotal covers task 4.3 at the repository layer: an
// offset past the end is an empty page, not an error, and total stays truthful.
func TestListEventsOffsetBeyondTotal(t *testing.T) {
	ctx := context.Background()

	eventDate := orderingWindowStart.Add(96 * time.Hour)
	windowEnd := eventDate.Add(24 * time.Hour)
	const n = 3
	seedTiedEvents(t, ctx, "TEST_ORDER_BEYOND", n, eventDate)

	events, total, err := testRepo.ListEvents(ctx, database.EventFilters{
		DateFrom: &eventDate,
		DateTo:   &windowEnd,
		Limit:    50,
		Offset:   n + 10,
	})
	if err != nil {
		t.Fatalf("ListEvents beyond the end failed: %v", err)
	}
	if events == nil {
		t.Error("expected an allocated empty slice, got nil")
	}
	if len(events) != 0 {
		t.Errorf("expected an empty page beyond the end, got %d events", len(events))
	}
	if total != n {
		t.Errorf("expected total to remain %d beyond the end, got %d", n, total)
	}
}
