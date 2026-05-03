package ingestor

import (
	"errors"
	"testing"
	"time"

	"vigilafrica/api/internal/models"
)

func TestFailureAlertRunUsesCurrentRunMetadata(t *testing.T) {
	errMsg := "current country failed"
	startedAt := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	completedAt := startedAt.Add(time.Second)
	result := &IngestResult{
		EventsFetched: 3,
		EventsStored:  1,
		Run: &models.IngestionRun{
			ID:            42,
			CountryCode:   "GH",
			StartedAt:     startedAt,
			CompletedAt:   &completedAt,
			Status:        models.RunStatusFailure,
			EventsFetched: 3,
			EventsStored:  1,
			Error:         &errMsg,
		},
	}

	run := failureAlertRun(result, errors.New("fallback error"), CountryConfig{Code: "NG"})

	if run.ID != 42 {
		t.Fatalf("expected current run ID 42, got %d", run.ID)
	}
	if run.CountryCode != "GH" {
		t.Fatalf("expected current run country GH, got %q", run.CountryCode)
	}
	if run.Error == nil || *run.Error != errMsg {
		t.Fatalf("expected current run error %q, got %v", errMsg, run.Error)
	}
}

func TestFailureAlertRunBuildsSyntheticFallback(t *testing.T) {
	ingestErr := errors.New("synthetic failure")
	run := failureAlertRun(&IngestResult{EventsFetched: 2, EventsStored: 1}, ingestErr, CountryConfig{Code: "NG"})

	if run.CountryCode != "NG" {
		t.Fatalf("expected fallback country NG, got %q", run.CountryCode)
	}
	if run.EventsFetched != 2 || run.EventsStored != 1 {
		t.Fatalf("expected fallback counts 2/1, got %d/%d", run.EventsFetched, run.EventsStored)
	}
	if run.Error == nil || *run.Error != ingestErr.Error() {
		t.Fatalf("expected fallback error %q, got %v", ingestErr.Error(), run.Error)
	}
}
