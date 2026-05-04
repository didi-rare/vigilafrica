CREATE TABLE IF NOT EXISTS scheduler_locks (
    lock_name TEXT PRIMARY KEY,
    holder TEXT NOT NULL,
    locked_until TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_scheduler_locks_locked_until
    ON scheduler_locks (locked_until);
