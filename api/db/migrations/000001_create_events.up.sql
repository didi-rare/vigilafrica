CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE events (
    id            UUID             PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id     TEXT             NOT NULL UNIQUE,
    source        TEXT             NOT NULL DEFAULT 'eonet',
    title         TEXT             NOT NULL,
    category      TEXT             NOT NULL CHECK (category IN ('floods', 'wildfires')),
    status        TEXT             NOT NULL DEFAULT 'open'
                                   CHECK (status IN ('open', 'closed')),
    geom          geometry(Geometry, 4326),
    geom_type     TEXT,
    latitude      DOUBLE PRECISION,
    longitude     DOUBLE PRECISION,
    country_name  TEXT,
    state_name    TEXT,
    event_date    TIMESTAMPTZ,
    source_url    TEXT,
    raw_payload   JSONB,
    ingested_at   TIMESTAMPTZ      NOT NULL DEFAULT now(),
    enriched_at   TIMESTAMPTZ
);

-- Performance indexes
CREATE INDEX idx_events_category    ON events(category);
CREATE INDEX idx_events_status      ON events(status);
CREATE INDEX idx_events_state_name  ON events(state_name);
CREATE INDEX idx_events_event_date  ON events(event_date DESC);
CREATE INDEX idx_events_geom        ON events USING GIST(geom);
