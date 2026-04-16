package ingestor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/models"
	"vigilafrica/api/internal/normalizer"
)

const eonetURL = "https://eonet.gsfc.nasa.gov/api/v3/events"

// Nigeria bounding box: [min_lon, min_lat, max_lon, max_lat]
const queryParams = "?bbox=2.0,4.0,15.0,14.0&category=floods,wildfires&status=open,closed"

// IngestResult holds the outcome of a single ingestion run.
type IngestResult struct {
	EventsFetched int
	EventsStored  int
}

// Ingest pulls events from NASA EONET, upserts them (F-013 deduplication),
// and records the run in ingestion_runs (ADR-011).
// It returns the run result and any fatal error.
func Ingest(ctx context.Context, repo database.Repository) (*IngestResult, error) {
	startedAt := time.Now()

	// Open a run record (status=running) — ADR-011
	runID, err := repo.CreateIngestionRun(ctx, startedAt)
	if err != nil {
		// If we can't write a run record, log and continue — do not abort ingestion
		slog.Error("ingestion: failed to create run record", "err", err)
		runID = 0
	}

	slog.Info("ingestion: run started", "run_id", runID, "started_at", startedAt.Format(time.RFC3339))

	result, ingestErr := runIngest(ctx, repo)

	duration := time.Since(startedAt)

	if runID > 0 {
		status := models.RunStatusSuccess
		var errMsg *string
		if ingestErr != nil {
			status = models.RunStatusFailure
			msg := ingestErr.Error()
			errMsg = &msg
		}
		if completeErr := repo.CompleteIngestionRun(ctx, runID, status, result.EventsFetched, result.EventsStored, errMsg); completeErr != nil {
			slog.Error("ingestion: failed to complete run record", "run_id", runID, "err", completeErr)
		}
	}

	if ingestErr != nil {
		slog.Error("ingestion: run failed",
			"run_id", runID,
			"duration_ms", duration.Milliseconds(),
			"events_fetched", result.EventsFetched,
			"events_stored", result.EventsStored,
			"err", ingestErr,
		)
		return result, ingestErr
	}

	slog.Info("ingestion: run complete",
		"run_id", runID,
		"duration_ms", duration.Milliseconds(),
		"events_fetched", result.EventsFetched,
		"events_stored", result.EventsStored,
	)
	return result, nil
}

// runIngest is the core fetch-normalize-upsert loop.
// Separated from Ingest so run record tracking wraps cleanly.
func runIngest(ctx context.Context, repo database.Repository) (*IngestResult, error) {
	result := &IngestResult{}

	client := &http.Client{Timeout: 30 * time.Second}

	reqURL := eonetURL + queryParams
	slog.Info("ingestion: fetching from EONET", "url", reqURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return result, fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "VigilAfrica-Ingestor/0.5")

	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("unexpected EONET status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, fmt.Errorf("failed to read response body: %w", err)
	}

	var root struct {
		Events []normalizer.RawEONETEvent `json:"events"`
	}
	if err := json.Unmarshal(body, &root); err != nil {
		return result, fmt.Errorf("failed to decode EONET JSON: %w", err)
	}

	result.EventsFetched = len(root.Events)
	slog.Info("ingestion: events fetched", "count", result.EventsFetched)

	for _, rawEvt := range root.Events {
		rawEvtBytes, _ := json.Marshal(rawEvt)

		event, geoJSON, err := normalizer.Normalize(rawEvt, rawEvtBytes)
		if err != nil {
			slog.Warn("ingestion: normalize failed", "source_id", rawEvt.ID, "err", err)
			continue
		}
		if geoJSON == "" {
			slog.Warn("ingestion: skipping event with no geometry", "source_id", event.SourceID)
			continue
		}

		// F-013: upsert on source_id — idempotent, no duplicates
		if err := repo.UpsertEvent(ctx, event, geoJSON); err != nil {
			slog.Error("ingestion: upsert failed", "source_id", event.SourceID, "err", err)
			continue
		}
		result.EventsStored++
	}

	return result, nil
}
