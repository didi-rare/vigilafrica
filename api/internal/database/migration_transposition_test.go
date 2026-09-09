//go:build integration

package database_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

const migration000015 = "../../db/migrations/000015_correct_transposed_polygon_geometry.up.sql"

// TestMigration000015CorrectsTransposedGeometry exercises the MIGRATION, not a
// stand-in for it.
//
// ⚠️ An earlier version of this test inserted rows directly at the corrected
// coordinates and asserted the trigger labelled them properly. That passes with
// the migration's UPDATE statements deleted, so it proved nothing about the
// migration at all. This one seeds the transposed geometry exactly as production
// held it, replays the migration file from disk, and asserts the correction —
// so removing either UPDATE fails the test.
func TestMigration000015CorrectsTransposedGeometry(t *testing.T) {
	ctx := context.Background()

	conn, err := pgx.Connect(ctx, testDSN)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	// Seed both rows with the TRANSPOSED geometry EONET supplied. These rings are
	// the real published extents with the axes the wrong way round, which is why
	// 22248 lands in Kwara rather than Cameroon.
	seed := []struct {
		sourceID string
		geoJSON  string
	}{
		{
			"EONET_22248",
			`{"type":"Polygon","coordinates":[[[4.377,9.216],[4.828,9.216],[4.828,9.569],[4.377,9.569],[4.377,9.216]]]}`,
		},
		{
			"EONET_23208",
			`{"type":"Polygon","coordinates":[[[12.468,9.852],[12.589,9.852],[12.589,9.931],[12.468,9.931],[12.468,9.852]]]}`,
		},
	}
	for _, s := range seed {
		_, err := conn.Exec(ctx, `
			INSERT INTO events (source_id, source, title, category, status, geom, geom_type, event_date)
			VALUES ($1, 'eonet', 'transposed fixture', 'floods', 'closed',
			        ST_SetSRID(ST_GeomFromGeoJSON($2), 4326), 'Polygon', now())
			ON CONFLICT (source_id) DO UPDATE
			SET geom = EXCLUDED.geom, geom_type = EXCLUDED.geom_type`,
			s.sourceID, s.geoJSON)
		if err != nil {
			t.Fatalf("seed %s: %v", s.sourceID, err)
		}
	}

	// Precondition: the transposed fixture really is mislabelled. If this ever
	// stops holding, the test below would pass for the wrong reason.
	if country := countryOf(t, ctx, conn, "EONET_22248"); country != "Nigeria" {
		t.Fatalf("precondition failed: transposed 22248 should label as Nigeria, got %q", country)
	}

	sql, err := os.ReadFile(migration000015)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if !strings.Contains(string(sql), "EONET_22248") || !strings.Contains(string(sql), "EONET_23208") {
		t.Fatal("the migration no longer targets both affected rows")
	}
	if _, err := conn.Exec(ctx, string(sql)); err != nil {
		t.Fatalf("replay migration: %v", err)
	}

	t.Run("22248 becomes a Cameroonian flood, not a Kwara one", func(t *testing.T) {
		if got := countryOf(t, ctx, conn, "EONET_22248"); got != "Cameroon" {
			t.Errorf("country_name = %q, want Cameroon", got)
		}
		// ⚠️ state must be NULL: it is outside every loaded Nigerian ADM1 polygon,
		// and we do not invent a state for a country whose states we do not hold.
		var state *string
		if err := conn.QueryRow(ctx,
			`SELECT state_name FROM events WHERE source_id = 'EONET_22248'`).Scan(&state); err != nil {
			t.Fatalf("query state: %v", err)
		}
		if state != nil {
			t.Errorf("state_name = %q, want NULL", *state)
		}
	})

	t.Run("23208 moves to northern Nigeria, away from Adamawa", func(t *testing.T) {
		if got := countryOf(t, ctx, conn, "EONET_23208"); got != "Nigeria" {
			t.Errorf("country_name = %q, want Nigeria", got)
		}
		var state *string
		if err := conn.QueryRow(ctx,
			`SELECT state_name FROM events WHERE source_id = 'EONET_23208'`).Scan(&state); err != nil {
			t.Fatalf("query state: %v", err)
		}
		if state == nil {
			t.Fatal("state_name is NULL; the corrected extent should fall inside a Nigerian state")
		}
		if *state == "Adamawa" {
			t.Errorf("state_name is still Adamawa — the transposed location survived the migration")
		}
		t.Logf("23208 corrected to %s", *state)
	})

	t.Run("the geometry stays a polygon and carries a centroid", func(t *testing.T) {
		// ⚠️ Both properties matter. GetNearbyEvents measures ST_DWithin against
		// geom, so reducing the flood to a point would shrink who sees it; and the
		// frontend drops null-coordinate events from the map, so without lat/lon
		// the event renders in the list but never as a marker.
		for _, id := range []string{"EONET_22248", "EONET_23208"} {
			var geomType string
			var lon, lat *float64
			err := conn.QueryRow(ctx, `
				SELECT geom_type, longitude, latitude FROM events WHERE source_id = $1`, id).
				Scan(&geomType, &lon, &lat)
			if err != nil {
				t.Fatalf("%s: %v", id, err)
			}
			if geomType != "Polygon" {
				t.Errorf("%s: geom_type = %q, want Polygon", id, geomType)
			}
			if lon == nil || lat == nil {
				t.Errorf("%s: centroid missing, so the event cannot render on the map", id)
			}
		}
	})

	t.Run("replaying the migration is idempotent", func(t *testing.T) {
		before := countryOf(t, ctx, conn, "EONET_22248")
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("second replay: %v", err)
		}
		if after := countryOf(t, ctx, conn, "EONET_22248"); after != before {
			t.Errorf("re-running the migration changed the result: %q -> %q", before, after)
		}
	})
}

func countryOf(t *testing.T, ctx context.Context, conn *pgx.Conn, sourceID string) string {
	t.Helper()
	var country *string
	if err := conn.QueryRow(ctx,
		`SELECT country_name FROM events WHERE source_id = $1`, sourceID).Scan(&country); err != nil {
		t.Fatalf("query country for %s: %v", sourceID, err)
	}
	if country == nil {
		return ""
	}
	return *country
}
