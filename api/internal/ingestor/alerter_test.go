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
