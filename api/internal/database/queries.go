package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"vigilafrica/api/internal/models"
)

type EventFilters struct {
	Category string
	Country  string
	State    string
	Status   string
	Limit    int
	Offset   int
}

// ListEvents retrieves a paginated and filtered list of events.
// It also returns the total count of matched records for pagination.
func (r *pgRepo) ListEvents(ctx context.Context, filters EventFilters) ([]models.Event, int, error) {
	var conditions []string
	var args []interface{}
	argID := 1

	if filters.Category != "" {
		conditions = append(conditions, fmt.Sprintf("category = $%d", argID))
		args = append(args, filters.Category)
		argID++
	}

	if filters.Country != "" {
		conditions = append(conditions, fmt.Sprintf("country_name ILIKE $%d", argID))
		args = append(args, filters.Country)
		argID++
	}

	if filters.State != "" {
		conditions = append(conditions, fmt.Sprintf("state_name ILIKE $%d", argID))
		args = append(args, filters.State)
		argID++
	}

	if filters.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argID))
		args = append(args, filters.Status)
		argID++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// First query: get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM events %s", whereClause)
	var total int
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get events count: %w", err)
	}

	if total == 0 {
		return []models.Event{}, 0, nil
	}

	// Ensure sensible pagination defaults if not parsed correctly
	limit := filters.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	// Add pagination parameters
	args = append(args, limit, offset)

	// Second query: fetch actual data
	query := fmt.Sprintf(`
		SELECT 
			id, source_id, source, title, category, status,
			geom_type, latitude, longitude, country_name, state_name,
			event_date, source_url, ingested_at, enriched_at
		FROM events 
		%s
		ORDER BY event_date DESC NULLS LAST, ingested_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argID, argID+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var e models.Event
		err := rows.Scan(
			&e.ID, &e.SourceID, &e.Source, &e.Title, &e.Category, &e.Status,
			&e.GeomType, &e.Latitude, &e.Longitude, &e.CountryName, &e.StateName,
			&e.EventDate, &e.SourceURL, &e.IngestedAt, &e.EnrichedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan event row: %w", err)
		}
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}

	// Always return an allocated slice, not nil per api-contract
	if events == nil {
		events = make([]models.Event, 0)
	}

	return events, total, nil
}

// GetEventByID fetches a single event by UUID.
func (r *pgRepo) GetEventByID(ctx context.Context, id uuid.UUID) (*models.Event, error) {
	query := `
		SELECT 
			id, source_id, source, title, category, status,
			geom_type, latitude, longitude, country_name, state_name,
			event_date, source_url, ingested_at, enriched_at
		FROM events 
		WHERE id = $1
	`

	var e models.Event
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&e.ID, &e.SourceID, &e.Source, &e.Title, &e.Category, &e.Status,
		&e.GeomType, &e.Latitude, &e.Longitude, &e.CountryName, &e.StateName,
		&e.EventDate, &e.SourceURL, &e.IngestedAt, &e.EnrichedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get event by id: %w", err)
	}

	return &e, nil
}

// GetNearbyEvents fetches events within a given radius (in kilometers) from a central coordinate.
// It uses PostGIS ST_DWithin and Geography casting to accurately calculate distance over the sphere.
func (r *pgRepo) GetNearbyEvents(ctx context.Context, lat, lng float64, radiusKm float64, limit int) ([]models.Event, error) {
	radiusMeters := radiusKm * 1000

	query := `
		SELECT 
			id, source_id, source, title, category, status,
			geom_type, latitude, longitude, country_name, state_name,
			event_date, source_url, ingested_at, enriched_at
		FROM events 
		WHERE geom IS NOT NULL
		  AND ST_DWithin(
				geom::geography, 
				ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, 
				$3
			)
		ORDER BY ST_Distance(
				geom::geography, 
				ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
			) ASC, event_date DESC
		LIMIT $4
	`

	rows, err := r.pool.Query(ctx, query, lng, lat, radiusMeters, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search nearby events: %w", err)
	}
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var e models.Event
		err := rows.Scan(
			&e.ID, &e.SourceID, &e.Source, &e.Title, &e.Category, &e.Status,
			&e.GeomType, &e.Latitude, &e.Longitude, &e.CountryName, &e.StateName,
			&e.EventDate, &e.SourceURL, &e.IngestedAt, &e.EnrichedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan nearby event row: %w", err)
		}
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error in nearby events: %w", err)
	}

	if events == nil {
		events = make([]models.Event, 0)
	}

	return events, nil
}

// ─── Ingestion Run Methods (ADR-011) ─────────────────────────────────────────

// CreateIngestionRun inserts a new run record with status=running and returns its ID.
// Called at the start of every ingestion cycle.
func (r *pgRepo) CreateIngestionRun(ctx context.Context, startedAt time.Time, countryCode string) (int64, error) {
	query := `
		INSERT INTO ingestion_runs (started_at, status, country_code)
		VALUES ($1, 'running', $2)
		RETURNING id
	`
	var id int64
	err := r.pool.QueryRow(ctx, query, startedAt, countryCode).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to create ingestion run: %w", err)
	}
	return id, nil
}

// CompleteIngestionRun updates an existing run record with final status, counts, and error.
// Called at the end of every ingestion cycle (success or failure).
func (r *pgRepo) CompleteIngestionRun(ctx context.Context, id int64, status models.IngestionRunStatus, fetched, stored int, errMsg *string) error {
	query := `
		UPDATE ingestion_runs
		SET
			completed_at   = NOW(),
			status         = $2,
			events_fetched = $3,
			events_stored  = $4,
			error          = $5
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, id, status, fetched, stored, errMsg)
	if err != nil {
		return fmt.Errorf("failed to complete ingestion run %d: %w", id, err)
	}
	return nil
}

// GetLastIngestionRun returns the most recent ingestion run record across all countries.
// Used by: staleness watchdog, failure alerter.
func (r *pgRepo) GetLastIngestionRun(ctx context.Context) (*models.IngestionRun, error) {
	query := `
		SELECT id, country_code, started_at, completed_at, status, events_fetched, events_stored, error, created_at
		FROM ingestion_runs
		ORDER BY started_at DESC
		LIMIT 1
	`
	var run models.IngestionRun
	err := r.pool.QueryRow(ctx, query).Scan(
		&run.ID,
		&run.CountryCode,
		&run.StartedAt,
		&run.CompletedAt,
		&run.Status,
		&run.EventsFetched,
		&run.EventsStored,
		&run.Error,
		&run.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get last ingestion run: %w", err)
	}
	return &run, nil
}

// GetLastSuccessfulIngestionRun returns the most recent successful ingestion run.
// Used by: staleness watchdog.
func (r *pgRepo) GetLastSuccessfulIngestionRun(ctx context.Context) (*models.IngestionRun, error) {
	query := `
		SELECT id, country_code, started_at, completed_at, status, events_fetched, events_stored, error, created_at
		FROM ingestion_runs
		WHERE status = 'success'
		ORDER BY completed_at DESC NULLS LAST, started_at DESC
		LIMIT 1
	`
	var run models.IngestionRun
	err := r.pool.QueryRow(ctx, query).Scan(
		&run.ID,
		&run.CountryCode,
		&run.StartedAt,
		&run.CompletedAt,
		&run.Status,
		&run.EventsFetched,
		&run.EventsStored,
		&run.Error,
		&run.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get last successful ingestion run: %w", err)
	}
	return &run, nil
}

// GetFirstIngestionRun returns the oldest recorded ingestion run.
// Used by: staleness watchdog when the system has not had a successful run yet.
func (r *pgRepo) GetFirstIngestionRun(ctx context.Context) (*models.IngestionRun, error) {
	query := `
		SELECT id, country_code, started_at, completed_at, status, events_fetched, events_stored, error, created_at
		FROM ingestion_runs
		ORDER BY started_at ASC
		LIMIT 1
	`
	var run models.IngestionRun
	err := r.pool.QueryRow(ctx, query).Scan(
		&run.ID,
		&run.CountryCode,
		&run.StartedAt,
		&run.CompletedAt,
		&run.Status,
		&run.EventsFetched,
		&run.EventsStored,
		&run.Error,
		&run.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get first ingestion run: %w", err)
	}
	return &run, nil
}

// GetLastIngestionRunAllCountries returns the most recent run per country.
// Used by: /health last_ingestion_by_country map.
func (r *pgRepo) GetLastIngestionRunAllCountries(ctx context.Context) (map[string]*models.IngestionRun, error) {
	query := `
		SELECT DISTINCT ON (country_code)
			id, country_code, started_at, completed_at, status, events_fetched, events_stored, error, created_at
		FROM ingestion_runs
		ORDER BY country_code, started_at DESC
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query runs by country: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*models.IngestionRun)
	for rows.Next() {
		var run models.IngestionRun
		if err := rows.Scan(
			&run.ID,
			&run.CountryCode,
			&run.StartedAt,
			&run.CompletedAt,
			&run.Status,
			&run.EventsFetched,
			&run.EventsStored,
			&run.Error,
			&run.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ingestion run row: %w", err)
		}
		cp := run
		result[cp.CountryCode] = &cp
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error in GetLastIngestionRunAllCountries: %w", err)
	}
	return result, nil
}

// EnrichmentStat holds per-country enrichment quality metrics.
type EnrichmentStat struct {
	CountryName    *string `json:"country_name"`
	TotalEvents    int     `json:"total_events"`
	EnrichedEvents int     `json:"enriched_events"`
	SuccessRatePct float64 `json:"success_rate_pct"`
}

// GetEnrichmentStats returns per-country enrichment success rates.
// Used by: GET /v1/enrichment-stats
func (r *pgRepo) GetEnrichmentStats(ctx context.Context) ([]EnrichmentStat, error) {
	query := `
		SELECT
			country_name,
			COUNT(*)                                                   AS total_events,
			COUNT(*) FILTER (WHERE state_name IS NOT NULL)             AS enriched_events,
			ROUND(
				COUNT(*) FILTER (WHERE state_name IS NOT NULL)::numeric /
				NULLIF(COUNT(*), 0) * 100, 1
			)                                                          AS success_rate_pct
		FROM events
		GROUP BY country_name
		ORDER BY country_name NULLS LAST
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query enrichment stats: %w", err)
	}
	defer rows.Close()

	var stats []EnrichmentStat
	for rows.Next() {
		var s EnrichmentStat
		if err := rows.Scan(&s.CountryName, &s.TotalEvents, &s.EnrichedEvents, &s.SuccessRatePct); err != nil {
			return nil, fmt.Errorf("failed to scan enrichment stat row: %w", err)
		}
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error in GetEnrichmentStats: %w", err)
	}
	if stats == nil {
		stats = make([]EnrichmentStat, 0)
	}
	return stats, nil
}

// GetDistinctStatesByCountry returns distinct state names for a given country.
// Used by: GET /v1/states
func (r *pgRepo) GetDistinctStatesByCountry(ctx context.Context, country string) ([]string, error) {
	var query string
	var args []interface{}

	if country != "" {
		query = `
			SELECT DISTINCT state_name
			FROM events
			WHERE country_name ILIKE $1
			  AND state_name IS NOT NULL
			ORDER BY state_name
		`
		args = append(args, country)
	} else {
		query = `
			SELECT DISTINCT state_name
			FROM events
			WHERE state_name IS NOT NULL
			ORDER BY state_name
		`
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query distinct states: %w", err)
	}
	defer rows.Close()

	var states []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("failed to scan state row: %w", err)
		}
		states = append(states, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error in GetDistinctStatesByCountry: %w", err)
	}
	if states == nil {
		states = make([]string, 0)
	}
	return states, nil
}
