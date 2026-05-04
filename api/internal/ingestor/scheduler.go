package ingestor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"vigilafrica/api/internal/alert"
	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/models"
)

const schedulerLockName = "ingestion-scheduler"

type schedulerLockRepository interface {
	TryAcquireSchedulerLock(ctx context.Context, lockName, holder string, ttl time.Duration) (bool, error)
	ReleaseSchedulerLock(ctx context.Context, lockName, holder string) error
}

// StartScheduler launches a background goroutine that runs ingestion for all
// countries in DefaultCountries at a configurable interval (F-012).
// Uses stdlib time.Ticker — no external deps.
//
// Default interval: 60 minutes, configurable via INGEST_INTERVAL_MIN.
// The goroutine exits cleanly when ctx is cancelled (SIGTERM/SIGINT).
func StartScheduler(ctx context.Context, repo database.Repository, alertClient *alert.Client) {
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
		runAllCountriesWithLock(ctx, repo, alertClient, interval)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				slog.Info("scheduler: shutdown signal received — stopping")
				return
			case <-ticker.C:
				slog.Info("scheduler: tick — starting scheduled ingestion")
				runAllCountriesWithLock(ctx, repo, alertClient, interval)
			}
		}
	}()
}

func runAllCountriesWithLock(ctx context.Context, repo database.Repository, alertClient *alert.Client, interval time.Duration) {
	lockRepo, ok := repo.(schedulerLockRepository)
	if !ok {
		runAllCountries(ctx, repo, alertClient)
		return
	}

	holder := schedulerLockHolder()
	ttl := interval
	if ttl < 5*time.Minute {
		ttl = 5 * time.Minute
	}
	acquired, err := lockRepo.TryAcquireSchedulerLock(ctx, schedulerLockName, holder, ttl)
	if err != nil {
		slog.Error("scheduler: failed to acquire scheduler lock", "err", err)
		return
	}
	if !acquired {
		slog.Info("scheduler: another instance holds scheduler lock; skipping ingestion")
		return
	}
	defer func() {
		if err := lockRepo.ReleaseSchedulerLock(ctx, schedulerLockName, holder); err != nil {
			slog.Error("scheduler: failed to release scheduler lock", "err", err)
		}
	}()

	runAllCountries(ctx, repo, alertClient)
}

func schedulerLockHolder() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown-host"
	}
	return fmt.Sprintf("%s:%d", host, os.Getpid())
}

// runAllCountries iterates over DefaultCountries and ingests each in sequence.
// A failure for one country is logged and alerted but does not abort the others.
func runAllCountries(ctx context.Context, repo database.Repository, alertClient *alert.Client) {
	for _, country := range DefaultCountries {
		if err := ctx.Err(); err != nil {
			slog.Info("scheduler: context cancelled before country ingestion", "country", country.Code, "err", err)
			return
		}
		runScheduledIngest(ctx, repo, alertClient, country)
	}
}

// runScheduledIngest executes a single ingestion cycle for one country and fires
// a Resend failure alert if the run fails.
func runScheduledIngest(ctx context.Context, repo database.Repository, alertClient *alert.Client, country CountryConfig) {
	result, err := Ingest(ctx, repo, country)
	if err == nil {
		return
	}
	if alertClient == nil {
		return
	}

	alertRun := failureAlertRun(result, err, country)
	if err := alertClient.SendIngestFailure(ctx, alertRun); err != nil {
		slog.Error("scheduler: failed to send failure alert", "country", country.Code, "err", err)
	}
}

func failureAlertRun(result *IngestResult, ingestErr error, country CountryConfig) *models.IngestionRun {
	if result != nil && result.Run != nil {
		return result.Run
	}

	errMsg := ""
	if ingestErr != nil {
		errMsg = ingestErr.Error()
	}
	fetched := 0
	stored := 0
	if result != nil {
		fetched = result.EventsFetched
		stored = result.EventsStored
	}
	return &models.IngestionRun{
		StartedAt:     time.Now(),
		CountryCode:   country.Code,
		Status:        models.RunStatusFailure,
		EventsFetched: fetched,
		EventsStored:  stored,
		Error:         &errMsg,
	}
}
