package database

import (
	"context"
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
func (r *pgRepo) CreateIngestionRun(ctx context.Context, startedAt time.Time) (int64, error) {
	query := `
		INSERT INTO ingestion_runs (started_at, status)
		VALUES ($1, 'running')
		RETURNING id
	`
	var id int64
	err := r.pool.QueryRow(ctx, query, startedAt).Scan(&id)
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

// GetLastIngestionRun returns the most recent ingestion run record, or nil if none exist.
// Used by: /health endpoint, staleness watchdog.
func (r *pgRepo) GetLastIngestionRun(ctx context.Context) (*models.IngestionRun, error) {
	query := `
		SELECT id, started_at, completed_at, status, events_fetched, events_stored, error, created_at
		FROM ingestion_runs
		ORDER BY started_at DESC
		LIMIT 1
	`
	var run models.IngestionRun
	err := r.pool.QueryRow(ctx, query).Scan(
		&run.ID,
		&run.StartedAt,
		&run.CompletedAt,
		&run.Status,
		&run.EventsFetched,
		&run.EventsStored,
		&run.Error,
		&run.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get last ingestion run: %w", err)
	}
	return &run, nil
}

