package alert

import (
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
