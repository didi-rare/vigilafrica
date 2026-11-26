package boundaries

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/models"
)

// Sentinel errors a caller or test may branch on (§4.8).
var (
	// ErrLeaseRenewal: the scheduler lease could not be renewed (an error, or
	// another holder has it). The run stops; nothing more is written.
	ErrLeaseRenewal = errors.New("boundary loader: scheduler lease renewal failed")
	// ErrChecksumMismatch: the downloaded member is not the file the manifest
	// names (decision P7). Fixed by a reviewed manifest update.
	ErrChecksumMismatch = errors.New("boundary member checksum mismatch")
	// ErrArchiveTooLarge: the archive or member exceeds BOUNDARY_MAX_ARCHIVE_BYTES.
	ErrArchiveTooLarge = errors.New("boundary archive exceeds the size limit")
	// ErrMemberNotFound: the archive has no member with the manifest's name.
	ErrMemberNotFound = errors.New("boundary member not found in archive")
	// ErrNoFeatures: the member has no features.
	ErrNoFeatures = errors.New("boundary file has no features")
	// ErrFeatureWithoutName: an ADM1 feature has no value in name_field.
	ErrFeatureWithoutName = errors.New("boundary feature has no name")
	// ErrBadGeoJSON: the member is not a usable WGS84 polygon FeatureCollection.
	ErrBadGeoJSON = errors.New("boundary file is not a usable GeoJSON FeatureCollection")
)

const (
	userAgent = "VigilAfrica-boundary-loader/1.0"
	// maxLastErrorLen bounds boundary_sources.last_error.
	maxLastErrorLen = 2000
	// failureRecordTimeout bounds recording a failure after the run's context
	// has been cancelled.
	failureRecordTimeout = 10 * time.Second
	// maxRedirects caps redirect hops on a download (HDX answers with one, to
	// its storage).
	maxRedirects = 5
)

// ErrInsecureRedirect: a download redirected to a non-https URL, or redirected
// more than maxRedirects times.
var ErrInsecureRedirect = errors.New("boundary download redirect refused")

// httpsOnlyRedirect keeps every hop of a download on https; the manifest's own
// URL is already https-only (validate).
func httpsOnlyRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("%w: more than %d redirects", ErrInsecureRedirect, maxRedirects)
	}
	if req.URL.Scheme != "https" {
		return fmt.Errorf("%w: to %s URL %s", ErrInsecureRedirect, req.URL.Scheme, req.URL.Redacted())
	}
	return nil
}

// Loader loads the manifest's boundaries. It is safe to call Run repeatedly;
// it is not safe to call Run concurrently with itself (the scheduler lease is
// what serialises runs across processes, and the scheduler goroutine within one).
type Loader struct {
	manifest *Manifest
	store    database.BoundaryRepository
	client   *http.Client
	cfg      Config
	log      *slog.Logger

	// Seams for tests in this package; set by NewLoader.
	now   func() time.Time
	sleep func(context.Context, time.Duration) error
}

// NewLoader builds a loader. client may be nil (a client with
// cfg.DownloadTimeout is made); its redirect policy is replaced by an
// https-only one. log may be nil (slog.Default()).
func NewLoader(m *Manifest, store database.BoundaryRepository, client *http.Client, cfg Config, log *slog.Logger) *Loader {
	if client == nil {
		client = &http.Client{Timeout: cfg.DownloadTimeout}
	}
	// A copy, so the caller's client is not modified; the redirect policy is
	// enforced whatever client is passed in.
	c := *client
	c.CheckRedirect = httpsOnlyRedirect
	client = &c
	if log == nil {
		log = slog.Default()
	}
	return &Loader{
		manifest: m,
		store:    store,
		client:   client,
		cfg:      cfg,
		log:      log,
		now:      time.Now,
		sleep:    sleepCtx,
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// RunResult summarises one run by country code.
type RunResult struct {
	Loaded   []string // loaded in this run
	Skipped  []string // entry unchanged since its last load
	Deferred []string // failed recently with this entry; retry window not over
	Failed   []string // attempted in this run and failed
}

// Run implements the scheduler's boundary step; see RunOnce.
func (l *Loader) Run(ctx context.Context, renewLease func(context.Context) error) error {
	_, err := l.RunOnce(ctx, renewLease)
	return err
}

// RunOnce visits every manifest entry in order (spec §2). One country's
// failure is recorded and logged and does not stop the others. The returned
// error is reserved for what stops the whole run: the context ending, the
// lease renewal failing (wraps ErrLeaseRenewal), or boundary_sources being
// unreadable.
func (l *Loader) RunOnce(ctx context.Context, renewLease func(context.Context) error) (RunResult, error) {
	var res RunResult
	start := l.now()
	l.log.Info("boundary load started", "countries", len(l.manifest.Entries), "manifest_sha256", l.manifest.Checksum())

	sources, err := l.store.ListBoundarySources(ctx)
	if err != nil {
		return res, fmt.Errorf("boundary loader: %w", err)
	}

	for i, e := range l.manifest.Entries {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		elog := l.log.With("country", e.Code)
		src, known := sources[e.Code]

		if known && src.EntrySHA256 != nil && *src.EntrySHA256 == e.Hash() && src.ReenrichedAt != nil {
			res.Skipped = append(res.Skipped, e.Code)
			elog.Debug("boundary load skipped: entry unchanged")
			continue
		}
		if known && src.FailedEntrySHA256 != nil && *src.FailedEntrySHA256 == e.Hash() &&
			src.FailedAt != nil && l.now().Sub(*src.FailedAt) < l.cfg.RetryAfter {
			res.Deferred = append(res.Deferred, e.Code)
			elog.Warn("boundary load deferred: this entry failed recently",
				"failed_at", src.FailedAt.UTC(), "retry_after", src.FailedAt.Add(l.cfg.RetryAfter).UTC())
			continue
		}

		err := l.loadEntry(ctx, elog, e, renewLease)
		switch {
		case err == nil:
			res.Loaded = append(res.Loaded, e.Code)
		case errors.Is(err, ErrLeaseRenewal):
			return res, err
		case ctx.Err() != nil:
			// Shutdown mid-load is not the entry's fault: recording it would
			// hold the country back for a whole retry window.
			return res, ctx.Err()
		default:
			res.Failed = append(res.Failed, e.Code)
			elog.Error("boundary load failed", "err", err, "entry_sha256", e.Hash())
			l.recordFailure(ctx, elog, e, err)
		}

		if i < len(l.manifest.Entries)-1 {
			if err := l.sleep(ctx, l.cfg.CountryPause); err != nil {
				return res, err
			}
		}
		if err := renewLease(ctx); err != nil {
			return res, fmt.Errorf("%w: after %s: %w", ErrLeaseRenewal, e.Code, err)
		}
	}

	l.log.Info("boundary load complete",
		"duration_ms", l.now().Sub(start).Milliseconds(),
		"loaded", res.Loaded, "skipped", len(res.Skipped), "deferred", res.Deferred, "failed", res.Failed)
	return res, nil
}

func (l *Loader) recordFailure(ctx context.Context, elog *slog.Logger, e Entry, loadErr error) {
	msg := loadErr.Error()
	if len(msg) > maxLastErrorLen {
		msg = msg[:maxLastErrorLen]
	}
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failureRecordTimeout)
	defer cancel()
	if err := l.store.RecordBoundaryFailure(rctx, e.Code, e.Hash(), msg, l.now()); err != nil {
		elog.Error("boundary load failure could not be recorded; it will be retried next run", "err", err)
	}
}

// loadEntry downloads, verifies, parses and swaps one country.
func (l *Loader) loadEntry(ctx context.Context, elog *slog.Logger, e Entry, renewLease func(context.Context) error) error {
	t0 := l.now()
	member, archiveBytes, err := l.fetchMember(ctx, e)
	if err != nil {
		return err
	}
	units, err := parseUnits(member, e, elog)
	if err != nil {
		return err
	}

	// The download may have taken a while: hold a fresh lease before writing,
	// so the transaction never runs after another instance took over.
	if err := renewLease(ctx); err != nil {
		return fmt.Errorf("%w: before writing %s: %w", ErrLeaseRenewal, e.Code, err)
	}

	// Two bounds: statement_timeout (server side, per statement) and a context
	// deadline over the whole transaction, both BOUNDARY_TX_TIMEOUT, so the
	// transaction as a whole stays under the lease TTL.
	txCtx, cancel := context.WithTimeout(ctx, l.cfg.TxTimeout)
	defer cancel()
	txStart := l.now()
	tx, err := l.store.BeginBoundaryLoad(txCtx, l.cfg.TxTimeout)
	if err != nil {
		return err
	}
	defer func() {
		// No-op after Commit. With txCtx possibly expired, roll back on a
		// fresh context so the connection is released cleanly.
		rctx, rcancel := context.WithTimeout(context.WithoutCancel(ctx), failureRecordTimeout)
		defer rcancel()
		if rbErr := tx.Rollback(rctx); rbErr != nil {
			elog.Error("boundary load rollback failed", "err", rbErr)
		}
	}()

	replaced, err := tx.ReplaceBoundaries(txCtx, models.BoundaryReplacement{
		CountryCode: e.Code, CountryName: e.Name, Levels: e.Levels, Units: units,
	})
	if err != nil {
		return err
	}
	names := append([]string{e.Name}, replaced.PreviousCountryNames...)
	reenriched, err := tx.ReenrichEvents(txCtx, e.BBox(), names)
	if err != nil {
		return err
	}
	var memberName *string
	if e.Member != "" {
		memberName = &e.Member
	}
	if err := tx.UpsertBoundarySource(txCtx, models.BoundaryLoadRecord{
		CountryCode: e.Code, CountryName: e.Name, Levels: e.Levels, Source: e.Source,
		ArchiveURL: e.ArchiveURL, Member: memberName, MemberSHA256: e.MemberSHA256,
		EntrySHA256: e.Hash(), Vintage: e.VintageDate(), UnitCount: replaced.Units,
	}); err != nil {
		return err
	}
	if err := tx.Commit(txCtx); err != nil {
		return err
	}
	txDur := l.now().Sub(txStart)

	elog.Info("boundary load committed",
		"units", replaced.Units, "events_reenriched", reenriched,
		"archive_bytes", archiveBytes, "member_bytes", len(member),
		"tx_ms", txDur.Milliseconds(), "duration_ms", l.now().Sub(t0).Milliseconds(),
		"vintage", e.Vintage, "entry_sha256", e.Hash())

	unnamed, err := l.store.CountUnnamedEventsInBBox(ctx, e.BBox())
	switch {
	case err != nil:
		elog.Warn("could not count events left without a country after the load", "err", err)
	case unnamed > 0:
		// Recorded for the null-country cleanup (feature-continental-coverage
		// task 5.10), which is a later PR; nothing to run yet.
		elog.Warn("events inside the country's box have no country after the load; count recorded for the null-country cleanup",
			"events_null_country", unnamed)
	default:
		elog.Info("no events inside the country's box are left without a country", "events_null_country", 0)
	}
	return nil
}

// fetchMember downloads the entry's archive and returns the verified member
// bytes and the archive size.
func (l *Loader) fetchMember(ctx context.Context, e Entry) ([]byte, int, error) {
	dctx, cancel := context.WithTimeout(ctx, l.cfg.DownloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(dctx, http.MethodGet, e.ArchiveURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("build boundary download request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := l.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("download boundary archive: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("download boundary archive: HTTP %d", resp.StatusCode)
	}
	maxBytes := l.cfg.MaxArchiveBytes
	if resp.ContentLength > maxBytes {
		return nil, 0, fmt.Errorf("%w: Content-Length %d > %d bytes", ErrArchiveTooLarge, resp.ContentLength, maxBytes)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, 0, fmt.Errorf("read boundary archive: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return nil, 0, fmt.Errorf("%w: more than %d bytes", ErrArchiveTooLarge, maxBytes)
	}

	member := body
	if e.Member != "" {
		member, err = extractMember(body, e.Member, maxBytes)
		if err != nil {
			return nil, 0, err
		}
	}
	sum := sha256.Sum256(member)
	if got := hex.EncodeToString(sum[:]); got != e.MemberSHA256 {
		return nil, 0, fmt.Errorf("%w: %s: manifest %s, downloaded %s", ErrChecksumMismatch, memberLabel(e), e.MemberSHA256, got)
	}
	return member, len(body), nil
}

func memberLabel(e Entry) string {
	if e.Member != "" {
		return e.Member
	}
	return e.ArchiveURL
}

// extractMember reads exactly one named member; nothing else in the archive is
// decompressed.
func extractMember(archive []byte, name string, maxBytes int64) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("open boundary archive: %w", err)
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		if f.UncompressedSize64 > uint64(maxBytes) {
			return nil, fmt.Errorf("%w: member %s is %d bytes uncompressed", ErrArchiveTooLarge, name, f.UncompressedSize64)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open boundary member %s: %w", name, err)
		}
		defer rc.Close()
		// The declared size is not trusted: the read is capped too.
		data, err := io.ReadAll(io.LimitReader(rc, maxBytes+1))
		if err != nil {
			return nil, fmt.Errorf("read boundary member %s: %w", name, err)
		}
		if int64(len(data)) > maxBytes {
			return nil, fmt.Errorf("%w: member %s", ErrArchiveTooLarge, name)
		}
		return data, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrMemberNotFound, name)
}
