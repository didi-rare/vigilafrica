//go:build integration

package database_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"vigilafrica/api/internal/models"
)

const (
	migration000016Up   = "../../db/migrations/000016_add_degraded_ingestion_status.up.sql"
	migration000016Down = "../../db/migrations/000016_add_degraded_ingestion_status.down.sql"
)

// TestDegradedRunIsStoredAndCountsForStaleness covers fix-gdacs-degraded-run-status
// against real Postgres, through the repository the application actually uses.
func TestDegradedRunIsStoredAndCountsForStaleness(t *testing.T) {
	ctx := context.Background()

	id, err := testRepo.CreateIngestionRun(ctx, time.Now(), "NG")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	msg := "degraded: GDACS could not be reached for 1 polygon event(s)"
	if err := testRepo.CompleteIngestionRun(ctx, id, models.RunStatusDegraded, 10, 9, &msg); err != nil {
		t.Fatalf("a degraded run must be storable (migration 000016): %v", err)
	}

	// ⚠️ The staleness watchdog reads this. If a degraded run were not counted,
	// a GDACS outage would ALSO fire a false "ingestion has stopped" alert.
	last, err := testRepo.GetLastSuccessfulIngestionRun(ctx)
	if err != nil {
		t.Fatalf("last successful run: %v", err)
	}
	if last == nil || last.ID != id {
		t.Fatalf("staleness must treat the degraded run %d as the latest completed run, got %+v", id, last)
	}
	if last.Status != models.RunStatusDegraded {
		t.Errorf("status = %q, want degraded", last.Status)
	}
}

// TestIngestionStatusConstraintStillEnforced proves 000016 REPLACED the CHECK
// rather than merely dropping it: an unknown status must still be rejected.
func TestIngestionStatusConstraintStillEnforced(t *testing.T) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, testDSN)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, `INSERT INTO ingestion_runs (started_at, status, country_code) VALUES (now(), 'bogus', 'NG')`)
	if err == nil {
		t.Fatal("status 'bogus' was accepted: the CHECK constraint is gone, not replaced")
	}
}

// TestMigration000016RoundTrip replays the real migration files, down then up,
// inside a transaction that is rolled back so the shared database is untouched.
func TestMigration000016RoundTrip(t *testing.T) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, testDSN)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	down, err := os.ReadFile(migration000016Down)
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	up, err := os.ReadFile(migration000016Up)
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO ingestion_runs (started_at, status, country_code) VALUES (now(), 'degraded', 'GH') RETURNING id`,
	).Scan(&id); err != nil {
		t.Fatalf("seed degraded row: %v", err)
	}

	if _, err := tx.Exec(ctx, string(down)); err != nil {
		t.Fatalf("down migration failed: %v", err)
	}
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM ingestion_runs WHERE id = $1`, id).Scan(&status); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if status != "success" {
		t.Errorf("down migration: degraded row became %q, want success (it DID complete)", status)
	}
	if _, err := tx.Exec(ctx, `SAVEPOINT probe`); err != nil {
		t.Fatalf("savepoint: %v", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE ingestion_runs SET status = 'degraded' WHERE id = $1`, id); err == nil {
		t.Error("after down migration the old constraint must reject 'degraded'")
	}
	if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT probe`); err != nil {
		t.Fatalf("rollback to savepoint: %v", err)
	}

	if _, err := tx.Exec(ctx, string(up)); err != nil {
		t.Fatalf("up migration failed when replayed: %v", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE ingestion_runs SET status = 'degraded' WHERE id = $1`, id); err != nil {
		t.Errorf("after up migration 'degraded' must be accepted: %v", err)
	}
}

// TestPreviousCompletedRunSkipsRunningAndOtherCountries proves the SQL behind
// degraded-alert dedupe (independent review, PR #280): an orphaned 'running'
// row must not mask the streak, and another country's run must not count.
func TestPreviousCompletedRunSkipsRunningAndOtherCountries(t *testing.T) {
	ctx := context.Background()
	const cc = "ZZ" // a country code no other test uses

	degraded, err := testRepo.CreateIngestionRun(ctx, time.Now(), cc)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	msg := "degraded"
	if err := testRepo.CompleteIngestionRun(ctx, degraded, models.RunStatusDegraded, 1, 0, &msg); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if err := testRepo.MarkIngestionRunAlerted(ctx, degraded); err != nil {
		t.Fatalf("mark alerted: %v", err)
	}
	if _, err := testRepo.CreateIngestionRun(ctx, time.Now(), cc); err != nil { // orphan: never completed
		t.Fatalf("create orphan: %v", err)
	}
	other, err := testRepo.CreateIngestionRun(ctx, time.Now(), "YY")
	if err != nil {
		t.Fatalf("create other country: %v", err)
	}
	if err := testRepo.CompleteIngestionRun(ctx, other, models.RunStatusSuccess, 1, 1, nil); err != nil {
		t.Fatalf("complete other: %v", err)
	}
	current, err := testRepo.CreateIngestionRun(ctx, time.Now(), cc)
	if err != nil {
		t.Fatalf("create current: %v", err)
	}

	prev, err := testRepo.GetPreviousCompletedIngestionRun(ctx, cc, current)
	if err != nil {
		t.Fatalf("previous: %v", err)
	}
	if prev == nil || prev.ID != degraded {
		t.Fatalf("previous completed run = %+v, want id %d (skipping the orphan and country YY)", prev, degraded)
	}
	if prev.Status != models.RunStatusDegraded || prev.AlertSentAt == nil {
		t.Errorf("previous run status=%q alerted=%v, want degraded and alerted", prev.Status, prev.AlertSentAt)
	}

	none, err := testRepo.GetPreviousCompletedIngestionRun(ctx, "XX", current)
	if err != nil || none != nil {
		t.Errorf("unknown country: got %+v, %v — want nil, nil", none, err)
	}
}

// TestMigration000016IsReplayable: developers-go.md §11.4. Re-running the up
// migration on an already-migrated schema must not wedge (independent review,
// PR #280, round 2). Inside a rolled-back transaction.
func TestMigration000016IsReplayable(t *testing.T) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, testDSN)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)
	up, err := os.ReadFile(migration000016Up)
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, string(up)); err != nil {
		t.Fatalf("replaying 000016 on an already-migrated schema failed: %v", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO ingestion_runs (started_at, status, country_code) VALUES (now(), 'degraded', 'NG')`); err != nil {
		t.Errorf("after replay, 'degraded' must still be accepted: %v", err)
	}
}
