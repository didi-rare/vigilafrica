package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"vigilafrica/api/internal/models"
)

type EventFilters struct {
	Category string
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

	if filters.State != "" {
		conditions = append(conditions, fmt.Sprintf("state_name ILIKE $%d", argID))
		args = append(args, filters.State) // Exact match but case insensitive
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
