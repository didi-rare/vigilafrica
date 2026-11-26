package boundaries

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/models"
)

// ── Fixtures ──────────────────────────────────────────────────────────────────

// square is a closed GeoJSON polygon ring around (lon, lat), size deg.
func square(lon, lat, size float64) map[string]any {
	return map[string]any{"type": "Polygon", "coordinates": [][][]float64{{
		{lon, lat}, {lon + size, lat}, {lon + size, lat + size}, {lon, lat + size}, {lon, lat},
	}}}
}

type unit struct {
	name, pcode string
	geom        map[string]any
}

func fixtureFC(t *testing.T, units ...unit) []byte {
	t.Helper()
	features := make([]any, 0, len(units))
	for _, u := range units {
		props := map[string]any{"adm1_name": u.name}
		if u.pcode != "" {
			props["adm1_pcode"] = u.pcode
		}
		features = append(features, map[string]any{"type": "Feature", "properties": props, "geometry": u.geom})
	}
	b, err := json.Marshal(map[string]any{
		"type":     "FeatureCollection",
		"crs":      map[string]any{"type": "name", "properties": map[string]any{"name": "urn:ogc:def:crs:OGC:1.3:CRS84"}},
		"features": features,
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func twoStates(t *testing.T) []byte {
	return fixtureFC(t,
		unit{"Alpha", "XA01", square(50, -30, 0.5)},
		unit{"Beta", "XA02", square(50.5, -30, 0.5)},
	)
}

func zipOf(t *testing.T, members map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range members {
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
	return buf.Bytes()
}

func sha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// archiveServer serves fixed bodies by path over TLS and counts requests.
type archiveServer struct {
	*httptest.Server
	mu     sync.Mutex
	bodies map[string][]byte
	hits   map[string]int
}

func newArchiveServer(t *testing.T) *archiveServer {
	t.Helper()
	s := &archiveServer{bodies: map[string][]byte{}, hits: map[string]int{}}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.hits[r.URL.Path]++
		body, ok := s.bodies[r.URL.Path]
		s.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body) // a failed write surfaces as a client-side error in the test
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *archiveServer) serve(path string, body []byte) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bodies[path] = body
	return s.URL + path
}

func (s *archiveServer) hitCount(path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits[path]
}

// entryFor builds an adm1 manifest entry for a zip served at url whose member
// is memberBytes.
func entryFor(code, url string, memberBytes []byte) map[string]any {
	return map[string]any{
		"code": code, "name": "Land " + code, "levels": "adm1", "source": "hdx-cod-ab", "enabled": true,
		"archive_url": url, "member": strings.ToLower(code) + "_admin1.geojson",
		"member_sha256": sha(memberBytes), "vintage": "2025-10-30",
		"name_field": "adm1_name", "pcode_field": "adm1_pcode",
		"bbox": []float64{49.9, -30.1, 51.1, -29.4},
	}
}

func parseManifest(t *testing.T, entries ...map[string]any) *Manifest {
	t.Helper()
	m, err := Parse(manifestJSON(t, entries...))
	if err != nil {
		t.Fatalf("fixture manifest: %v", err)
	}
	return m
}

// ── Fake store ────────────────────────────────────────────────────────────────

type fakeStore struct {
	mu        sync.Mutex
	sources   map[string]models.BoundarySource
	loads     []models.BoundaryReplacement
	failures  []string // country codes, in order
	begun     int
	committed int
	rolled    int
	// failReenrich makes ReenrichEvents fail, after ReplaceBoundaries succeeded.
	failReenrich bool
	listErr      error
}

func newFakeStore() *fakeStore { return &fakeStore{sources: map[string]models.BoundarySource{}} }

func (f *fakeStore) ListBoundarySources(context.Context) (map[string]models.BoundarySource, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make(map[string]models.BoundarySource, len(f.sources))
	for k, v := range f.sources {
		out[k] = v
	}
	return out, nil
}

func (f *fakeStore) RecordBoundaryFailure(_ context.Context, code, entrySHA, msg string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.sources[code]
	s.CountryCode = code
	s.FailedEntrySHA256, s.FailedAt, s.LastError = &entrySHA, &at, &msg
	f.sources[code] = s
	f.failures = append(f.failures, code)
	return nil
}

func (f *fakeStore) CountUnnamedEventsInBBox(context.Context, [4]float64) (int, error) { return 0, nil }

func (f *fakeStore) BeginBoundaryLoad(context.Context, time.Duration) (database.BoundaryLoadTx, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.begun++
	return &fakeTx{store: f}, nil
}

type fakeTx struct {
	store    *fakeStore
	pending  *models.BoundaryReplacement
	record   *models.BoundaryLoadRecord
	finished bool
}

func (t *fakeTx) ReplaceBoundaries(_ context.Context, r models.BoundaryReplacement) (models.BoundaryReplaceResult, error) {
	t.pending = &r
	return models.BoundaryReplaceResult{Units: len(r.Units)}, nil
}

func (t *fakeTx) ReenrichEvents(context.Context, [4]float64, []string) (int64, error) {
	if t.store.failReenrich {
		return 0, errors.New("injected crash between the boundary write and re-enrichment")
	}
	return 3, nil
}

func (t *fakeTx) UpsertBoundarySource(_ context.Context, rec models.BoundaryLoadRecord) error {
	t.record = &rec
	return nil
}

func (t *fakeTx) Commit(context.Context) error {
	t.store.mu.Lock()
	defer t.store.mu.Unlock()
	t.finished = true
	t.store.committed++
	t.store.loads = append(t.store.loads, *t.pending)
	now := time.Now()
	entry := t.record.EntrySHA256
	t.store.sources[t.record.CountryCode] = models.BoundarySource{
		CountryCode: t.record.CountryCode, EntrySHA256: &entry, ReenrichedAt: &now, LoadedAt: &now,
	}
	return nil
}

func (t *fakeTx) Rollback(context.Context) error {
	if t.finished {
		return nil
	}
	t.finished = true
	t.store.mu.Lock()
	defer t.store.mu.Unlock()
	t.store.rolled++
	return nil
}

// ── Harness ───────────────────────────────────────────────────────────────────

type renewCounter struct {
	mu    sync.Mutex
	calls int
	// failAfter: renewals succeed this many times, then fail. <0 never fails.
	failAfter int
}

func (r *renewCounter) renew(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.failAfter >= 0 && r.calls > r.failAfter {
		return errors.New("lease held by another instance")
	}
	return nil
}

func newTestLoader(m *Manifest, store database.BoundaryRepository, srv *archiveServer) *Loader {
	cfg := DefaultConfig()
	cfg.CountryPause = 0
	l := NewLoader(m, store, srv.Client(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	l.sleep = func(ctx context.Context, _ time.Duration) error { return ctx.Err() }
	return l
}

func alwaysRenew(context.Context) error { return nil }

// ── Tests (task 3.6) ──────────────────────────────────────────────────────────

func TestLoaderLoadsAndSecondRunSkips(t *testing.T) {
	srv := newArchiveServer(t)
	member := twoStates(t)
	url := srv.serve("/xa.zip", zipOf(t, map[string][]byte{
		"xa_admin1.geojson":    member,
		"xa_admin1_em.geojson": []byte("not this one"),
		"xa_admin2.geojson":    []byte("nor this"),
	}))
	store := newFakeStore()
	l := newTestLoader(parseManifest(t, entryFor("XA", url, member)), store, srv)

	res, err := l.RunOnce(context.Background(), alwaysRenew)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if len(res.Loaded) != 1 || store.committed != 1 {
		t.Fatalf("first run loaded %v, commits %d; want XA loaded once", res.Loaded, store.committed)
	}
	got := store.loads[0]
	if got.CountryName != "Land XA" || len(got.Units) != 2 || got.Units[0].Name != "Alpha" ||
		got.Units[0].PCode == nil || *got.Units[0].PCode != "XA01" {
		t.Errorf("loaded %+v; want two units Alpha/XA01, Beta/XA02 named for Land XA", got)
	}

	res, err = l.RunOnce(context.Background(), alwaysRenew)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if hits := srv.hitCount("/xa.zip"); hits != 1 {
		t.Errorf("unchanged entry downloaded %d times in two runs; want 1", hits)
	}
	if len(res.Skipped) != 1 || store.committed != 1 {
		t.Errorf("second run skipped %v, commits %d; want XA skipped, no new commit", res.Skipped, store.committed)
	}
}

func TestLoaderRejectsWrongChecksum(t *testing.T) {
	srv := newArchiveServer(t)
	member := twoStates(t)
	url := srv.serve("/xa.zip", zipOf(t, map[string][]byte{"xa_admin1.geojson": member}))
	e := entryFor("XA", url, member)
	e["member_sha256"] = sha([]byte("some other file"))
	store := newFakeStore()

	res, err := newTestLoader(parseManifest(t, e), store, srv).RunOnce(context.Background(), alwaysRenew)
	if err != nil {
		t.Fatalf("a checksum failure must not fail the run: %v", err)
	}
	if len(res.Failed) != 1 || store.begun != 0 {
		t.Fatalf("failed=%v, transactions begun=%d; want XA failed before any write", res.Failed, store.begun)
	}
	src := store.sources["XA"]
	if src.LastError == nil || !strings.Contains(*src.LastError, "checksum mismatch") {
		t.Errorf("last_error = %v, want a checksum mismatch", src.LastError)
	}
	if src.EntrySHA256 != nil {
		t.Errorf("a failed load recorded a successful entry hash")
	}
}

func TestLoaderRejectsUnusableFiles(t *testing.T) {
	noName := fixtureFC(t, unit{"Alpha", "XA01", square(50, -30, 0.5)}, unit{"", "XA02", square(50.5, -30, 0.5)})
	lineGeom := fixtureFC(t, unit{"Alpha", "XA01", map[string]any{"type": "LineString", "coordinates": [][]float64{{50, -30}, {51, -29}}}})
	tests := []struct {
		name   string
		member []byte
		want   error
	}{
		{"zero features", fixtureFC(t), ErrNoFeatures},
		{"feature without a name", noName, ErrFeatureWithoutName},
		{"non-polygon geometry", lineGeom, ErrBadGeoJSON},
		{"not a FeatureCollection", []byte(`{"type":"Feature"}`), ErrBadGeoJSON},
		{"projected CRS", []byte(`{"type":"FeatureCollection","crs":{"type":"name","properties":{"name":"urn:ogc:def:crs:EPSG::32631"}},"features":[]}`), ErrBadGeoJSON},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			srv := newArchiveServer(t)
			url := srv.serve("/xa.zip", zipOf(t, map[string][]byte{"xa_admin1.geojson": tt.member}))
			store := newFakeStore()
			l := newTestLoader(parseManifest(t, entryFor("XA", url, tt.member)), store, srv)
			var loadErr error
			l.log = slog.New(slog.NewTextHandler(io.Discard, nil))
			_, err := l.RunOnce(context.Background(), alwaysRenew)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if store.begun != 0 {
				t.Errorf("an unusable file reached the database")
			}
			e, _ := l.manifest.Entry("XA")
			_, _, loadErr = l.fetchMember(context.Background(), e)
			if loadErr != nil {
				t.Fatalf("fixture should pass the checksum: %v", loadErr)
			}
			if _, perr := parseUnits(tt.member, e, l.log); !errors.Is(perr, tt.want) {
				t.Errorf("parse error = %v, want %v", perr, tt.want)
			}
			if len(store.failures) != 1 {
				t.Errorf("failure not recorded: %v", store.failures)
			}
		})
	}
}

func TestLoaderRejectsOversizedArchive(t *testing.T) {
	srv := newArchiveServer(t)
	member := twoStates(t)
	url := srv.serve("/xa.zip", zipOf(t, map[string][]byte{"xa_admin1.geojson": member}))
	store := newFakeStore()
	l := newTestLoader(parseManifest(t, entryFor("XA", url, member)), store, srv)
	l.cfg.MaxArchiveBytes = 64

	e, _ := l.manifest.Entry("XA")
	if _, _, err := l.fetchMember(context.Background(), e); !errors.Is(err, ErrArchiveTooLarge) {
		t.Fatalf("err = %v, want ErrArchiveTooLarge", err)
	}
	if _, err := l.RunOnce(context.Background(), alwaysRenew); err != nil {
		t.Fatal(err)
	}
	if len(store.failures) != 1 || store.begun != 0 {
		t.Errorf("oversized archive: failures=%v begun=%d", store.failures, store.begun)
	}
}

func TestLoaderRejectsMissingMember(t *testing.T) {
	srv := newArchiveServer(t)
	member := twoStates(t)
	url := srv.serve("/xa.zip", zipOf(t, map[string][]byte{"xa_admin1_em.geojson": member}))
	l := newTestLoader(parseManifest(t, entryFor("XA", url, member)), newFakeStore(), srv)
	e, _ := l.manifest.Entry("XA")
	if _, _, err := l.fetchMember(context.Background(), e); !errors.Is(err, ErrMemberNotFound) {
		t.Fatalf("err = %v, want ErrMemberNotFound (the _em variant must never stand in)", err)
	}
}

func TestLoaderReloadsWhenOverrideChangesWithSameMember(t *testing.T) {
	srv := newArchiveServer(t)
	member := twoStates(t)
	url := srv.serve("/xa.zip", zipOf(t, map[string][]byte{"xa_admin1.geojson": member}))
	store := newFakeStore()
	entry := entryFor("XA", url, member)

	if _, err := newTestLoader(parseManifest(t, entry), store, srv).RunOnce(context.Background(), alwaysRenew); err != nil {
		t.Fatal(err)
	}
	entry["name_overrides"] = map[string]string{"Beta": "Beta Corrected"}
	res, err := newTestLoader(parseManifest(t, entry), store, srv).RunOnce(context.Background(), alwaysRenew)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Loaded) != 1 || store.committed != 2 || srv.hitCount("/xa.zip") != 2 {
		t.Fatalf("override change: loaded=%v commits=%d downloads=%d; want a reload", res.Loaded, store.committed, srv.hitCount("/xa.zip"))
	}
	if got := store.loads[1].Units[1].Name; got != "Beta Corrected" {
		t.Errorf("override not applied: %q", got)
	}
}

func TestLoaderOneCountryFailureDoesNotStopOthers(t *testing.T) {
	srv := newArchiveServer(t)
	member := twoStates(t)
	good := srv.serve("/xb.zip", zipOf(t, map[string][]byte{"xb_admin1.geojson": member}))
	bad := srv.URL + "/missing.zip" // 404
	store := newFakeStore()
	renew := &renewCounter{failAfter: -1}

	res, err := newTestLoader(parseManifest(t, entryFor("XA", bad, member), entryFor("XB", good, member)), store, srv).
		RunOnce(context.Background(), renew.renew)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(res.Failed, ",") != "XA" || strings.Join(res.Loaded, ",") != "XB" {
		t.Errorf("failed=%v loaded=%v; want XA failed, XB loaded", res.Failed, res.Loaded)
	}
	// XA: renew after the entry. XB: renew before writing + after the entry.
	if renew.calls != 3 {
		t.Errorf("lease renewed %d times, want 3", renew.calls)
	}
}

func TestLoaderDefersFailedEntryUntilRetryWindow(t *testing.T) {
	srv := newArchiveServer(t)
	member := twoStates(t)
	url := srv.serve("/xa.zip", zipOf(t, map[string][]byte{"xa_admin1.geojson": member}))
	e := entryFor("XA", url, member)
	e["member_sha256"] = sha([]byte("stale checksum"))
	store := newFakeStore()
	l := newTestLoader(parseManifest(t, e), store, srv)
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }

	if _, err := l.RunOnce(context.Background(), alwaysRenew); err != nil {
		t.Fatal(err)
	}
	if srv.hitCount("/xa.zip") != 1 {
		t.Fatalf("first attempt did not download")
	}

	now = now.Add(23 * time.Hour)
	res, err := l.RunOnce(context.Background(), alwaysRenew)
	if err != nil {
		t.Fatal(err)
	}
	if srv.hitCount("/xa.zip") != 1 || len(res.Deferred) != 1 {
		t.Errorf("within the retry window: downloads=%d deferred=%v; want no download", srv.hitCount("/xa.zip"), res.Deferred)
	}

	now = now.Add(2 * time.Hour) // 25 h after the failure
	if _, err := l.RunOnce(context.Background(), alwaysRenew); err != nil {
		t.Fatal(err)
	}
	if srv.hitCount("/xa.zip") != 2 {
		t.Errorf("after the retry window the entry was not retried")
	}
}

func TestLoaderRetriesAtOnceWhenTheFailedEntryChanges(t *testing.T) {
	srv := newArchiveServer(t)
	member := twoStates(t)
	url := srv.serve("/xa.zip", zipOf(t, map[string][]byte{"xa_admin1.geojson": member}))
	e := entryFor("XA", url, member)
	good := e["member_sha256"]
	e["member_sha256"] = sha([]byte("stale checksum"))
	store := newFakeStore()
	if _, err := newTestLoader(parseManifest(t, e), store, srv).RunOnce(context.Background(), alwaysRenew); err != nil {
		t.Fatal(err)
	}
	e["member_sha256"] = good // the reviewed manifest fix (P7)
	res, err := newTestLoader(parseManifest(t, e), store, srv).RunOnce(context.Background(), alwaysRenew)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Loaded) != 1 {
		t.Errorf("a corrected entry waited for the retry window: %+v", res)
	}
	if st := parseManifest(t, e).Status(store.sources); st.Loaded != 1 || len(st.Failed) != 0 {
		t.Errorf("status after the fix = %+v, want loaded 1, failed []", st)
	}
}

func TestLoaderStopsWhenLeaseRenewalFails(t *testing.T) {
	srv := newArchiveServer(t)
	member := twoStates(t)
	a := srv.serve("/xa.zip", zipOf(t, map[string][]byte{"xa_admin1.geojson": member}))
	b := srv.serve("/xb.zip", zipOf(t, map[string][]byte{"xb_admin1.geojson": member}))
	store := newFakeStore()
	renew := &renewCounter{failAfter: 1} // the pre-write renewal for XA succeeds, the next fails

	res, err := newTestLoader(parseManifest(t, entryFor("XA", a, member), entryFor("XB", b, member)), store, srv).
		RunOnce(context.Background(), renew.renew)
	if !errors.Is(err, ErrLeaseRenewal) {
		t.Fatalf("err = %v, want ErrLeaseRenewal", err)
	}
	if srv.hitCount("/xb.zip") != 0 || len(res.Loaded) != 1 {
		t.Errorf("after a failed renewal: XB downloads=%d loaded=%v; want the run stopped after XA", srv.hitCount("/xb.zip"), res.Loaded)
	}
}

func TestLoaderRenewsBeforeWriting(t *testing.T) {
	srv := newArchiveServer(t)
	member := twoStates(t)
	url := srv.serve("/xa.zip", zipOf(t, map[string][]byte{"xa_admin1.geojson": member}))
	store := newFakeStore()
	renew := &renewCounter{failAfter: 0} // the very first renewal fails

	_, err := newTestLoader(parseManifest(t, entryFor("XA", url, member)), store, srv).RunOnce(context.Background(), renew.renew)
	if !errors.Is(err, ErrLeaseRenewal) {
		t.Fatalf("err = %v, want ErrLeaseRenewal", err)
	}
	if store.begun != 0 || len(store.failures) != 0 {
		t.Errorf("lost lease: transactions=%d failures=%v; want nothing written or recorded", store.begun, store.failures)
	}
}

func TestLoaderRollsBackWhenReenrichmentFails(t *testing.T) {
	srv := newArchiveServer(t)
	member := twoStates(t)
	url := srv.serve("/xa.zip", zipOf(t, map[string][]byte{"xa_admin1.geojson": member}))
	store := newFakeStore()
	store.failReenrich = true

	res, err := newTestLoader(parseManifest(t, entryFor("XA", url, member)), store, srv).RunOnce(context.Background(), alwaysRenew)
	if err != nil {
		t.Fatal(err)
	}
	if store.committed != 0 || store.rolled != 1 || len(res.Failed) != 1 {
		t.Errorf("commits=%d rollbacks=%d failed=%v; want rolled back and recorded as failed", store.committed, store.rolled, res.Failed)
	}
}

func TestLoaderDoesNotRecordShutdownAsFailure(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-block:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() { close(block); srv.Close() })
	member := twoStates(t)
	store := newFakeStore()
	cfg := DefaultConfig()
	l := NewLoader(parseManifest(t, entryFor("XA", srv.URL+"/xa.zip", member)), store, srv.Client(), cfg,
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	_, err := l.RunOnce(ctx, alwaysRenew)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if len(store.failures) != 0 {
		t.Errorf("a shutdown was recorded as a failed entry: %v", store.failures)
	}
}

func TestLoaderStopsWhenSourcesUnreadable(t *testing.T) {
	srv := newArchiveServer(t)
	member := twoStates(t)
	store := newFakeStore()
	store.listErr = errors.New("db down")
	_, err := newTestLoader(parseManifest(t, entryFor("XA", srv.URL+"/xa.zip", member)), store, srv).
		RunOnce(context.Background(), alwaysRenew)
	if err == nil || errors.Is(err, ErrLeaseRenewal) {
		t.Fatalf("err = %v, want a non-lease error", err)
	}
	if srv.hitCount("/xa.zip") != 0 {
		t.Errorf("downloaded without knowing what is loaded")
	}
}

func TestLoaderLoadsPlainFileADM0Outline(t *testing.T) {
	srv := newArchiveServer(t)
	outline := fixtureFC(t, unit{"", "", square(50, -30, 1)})
	url := srv.serve("/xa.geojson", outline)
	e := map[string]any{
		"code": "XA", "name": "Land XA", "levels": "adm0", "source": "geoboundaries", "enabled": true,
		"archive_url": url, "member_sha256": sha(outline), "vintage": "2023-01-01",
		"bbox": []float64{49.9, -30.1, 51.1, -28.9},
	}
	store := newFakeStore()
	res, err := newTestLoader(parseManifest(t, e), store, srv).RunOnce(context.Background(), alwaysRenew)
	if err != nil || len(res.Loaded) != 1 {
		t.Fatalf("adm0 outline: res=%+v err=%v", res, err)
	}
	if got := store.loads[0]; got.Levels != "adm0" || len(got.Units) != 1 {
		t.Errorf("adm0 load = %+v", got)
	}
}

func TestStatusCountsLoadedAndCurrentFailures(t *testing.T) {
	member := twoStates(t)
	m := parseManifest(t, entryFor("XA", "https://e.org/a.zip", member), entryFor("XB", "https://e.org/b.zip", member),
		entryFor("XC", "https://e.org/c.zip", member))
	xa, _ := m.Entry("XA")
	xb, _ := m.Entry("XB")
	now := time.Now()
	old := "0000000000000000000000000000000000000000000000000000000000000000"
	xaHash, xbHash := xa.Hash(), xb.Hash()
	sources := map[string]models.BoundarySource{
		// loaded on an older entry, current entry failed: both loaded and failed
		"XA": {CountryCode: "XA", EntrySHA256: &old, ReenrichedAt: &now, FailedEntrySHA256: &xaHash, FailedAt: &now},
		// loaded on the current entry; a stale failure of an older entry
		"XB": {CountryCode: "XB", EntrySHA256: &xbHash, ReenrichedAt: &now, FailedEntrySHA256: &old, FailedAt: &now},
		// not in the manifest any more: ignored
		"ZZ": {CountryCode: "ZZ", EntrySHA256: &old, ReenrichedAt: &now},
	}
	// Commit acknowledgement lost: the current entry loaded, then a failure of
	// that same entry was recorded after the commit. Not failed.
	xc, _ := m.Entry("XC")
	xcHash := xc.Hash()
	later := now.Add(time.Second)
	sources["XC"] = models.BoundarySource{CountryCode: "XC", EntrySHA256: &xcHash, ReenrichedAt: &now, LoadedAt: &now,
		FailedEntrySHA256: &xcHash, FailedAt: &later}
	st := m.Status(sources)
	if st.Loaded != 3 || fmt.Sprint(st.Failed) != "[XA]" || st.ManifestSHA256 != m.Checksum() {
		t.Errorf("status = %+v, want loaded 3, failed [XA] (XC's failure is stale)", st)
	}
	if empty := m.Status(nil); empty.Failed == nil || empty.Loaded != 0 {
		t.Errorf("empty status = %+v, want loaded 0 and failed [] (not null)", empty)
	}
}

func TestConfigFromEnv(t *testing.T) {
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

	enabled, cfg := ConfigFromEnv(env(nil), 5*time.Minute, quiet)
	if !enabled || cfg != DefaultConfig() {
		t.Errorf("defaults: enabled=%v cfg=%+v", enabled, cfg)
	}
	if cfg.RetryAfter != 24*time.Hour || cfg.MaxArchiveBytes != 128<<20 || cfg.TxTimeout != 2*time.Minute || cfg.CountryPause != time.Second {
		t.Errorf("defaults differ from spec §2: %+v", cfg)
	}

	enabled, cfg = ConfigFromEnv(env(map[string]string{
		"BOUNDARY_LOADER_ENABLED":    "false",
		"BOUNDARY_RETRY_HOURS":       "6",
		"BOUNDARY_MAX_ARCHIVE_BYTES": "1000",
		"BOUNDARY_TX_TIMEOUT":        "90s",
		"BOUNDARY_COUNTRY_PAUSE_MS":  "0",
	}), 5*time.Minute, quiet)
	if enabled || cfg.RetryAfter != 6*time.Hour || cfg.MaxArchiveBytes != 1000 || cfg.TxTimeout != 90*time.Second || cfg.CountryPause != 0 {
		t.Errorf("overrides: enabled=%v cfg=%+v", enabled, cfg)
	}

	enabled, cfg = ConfigFromEnv(env(map[string]string{
		"BOUNDARY_LOADER_ENABLED": "nope",
		"BOUNDARY_RETRY_HOURS":    "-1",
		"BOUNDARY_TX_TIMEOUT":     "6m", // over the lease TTL
	}), 5*time.Minute, quiet)
	if !enabled || cfg.RetryAfter != DefaultRetryAfter || cfg.TxTimeout != DefaultTxTimeout {
		t.Errorf("invalid values must fall back to defaults: enabled=%v cfg=%+v", enabled, cfg)
	}
}

func TestLoaderRedirectPolicy(t *testing.T) {
	member := twoStates(t)
	archive := zipOf(t, map[string][]byte{"xa_admin1.geojson": member})
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive) // reaching this handler at all is the failure
	}))
	t.Cleanup(plain.Close)
	var tlsURL string
	tlsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/to-http":
			http.Redirect(w, r, plain.URL+"/xa.zip", http.StatusFound)
		case "/to-https":
			http.Redirect(w, r, tlsURL+"/xa.zip", http.StatusFound)
		case "/loop":
			http.Redirect(w, r, tlsURL+"/loop", http.StatusFound)
		case "/xa.zip":
			_, _ = w.Write(archive) // a failed write surfaces as a client error
		}
	}))
	t.Cleanup(tlsSrv.Close)
	tlsURL = tlsSrv.URL

	tests := []struct {
		path string
		want error
	}{
		{"/to-https", nil},
		{"/to-http", ErrInsecureRedirect},
		{"/loop", ErrInsecureRedirect},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.path, func(t *testing.T) {
			m := parseManifest(t, entryFor("XA", tlsURL+tt.path, member))
			l := NewLoader(m, newFakeStore(), tlsSrv.Client(), DefaultConfig(), slog.New(slog.NewTextHandler(io.Discard, nil)))
			e, _ := m.Entry("XA")
			_, _, err := l.fetchMember(context.Background(), e)
			if tt.want == nil && err != nil {
				t.Fatalf("https redirect refused: %v", err)
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
	if tlsSrv.Client().CheckRedirect != nil {
		t.Error("NewLoader modified the caller's http.Client")
	}
}
