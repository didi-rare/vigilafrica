//go:build integration

package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"vigilafrica/api/internal/models"
)

// TestUpdateEventMetadataPreservesGeometryAndDate is the round-3 P0 regression.
//
// The metadata-only path exists so an unverifiable geometry does not discard
// trusted changes — most importantly `status`, since a flood that has closed must
// not stay `open` while GDACS is unreachable.
//
// ⚠️ But `event_date` and `raw_payload` are DERIVED FROM THE GEOMETRY SNAPSHOT we
// just refused: event_date is parsed from the selected geometry's own date field.
// Writing either alongside the OLD geometry produces a row claiming an old
// polygon belongs to a new observation — and the digest selects by event_date, so
// a stale extent could surface as a current flood. An earlier revision wrote
// both, justified by the false claim that "dates do not depend on coordinates".
func TestUpdateEventMetadataPreservesGeometryAndDate(t *testing.T) {
	ctx := context.Background()

	conn, err := pgx.Connect(ctx, testDSN)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	original := time.Date(2026, 8, 3, 20, 0, 0, 0, time.UTC)
	seed := models.Event{
		SourceID:   "MD_PRESERVE",
		Source:     "eonet",
		Title:      "original title",
		Category:   models.CategoryFloods,
		Status:     models.StatusOpen,
		GeomType:   ptrStr("Polygon"),
		EventDate:  &original,
		RawPayload: []byte(`{"marker":"original"}`),
	}
	geoJSON := `{"type":"Polygon","coordinates":[[[9.216,4.377],[9.216,4.828],[9.569,4.828],[9.569,4.377],[9.216,4.377]]]}`
	if err := testRepo.UpsertEvent(ctx, seed, geoJSON); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// A later run: the event has CLOSED and been retitled, and carries a new
	// geometry snapshot with a new date — which we could not verify.
	newer := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	update := seed
	update.Title = "refreshed title"
	update.Status = models.StatusClosed
	update.EventDate = &newer
	update.RawPayload = []byte(`{"marker":"UNVERIFIED"}`)

	updated, err := testRepo.UpdateEventMetadata(ctx, update)
	if err != nil {
		t.Fatalf("UpdateEventMetadata: %v", err)
	}
	if !updated {
		t.Fatal("expected the existing row to be matched")
	}

	var (
		title, status string
		eventDate     time.Time
		raw           []byte
		geomText      string
	)
	err = conn.QueryRow(ctx, `
		SELECT title, status, event_date, raw_payload, ST_AsText(geom)
		FROM events WHERE source_id = 'MD_PRESERVE'`).
		Scan(&title, &status, &eventDate, &raw, &geomText)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	// The point of the path: trusted fields DO refresh.
	if title != "refreshed title" {
		t.Errorf("title = %q, want the refreshed value", title)
	}
	if status != string(models.StatusClosed) {
		t.Errorf("status = %q, want closed — a flood that ended must not stay open", status)
	}

	// ⚠️ And the geometry-derived fields do NOT.
	if !eventDate.UTC().Equal(original) {
		t.Errorf("event_date = %v, want the ORIGINAL %v — it is derived from the rejected geometry snapshot",
			eventDate.UTC(), original)
	}
	if string(raw) == `{"marker":"UNVERIFIED"}` {
		t.Error("raw_payload was overwritten with the unverified snapshot")
	}
	if geomText == "" {
		t.Fatal("geometry disappeared")
	}
	// Geometry must be byte-for-byte the seeded ring.
	var stillMatches bool
	if err := conn.QueryRow(ctx, `
		SELECT ST_Equals(geom, ST_SetSRID(ST_GeomFromGeoJSON($1), 4326))
		FROM events WHERE source_id = 'MD_PRESERVE'`, geoJSON).Scan(&stillMatches); err != nil {
		t.Fatalf("ST_Equals: %v", err)
	}
	if !stillMatches {
		t.Errorf("geometry changed: %s", geomText)
	}
}

// TestUpdateEventMetadataNeverInserts pins that the metadata path cannot invent
// a row for an event we have never successfully placed.
func TestUpdateEventMetadataNeverInserts(t *testing.T) {
	ctx := context.Background()

	updated, err := testRepo.UpdateEventMetadata(ctx, models.Event{
		SourceID: "MD_DOES_NOT_EXIST",
		Source:   "eonet",
		Title:    "should not appear",
		Category: models.CategoryFloods,
		Status:   models.StatusOpen,
	})
	if err != nil {
		t.Fatalf("UpdateEventMetadata: %v", err)
	}
	if updated {
		t.Error("reported a match for a source_id that does not exist")
	}

	conn, err := pgx.Connect(ctx, testDSN)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	var count int
	if err := conn.QueryRow(ctx,
		`SELECT count(*) FROM events WHERE source_id = 'MD_DOES_NOT_EXIST'`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("the metadata path inserted %d row(s); it must never insert", count)
	}
}
