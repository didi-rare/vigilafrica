//go:build integration

package database_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"

	"vigilafrica/api/internal/models"
)

// Adversarial regression tests for trg_enrich_event_location()
// (chore-enrichment-regression-tests, deferred-work register item B2).
//
// Migration 000013 states the trigger's semantics in its own header:
//
//   1. the SMALLEST intersecting ADM1 polygon wins, with `id` as the final
//      tie-breaker so the ordering is total;
//   2. when no ADM1 matches, the smallest intersecting ADM0 polygon supplies the
//      country and state_name is left NULL;
//   3. the trigger fires BEFORE INSERT OR UPDATE OF geom, so a move re-labels and
//      a label from the old position must not survive a move it no longer fits.
//
// When 000013 shipped, an independent reviewer proved those semantics were
// preserved with a hand-built suite (shared-border points, ADM0 interiors,
// points matching nothing, every vertex of Nigeria's exterior ring) and then
// discarded it. These tests are that suite, made permanent. Each group is built
// so a specific mutation of the trigger breaks it — see the spec for the table.
//
// Every oracle here is computed INDEPENDENTLY of the trigger: areas via
// ST_Area(geom::geography) rather than the stored area_m2 column the trigger
// reads, so a drift in the generated column would also surface.
//
// Isolation: the suite shares one database and never truncates. Every event this
// file inserts has a source_id prefixed ADV_, every synthetic boundary carries
// country_code 'ZZ', and each test function cleans both up. The last test in the
// file proves the cleanup happened.

// advConn opens a raw connection for the things the repository does not (and
// should not) expose: boundary discovery, synthetic boundaries, oracles.
func advConn(t *testing.T, ctx context.Context) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(ctx, testDSN)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(ctx) })
	return conn
}

// advCleanup removes everything this file can leave behind. Registered per test
// function, not once per file, so a single failing function still cleans up.
func advCleanup(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = conn.Exec(ctx, `DELETE FROM events WHERE source_id LIKE 'ADV\_%'`)
		_, _ = conn.Exec(ctx, `DELETE FROM admin_boundaries WHERE country_code = 'ZZ'`)
	})
}

// advLabels reads the two enrichment columns straight from the row, bypassing
// ListEvents and its page limit.
func advLabels(t *testing.T, ctx context.Context, conn *pgx.Conn, sourceID string) (state, country *string) {
	t.Helper()
	err := conn.QueryRow(ctx,
		`SELECT state_name, country_name FROM events WHERE source_id = $1`, sourceID,
	).Scan(&state, &country)
	if err != nil {
		t.Fatalf("read labels for %s: %v", sourceID, err)
	}
	return state, country
}

// advUpsertPoint drives an event through the real repository path at (lon, lat).
func advUpsertPoint(t *testing.T, ctx context.Context, sourceID, title string, lon, lat float64) {
	t.Helper()
	geoJSON := fmt.Sprintf(`{"type":"Point","coordinates":[%f,%f]}`, lon, lat)
	ev := models.Event{
		SourceID:  sourceID,
		Source:    "eonet",
		Title:     title,
		Category:  models.CategoryFloods,
		Status:    models.StatusOpen,
		GeomType:  ptrStr("Point"),
		Longitude: ptrF64(lon),
		Latitude:  ptrF64(lat),
	}
	if err := testRepo.UpsertEvent(ctx, ev, geoJSON); err != nil {
		t.Fatalf("upsert %s: %v", sourceID, err)
	}
}

// ── Group 1: shared-border vertices ─────────────────────────────────────────

// TestEnrichment_SharedBorderPicksSmallerState finds adjacent ADM1 pairs in the
// fixture itself, takes a point where their boundaries meet, and asserts the
// smaller state wins. Breaks if ASC becomes DESC or the order stops being by
// area.
//
// The discovery query already filters to points that genuinely intersect at
// least two ADM1 polygons, so a point that PostGIS does not consider on both
// rings (floating-point boundary arithmetic) is excluded rather than asserted
// on. What IS asserted is that enough real contests exist: fewer than eight
// means the fixture or the query changed, and that fails rather than skips.
func TestEnrichment_SharedBorderPicksSmallerState(t *testing.T) {
	ctx := context.Background()
	conn := advConn(t, ctx)
	advCleanup(t, ctx, conn)

	type borderCase struct {
		a, b, country string
		lon, lat      float64
		candidates    int
	}

	rows, err := conn.Query(ctx, `
		WITH pairs AS (
			SELECT a.id AS aid, b.id AS bid,
			       a.adm_name AS a_name, b.adm_name AS b_name, a.country_name,
			       (ST_DumpPoints(ST_Intersection(ST_Boundary(a.geom), ST_Boundary(b.geom)))).geom AS p
			FROM admin_boundaries a
			JOIN admin_boundaries b
			  ON a.adm_level = 1 AND b.adm_level = 1
			 AND a.country_code = b.country_code
			 AND a.id < b.id
			 AND ST_Intersects(a.geom, b.geom)
		),
		first_point AS (
			SELECT DISTINCT ON (aid, bid) aid, bid, a_name, b_name, country_name, p
			FROM pairs
			ORDER BY aid, bid
		)
		SELECT a_name, b_name, country_name, ST_X(p), ST_Y(p),
		       (SELECT count(*) FROM admin_boundaries c
		         WHERE c.adm_level = 1 AND ST_Intersects(p, c.geom)) AS candidates
		FROM first_point
		ORDER BY aid, bid`)
	if err != nil {
		t.Fatalf("discover adjacent pairs: %v", err)
	}
	defer rows.Close()

	var contests []borderCase
	var discovered int
	for rows.Next() {
		var c borderCase
		if err := rows.Scan(&c.a, &c.b, &c.country, &c.lon, &c.lat, &c.candidates); err != nil {
			t.Fatalf("scan pair: %v", err)
		}
		discovered++
		if c.candidates >= 2 {
			contests = append(contests, c)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate pairs: %v", err)
	}
	t.Logf("adjacent ADM1 pairs discovered: %d, of which %d are real two-candidate contests", discovered, len(contests))
	if len(contests) < 8 {
		t.Fatalf("only %d adjacent pairs yield a point intersecting >= 2 states; the fixture or the discovery query changed (want >= 8)", len(contests))
	}
	if len(contests) > 12 {
		contests = contests[:12]
	}

	for i, tt := range contests {
		tt := tt
		sourceID := fmt.Sprintf("ADV_BORDER_%02d", i)
		t.Run(fmt.Sprintf("%s|%s", tt.a, tt.b), func(t *testing.T) {
			// Oracle: smallest intersecting ADM1 by an area computed independently of
			// the trigger's stored column.
			var want string
			err := conn.QueryRow(ctx, `
				SELECT adm_name FROM admin_boundaries
				WHERE adm_level = 1
				  AND ST_Intersects(ST_SetSRID(ST_MakePoint($1, $2), 4326), geom)
				ORDER BY ST_Area(geom::geography) ASC, id ASC
				LIMIT 1`, tt.lon, tt.lat).Scan(&want)
			if err != nil {
				t.Fatalf("oracle: %v", err)
			}
			if want != tt.a && want != tt.b {
				// A third, smaller state also touches this point (a tripoint). Still a
				// valid contest; the oracle just names the real winner.
				t.Logf("tripoint: smallest candidate is %q, not %q or %q", want, tt.a, tt.b)
			}

			advUpsertPoint(t, ctx, sourceID, "shared-border probe", tt.lon, tt.lat)
			state, country := advLabels(t, ctx, conn, sourceID)
			assertOptString(t, "state_name", state, want)
			assertOptString(t, "country_name", country, tt.country)

			// Stability: the same point re-upserted resolves identically.
			advUpsertPoint(t, ctx, sourceID, "shared-border probe (again)", tt.lon, tt.lat)
			state2, _ := advLabels(t, ctx, conn, sourceID)
			assertOptString(t, "state_name after re-upsert", state2, want)
		})
	}
}

// ── Group 2: exact-area tie-break ───────────────────────────────────────────

// TestEnrichment_EqualAreaTieBreaksOnLowestID inserts two synthetic ADM1 rows with
// byte-identical geometry — so area_m2 ties to the bit — and shows that the
// lower id wins, whichever name was inserted first. Breaks if `, id ASC` is
// dropped from the trigger's ORDER BY: without it the winner is whatever the
// plan happens to emit first, and the two subtests cannot both hold.
func TestEnrichment_EqualAreaTieBreaksOnLowestID(t *testing.T) {
	ctx := context.Background()
	conn := advConn(t, ctx)
	advCleanup(t, ctx, conn)

	const lon, lat = -30.0, -30.0 // South Atlantic: no real boundary, no other test's event
	insertTie := func(t *testing.T, name string) int {
		t.Helper()
		var id int
		err := conn.QueryRow(ctx, `
			INSERT INTO admin_boundaries (country_code, country_name, adm_level, adm_name, geom)
			VALUES ('ZZ', 'Testland', 1, $1,
			        ST_Multi(ST_SetSRID(ST_MakeEnvelope(-30.5, -30.5, -29.5, -29.5), 4326)))
			RETURNING id`, name).Scan(&id)
		if err != nil {
			t.Fatalf("insert synthetic boundary %q: %v", name, err)
		}
		return id
	}
	reset := func(t *testing.T) {
		t.Helper()
		if _, err := conn.Exec(ctx, `DELETE FROM events WHERE source_id LIKE 'ADV\_TIE%'`); err != nil {
			t.Fatalf("reset events: %v", err)
		}
		if _, err := conn.Exec(ctx, `DELETE FROM admin_boundaries WHERE country_code = 'ZZ'`); err != nil {
			t.Fatalf("reset boundaries: %v", err)
		}
	}

	cases := []struct {
		name          string
		first, second string
	}{
		{"alpha inserted first wins", "Tie Alpha", "Tie Beta"},
		{"beta inserted first wins", "Tie Beta", "Tie Alpha"},
	}
	for i, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			reset(t)
			firstID := insertTie(t, tt.first)
			secondID := insertTie(t, tt.second)
			if !(firstID < secondID) {
				t.Fatalf("serial ids not increasing: %d then %d", firstID, secondID)
			}

			// Precondition, checked not assumed: the stored areas tie exactly.
			var distinctAreas int
			if err := conn.QueryRow(ctx,
				`SELECT count(DISTINCT area_m2) FROM admin_boundaries WHERE country_code = 'ZZ'`,
			).Scan(&distinctAreas); err != nil {
				t.Fatalf("area precondition: %v", err)
			}
			if distinctAreas != 1 {
				t.Fatalf("synthetic boundaries do not tie on area_m2 (%d distinct values); the tie-break is untested", distinctAreas)
			}

			sourceID := fmt.Sprintf("ADV_TIE_%d", i)
			advUpsertPoint(t, ctx, sourceID, "tie probe", lon, lat)
			state, country := advLabels(t, ctx, conn, sourceID)
			assertOptString(t, "state_name", state, tt.first) // lowest id = inserted first
			assertOptString(t, "country_name", country, "Testland")
		})
	}
}

// ── Group 3: geometry update ────────────────────────────────────────────────

// TestEnrichment_GeometryUpdateRelabelsAndClears moves one event through the real
// upsert path and asserts every move re-labels exactly — including CLEARING a
// state that no longer fits — and that a metadata-only update leaves the labels
// alone. Breaks if the trigger stops firing on UPDATE OF geom, or if a rewrite
// assigns state_name only when the ADM1 query matches (leaving "Kano" on an
// event in Cameroon).
func TestEnrichment_GeometryUpdateRelabelsAndClears(t *testing.T) {
	ctx := context.Background()
	conn := advConn(t, ctx)
	advCleanup(t, ctx, conn)

	const sourceID = "ADV_MOVER"

	// Fixture-checked constants. Lagos and Cameroon are the staging values the
	// existing six-point test uses; Kano is checked against the fixture here so a
	// wrong constant fails loudly instead of mislabelling the expectation.
	const (
		lagosLon, lagosLat = 3.3941795, 6.4550575
		kanoLon, kanoLat   = 8.5167, 12.0022
		cmrLon, cmrLat     = 11.601622, 5.707452
		seaLon, seaLat     = 0.0, 0.0
	)
	var kanoOK bool
	if err := conn.QueryRow(ctx, `
		SELECT ST_Intersects(ST_SetSRID(ST_MakePoint($1, $2), 4326), geom)
		FROM admin_boundaries WHERE adm_level = 1 AND adm_name = 'Kano'`,
		kanoLon, kanoLat).Scan(&kanoOK); err != nil || !kanoOK {
		t.Fatalf("Kano probe point is not inside the Kano ADM1 polygon (err=%v, inside=%v)", err, kanoOK)
	}

	steps := []struct {
		name        string
		lon, lat    float64
		wantState   string // "" = NULL
		wantCountry string // "" = NULL
	}{
		{"insert in Lagos", lagosLon, lagosLat, "Lagos", "Nigeria"},
		{"move to Kano re-labels the state", kanoLon, kanoLat, "Kano", "Nigeria"},
		{"move to Cameroon clears the state and falls back to ADM0", cmrLon, cmrLat, "", "Cameroon"},
		{"move to open ocean clears both", seaLon, seaLat, "", ""},
		{"move back to Lagos", lagosLon, lagosLat, "Lagos", "Nigeria"},
	}
	for _, tt := range steps {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			advUpsertPoint(t, ctx, sourceID, "mover: "+tt.name, tt.lon, tt.lat)
			state, country := advLabels(t, ctx, conn, sourceID)
			assertOptString(t, "state_name", state, tt.wantState)
			assertOptString(t, "country_name", country, tt.wantCountry)
		})
	}

	t.Run("metadata-only update leaves the labels untouched", func(t *testing.T) {
		// The trigger is UPDATE OF geom; a title change is not a move, and the
		// metadata path exists precisely to refresh a row WITHOUT touching geometry.
		found, err := testRepo.UpdateEventMetadata(ctx, models.Event{
			SourceID: sourceID,
			Source:   "eonet",
			Title:    "mover: title changed, geometry not",
			Category: models.CategoryFloods,
			Status:   models.StatusClosed,
		})
		if err != nil {
			t.Fatalf("UpdateEventMetadata: %v", err)
		}
		if !found {
			t.Fatalf("UpdateEventMetadata reported no row for %s", sourceID)
		}
		state, country := advLabels(t, ctx, conn, sourceID)
		assertOptString(t, "state_name", state, "Lagos")
		assertOptString(t, "country_name", country, "Nigeria")
	})
}

// ── Group 4: exterior-ring safety net ───────────────────────────────────────

// TestEnrichment_NigeriaExteriorRingAlwaysLabelled inserts an event at vertices of
// Nigeria's national boundary and asserts every one gets a non-NULL country.
// These are the hardest points the ADM0 fallback has to catch: ADM1 coverage at
// the border is allowed to have gaps, so state_name is not asserted, but the
// country must never be lost. Breaks if the fallback block is removed.
//
// The NG ADM0 row is ST_Union of the states, so its ring has many vertices; the
// test samples evenly to at most 200 and asserts at least 30 exist, so a
// silently empty or degenerate ring cannot pass.
func TestEnrichment_NigeriaExteriorRingAlwaysLabelled(t *testing.T) {
	ctx := context.Background()
	conn := advConn(t, ctx)
	advCleanup(t, ctx, conn)

	rows, err := conn.Query(ctx, `
		WITH parts AS (
			SELECT (ST_Dump(geom)).geom AS g
			FROM admin_boundaries WHERE country_code = 'NG' AND adm_level = 0
		),
		mainland AS (SELECT g FROM parts ORDER BY ST_Area(g) DESC LIMIT 1)
		SELECT ST_X((dp).geom), ST_Y((dp).geom)
		FROM (SELECT ST_DumpPoints(ST_ExteriorRing(g)) AS dp FROM mainland) q
		ORDER BY (dp).path[1]`)
	if err != nil {
		t.Fatalf("dump exterior ring: %v", err)
	}
	defer rows.Close()

	type pt struct{ lon, lat float64 }
	var ring []pt
	for rows.Next() {
		var p pt
		if err := rows.Scan(&p.lon, &p.lat); err != nil {
			t.Fatalf("scan vertex: %v", err)
		}
		ring = append(ring, p)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate vertices: %v", err)
	}
	if len(ring) < 30 {
		t.Fatalf("Nigeria exterior ring has only %d vertices; fixture changed (want >= 30)", len(ring))
	}

	const maxProbes = 200
	step := 1
	if len(ring) > maxProbes {
		step = (len(ring) + maxProbes - 1) / maxProbes
	}
	var inserted int
	for i := 0; i < len(ring); i += step {
		p := ring[i]
		sourceID := fmt.Sprintf("ADV_NGRING_%04d", i)
		if _, err := conn.Exec(ctx, `
			INSERT INTO events (source_id, source, title, category, status, geom, geom_type, latitude, longitude)
			VALUES ($1, 'eonet', 'ng ring vertex', 'floods', 'open',
			        ST_SetSRID(ST_MakePoint($2, $3), 4326), 'Point', $3, $2)
			ON CONFLICT (source_id) DO UPDATE SET geom = EXCLUDED.geom`,
			sourceID, p.lon, p.lat); err != nil {
			t.Fatalf("insert vertex %d: %v", i, err)
		}
		inserted++
	}
	t.Logf("exterior ring: %d vertices, %d probed (step %d)", len(ring), inserted, step)

	var unlabelled int
	if err := conn.QueryRow(ctx,
		`SELECT count(*) FROM events WHERE source_id LIKE 'ADV\_NGRING\_%' AND country_name IS NULL`,
	).Scan(&unlabelled); err != nil {
		t.Fatalf("count unlabelled: %v", err)
	}
	if unlabelled != 0 {
		var examples []string
		exRows, err := conn.Query(ctx,
			`SELECT source_id || ' @ ' || ST_X(geom) || ',' || ST_Y(geom)
			   FROM events WHERE source_id LIKE 'ADV\_NGRING\_%' AND country_name IS NULL
			   ORDER BY source_id LIMIT 5`)
		if err == nil {
			defer exRows.Close()
			for exRows.Next() {
				var s string
				_ = exRows.Scan(&s)
				examples = append(examples, s)
			}
		}
		t.Errorf("%d of %d border vertices got a NULL country_name; the ADM0 fallback missed them, e.g. %v", unlabelled, inserted, examples)
	}
}

// ── Cleanup proof ───────────────────────────────────────────────────────────

// TestEnrichment_ZZCleanupLeftNothing runs last in this file (Go executes a
// package's tests in source order) and proves the per-test cleanups above
// actually ran. A shared, never-truncated database makes "we clean up" a claim
// worth asserting rather than trusting.
func TestEnrichment_ZZCleanupLeftNothing(t *testing.T) {
	ctx := context.Background()
	conn := advConn(t, ctx)

	var events, boundaries int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM events WHERE source_id LIKE 'ADV\_%'`).Scan(&events); err != nil {
		t.Fatalf("count ADV_ events: %v", err)
	}
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM admin_boundaries WHERE country_code = 'ZZ'`).Scan(&boundaries); err != nil {
		t.Fatalf("count ZZ boundaries: %v", err)
	}
	if events != 0 || boundaries != 0 {
		t.Errorf("adversarial fixtures leaked: %d ADV_ events, %d ZZ boundaries", events, boundaries)
	}
}
