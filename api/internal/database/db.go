package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"

	"vigilafrica/api/db"
	"vigilafrica/api/internal/models"
)

// Repository defines the data access methods for VigilAfrica.
type Repository interface {
	UpsertEvent(ctx context.Context, e models.Event, geoJSON string) error
	Close()
}

type pgRepo struct {
	pool *pgxpool.Pool
}

// NewRepository initializes a new PostgreSQL connection pool and runs migrations.
func NewRepository(ctx context.Context, databaseURL string) (Repository, error) {
	// 1. Run migrations before opening the main connection pool
	d, err := iofs.New(db.FS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to load migration files: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", d, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return nil, fmt.Errorf("database migration failed: %w", err)
	}

	// 2. Initialize the application connection pool
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database DSN: %w", err)
	}

	// Healthy connection pool settings
	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Double-check liveness (health check capability)
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return &pgRepo{pool: pool}, nil
}

// UpsertEvent implements the idempotent UPSERT strategy for EONET events.
func (r *pgRepo) UpsertEvent(ctx context.Context, e models.Event, geoJSON string) error {
	query := `
		INSERT INTO events (
			source_id, source, title, category, status, 
			geom, geom_type, latitude, longitude, 
			event_date, source_url, raw_payload
		)
		VALUES (
			$1, $2, $3, $4, $5, 
			ST_GeomFromGeoJSON($6), $7, $8, $9, 
			$10, $11, $12
		)
		ON CONFLICT (source_id)
		DO UPDATE SET
			status      = EXCLUDED.status,
			title       = EXCLUDED.title,
			raw_payload = EXCLUDED.raw_payload;`

	_, err := r.pool.Exec(ctx, query,
		e.SourceID,
		e.Source,
		e.Title,
		e.Category,
		e.Status,
		geoJSON, // Needed for PostGIS geometry construction
		e.GeomType,
		e.Latitude,
		e.Longitude,
		e.EventDate,
		e.SourceURL,
		e.RawPayload,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert event %s: %w", e.SourceID, err)
	}
	return nil
}

func (r *pgRepo) Close() {
	if r.pool != nil {
		r.pool.Close()
	}
}
