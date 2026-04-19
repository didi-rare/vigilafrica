package ingestor

import (
	"testing"
	"time"

	"vigilafrica/api/internal/models"
)

func TestStalenessReferenceTime_UsesLastSuccessfulCompletion(t *testing.T) {
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
		t.Fatal("Expected a reference time")
	}
	if !referenceTime.Equal(completedAt) {
		t.Fatalf("Expected completed_at %s, got %s", completedAt, referenceTime)
	}
}

func TestStalenessReferenceTime_FallsBackToFirstRunWhenNoSuccessExists(t *testing.T) {
	firstStartedAt := time.Date(2026, 4, 19, 6, 0, 0, 0, time.UTC)
	firstRun := &models.IngestionRun{
		Status:    models.RunStatusFailure,
		StartedAt: firstStartedAt,
	}

	referenceTime, ok := stalenessReferenceTime(nil, firstRun)
	if !ok {
		t.Fatal("Expected a reference time")
	}
	if !referenceTime.Equal(firstStartedAt) {
		t.Fatalf("Expected first run started_at %s, got %s", firstStartedAt, referenceTime)
	}
}

func TestStalenessReferenceTime_ReturnsFalseWhenNoRunsExist(t *testing.T) {
	if _, ok := stalenessReferenceTime(nil, nil); ok {
		t.Fatal("Expected no reference time when there are no runs")
	}
}

func TestShouldSendStalenessAlert_OnlyAlertsOncePerReferenceTime(t *testing.T) {
	threshold := 2 * time.Hour
	referenceTime := time.Now().Add(-3 * time.Hour)

	if !shouldSendStalenessAlert(referenceTime, threshold, time.Time{}) {
		t.Fatal("Expected stale reference time to trigger an alert")
	}

	if shouldSendStalenessAlert(referenceTime, threshold, referenceTime) {
		t.Fatal("Expected duplicate alert for the same stale reference time to be suppressed")
	}
}

func TestShouldSendStalenessAlert_AllowsFreshReferenceTimesToResetSuppression(t *testing.T) {
	threshold := 2 * time.Hour
	lastAlertReference := time.Now().Add(-4 * time.Hour)
	freshReferenceTime := time.Now().Add(-30 * time.Minute)

	if shouldSendStalenessAlert(freshReferenceTime, threshold, lastAlertReference) {
		t.Fatal("Expected fresh reference time not to trigger an alert")
	}
}

func TestShouldSendStalenessAlert_AllowsNewStaleReferenceTimeAfterPreviousAlert(t *testing.T) {
	threshold := 2 * time.Hour
	lastAlertReference := time.Now().Add(-4 * time.Hour)
	newReferenceTime := time.Now().Add(-3 * time.Hour)

	if !shouldSendStalenessAlert(newReferenceTime, threshold, lastAlertReference) {
		t.Fatal("Expected a new stale reference time to trigger a fresh alert")
	}
}
