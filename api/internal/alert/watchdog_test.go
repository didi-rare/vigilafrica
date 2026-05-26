package alert

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"vigilafrica/api/internal/models"
)

func TestStalenessReferenceTimeUsesLastSuccessfulCompletion(t *testing.T) {
	completedAt := time.Date(2026, 4, 19, 8, 0, 0, 0, time.UTC)
	lastSuccessRun := &models.IngestionRun{
		Status:      models.RunStatusSuccess,
		StartedAt:   completedAt.Add(-15 * time.Minute),
		CompletedAt: &completedAt,
	}
	firstRun := &models.IngestionRun{
		Status:    models.RunStatusFailure,
		StartedAt: time.Date(2026, 4, 18, 8, 0, 0, 0, time.UTC),
	}

	referenceTime, ok := stalenessReferenceTime(lastSuccessRun, firstRun)
	if !ok {
		t.Fatal("expected a reference time")
	}
	if !referenceTime.Equal(completedAt) {
		t.Fatalf("expected completed_at %s, got %s", completedAt, referenceTime)
	}
}

func TestStalenessReferenceTimeFallsBackToFirstRunWhenNoSuccessExists(t *testing.T) {
	firstStartedAt := time.Date(2026, 4, 19, 6, 0, 0, 0, time.UTC)
	firstRun := &models.IngestionRun{
		Status:    models.RunStatusFailure,
		StartedAt: firstStartedAt,
	}

	referenceTime, ok := stalenessReferenceTime(nil, firstRun)
	if !ok {
		t.Fatal("expected a reference time")
	}
	if !referenceTime.Equal(firstStartedAt) {
		t.Fatalf("expected first run started_at %s, got %s", firstStartedAt, referenceTime)
	}
}

func TestStalenessReferenceTimeReturnsFalseWhenNoRunsExist(t *testing.T) {
	if _, ok := stalenessReferenceTime(nil, nil); ok {
		t.Fatal("expected no reference time when there are no runs")
	}
}

func TestShouldSendStalenessAlertOnlyAlertsOncePerReferenceTime(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	threshold := 2 * time.Hour
	referenceTime := now.Add(-3 * time.Hour)

	if !shouldSendStalenessAlert(now, referenceTime, threshold, time.Time{}) {
		t.Fatal("expected stale reference time to trigger an alert")
	}

	if shouldSendStalenessAlert(now, referenceTime, threshold, referenceTime) {
		t.Fatal("expected duplicate alert for the same stale reference time to be suppressed")
	}
}

func TestShouldSendStalenessAlertAllowsFreshReferenceTimesToResetSuppression(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	threshold := 2 * time.Hour
	lastAlertReference := now.Add(-4 * time.Hour)
	freshReferenceTime := now.Add(-30 * time.Minute)

	if shouldSendStalenessAlert(now, freshReferenceTime, threshold, lastAlertReference) {
		t.Fatal("expected fresh reference time not to trigger an alert")
	}
}

func TestShouldSendStalenessAlertAllowsNewStaleReferenceTimeAfterPreviousAlert(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	threshold := 2 * time.Hour
	lastAlertReference := now.Add(-4 * time.Hour)
	newReferenceTime := now.Add(-3 * time.Hour)

	if !shouldSendStalenessAlert(now, newReferenceTime, threshold, lastAlertReference) {
		t.Fatal("expected a new stale reference time to trigger a fresh alert")
	}
}

// ─── B1: dedupe-vs-send order tests (chore-post-v11-quality-sweep) ───────────

// fakeStalenessRecorder simulates the alert_dedupe table in memory so we can
// drive every branch of the watchdog's claim-send-release flow without a
// running Postgres.
type fakeStalenessRecorder struct {
	mu              sync.Mutex
	rows            map[time.Time]bool // reference_time → claimed
	tryRecordCalls  []time.Time
	releaseCalls    []time.Time
	tryRecordErr    error
	releaseErr      error
	preExistingRows map[time.Time]bool // simulates rows from "another replica"
}

func newFakeRecorder() *fakeStalenessRecorder {
	return &fakeStalenessRecorder{
		rows:            map[time.Time]bool{},
		preExistingRows: map[time.Time]bool{},
	}
}

func (f *fakeStalenessRecorder) TryRecordStalenessAlert(_ context.Context, referenceTime time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := referenceTime.UTC()
	f.tryRecordCalls = append(f.tryRecordCalls, key)
	if f.tryRecordErr != nil {
		return false, f.tryRecordErr
	}
	if f.preExistingRows[key] || f.rows[key] {
		return false, nil
	}
	f.rows[key] = true
	return true, nil
}

func (f *fakeStalenessRecorder) ReleaseStalenessAlertClaim(_ context.Context, referenceTime time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := referenceTime.UTC()
	f.releaseCalls = append(f.releaseCalls, key)
	if f.releaseErr != nil {
		return f.releaseErr
	}
	delete(f.rows, key)
	return nil
}

// processOneTick runs the same per-iteration body that StartStalenessWatchdog
// runs inside its goroutine. Factored out so tests can drive it deterministically
// without spinning a real ticker.
func processOneTick(
	ctx context.Context,
	now time.Time,
	referenceTime time.Time,
	threshold time.Duration,
	lastAlertReference time.Time,
	recorder *fakeStalenessRecorder,
	send func() error,
) time.Time {
	if !shouldSendStalenessAlert(now, referenceTime, threshold, lastAlertReference) {
		if now.Sub(referenceTime) <= threshold {
			return time.Time{}
		}
		return lastAlertReference
	}
	if recorder != nil {
		recorded, err := recorder.TryRecordStalenessAlert(ctx, referenceTime)
		if err != nil {
			return lastAlertReference
		}
		if !recorded {
			return referenceTime
		}
	}
	if err := send(); err != nil {
		if recorder != nil {
			_ = recorder.ReleaseStalenessAlertClaim(ctx, referenceTime)
		}
		return lastAlertReference
	}
	return referenceTime
}

// B1.4 (a): send succeeds → claim persists → next tick suppressed.
func TestWatchdog_SendSucceeds_NextTickSuppressed(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	referenceTime := now.Add(-3 * time.Hour)
	threshold := 2 * time.Hour
	recorder := newFakeRecorder()

	// Tick 1: send succeeds.
	last := processOneTick(ctx, now, referenceTime, threshold, time.Time{}, recorder, func() error { return nil })
	if !last.Equal(referenceTime) {
		t.Fatalf("tick 1: lastAlertReference = %v, want %v", last, referenceTime)
	}
	if len(recorder.releaseCalls) != 0 {
		t.Errorf("tick 1: expected no Release calls, got %d", len(recorder.releaseCalls))
	}

	// Tick 2: same reference time, in-memory dedupe suppresses without hitting the recorder.
	sent := false
	last2 := processOneTick(ctx, now.Add(15*time.Minute), referenceTime, threshold, last, recorder, func() error { sent = true; return nil })
	if sent {
		t.Error("tick 2: expected send NOT to fire when lastAlertReference matches")
	}
	if !last2.Equal(referenceTime) {
		t.Errorf("tick 2: lastAlertReference changed unexpectedly: %v", last2)
	}
}

// B1.4 (b): send fails → claim released → next tick retries.
func TestWatchdog_SendFails_NextTickRetries(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	referenceTime := now.Add(-3 * time.Hour)
	threshold := 2 * time.Hour
	recorder := newFakeRecorder()

	// Tick 1: send fails. Claim should be released; lastAlertReference must NOT advance.
	last := processOneTick(ctx, now, referenceTime, threshold, time.Time{}, recorder, func() error { return errors.New("resend timeout") })
	if !last.Equal(time.Time{}) {
		t.Fatalf("tick 1: expected lastAlertReference unchanged on send failure, got %v", last)
	}
	if len(recorder.releaseCalls) != 1 {
		t.Fatalf("tick 1: expected exactly one Release call, got %d", len(recorder.releaseCalls))
	}
	if _, stillClaimed := recorder.rows[referenceTime.UTC()]; stillClaimed {
		t.Error("tick 1: claim row should have been released")
	}

	// Tick 2 (next watchdog interval): same reference time, send succeeds this time.
	sent := false
	last2 := processOneTick(ctx, now.Add(15*time.Minute), referenceTime, threshold, last, recorder, func() error { sent = true; return nil })
	if !sent {
		t.Error("tick 2: expected send to be retried after the released claim")
	}
	if !last2.Equal(referenceTime) {
		t.Errorf("tick 2: lastAlertReference = %v, want %v", last2, referenceTime)
	}
	if len(recorder.tryRecordCalls) != 2 {
		t.Errorf("expected 2 TryRecord calls across both ticks, got %d", len(recorder.tryRecordCalls))
	}
}

// B1.4 (c): another replica already sent (row exists pre-tick) → this replica suppresses.
func TestWatchdog_AnotherReplicaSent_ThisReplicaSuppresses(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	referenceTime := now.Add(-3 * time.Hour)
	threshold := 2 * time.Hour
	recorder := newFakeRecorder()
	// Simulate the other replica having already inserted the dedupe row.
	recorder.preExistingRows[referenceTime.UTC()] = true

	sent := false
	last := processOneTick(ctx, now, referenceTime, threshold, time.Time{}, recorder, func() error { sent = true; return nil })
	if sent {
		t.Error("expected send NOT to fire when another replica already claimed")
	}
	if !last.Equal(referenceTime) {
		t.Errorf("lastAlertReference = %v, want %v (so future ticks stay suppressed)", last, referenceTime)
	}
	if len(recorder.releaseCalls) != 0 {
		t.Error("must NOT release a claim we did not make")
	}
}

