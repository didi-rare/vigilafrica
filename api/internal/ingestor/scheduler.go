package ingestor

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"time"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/models"
)

// StartScheduler launches a background goroutine that runs ingestion for all
// countries in DefaultCountries at a configurable interval (F-012).
// Uses stdlib time.Ticker — no external deps.
//
// Default interval: 60 minutes, configurable via INGEST_INTERVAL_MIN.
// The goroutine exits cleanly when ctx is cancelled (SIGTERM/SIGINT).
func StartScheduler(ctx context.Context, repo database.Repository, alertCfg AlertConfig) {
	intervalMin := 60
	if v := os.Getenv("INGEST_INTERVAL_MIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			if n == 0 {
				slog.Info("scheduler: disabled via INGEST_INTERVAL_MIN=0")
				return
			}
			if n > 0 {
				intervalMin = n
			}
		}
	}

	interval := time.Duration(intervalMin) * time.Minute
	slog.Info("scheduler: starting",
		"interval_minutes", intervalMin,
		"countries", len(DefaultCountries),
	)

	go func() {
		// Run once immediately on startup so there is data on first boot
		slog.Info("scheduler: running initial ingestion on startup")
		runAllCountries(ctx, repo, alertCfg)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				slog.Info("scheduler: shutdown signal received — stopping")
				return
			case <-ticker.C:
				slog.Info("scheduler: tick — starting scheduled ingestion")
				runAllCountries(ctx, repo, alertCfg)
			}
		}
	}()
}

// runAllCountries iterates over DefaultCountries and ingests each in sequence.
// A failure for one country is logged and alerted but does not abort the others.
func runAllCountries(ctx context.Context, repo database.Repository, alertCfg AlertConfig) {
	for _, country := range DefaultCountries {
		runScheduledIngest(ctx, repo, alertCfg, country)
	}
}

// runScheduledIngest executes a single ingestion cycle for one country and fires
// a Resend failure alert if the run fails.
func runScheduledIngest(ctx context.Context, repo database.Repository, alertCfg AlertConfig, country CountryConfig) {
	result, err := Ingest(ctx, repo, country)
	if err == nil {
		return
	}

	// Attempt to load the persisted run record for the alert body
	lastRun, dbErr := repo.GetLastIngestionRun(ctx)
	if dbErr == nil && lastRun != nil && lastRun.Status == models.RunStatusFailure {
		SendFailureAlert(alertCfg, lastRun)
		return
	}

	// Fallback: build a synthetic run record from what we know
	errMsg := err.Error()
	synthetic := &models.IngestionRun{
		StartedAt:     time.Now(),
		Status:        models.RunStatusFailure,
		EventsFetched: result.EventsFetched,
		EventsStored:  result.EventsStored,
		Error:         &errMsg,
	}
	SendFailureAlert(alertCfg, synthetic)
}
