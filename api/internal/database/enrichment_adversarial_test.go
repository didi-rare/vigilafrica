//go:build integration

package database_test

import (
	"context"
	"fmt"
	"strconv"
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
		// Best-effort by nature (cleanup must not mask the test's own verdict),
		// but never silent: a failed delete is logged here and would be caught by
		// TestEnrichment_ZZCleanupLeftNothing at the end of the file.
		if _, err := conn.Exec(ctx, `DELETE FROM events WHERE source_id LIKE 'ADV\_%'`); err != nil {
			t.Logf("cleanup: delete ADV_ events: %v", err)
		}
		if _, err := conn.Exec(ctx, `DELETE FROM admin_boundaries WHERE country_code = 'ZZ'`); err != nil {
			t.Logf("cleanup: delete ZZ boundaries: %v", err)
		}
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
//
// Coordinates are formatted at full precision, not with %f. The first CI run
// of the shared-border group failed 6 of 12 pairs because %f rounds to six
// decimals: a vertex shared by two rings moved by up to half a microdegree
// lands INSIDE one of them, so the trigger saw a one-candidate point while the
// oracle evaluated the exact vertex — and the two disagreed exactly when the
// rounding fell into the larger polygon. The test was wrong, not the trigger.
func advUpsertPoint(t *testing.T, ctx context.Context, sourceID, title string, lon, lat float64) {
	t.Helper()
	geoJSON := fmt.Sprintf(`{"type":"Point","coordinates":[%s,%s]}`,
		strconv.FormatFloat(lon, 'f', -1, 64), strconv.FormatFloat(lat, 'f', -1, 64))
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
			       (dp).geom AS p, (dp).path AS p_path
			FROM admin_boundaries a
			JOIN admin_boundaries b
			  ON a.adm_level = 1 AND b.adm_level = 1
			 AND a.country_code = b.country_code
			 AND a.id < b.id
			 AND ST_Intersects(a.geom, b.geom)
			CROSS JOIN LATERAL ST_DumpPoints(ST_Intersection(ST_Boundary(a.geom), ST_Boundary(b.geom))) AS dp
		),
		first_point AS (
			-- DISTINCT ON keeps the first row per (aid, bid) in ORDER BY order; the
			-- dump path is part of that order so "first point" is defined, not
			-- whichever row the plan happened to emit first.
			SELECT DISTINCT ON (aid, bid) aid, bid, a_name, b_name, country_name, p
			FROM pairs
			ORDER BY aid, bid, p_path
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
			advUpsertPoint(t, ctx, sourceID, "shared-border probe", tt.lon, tt.lat)

			// The contest must still be real at the point the trigger actually saw:
			// the STORED geometry, after the GeoJSON round trip. If it intersects
			// only one state, the probe drifted off the border and the comparison
			// below is meaningless — fail here, naming the cause, rather than
			// reporting a "wrong" label.
			var storedCandidates int
			if err := conn.QueryRow(ctx, `
				SELECT count(*) FROM admin_boundaries b, events e
				WHERE e.source_id = $1 AND b.adm_level = 1 AND ST_Intersects(e.geom, b.geom)`,
				sourceID).Scan(&storedCandidates); err != nil {
				t.Fatalf("stored-candidate precondition: %v", err)
			}
			if storedCandidates < 2 {
				t.Fatalf("stored event point intersects %d ADM1 polygon(s), discovery saw %d: the coordinates lost precision on the way in", storedCandidates, tt.candidates)
			}

			// Oracle, evaluated against the SAME stored geometry the trigger saw, so
			// the two cannot disagree about which point is being judged. Ordered by an
			// area computed here, not by the trigger's stored area_m2 column: the
			// generated column should equal this, and if it ever drifted the two
			// orderings would diverge and this test would say so.
			var want string
			err := conn.QueryRow(ctx, `
				SELECT b.adm_name FROM admin_boundaries b, events e
				WHERE e.source_id = $1 AND b.adm_level = 1 AND ST_Intersects(e.geom, b.geom)
				ORDER BY ST_Area(b.geom::geography) ASC, b.id ASC
				LIMIT 1`, sourceID).Scan(&want)
			if err != nil {
				t.Fatalf("oracle: %v", err)
			}
			if want != tt.a && want != tt.b {
				// A third, smaller state also touches this point (a tripoint). Still a
				// valid contest; the oracle just names the real winner.
				t.Logf("tripoint: smallest candidate is %q, not %q or %q", want, tt.a, tt.b)
			}

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
// LOWER id wins, whichever name was inserted first.
//
// The ids are set explicitly, and the row inserted FIRST gets the HIGHER id.
// That inversion is the whole test. Review of the first version found that with
// SERIAL ids, id order equals insertion order equals heap order, and a trigger
// with `, id ASC` removed still returns the first tuple its scan visits — the
// first inserted — which was exactly the expectation, so the mutant passed
// both subtests. Decorrelating id order from physical order means the mutant
// (first inserted, higher id) and the real trigger (lower id, second inserted)
// now name different rows, in both orderings.
func TestEnrichment_EqualAreaTieBreaksOnLowestID(t *testing.T) {
	ctx := context.Background()
	conn := advConn(t, ctx)
	advCleanup(t, ctx, conn)

	const lon, lat = -30.0, -30.0 // South Atlantic: no real boundary, no other test's event
	// Far above the SERIAL range (real rows are two-digit ids) so the explicit
	// ids can never collide with the sequence; cleanup deletes by country_code.
	const higherID, lowerID = 2000000002, 2000000001
	insertTie := func(t *testing.T, id int, name string) {
		t.Helper()
		_, err := conn.Exec(ctx, `
			INSERT INTO admin_boundaries (id, country_code, country_name, adm_level, adm_name, geom)
			VALUES ($1, 'ZZ', 'Testland', 1, $2,
			        ST_Multi(ST_SetSRID(ST_MakeEnvelope(-30.5, -30.5, -29.5, -29.5), 4326)))`,
			id, name)
		if err != nil {
			t.Fatalf("insert synthetic boundary %q as id %d: %v", name, id, err)
		}
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
		first, second string // insertion order; first gets higherID, second gets lowerID
	}{
		{"alpha inserted first, beta has the lower id and wins", "Tie Alpha", "Tie Beta"},
		{"beta inserted first, alpha has the lower id and wins", "Tie Beta", "Tie Alpha"},
	}
	for i, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			reset(t)
			insertTie(t, higherID, tt.first)
			insertTie(t, lowerID, tt.second)

			// Preconditions, checked not assumed: the stored areas tie exactly, and
			// the id inversion really is in place.
			var distinctAreas, lowestIDName int
			if err := conn.QueryRow(ctx, `
				SELECT count(DISTINCT area_m2),
				       count(*) FILTER (WHERE id = $1 AND adm_name = $2)
				FROM admin_boundaries WHERE country_code = 'ZZ'`,
				lowerID, tt.second).Scan(&distinctAreas, &lowestIDName); err != nil {
				t.Fatalf("precondition: %v", err)
			}
			if distinctAreas != 1 {
				t.Fatalf("synthetic boundaries do not tie on area_m2 (%d distinct values); the tie-break is untested", distinctAreas)
			}
			if lowestIDName != 1 {
				t.Fatalf("id inversion not in place: %q should hold id %d", tt.second, lowerID)
			}

			sourceID := fmt.Sprintf("ADV_TIE_%d", i)
			advUpsertPoint(t, ctx, sourceID, "tie probe", lon, lat)
			state, country := advLabels(t, ctx, conn, sourceID)
			// Lower id wins: that is the SECOND-inserted row. A trigger without the
			// id terminator returns the first-visited (first-inserted) row instead.
			assertOptString(t, "state_name", state, tt.second)
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
		//
		// A sentinel makes this discriminating. Without it, a trigger widened to
		// every UPDATE would re-enrich Lagos to "Lagos" and the assertion would
		// still hold. Planting a value the trigger would never produce, through
		// an UPDATE that does not touch geom either, means: if the trigger fires
		// on non-geom updates, the sentinel is overwritten before or during the
		// metadata update, and the assertion fails.
		if _, err := conn.Exec(ctx,
			`UPDATE events SET state_name = 'SENTINEL' WHERE source_id = $1`, sourceID); err != nil {
			t.Fatalf("plant sentinel: %v", err)
		}
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
		assertOptString(t, "state_name", state, "SENTINEL")
		assertOptString(t, "country_name", country, "Nigeria")
	})
}

// ── Group 4: exterior-ring safety net ───────────────────────────────────────

// TestEnrichment_NigeriaExteriorRingAlwaysLabelled inserts an event at vertices of
// Nigeria's national boundary and asserts every one gets a non-NULL country.
// ADM1 coverage at the border is allowed to have gaps, so state_name is not
// asserted, but the country must never be lost.
//
// Honest scope: the NG ADM0 row is ST_Union of the states, so nearly every
// vertex of its exterior ring is also a vertex of some ADM1 polygon and is
// labelled by the ADM1 branch without ever reaching the ADM0 fallback. This
// group is therefore a border-coverage safety net, NOT a proof that the
// fallback block exists — the Cameroon step of the geometry-update group and
// the existing six-point test are what pin the fallback. The test counts and
// logs how many probes were actually labelled by the fallback (state NULL,
// country set) so the number is visible rather than assumed; it does not
// assert on it, because zero is a legitimate outcome for a union-derived ring.
//
// The ring has many vertices; the test samples evenly to at most 200 and
// asserts at least 30 exist, so a silently empty or degenerate ring cannot pass.
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

	var unlabelled, viaFallback int
	if err := conn.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE country_name IS NULL),
		       count(*) FILTER (WHERE state_name IS NULL AND country_name IS NOT NULL)
		FROM events WHERE source_id LIKE 'ADV\_NGRING\_%'`,
	).Scan(&unlabelled, &viaFallback); err != nil {
		t.Fatalf("count labels: %v", err)
	}
	t.Logf("probes labelled by the ADM0 fallback (state NULL, country set): %d of %d", viaFallback, inserted)
	if unlabelled != 0 {
		var examples []string
		exRows, err := conn.Query(ctx,
			`SELECT source_id || ' @ ' || ST_X(geom) || ',' || ST_Y(geom)
			   FROM events WHERE source_id LIKE 'ADV\_NGRING\_%' AND country_name IS NULL
			   ORDER BY source_id LIMIT 5`)
		if err != nil {
			t.Logf("could not list examples: %v", err)
		} else {
			defer exRows.Close()
			for exRows.Next() {
				var s string
				if err := exRows.Scan(&s); err != nil {
					t.Logf("could not scan example: %v", err)
					break
				}
				examples = append(examples, s)
			}
			if err := exRows.Err(); err != nil {
				t.Logf("could not iterate examples: %v", err)
			}
		}
		t.Errorf("%d of %d border vertices got a NULL country_name, e.g. %v", unlabelled, inserted, examples)
	}
}

// ── Cleanup proof ───────────────────────────────────────────────────────────

// TestEnrichment_ZZCleanupLeftNothing runs last in this file and proves the
// per-test cleanups above actually ran. A shared, never-truncated database
// makes "we clean up" a claim worth asserting rather than trusting.
//
// Ordering relies on `go test` running a package's tests in source order
// (files sorted by name, then top to bottom within a file) — which it does
// unless `-shuffle` is passed, and CI passes no `-shuffle`. The "ZZ" in the
// name is a reminder of the intent, not a sorting mechanism: Go does not order
// tests alphabetically. If this file is ever split, keep this function last.
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
