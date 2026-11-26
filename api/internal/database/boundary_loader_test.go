//go:build integration

package database_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"vigilafrica/api/internal/boundaries"
	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/models"
)

// Integration tests for the boundary loader (feature-continental-coverage
// task 3.6) against real PostGIS, through the real repository.
//
// Isolation: the fixture country is 'XB', drawn in the Indian Ocean south of
// Madagascar (lon 50-51, lat -30..-29) where no migration loads a boundary.
// Every event carries a source_id prefixed BND_. Each test cleans both up.

const (
	migration000017Up   = "../../db/migrations/000017_boundary_sources.up.sql"
	migration000017Down = "../../db/migrations/000017_boundary_sources.down.sql"
)

func bndCleanup(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()
	clean := func() {
		for _, q := range []string{
			`DELETE FROM events WHERE source_id LIKE 'BND\_%'`,
			`DELETE FROM admin_boundaries WHERE country_code = 'XB'`,
			`DELETE FROM boundary_sources WHERE country_code = 'XB'`,
		} {
			if _, err := conn.Exec(ctx, q); err != nil {
				t.Logf("cleanup %q: %v", q, err)
			}
		}
	}
	clean()
	t.Cleanup(clean)
}

func bndPolygon(ring [][2]float64) map[string]any {
	coords := make([][]float64, len(ring))
	for i, p := range ring {
		coords[i] = []float64{p[0], p[1]}
	}
	return map[string]any{"type": "Polygon", "coordinates": [][][]float64{coords}}
}

func bndSquare(lon, lat, size float64) map[string]any {
	return bndPolygon([][2]float64{{lon, lat}, {lon + size, lat}, {lon + size, lat + size}, {lon, lat + size}, {lon, lat}})
}

// bndMember is the fixture ADM1 file: two squares and a self-intersecting
// "bowtie" ring that only ST_MakeValid can turn into polygons.
func bndMember(t *testing.T) []byte {
	t.Helper()
	feat := func(name, pcode string, g map[string]any) map[string]any {
		return map[string]any{"type": "Feature", "properties": map[string]any{"adm1_name": name, "adm1_pcode": pcode}, "geometry": g}
	}
	bowtie := bndPolygon([][2]float64{{50, -29.5}, {50.5, -29.0}, {50.5, -29.5}, {50, -29.0}, {50, -29.5}})
	b, err := json.Marshal(map[string]any{
		"type": "FeatureCollection",
		"features": []any{
			feat("Alpha", "XB01", bndSquare(50, -30, 0.5)),
			feat("Beta", "XB02", bndSquare(50.5, -30, 0.5)),
			feat("Gamma", "XB03", bowtie),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type bndServer struct {
	*httptest.Server
	mu   sync.Mutex
	hits int
}

func newBndServer(t *testing.T, member []byte) *bndServer {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range map[string][]byte{"xbx_admin1.geojson": member, "xbx_admin1_em.geojson": []byte("{}")} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	archive := buf.Bytes()
	s := &bndServer{}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.hits++
		s.mu.Unlock()
		_, _ = w.Write(archive) // a failed write shows up as a load failure
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *bndServer) hitCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits
}

func bndManifest(t *testing.T, url string, member []byte, overrides map[string]string) *boundaries.Manifest {
	t.Helper()
	sum := sha256.Sum256(member)
	entry := map[string]any{
		"code": "XB", "name": "Land XB", "levels": "adm1", "source": "hdx-cod-ab", "enabled": true,
		"archive_url": url + "/xbx_admin_boundaries.geojson.zip", "member": "xbx_admin1.geojson",
		"member_sha256": hex.EncodeToString(sum[:]), "vintage": "2025-10-30",
		"name_field": "adm1_name", "pcode_field": "adm1_pcode", "name_overrides": overrides,
		"bbox": []float64{49.9, -30.1, 51.1, -28.9},
	}
	b, err := json.Marshal(map[string]any{"schema_version": 1, "countries": []any{entry}})
	if err != nil {
		t.Fatal(err)
	}
	m, err := boundaries.Parse(b)
	if err != nil {
		t.Fatalf("fixture manifest: %v", err)
	}
	return m
}

func bndLoader(t *testing.T, m *boundaries.Manifest, store database.BoundaryRepository, srv *bndServer) *boundaries.Loader {
	t.Helper()
	cfg := boundaries.DefaultConfig()
	cfg.CountryPause = 0
	return boundaries.NewLoader(m, store, srv.Client(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func noRenew(context.Context) error { return nil }

func bndRepo(t *testing.T) database.BoundaryRepository {
	t.Helper()
	repo, ok := testRepo.(database.BoundaryRepository)
	if !ok {
		t.Fatal("the repository does not implement BoundaryRepository")
	}
	return repo
}

type bndLabels struct{ state, country *string }

func bndEventLabels(t *testing.T, ctx context.Context, conn *pgx.Conn, sourceID string) bndLabels {
	t.Helper()
	var l bndLabels
	if err := conn.QueryRow(ctx, `SELECT state_name, country_name FROM events WHERE source_id = $1`, sourceID).Scan(&l.state, &l.country); err != nil {
		t.Fatalf("labels for %s: %v", sourceID, err)
	}
	return l
}

func deref(s *string) string {
	if s == nil {
		return "<null>"
	}
	return *s
}

// TestBoundaryLoaderEndToEnd: load, skip, reload on an override change, and a
// crash before commit — the whole loader against real PostGIS.
func TestBoundaryLoaderEndToEnd(t *testing.T) {
	ctx := context.Background()
	conn := advConn(t, ctx)
	bndCleanup(t, ctx, conn)

	// What was there before: a 000012-style simplified outline for XB, and an
	// ADM1 row from an older source outside the new box, under an old name.
	if _, err := conn.Exec(ctx, `
		INSERT INTO admin_boundaries (country_code, country_name, adm_level, adm_name, geom) VALUES
		('XB', 'Land XB', 0, 'Land XB', ST_Multi(ST_MakeEnvelope(49.5, -30.5, 51.5, -28.5, 4326))),
		('XB', 'Old XB', 1, 'Old State', ST_Multi(ST_MakeEnvelope(55, -30, 56, -29, 4326)))`); err != nil {
		t.Fatalf("seed old boundaries: %v", err)
	}
	for _, ev := range []struct {
		id       string
		lon, lat float64
	}{
		{"BND_ALPHA", 50.25, -29.75}, {"BND_BETA", 50.75, -29.75}, {"BND_GAMMA", 50.1, -29.25},
		{"BND_OLD", 55.5, -29.5}, {"BND_SEA", 51.05, -29.05},
	} {
		advUpsertPoint(t, ctx, ev.id, "boundary loader fixture", ev.lon, ev.lat)
	}
	if got := bndEventLabels(t, ctx, conn, "BND_OLD"); deref(got.country) != "Old XB" {
		t.Fatalf("precondition: BND_OLD named %s, want Old XB", deref(got.country))
	}

	member := bndMember(t)
	srv := newBndServer(t, member)
	store := bndRepo(t)

	// ── First load ──
	m1 := bndManifest(t, srv.URL, member, nil)
	res, err := bndLoader(t, m1, store, srv).RunOnce(ctx, noRenew)
	if err != nil || len(res.Loaded) != 1 {
		t.Fatalf("first load: res=%+v err=%v", res, err)
	}

	var adm1, adm0, multis, invalid, pcodes int
	if err := conn.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE adm_level = 1),
		       count(*) FILTER (WHERE adm_level = 0),
		       count(*) FILTER (WHERE GeometryType(geom) = 'MULTIPOLYGON'),
		       count(*) FILTER (WHERE NOT ST_IsValid(geom)),
		       count(*) FILTER (WHERE adm_level = 1 AND adm_pcode LIKE 'XB0_')
		FROM admin_boundaries WHERE country_code = 'XB'`).Scan(&adm1, &adm0, &multis, &invalid, &pcodes); err != nil {
		t.Fatal(err)
	}
	if adm1 != 3 || adm0 != 1 || multis != 4 || invalid != 0 || pcodes != 3 {
		t.Errorf("XB rows: adm1=%d adm0=%d multipolygons=%d invalid=%d pcodes=%d; want 3, 1 (old outline replaced), 4, 0, 3",
			adm1, adm0, multis, invalid, pcodes)
	}
	for id, want := range map[string]string{"BND_ALPHA": "Alpha", "BND_BETA": "Beta", "BND_GAMMA": "Gamma"} {
		if got := bndEventLabels(t, ctx, conn, id); deref(got.state) != want || deref(got.country) != "Land XB" {
			t.Errorf("%s: state=%s country=%s; want %s, Land XB", id, deref(got.state), deref(got.country), want)
		}
	}
	if got := bndEventLabels(t, ctx, conn, "BND_OLD"); got.country != nil {
		t.Errorf("BND_OLD kept the name of a deleted boundary: %s", deref(got.country))
	}

	sources, err := store.ListBoundarySources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	src := sources["XB"]
	if src.EntrySHA256 == nil || *src.EntrySHA256 != m1.Entries[0].Hash() || src.ReenrichedAt == nil ||
		src.UnitCount == nil || *src.UnitCount != 3 || src.FailedEntrySHA256 != nil {
		t.Errorf("boundary_sources after load = %+v", src)
	}

	// ── Second run, unchanged entry: nothing downloaded ──
	res, err = bndLoader(t, m1, store, srv).RunOnce(ctx, noRenew)
	if err != nil || len(res.Skipped) != 1 || srv.hitCount() != 1 {
		t.Errorf("second run: res=%+v err=%v downloads=%d; want skipped with one download in total", res, err, srv.hitCount())
	}

	// ── Changed override, same member: reloads ──
	m2 := bndManifest(t, srv.URL, member, map[string]string{"Beta": "Beta Fixed"})
	res, err = bndLoader(t, m2, store, srv).RunOnce(ctx, noRenew)
	if err != nil || len(res.Loaded) != 1 || srv.hitCount() != 2 {
		t.Fatalf("override reload: res=%+v err=%v downloads=%d", res, err, srv.hitCount())
	}
	if got := bndEventLabels(t, ctx, conn, "BND_BETA"); deref(got.state) != "Beta Fixed" {
		t.Errorf("after the override, BND_BETA state = %s, want Beta Fixed", deref(got.state))
	}

	// ── Crash between the boundary write and re-enrichment ──
	m3 := bndManifest(t, srv.URL, member, map[string]string{"Beta": "Beta Again"})
	res, err = bndLoader(t, m3, crashingStore{store}, srv).RunOnce(ctx, noRenew)
	if err != nil || len(res.Failed) != 1 {
		t.Fatalf("crash run: res=%+v err=%v; want XB failed", res, err)
	}
	var betaRows int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM admin_boundaries WHERE country_code = 'XB' AND adm_name = 'Beta Fixed'`).Scan(&betaRows); err != nil {
		t.Fatal(err)
	}
	if betaRows != 1 {
		t.Errorf("after the crash, %d 'Beta Fixed' rows; want the old boundaries (1) in place", betaRows)
	}
	if got := bndEventLabels(t, ctx, conn, "BND_BETA"); deref(got.state) != "Beta Fixed" {
		t.Errorf("after the crash, BND_BETA state = %s, want the old name", deref(got.state))
	}
	sources, err = store.ListBoundarySources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	src = sources["XB"]
	if src.EntrySHA256 == nil || *src.EntrySHA256 != m2.Entries[0].Hash() {
		t.Errorf("after the crash, entry_sha256 moved off the last committed entry")
	}
	if src.FailedEntrySHA256 == nil || *src.FailedEntrySHA256 != m3.Entries[0].Hash() || src.LastError == nil {
		t.Errorf("crash not recorded as a failure of the new entry: %+v", src)
	}
	if st := m3.Status(sources); st.Loaded != 1 || len(st.Failed) != 1 || st.Failed[0] != "XB" {
		t.Errorf("status after the crash = %+v, want loaded 1 and failed [XB]", st)
	}

	// Inside the box but in no state, and named nothing: the count the loader logs.
	n, err := store.CountUnnamedEventsInBBox(ctx, m3.Entries[0].BBox())
	if err != nil || n != 1 {
		t.Errorf("unnamed events in the box = %d (err %v), want 1 (BND_SEA)", n, err)
	}
}

// crashingStore fails every load transaction at re-enrichment, after the
// boundary rows were already written inside it.
type crashingStore struct{ database.BoundaryRepository }

func (s crashingStore) BeginBoundaryLoad(ctx context.Context, d time.Duration) (database.BoundaryLoadTx, error) {
	tx, err := s.BoundaryRepository.BeginBoundaryLoad(ctx, d)
	if err != nil {
		return nil, err
	}
	return crashingTx{tx}, nil
}

type crashingTx struct{ database.BoundaryLoadTx }

func (crashingTx) ReenrichEvents(context.Context, [4]float64, []string) (int64, error) {
	return 0, errors.New("injected crash between the boundary write and re-enrichment")
}

func TestBoundaryFailureOnFreshDatabaseRecordsOnlyTheFailure(t *testing.T) {
	ctx := context.Background()
	conn := advConn(t, ctx)
	bndCleanup(t, ctx, conn)
	store := bndRepo(t)

	// A first-ever failure creates a row with only the failure columns (P7 on a fresh database).
	at := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	if err := store.RecordBoundaryFailure(ctx, "XB", "aaaa", "checksum mismatch", at); err != nil {
		t.Fatalf("record failure: %v", err)
	}
	src := mustSource(t, ctx, store, "XB")
	if src.EntrySHA256 != nil || src.FailedAt == nil || !src.FailedAt.Equal(at) || deref(src.LastError) != "checksum mismatch" {
		t.Errorf("failure-only row = %+v", src)
	}
}

func mustSource(t *testing.T, ctx context.Context, store database.BoundaryRepository, code string) models.BoundarySource {
	t.Helper()
	all, err := store.ListBoundarySources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := all[code]
	if !ok {
		t.Fatalf("no boundary_sources row for %s", code)
	}
	return got
}

// TestMigration000017RoundTripAndReplay: down then up (twice, §11.4) inside a
// rolled-back transaction, so the shared database is untouched.
func TestMigration000017RoundTripAndReplay(t *testing.T) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, testDSN)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)
	down, err := os.ReadFile(migration000017Down)
	if err != nil {
		t.Fatal(err)
	}
	up, err := os.ReadFile(migration000017Up)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	exists := func(q string) bool {
		var ok bool
		if err := tx.QueryRow(ctx, q).Scan(&ok); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return ok
	}
	const tableQ = `SELECT to_regclass('public.boundary_sources') IS NOT NULL`
	const columnQ = `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'admin_boundaries' AND column_name = 'adm_pcode')`

	if !exists(tableQ) || !exists(columnQ) {
		t.Fatal("precondition: 000017 is applied")
	}
	if _, err := tx.Exec(ctx, string(down)); err != nil {
		t.Fatalf("down: %v", err)
	}
	if exists(tableQ) || exists(columnQ) {
		t.Error("down left boundary_sources or adm_pcode behind")
	}
	for i := 0; i < 2; i++ {
		if _, err := tx.Exec(ctx, string(up)); err != nil {
			t.Fatalf("up (pass %d): %v", i+1, err)
		}
	}
	if !exists(tableQ) || !exists(columnQ) {
		t.Error("up did not restore boundary_sources and adm_pcode")
	}
	// The constraints hold: a row with neither a load nor a failure is refused.
	if _, err := tx.Exec(ctx, `SAVEPOINT probe`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO boundary_sources (country_code) VALUES ('XQ')`); err == nil {
		t.Error("an empty boundary_sources row was accepted")
	}
	if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT probe`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO boundary_sources (country_code, failed_entry_sha256, failed_at) VALUES ('nga', 'x', now())`); err == nil {
		t.Error("a non-alpha-2 country code was accepted")
	}
}
