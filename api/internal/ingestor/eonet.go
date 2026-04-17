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

// CountryConfig defines a country's ingestion parameters.
// BBox is [min_lon, min_lat, max_lon, max_lat] in WGS84 (EPSG:4326).
// Add new countries to DefaultCountries — the scheduler picks them up automatically.
// See: openspec/specs/vigilafrica/country-onboarding-template.md §2.2
type CountryConfig struct {
	Code string     // ISO 3166-1 alpha-2 (e.g. "NG", "GH")
	Name string     // English country name (e.g. "Nigeria", "Ghana")
	BBox [4]float64 // [min_lon, min_lat, max_lon, max_lat]
}

// DefaultCountries is the list of countries ingested on every scheduled tick.
// Bounding boxes are derived from HDX COD ADM0 boundaries + 0.1° buffer.
var DefaultCountries = []CountryConfig{
	{Code: "NG", Name: "Nigeria", BBox: [4]float64{2.0, 4.0, 15.0, 14.0}},
	{Code: "GH", Name: "Ghana", BBox: [4]float64{-3.5, 4.5, 1.2, 11.2}},
}

// IngestResult holds the outcome of a single ingestion run for one country.
type IngestResult struct {
	EventsFetched int
	EventsStored  int
}

// Ingest pulls events from NASA EONET for the given country, upserts them
// (F-013 deduplication), and records the run in ingestion_runs (ADR-011).
func Ingest(ctx context.Context, repo database.Repository, country CountryConfig) (*IngestResult, error) {
	startedAt := time.Now()

	runID, err := repo.CreateIngestionRun(ctx, startedAt)
	if err != nil {
		slog.Error("ingestion: failed to create run record", "country", country.Code, "err", err)
		runID = 0
	}

	slog.Info("ingestion: run started",
		"run_id", runID,
		"country", country.Code,
		"started_at", startedAt.Format(time.RFC3339),
	)

	result, ingestErr := runIngest(ctx, repo, country)

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
			"country", country.Code,
			"duration_ms", duration.Milliseconds(),
			"events_fetched", result.EventsFetched,
			"events_stored", result.EventsStored,
			"err", ingestErr,
		)
		return result, ingestErr
	}

	slog.Info("ingestion: run complete",
		"run_id", runID,
		"country", country.Code,
		"duration_ms", duration.Milliseconds(),
		"events_fetched", result.EventsFetched,
		"events_stored", result.EventsStored,
	)
	return result, nil
}

// runIngest is the core fetch-normalise-upsert loop for a single country.
func runIngest(ctx context.Context, repo database.Repository, country CountryConfig) (*IngestResult, error) {
	result := &IngestResult{}

	client := &http.Client{Timeout: 30 * time.Second}

	bbox := fmt.Sprintf("%.4f,%.4f,%.4f,%.4f",
		country.BBox[0], country.BBox[1],
		country.BBox[2], country.BBox[3],
	)
	reqURL := fmt.Sprintf("%s?bbox=%s&category=floods,wildfires&status=open,closed", eonetURL, bbox)

	slog.Info("ingestion: fetching from EONET", "country", country.Code, "url", reqURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return result, fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "VigilAfrica-Ingestor/0.6")

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
	slog.Info("ingestion: events fetched", "country", country.Code, "count", result.EventsFetched)

	for _, rawEvt := range root.Events {
		rawEvtBytes, _ := json.Marshal(rawEvt)

		event, geoJSON, err := normalizer.Normalize(rawEvt, rawEvtBytes)
		if err != nil {
			slog.Warn("ingestion: normalize failed", "country", country.Code, "source_id", rawEvt.ID, "err", err)
			continue
		}
		if geoJSON == "" {
			slog.Warn("ingestion: skipping event with no geometry", "country", country.Code, "source_id", event.SourceID)
			continue
		}

		// F-013: upsert on source_id — idempotent, no duplicates
		if err := repo.UpsertEvent(ctx, event, geoJSON); err != nil {
			slog.Error("ingestion: upsert failed", "country", country.Code, "source_id", event.SourceID, "err", err)
			continue
		}
		result.EventsStored++
	}

	return result, nil
}
