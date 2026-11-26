package ingestor

import (
	"context"
	"errors"
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

// BoundaryLoader is the boundary step that runs at the start of every locked
// scheduler run, before ingestion (feature-continental-coverage spec §2, P1).
// renewLease extends the scheduler lease; it returns an error wrapping
// ErrLeaseLost when the lease could not be renewed, and the loader must then
// stop. An error wrapping ErrLeaseLost stops the whole run; any other error is
// logged and ingestion goes ahead. Implemented by boundaries.Loader.
type BoundaryLoader interface {
	Run(ctx context.Context, renewLease func(context.Context) error) error
}

// ErrLeaseLost: renewing the scheduler lease failed or another holder has it.
var ErrLeaseLost = errors.New("scheduler lease lost")

// lockReleaseTimeout bounds the lock release, which runs on a fresh context so
// that a run ended by SIGTERM still frees the lease at once instead of leaving
// it to expire (the shutdown context is already cancelled by then).
const lockReleaseTimeout = 10 * time.Second

// StartScheduler launches a background goroutine that runs ingestion for all
// countries in DefaultCountries at a configurable interval (F-012).
// Uses stdlib time.Ticker — no external deps.
//
// Every run — the one on startup and each tick — takes the scheduler lease,
// runs the boundary loader (when boundaryLoader is non-nil), then ingests. The
// function returns as soon as the goroutine is launched, so boot never waits
// on a boundary download.
//
// Default interval: 60 minutes, configurable via INGEST_INTERVAL_MIN; 0 turns
// the whole scheduler off, boundary loader included.
// The goroutine exits cleanly when ctx is cancelled (SIGTERM/SIGINT).
//
// The returned channel is closed when the goroutine has returned, after the
// run in progress has released the scheduler lease (already closed when the
// scheduler is disabled). The caller must wait on it before closing the
// repository: releasing the lease needs the pool, and a release on a closed
// pool leaves the lease held until it expires (developers-go.md §7.1).
func StartScheduler(ctx context.Context, repo database.Repository, alertClient *alert.Client, boundaryLoader BoundaryLoader) <-chan struct{} {
	done := make(chan struct{})
	intervalMin := 60
	if v := os.Getenv("INGEST_INTERVAL_MIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			if n == 0 {
				slog.Info("scheduler: disabled via INGEST_INTERVAL_MIN=0")
				close(done)
				return done
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
		"boundary_loader", boundaryLoader != nil,
	)

	go func() {
		defer close(done)
		// Run once immediately on startup so there is data on first boot
		slog.Info("scheduler: running initial ingestion on startup")
		runAllCountriesWithLock(ctx, repo, alertClient, boundaryLoader)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				slog.Info("scheduler: shutdown signal received — stopping")
				return
			case <-ticker.C:
				slog.Info("scheduler: tick — starting scheduled ingestion")
				runAllCountriesWithLock(ctx, repo, alertClient, boundaryLoader)
			}
		}
	}()
	return done
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

// SchedulerLockTTL is schedulerLockTTL for callers that must stay under it
// (the boundary loader's BOUNDARY_TX_TIMEOUT).
const SchedulerLockTTL = schedulerLockTTL

func runAllCountriesWithLock(ctx context.Context, repo database.Repository, alertClient *alert.Client, boundaryLoader BoundaryLoader) {
	ingest := func(ctx context.Context) { runAllCountries(ctx, repo, alertClient) }

	lockRepo, ok := repo.(schedulerLockRepository)
	if !ok {
		// No lease to take or renew: a repository without the lock (test doubles).
		runLocked(ctx, nil, "", boundaryLoader, ingest)
		return
	}
	runLocked(ctx, lockRepo, schedulerLockHolder(), boundaryLoader, ingest)
}

// runLocked is one scheduler run: take the lease (when lockRepo is non-nil),
// load boundaries, ingest, release the lease. A run that finds the lease held
// does nothing, so a startup run that loses the race loads boundaries on the
// next tick instead.
func runLocked(ctx context.Context, lockRepo schedulerLockRepository, holder string, boundaryLoader BoundaryLoader, ingest func(context.Context)) {
	renew := func(context.Context) error { return nil }

	if lockRepo != nil {
		acquired, err := lockRepo.TryAcquireSchedulerLock(ctx, schedulerLockName, holder, schedulerLockTTL)
		if err != nil {
			slog.Error("scheduler: failed to acquire scheduler lock", "err", err)
			return
		}
		if !acquired {
			slog.Info("scheduler: another instance holds scheduler lock; skipping this run (boundary load and ingestion)")
			return
		}
		defer releaseSchedulerLock(ctx, lockRepo, holder)

		// Calling TryAcquire with our own holder extends the lease
		// (TryAcquireSchedulerLock in database/queries.go).
		renew = func(ctx context.Context) error {
			ok, err := lockRepo.TryAcquireSchedulerLock(ctx, schedulerLockName, holder, schedulerLockTTL)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrLeaseLost, err)
			}
			if !ok {
				return fmt.Errorf("%w: held by another instance", ErrLeaseLost)
			}
			return nil
		}
	}

	if boundaryLoader != nil {
		if err := boundaryLoader.Run(ctx, renew); err != nil {
			switch {
			case errors.Is(err, ErrLeaseLost):
				slog.Error("scheduler: lease lost during the boundary load; stopping this run", "err", err)
				return
			case ctx.Err() != nil:
				slog.Info("scheduler: context cancelled during the boundary load", "err", err)
				return
			default:
				slog.Error("scheduler: boundary load did not complete; ingesting on the boundaries already stored", "err", err)
			}
		}
	}

	ingest(ctx)
}

// releaseSchedulerLock frees the lease on a fresh, short-lived context.
func releaseSchedulerLock(ctx context.Context, lockRepo schedulerLockRepository, holder string) {
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), lockReleaseTimeout)
	defer cancel()
	if err := lockRepo.ReleaseSchedulerLock(rctx, schedulerLockName, holder); err != nil {
		slog.Error("scheduler: failed to release scheduler lock", "err", err)
	}
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
	result, err := IngestWithBudget(ctx, repo, country, budget)
	if err == nil {
		if alertClient != nil {
			notifyIfDegraded(ctx, repo, alertClient, result, country)
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

// degradedAction is what to do about a completed run's degraded status.
type degradedAction int

const (
	degradedNoAlert degradedAction = iota // run is not degraded
	degradedSend                          // first degraded run of a streak, or the last send failed
	degradedCarry                         // streak already alerted: record that, do not re-send
)

// degradedAlertAction decides whether a completed run warrants a degraded alert.
//
// It alerts once per degraded STREAK, keyed on DELIVERY rather than on status
// (independent review, PR #280):
//   - prev is the previous COMPLETED run for the country (never a 'running'
//     row: an orphaned one would otherwise mask the streak and re-send).
//   - The alert is suppressed only if prev was degraded AND its alert was
//     actually delivered. If the last send failed, AlertSentAt is unset and this
//     run retries, instead of the streak being silenced forever.
//   - A lookup error errs towards sending: a duplicate email is recoverable, a
//     silent outage is the failure this change exists to remove.
func degradedAlertAction(result *IngestResult, prev *models.IngestionRun, lookupErr error) degradedAction {
	if status, _ := runOutcome(result, nil); status != models.RunStatusDegraded {
		return degradedNoAlert
	}
	if lookupErr == nil && prev != nil && prev.Status == models.RunStatusDegraded && prev.AlertSentAt != nil {
		return degradedCarry
	}
	return degradedSend
}

// notifyIfDegraded applies degradedAlertAction for a completed run.
//
// ⚠️ Delivery is AT-LEAST-ONCE per streak, not exactly-once. A crash, or a failed
// MarkIngestionRunAlerted, between a successful send and recording it causes the
// next degraded run to send again. That is the deliberate trade: the opposite
// failure — recording before sending — can silence a real outage, and a
// duplicate email is the recoverable one (independent review, PR #280, round 2).
func notifyIfDegraded(ctx context.Context, repo database.Repository, alertClient *alert.Client, result *IngestResult, country CountryConfig) {
	// Disabled alerting must be checked BEFORE dedupe: SendIngestFailure returns
	// nil without sending when unconfigured, and recording that as delivered
	// would silence the streak once alerting is configured mid-outage.
	if !alertClient.Enabled() {
		// §8.3: a recoverable anomaly a human should know about is logged at Warn,
		// not dropped. The failure path gets this warning from SendIngestFailure;
		// the degraded path returns before reaching it, so it warns here
		// (independent review, PR #280, round 4).
		if status, _ := runOutcome(result, nil); status == models.RunStatusDegraded {
			// Through the client's own logger (§8.6) so a test can capture it
			// without swapping the process-wide default (§9.5).
			alertClient.Logger().Warn("scheduler: run is degraded but alerting is disabled (RESEND_API_KEY or recipients unset); no email was sent",
				"country", country.Code)
		}
		return
	}

	var runID int64
	if result != nil && result.Run != nil {
		runID = result.Run.ID
	}

	var prev *models.IngestionRun
	var lookupErr error
	if runID > 0 {
		prev, lookupErr = repo.GetPreviousCompletedIngestionRun(ctx, country.Code, runID)
		if lookupErr != nil {
			slog.Warn("scheduler: could not read previous run; degraded alert will not be deduplicated",
				"country", country.Code, "err", lookupErr)
		}
	} else {
		// No run row was written, so there is nothing to dedupe against or mark.
		lookupErr = errors.New("no persisted run record")
	}

	switch degradedAlertAction(result, prev, lookupErr) {
	case degradedNoAlert:
		return
	case degradedCarry:
		if runID > 0 {
			// Carry the streak's ORIGINAL delivery time, not now: nothing was sent.
			if err := repo.MarkIngestionRunAlerted(ctx, runID, *prev.AlertSentAt); err != nil {
				slog.Warn("scheduler: could not carry alerted flag; next degraded run may re-alert",
					"country", country.Code, "run_id", runID, "err", err)
			}
		}
	case degradedSend:
		if err := alertClient.SendIngestFailure(ctx, degradedAlertRun(result, country)); err != nil {
			// Deliberately NOT marked: the next degraded run will retry.
			slog.Error("scheduler: failed to send degraded alert; will retry on the next degraded run",
				"country", country.Code, "err", err)
			return
		}
		if runID > 0 {
			if err := repo.MarkIngestionRunAlerted(ctx, runID, time.Now()); err != nil {
				slog.Warn("scheduler: degraded alert sent but not recorded; next degraded run may re-alert",
					"country", country.Code, "run_id", runID, "err", err)
			}
		}
	}
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
