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
	// maxTransientRetries bounds retries for network errors and 5xx-non-503
	// responses (chore-eonet-retry-backoff). Distinct from the 429/503
	// retry budget which uses maxRetries (=3) inside runIngest.
	maxTransientRetries = 2
	// closedEventWindowDays bounds the days= window on the status=closed
	// request. EONET flood events close within ~48h, so 30 days is ~15x
	// margin and survives a multi-day ingestion outage without data loss.
	// Applied ONLY to the closed query — the open query is deliberately
	// unwindowed (see runIngest). See fix-eonet-closed-events-ingest.
	closedEventWindowDays = 30
)

// transientRetryDelays are the fixed waits before retry attempts 1 and 2 on
// the transient path (network errors, 5xx non-503). Indices 0 and 1 are
// consumed in order.
var transientRetryDelays = [maxTransientRetries]time.Duration{
	5 * time.Second,
	15 * time.Second,
}

// Package-level test seams below. Three vars (eonetHTTPClient, eonetURL,
// eonetSleepFn) form an implicit Ingestor surface — tests override them via
// the install* helpers in eonet_test.go. Consolidating these into an explicit
// Ingestor struct is **deferred to a follow-up PR** (chore-eonet-ingestor-struct
// or similar) — the change is larger than the rest of chore-post-v11-quality-sweep
// Phase 3 combined (touches scheduler.go, all eonet tests, and any other
// caller of Ingest), and R1 of the sweep spec explicitly authorizes splitting
// Phase 3 items into focused follow-ups when scope creep risks bloating one PR.
// See chore-post-v11-quality-sweep B6.

// eonetHTTPClient is the HTTP client used to fetch EONET events.
// Declared as a package-level var so tests can inject a transport that
// simulates network errors (which httptest.Server can't reproduce directly).
var eonetHTTPClient = &http.Client{Timeout: 30 * time.Second}

// eonetURL is the NASA EONET v3 events endpoint.
// Declared as var (not const) to allow override in tests via eonetURL = server.URL.
var eonetURL = "https://eonet.gsfc.nasa.gov/api/v3/events"

// eonetSleepFn is the sleep function used between retries.
// Replaced in tests to avoid real wall-clock waits (openspec-review §9.11).
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

// withinBBox reports whether (lon, lat) falls inside bbox, which is
// [min_lon, min_lat, max_lon, max_lat] in WGS84 (EPSG:4326).
//
// Bounds are inclusive, matching the semantics of EONET's own bbox query
// parameter. Longitudes may legitimately be negative (Ghana's min_lon is -3.5),
// so no sign assumptions are made.
func withinBBox(bbox [4]float64, lon, lat float64) bool {
	return lon >= bbox[0] && lon <= bbox[2] && lat >= bbox[1] && lat <= bbox[3]
}

// IngestResult holds the outcome of a single ingestion run for one country.
type IngestResult struct {
	EventsFetched int
	EventsStored  int
	// EventsSkippedBBox counts events dropped because their resolved point fell
	// outside the queried country's bounding box. Tracked separately from the
	// EventsFetched-EventsStored delta, which also absorbs normalise failures,
	// missing geometry, and upsert failures — so it cannot indicate an upstream
	// bbox leak on its own.
	EventsSkippedBBox int
	// EventsUnverifiedGeom counts events stored without a containment check
	// because their geometry yielded no point (Polygon). Reported once per run
	// rather than per event: a per-event line would be noise at Info, and Debug
	// is invisible in production (LOG_LEVEL defaults to info) — which would
	// defeat the point of recording it at all.
	EventsUnverifiedGeom int
	Run                  *models.IngestionRun
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
			"events_skipped_bbox", result.EventsSkippedBBox,
			"events_unverified_geom", result.EventsUnverifiedGeom,
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
		"events_skipped_bbox", result.EventsSkippedBBox,
		"events_unverified_geom", result.EventsUnverifiedGeom,
	)
	return result, nil
}

// runIngest is the core fetch-normalise-upsert loop for a single country.
//
// EONET is queried TWICE per country and the results unioned:
//
//  1. status=open  — deliberately WITHOUT a days window
//  2. status=closed&days=30
//
// This is not arbitrary. EONET accepts only open|closed|all for status; the
// previous single request used the invalid value "open,closed", which EONET
// silently degraded to open-only. Because every EONET flood event closes
// within ~48h, floods were never ingested at all (fix-eonet-closed-events-ingest).
//
// The obvious repair — status=all&days=N — regresses wildfires: they stay open
// for months (the oldest open Nigeria wildfire is dated 2024-10-24), and days
// filters on event date, so any practical window drops them. EONET cannot
// express "open OR closed-within-N-days" in one request, hence two.
//
// Overlap between the two responses is harmless: UpsertEvent is idempotent on
// source_id (F-013).
func runIngest(ctx context.Context, repo database.Repository, country CountryConfig) (*IngestResult, error) {
	result := &IngestResult{}

	bbox := fmt.Sprintf("%.4f,%.4f,%.4f,%.4f",
		country.BBox[0], country.BBox[1],
		country.BBox[2], country.BBox[3],
	)

	// No days window on the open query — long-burning wildfires must not be dropped.
	openURL := fmt.Sprintf("%s?bbox=%s&category=floods,wildfires&status=open", eonetURL, bbox)
	closedURL := fmt.Sprintf("%s?bbox=%s&category=floods,wildfires&status=closed&days=%d",
		eonetURL, bbox, closedEventWindowDays)

	for _, reqURL := range []string{openURL, closedURL} {
		// §3.6 — abort between requests rather than issuing the second one
		// against an already-cancelled context.
		if err := ctx.Err(); err != nil {
			return result, err
		}
		body, err := fetchEONET(ctx, reqURL, country.Code)
		if err != nil {
			return result, err
		}
		if err := processEONETBody(ctx, repo, country, body, result); err != nil {
			return result, err
		}
	}

	return result, nil
}

// fetchEONET performs a single EONET request with the full retry budget:
// 2 transient retries (5s/15s) for network errors and 5xx-non-503, and a
// 3-retry 429/503 budget honouring the server's retry_after hint.
func fetchEONET(ctx context.Context, reqURL string, countryCode string) ([]byte, error) {
	slog.Info("ingestion: fetching from EONET", "country", countryCode, "url", reqURL)

	var body []byte
	var reqErr error
	var resp *http.Response
	maxRetries := 3
	transientAttempt := 0 // counts retries on the transient path (network errors, 5xx non-503)

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create http request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "VigilAfrica-Ingestor")

		resp, reqErr = eonetHTTPClient.Do(req)
		if reqErr != nil {
			// Network error (TCP stall, connection reset, DNS, request timeout).
			// Retry up to maxTransientRetries with fixed 5s/15s delays.
			if transientAttempt >= maxTransientRetries {
				return nil, fmt.Errorf("http request failed after %d retries: %w", maxTransientRetries, reqErr)
			}
			sleepDuration := transientRetryDelays[transientAttempt]
			slog.Warn("ingestion: transient error, retrying",
				"country", countryCode,
				"cause", "network-error",
				"attempt", transientAttempt+1,
				"sleep_sec", int(sleepDuration.Seconds()),
				"err", reqErr,
			)
			transientAttempt++
			if err := eonetSleepFn(ctx, sleepDuration); err != nil {
				return nil, err
			}
			continue
		}

		if resp.StatusCode == http.StatusOK {
			body, reqErr = readLimitedResponseBody(resp.Body, maxEONETResponseBytes)
			resp.Body.Close()
			if reqErr != nil {
				return nil, fmt.Errorf("failed to read response body: %w", reqErr)
			}
			break
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			// best-effort: a read error here just yields empty bytes, which
			// fall through to the exponential-backoff branch below (§4.7).
			bodyBytes, _ := readLimitedResponseBody(resp.Body, 64*1024) //nolint:errcheck
			resp.Body.Close()

			if attempt == maxRetries {
				// The outer loop runs attempts 0..maxRetries inclusive, so reaching this
				// branch means we already tried maxRetries+1 times. Report that honestly.
				return nil, fmt.Errorf("unexpected EONET status after %d attempts: %d", maxRetries+1, resp.StatusCode)
			}

			var rateLimitData struct {
				RetryAfter int `json:"retry_after"`
			}

			// Exponential fallback: 5s, 10s, 20s if retry_after is absent or unparseable.
			sleepSeconds := 5 * (1 << attempt)
			if err := json.Unmarshal(bodyBytes, &rateLimitData); err == nil && rateLimitData.RetryAfter > 0 {
				if rateLimitData.RetryAfter > maxRetryAfterSeconds {
					return nil, fmt.Errorf("EONET retry_after %d exceeds maximum %d seconds", rateLimitData.RetryAfter, maxRetryAfterSeconds)
				}
				// Dynamic backoff: honour the server's hint + 5s buffer for clock drift.
				sleepSeconds = rateLimitData.RetryAfter + 5
			}

			slog.Warn("ingestion: high demand, retrying",
				"country", countryCode,
				"status", resp.StatusCode,
				"attempt", attempt+1,
				"sleep_sec", sleepSeconds,
			)

			if err := eonetSleepFn(ctx, time.Duration(sleepSeconds)*time.Second); err != nil {
				return nil, err
			}
			continue
		}

		// 5xx other than 503 — transient server error, retry with the same
		// 2-retry / 5s,15s budget as network errors.
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			resp.Body.Close()
			if transientAttempt >= maxTransientRetries {
				return nil, fmt.Errorf("unexpected EONET status %d after %d transient retries", resp.StatusCode, maxTransientRetries)
			}
			sleepDuration := transientRetryDelays[transientAttempt]
			slog.Warn("ingestion: transient error, retrying",
				"country", countryCode,
				"cause", "server-error-5xx",
				"status", resp.StatusCode,
				"attempt", transientAttempt+1,
				"sleep_sec", int(sleepDuration.Seconds()),
			)
			transientAttempt++
			if err := eonetSleepFn(ctx, sleepDuration); err != nil {
				return nil, err
			}
			continue
		}

		// 4xx (non-429) and any other unexpected status — no retry.
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected EONET status: %d", resp.StatusCode)
	}

	return body, nil
}

// processEONETBody decodes one EONET response and normalises + upserts each
// event into repo, accumulating counts into result. Called once per request in
// the open/closed union, so both counters sum across the two responses.
//
// The two result sets are disjoint in practice — an event is either open or
// closed, never both (verified against the live Nigeria bbox: 27 open, 1
// closed, empty intersection). The only overlap window is an event closing
// upstream between the two requests, in which case UpsertEvent's idempotency
// on source_id (F-013) still stores it once, though EventsFetched would count
// it twice. Treat EventsFetched as "records seen upstream", not a distinct count.
func processEONETBody(
	ctx context.Context,
	repo database.Repository,
	country CountryConfig,
	body []byte,
	result *IngestResult,
) error {
	var root struct {
		Events []normalizer.RawEONETEvent `json:"events"`
	}
	if err := json.Unmarshal(body, &root); err != nil {
		return fmt.Errorf("failed to decode EONET JSON: %w", err)
	}

	result.EventsFetched += len(root.Events)
	slog.Info("ingestion: events fetched", "country", country.Code, "count", len(root.Events))

	for _, rawEvt := range root.Events {
		rawEvtBytes, _ := json.Marshal(rawEvt)
		if len(rawEvtBytes) > maxEONETEventRawPayloadBytes {
			return fmt.Errorf("EONET event %s raw payload exceeds maximum %d bytes", rawEvt.ID, maxEONETEventRawPayloadBytes)
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

		// Containment guard: EONET's server-side bbox filter is a hint, not a
		// guarantee — it has been observed returning events wholly outside the
		// requested box (a Florida wildfire against the Nigeria bbox). Validate
		// client-side so foreign events never reach the database.
		if event.Longitude != nil && event.Latitude != nil {
			if !withinBBox(country.BBox, *event.Longitude, *event.Latitude) {
				slog.Warn("ingestion: skipping event outside country bbox",
					"country", country.Code,
					"source_id", event.SourceID,
					"lon", *event.Longitude,
					"lat", *event.Latitude,
				)
				result.EventsSkippedBBox++
				continue
			}
		} else {
			// Containment is unverifiable: the normalizer resolves lon/lat only for
			// Point geometry and leaves them nil for Polygon. Such events are stored
			// deliberately — we do not drop data we cannot verify — but the fact is
			// counted so a polygon-shaped upstream leak stays discoverable in the
			// run summary. Per-event detail stays at Debug for local diagnosis.
			result.EventsUnverifiedGeom++
			geomType := ""
			if event.GeomType != nil {
				geomType = *event.GeomType
			}
			slog.Debug("ingestion: storing event without bbox verification",
				"country", country.Code,
				"source_id", event.SourceID,
				"geom_type", geomType,
			)
		}

		// F-013: upsert on source_id — idempotent, no duplicates
		if err := repo.UpsertEvent(ctx, event, geoJSON); err != nil {
			slog.Error("ingestion: upsert failed", "country", country.Code, "source_id", event.SourceID, "err", err)
			continue
		}
		result.EventsStored++
	}

	return nil
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
