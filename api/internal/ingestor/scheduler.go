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

// StartScheduler launches a background goroutine that runs ingestion at a
// configurable interval (F-012). Uses stdlib time.Ticker — no external deps.
//
// Default interval: 60 minutes, configurable via INGESTION_INTERVAL_MINUTES.
// The goroutine exits cleanly when ctx is cancelled (SIGTERM/SIGINT).
func StartScheduler(ctx context.Context, repo database.Repository, alertCfg AlertConfig) {
	intervalMin := 60
	if v := os.Getenv("INGESTION_INTERVAL_MINUTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			intervalMin = n
		}
	}

	interval := time.Duration(intervalMin) * time.Minute
	slog.Info("scheduler: starting", "interval_minutes", intervalMin)

	go func() {
		// Run once immediately on startup so there is data on first boot
		slog.Info("scheduler: running initial ingestion on startup")
		runScheduledIngest(ctx, repo, alertCfg)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				slog.Info("scheduler: shutdown signal received — stopping")
				return
			case <-ticker.C:
				slog.Info("scheduler: tick — starting scheduled ingestion")
				runScheduledIngest(ctx, repo, alertCfg)
			}
		}
	}()
}

// runScheduledIngest executes a single ingestion cycle and fires a Resend
// failure alert if the run fails.
func runScheduledIngest(ctx context.Context, repo database.Repository, alertCfg AlertConfig) {
	result, err := Ingest(ctx, repo)
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
