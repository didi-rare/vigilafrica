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

const (
	maxEONETResponseBytes        = 5 * 1024 * 1024
	maxRetryAfterSeconds         = 60
	maxEONETEventRawPayloadBytes = 256 * 1024
)

// eonetURL is the NASA EONET v3 events endpoint.
// Declared as var (not const) to allow override in tests via eonetURL = server.URL.
// TODO(future): refactor into an Ingestor struct field to avoid global mutation (openspec-review §9.5).
var eonetURL = "https://eonet.gsfc.nasa.gov/api/v3/events"

// eonetSleepFn is the sleep function used between retries.
// Replaced in tests to avoid real wall-clock waits (openspec-review §9.11).
// TODO(future): inject via Ingestor struct rather than package-level var.
var eonetSleepFn = func(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

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
	Run           *models.IngestionRun
}

// Ingest pulls events from NASA EONET for the given country, upserts them
// (F-013 deduplication), and records the run in ingestion_runs (ADR-011).
func Ingest(ctx context.Context, repo database.Repository, country CountryConfig) (*IngestResult, error) {
	startedAt := time.Now()

	runID, err := repo.CreateIngestionRun(ctx, startedAt, country.Code)
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

	completedAt := time.Now()
	duration := completedAt.Sub(startedAt)

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
		result.Run = &models.IngestionRun{
			ID:            runID,
			CountryCode:   country.Code,
			StartedAt:     startedAt,
			CompletedAt:   &completedAt,
			Status:        status,
			EventsFetched: result.EventsFetched,
			EventsStored:  result.EventsStored,
			Error:         errMsg,
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

	var body []byte
	var reqErr error
	var resp *http.Response
	maxRetries := 3

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return result, fmt.Errorf("failed to create http request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "VigilAfrica-Ingestor")

		resp, reqErr = client.Do(req)
		if reqErr != nil {
			return result, fmt.Errorf("http request failed: %w", reqErr)
		}

		if resp.StatusCode == http.StatusOK {
			body, reqErr = readLimitedResponseBody(resp.Body, maxEONETResponseBytes)
			resp.Body.Close()
			if reqErr != nil {
				return result, fmt.Errorf("failed to read response body: %w", reqErr)
			}
			break
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			bodyBytes, _ := readLimitedResponseBody(resp.Body, 64*1024)
			resp.Body.Close()

			if attempt == maxRetries {
				return result, fmt.Errorf("unexpected EONET status after %d retries: %d", maxRetries, resp.StatusCode)
			}

			var rateLimitData struct {
				RetryAfter int `json:"retry_after"`
			}

			// Exponential fallback: 5s, 10s, 20s if retry_after is absent or unparseable.
			sleepSeconds := 5 * (1 << attempt)
			if err := json.Unmarshal(bodyBytes, &rateLimitData); err == nil && rateLimitData.RetryAfter > 0 {
				if rateLimitData.RetryAfter > maxRetryAfterSeconds {
					return result, fmt.Errorf("EONET retry_after %d exceeds maximum %d seconds", rateLimitData.RetryAfter, maxRetryAfterSeconds)
				}
				// Dynamic backoff: honour the server's hint + 5s buffer for clock drift.
				sleepSeconds = rateLimitData.RetryAfter + 5
			}

			slog.Warn("ingestion: high demand, retrying",
				"country", country.Code,
				"status", resp.StatusCode,
				"attempt", attempt+1,
				"sleep_sec", sleepSeconds,
			)

			if err := eonetSleepFn(ctx, time.Duration(sleepSeconds)*time.Second); err != nil {
				return result, err
			}
			continue
		}

		resp.Body.Close()
		return result, fmt.Errorf("unexpected EONET status: %d", resp.StatusCode)
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
		if len(rawEvtBytes) > maxEONETEventRawPayloadBytes {
			return result, fmt.Errorf("EONET event %s raw payload exceeds maximum %d bytes", rawEvt.ID, maxEONETEventRawPayloadBytes)
		}

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

func readLimitedResponseBody(r io.Reader, maxBytes int64) ([]byte, error) {
	limited := io.LimitReader(r, maxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("response body exceeds maximum %d bytes", maxBytes)
	}
	return body, nil
}
