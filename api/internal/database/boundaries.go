package database

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"vigilafrica/api/internal/models"
)

// BoundaryRepository is the boundary loader's storage surface
// (feature-continental-coverage spec §2). It is a separate interface, not part
// of Repository, for the same reason as the scheduler lock: only *pgRepo
// implements it, and callers that hold a Repository type-assert for it.
type BoundaryRepository interface {
	// ListBoundarySources returns every boundary_sources row keyed by country code.
	ListBoundarySources(ctx context.Context) (map[string]models.BoundarySource, error)
	// RecordBoundaryFailure records a failed load attempt for the entry with
	// the given hash. It never touches the columns of the last successful load,
	// so a country already loaded keeps its provenance.
	RecordBoundaryFailure(ctx context.Context, countryCode, entrySHA256, lastError string, failedAt time.Time) error
	// BeginBoundaryLoad opens the one transaction a country's load runs in,
	// with statement_timeout set for the transaction only (SET LOCAL).
	BeginBoundaryLoad(ctx context.Context, statementTimeout time.Duration) (BoundaryLoadTx, error)
	// CountUnnamedEventsInBBox counts stored events intersecting bbox
	// ([min_lon, min_lat, max_lon, max_lat]) with no country_name.
	CountUnnamedEventsInBBox(ctx context.Context, bbox [4]float64) (int, error)
}

// BoundaryLoadTx is one country's load transaction. The loader calls the steps
// in order and commits only when all succeed; Rollback after Commit is a no-op.
// Splitting the steps lets a test fail one of them against a real database and
// prove the earlier writes roll back (task 3.6).
type BoundaryLoadTx interface {
	// ReplaceBoundaries deletes the country's rows at the levels being loaded
	// (ADM0 and ADM1 for an adm1 entry, ADM0 for an adm0 entry, including a
	// 000012 simplified outline) and inserts the new ones.
	ReplaceBoundaries(ctx context.Context, r models.BoundaryReplacement) (models.BoundaryReplaceResult, error)
	// ReenrichEvents fires the enrichment trigger (UPDATE events SET geom = geom)
	// on every event intersecting bbox or still named by one of countryNames.
	ReenrichEvents(ctx context.Context, bbox [4]float64, countryNames []string) (int64, error)
	// UpsertBoundarySource records the load and clears the failed_* columns.
	UpsertBoundarySource(ctx context.Context, rec models.BoundaryLoadRecord) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// ErrEmptyBoundaryGeometry means a unit's geometry held no polygon once made
// valid (for example a degenerate ring), so it cannot name anything.
var ErrEmptyBoundaryGeometry = errors.New("boundary geometry is empty after ST_MakeValid")

// ListBoundarySources — see BoundaryRepository.
func (r *pgRepo) ListBoundarySources(ctx context.Context) (map[string]models.BoundarySource, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT country_code, country_name, levels, source, archive_url, member,
		       member_sha256, entry_sha256, vintage, unit_count, loaded_at,
		       reenriched_at, failed_entry_sha256, failed_at, last_error
		FROM boundary_sources`)
	if err != nil {
		return nil, fmt.Errorf("failed to query boundary sources: %w", err)
	}
	defer rows.Close()

	out := make(map[string]models.BoundarySource)
	for rows.Next() {
		var s models.BoundarySource
		if err := rows.Scan(&s.CountryCode, &s.CountryName, &s.Levels, &s.Source, &s.ArchiveURL, &s.Member,
			&s.MemberSHA256, &s.EntrySHA256, &s.Vintage, &s.UnitCount, &s.LoadedAt,
			&s.ReenrichedAt, &s.FailedEntrySHA256, &s.FailedAt, &s.LastError); err != nil {
			return nil, fmt.Errorf("failed to scan boundary source row: %w", err)
		}
		out[s.CountryCode] = s
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error in ListBoundarySources: %w", err)
	}
	return out, nil
}

// RecordBoundaryFailure — see BoundaryRepository.
func (r *pgRepo) RecordBoundaryFailure(ctx context.Context, countryCode, entrySHA256, lastError string, failedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO boundary_sources (country_code, failed_entry_sha256, failed_at, last_error)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (country_code) DO UPDATE
		SET failed_entry_sha256 = EXCLUDED.failed_entry_sha256,
		    failed_at           = EXCLUDED.failed_at,
		    last_error          = EXCLUDED.last_error`,
		countryCode, entrySHA256, failedAt.UTC(), lastError)
	if err != nil {
		return fmt.Errorf("failed to record boundary failure for %s: %w", countryCode, err)
	}
	return nil
}

// CountUnnamedEventsInBBox — see BoundaryRepository.
func (r *pgRepo) CountUnnamedEventsInBBox(ctx context.Context, bbox [4]float64) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM events
		WHERE country_name IS NULL
		  AND geom && ST_MakeEnvelope($1, $2, $3, $4, 4326)`,
		bbox[0], bbox[1], bbox[2], bbox[3]).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("failed to count unnamed events in box: %w", err)
	}
	return n, nil
}

// BeginBoundaryLoad — see BoundaryRepository.
func (r *pgRepo) BeginBoundaryLoad(ctx context.Context, statementTimeout time.Duration) (BoundaryLoadTx, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to begin boundary load: %w", err)
	}
	// set_config(..., true) is SET LOCAL: it ends with the transaction, so the
	// pooled connection goes back with its normal timeout.
	ms := strconv.FormatInt(statementTimeout.Milliseconds(), 10)
	if _, err := tx.Exec(ctx, `SELECT set_config('statement_timeout', $1, true)`, ms); err != nil {
		_ = tx.Rollback(ctx) // the set_config failure is the error worth returning
		return nil, fmt.Errorf("failed to set boundary load statement_timeout: %w", err)
	}
	return &pgBoundaryTx{tx: tx}, nil
}

type pgBoundaryTx struct {
	tx pgx.Tx
}

// makeValidMulti turns a GeoJSON geometry parameter into what admin_boundaries
// stores: SRID 4326, 2-D, valid, polygons only, MultiPolygon.
// ST_CollectionExtract(..., 3) runs before ST_Multi so an invalid ring that
// ST_MakeValid turns into a GeometryCollection cannot yield a non-polygon.
const makeValidMulti = `ST_Multi(ST_CollectionExtract(ST_MakeValid(ST_Force2D(ST_SetSRID(ST_GeomFromGeoJSON(%s), 4326))), 3))`

func (t *pgBoundaryTx) ReplaceBoundaries(ctx context.Context, r models.BoundaryReplacement) (models.BoundaryReplaceResult, error) {
	var res models.BoundaryReplaceResult

	levels := []int{0}
	if r.Levels == models.BoundaryLevelsADM1 {
		levels = []int{0, 1}
	} else if r.Levels != models.BoundaryLevelsADM0 {
		return res, fmt.Errorf("unknown boundary levels %q", r.Levels)
	}
	if len(r.Units) == 0 {
		return res, errors.New("no boundary units to load")
	}

	rows, err := t.tx.Query(ctx, `
		DELETE FROM admin_boundaries
		WHERE country_code = $1 AND adm_level = ANY($2)
		RETURNING country_name`, r.CountryCode, levels)
	if err != nil {
		return res, fmt.Errorf("failed to delete old boundaries for %s: %w", r.CountryCode, err)
	}
	seen := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return res, fmt.Errorf("failed to scan deleted boundary row: %w", err)
		}
		if !seen[name] {
			seen[name] = true
			res.PreviousCountryNames = append(res.PreviousCountryNames, name)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, fmt.Errorf("rows error deleting boundaries for %s: %w", r.CountryCode, err)
	}

	geoms := make([]string, len(r.Units))
	for i, u := range r.Units {
		geoms[i] = u.GeometryJSON
	}

	if r.Levels == models.BoundaryLevelsADM1 {
		names := make([]string, len(r.Units))
		pcodes := make([]*string, len(r.Units))
		for i, u := range r.Units {
			names[i], pcodes[i] = u.Name, u.PCode
		}
		tag, err := t.tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO admin_boundaries (country_code, country_name, adm_level, adm_name, adm_pcode, geom)
			SELECT $1, $2, 1, u.name, u.pcode, %s
			FROM unnest($3::text[], $4::text[], $5::text[]) AS u(name, pcode, geojson)`, fmt.Sprintf(makeValidMulti, "u.geojson")),
			r.CountryCode, r.CountryName, names, pcodes, geoms)
		if err != nil {
			return res, fmt.Errorf("failed to insert ADM1 boundaries for %s: %w", r.CountryCode, err)
		}
		res.Units = int(tag.RowsAffected())

		if err := t.rejectEmpty(ctx, r.CountryCode, 1); err != nil {
			return res, err
		}
		// ADM0 is the union of the ADM1 rows, as in 000010, so the ADM0
		// fallback covers exactly the area the states cover.
		if _, err := t.tx.Exec(ctx, `
			INSERT INTO admin_boundaries (country_code, country_name, adm_level, adm_name, geom)
			SELECT $1, $2, 0, $2, ST_Multi(ST_CollectionExtract(ST_Union(geom), 3))
			FROM admin_boundaries
			WHERE country_code = $1 AND adm_level = 1`, r.CountryCode, r.CountryName); err != nil {
			return res, fmt.Errorf("failed to insert ADM0 union for %s: %w", r.CountryCode, err)
		}
	} else {
		// An adm0 entry is the outline itself; a multi-feature file is unioned.
		if _, err := t.tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO admin_boundaries (country_code, country_name, adm_level, adm_name, geom)
			SELECT $1, $2, 0, $2, ST_Multi(ST_CollectionExtract(ST_Union(%s), 3))
			FROM unnest($3::text[]) AS u(geojson)`, fmt.Sprintf(makeValidMulti, "u.geojson")),
			r.CountryCode, r.CountryName, geoms); err != nil {
			return res, fmt.Errorf("failed to insert ADM0 outline for %s: %w", r.CountryCode, err)
		}
		res.Units = 1
	}
	if err := t.rejectEmpty(ctx, r.CountryCode, 0); err != nil {
		return res, err
	}
	return res, nil
}

func (t *pgBoundaryTx) rejectEmpty(ctx context.Context, countryCode string, level int) error {
	var empty int
	if err := t.tx.QueryRow(ctx, `
		SELECT count(*) FROM admin_boundaries
		WHERE country_code = $1 AND adm_level = $2 AND ST_IsEmpty(geom)`, countryCode, level).Scan(&empty); err != nil {
		return fmt.Errorf("failed to check boundary geometries for %s: %w", countryCode, err)
	}
	if empty > 0 {
		return fmt.Errorf("%s: %d ADM%d unit(s): %w", countryCode, empty, level, ErrEmptyBoundaryGeometry)
	}
	return nil
}

func (t *pgBoundaryTx) ReenrichEvents(ctx context.Context, bbox [4]float64, countryNames []string) (int64, error) {
	if countryNames == nil {
		countryNames = []string{}
	}
	// Touching geom fires enrich_event_location_trigger (000013), the method
	// 000010 and 000012 used. `&&` is bbox intersection, served by idx on geom.
	tag, err := t.tx.Exec(ctx, `
		UPDATE events SET geom = geom
		WHERE geom IS NOT NULL
		  AND (geom && ST_MakeEnvelope($1, $2, $3, $4, 4326)
		       OR country_name = ANY($5::text[]))`,
		bbox[0], bbox[1], bbox[2], bbox[3], countryNames)
	if err != nil {
		return 0, fmt.Errorf("failed to re-enrich events: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (t *pgBoundaryTx) UpsertBoundarySource(ctx context.Context, rec models.BoundaryLoadRecord) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO boundary_sources (
			country_code, country_name, levels, source, archive_url, member,
			member_sha256, entry_sha256, vintage, unit_count, loaded_at, reenriched_at,
			failed_entry_sha256, failed_at, last_error)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now(), clock_timestamp(), NULL, NULL, NULL)
		ON CONFLICT (country_code) DO UPDATE SET
			country_name        = EXCLUDED.country_name,
			levels              = EXCLUDED.levels,
			source              = EXCLUDED.source,
			archive_url         = EXCLUDED.archive_url,
			member              = EXCLUDED.member,
			member_sha256       = EXCLUDED.member_sha256,
			entry_sha256        = EXCLUDED.entry_sha256,
			vintage             = EXCLUDED.vintage,
			unit_count          = EXCLUDED.unit_count,
			loaded_at           = EXCLUDED.loaded_at,
			reenriched_at       = EXCLUDED.reenriched_at,
			failed_entry_sha256 = NULL,
			failed_at           = NULL,
			last_error          = NULL`,
		rec.CountryCode, rec.CountryName, rec.Levels, rec.Source, rec.ArchiveURL, rec.Member,
		rec.MemberSHA256, rec.EntrySHA256, rec.Vintage, rec.UnitCount)
	if err != nil {
		return fmt.Errorf("failed to record boundary source for %s: %w", rec.CountryCode, err)
	}
	return nil
}

func (t *pgBoundaryTx) Commit(ctx context.Context) error {
	if err := t.tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit boundary load: %w", err)
	}
	return nil
}

func (t *pgBoundaryTx) Rollback(ctx context.Context) error {
	if err := t.tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return fmt.Errorf("failed to roll back boundary load: %w", err)
	}
	return nil
}
