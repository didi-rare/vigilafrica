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

// schedulerLockTTL bounds how long a crashed/disappeared scheduler holds the
// lock before another replica may take over. Decoupled from the ingestion
// interval (chore-post-v11-quality-sweep B2) — the previous coupling meant a
// crash at the start of a 60-minute interval left the lock orphaned for the
// whole interval. 5 minutes comfortably covers the worst-case ingestion run
// today (~3 min for both NG and GH including retries) while keeping crash
// recovery tight.
//
// SCALE NOTE: if event volume or country count grows such that runAllCountries
// exceeds ~4 minutes, either bump this value or implement a heartbeat that
// extends the lock while the run is in progress. The latter is the proper fix
// for multi-replica HA and is captured as a follow-up.
const schedulerLockTTL = 5 * time.Minute

func runAllCountriesWithLock(ctx context.Context, repo database.Repository, alertClient *alert.Client, _ time.Duration) {
	lockRepo, ok := repo.(schedulerLockRepository)
	if !ok {
		runAllCountries(ctx, repo, alertClient)
		return
	}

	holder := schedulerLockHolder()
	acquired, err := lockRepo.TryAcquireSchedulerLock(ctx, schedulerLockName, holder, schedulerLockTTL)
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
	// ONE GDACS budget for the whole run, shared across every country (§3.5).
	budget := NewRunBudget()

	for _, country := range DefaultCountries {
		if err := ctx.Err(); err != nil {
			slog.Info("scheduler: context cancelled before country ingestion", "country", country.Code, "err", err)
			return
		}
		runScheduledIngest(ctx, repo, alertClient, country, budget)
	}
}

// runScheduledIngest executes a single ingestion cycle for one country and fires
// a Resend failure alert if the run fails.
func runScheduledIngest(ctx context.Context, repo database.Repository, alertClient *alert.Client, country CountryConfig, budget *RunBudget) {
	// Snapshot this country's previous run status BEFORE ingesting: IngestWithBudget
	// inserts a 'running' row first, which would otherwise mask the previous one.
	prev := previousRunStatus(ctx, repo, country.Code)

	result, err := IngestWithBudget(ctx, repo, country, budget)
	if err == nil {
		if alertClient != nil && shouldAlertDegraded(prev, result) {
			if aerr := alertClient.SendIngestFailure(ctx, degradedAlertRun(result, country)); aerr != nil {
				slog.Error("scheduler: failed to send degraded alert", "country", country.Code, "err", aerr)
			}
		}
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

// previousRunStatus returns the status of this country's most recent run, or ""
// when it cannot be determined. Unknown is treated as "not degraded" by
// shouldAlertDegraded, so a lookup failure errs towards alerting, never towards
// silence.
func previousRunStatus(ctx context.Context, repo database.Repository, countryCode string) models.IngestionRunStatus {
	byCountry, err := repo.GetLastIngestionRunAllCountries(ctx)
	if err != nil {
		slog.Warn("scheduler: could not read previous run status; degraded alert will not be deduplicated",
			"country", countryCode, "err", err)
		return ""
	}
	if run := byCountry[countryCode]; run != nil {
		return run.Status
	}
	return ""
}

// shouldAlertDegraded decides whether a completed run warrants a degraded alert.
//
// It fires on the TRANSITION into degraded only. A GDACS outage typically spans
// many scheduler cycles; alerting every cycle would train the on-call to ignore
// the mail, which defeats the point of making the outage visible. Recovery is
// visible on /health; this alert exists to say "it started".
func shouldAlertDegraded(prev models.IngestionRunStatus, result *IngestResult) bool {
	status, _ := runOutcome(result, nil)
	return status == models.RunStatusDegraded && prev != models.RunStatusDegraded
}

// degradedAlertRun is the run record sent with a degraded alert. It prefers the
// persisted record; if the run row could not be written it is rebuilt from the
// result, so a database hiccup cannot suppress the alert.
func degradedAlertRun(result *IngestResult, country CountryConfig) *models.IngestionRun {
	if result != nil && result.Run != nil {
		return result.Run
	}
	status, msg := runOutcome(result, nil)
	run := &models.IngestionRun{StartedAt: time.Now(), CountryCode: country.Code, Status: status, Error: msg}
	if result != nil {
		run.EventsFetched, run.EventsStored = result.EventsFetched, result.EventsStored
	}
	return run
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
