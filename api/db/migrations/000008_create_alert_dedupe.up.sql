CREATE TABLE IF NOT EXISTS alert_dedupe (
    id BIGSERIAL PRIMARY KEY,
    alert_kind TEXT NOT NULL,
    reference_time TIMESTAMPTZ NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (alert_kind, reference_time)
);
