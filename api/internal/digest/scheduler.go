package digest

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Mailer is the email sink the scheduler depends on. *alert.Client satisfies it
// (via Enabled + Send); tests use a fake. Defined here so the digest package
// does not depend on the alert package's concrete client.
type Mailer interface {
	Enabled() bool
	Send(ctx context.Context, subject, htmlBody, textBody string) error
}

// SchedulerConfig configures the daily send.
type SchedulerConfig struct {
	Hour        int    // UTC hour-of-day (0–23) to send the digest
	Environment string // APP_ENV, used in the subject label
}

// StartDigestScheduler launches the daily-digest goroutine. It sends once per
// day at cfg.Hour (UTC), following the watchdog/ingestor pattern: spawned from
// main, exits cleanly on ctx.Done(). When the mailer is disabled (no
// DIGEST_TO), it logs and does not start — so local dev and CI never send and
// never spin an idle goroutine.
//
// Single-replica assumption: VigilAfrica runs one API container per
// environment, so there is no cross-replica send-lock. If the deployment ever
// scales out, add a DB lock (see database.TryAcquireSchedulerLock) before the
// send to avoid duplicate emails.
func StartDigestScheduler(ctx context.Context, repo EventLister, mailer Mailer, cfg SchedulerConfig, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	if !mailer.Enabled() {
		logger.Info("daily digest disabled (no recipients configured); scheduler not started")
		return
	}

	logger.Info("daily digest scheduler started", "hour_utc", cfg.Hour)
	go func() {
		for {
			next := nextRun(time.Now(), cfg.Hour)
			timer := time.NewTimer(time.Until(next))
			select {
			case <-ctx.Done():
				timer.Stop()
				logger.Info("daily digest scheduler stopped")
				return
			case <-timer.C:
				if err := SendDigest(ctx, repo, mailer, cfg, time.Now(), logger); err != nil {
					// Log and continue — the next day's run is independent.
					logger.Error("daily digest send failed", "err", err)
				}
			}
		}
	}()
}

// nextRun returns the next UTC time at hour:00:00 strictly after now.
func nextRun(now time.Time, hour int) time.Time {
	now = now.UTC()
	candidate := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, time.UTC)
	if !candidate.After(now) {
		candidate = candidate.Add(24 * time.Hour)
	}
	return candidate
}

// SendDigest builds today's digest and emails it once. Exported so it can be
// triggered for a manual test send and exercised directly in tests with a fake
// mailer and an injected clock — no real network, no scheduler timing.
func SendDigest(ctx context.Context, repo EventLister, mailer Mailer, cfg SchedulerConfig, now time.Time, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}
	if !mailer.Enabled() {
		logger.Info("daily digest disabled; skipping send")
		return nil
	}

	d, err := BuildTodayDigest(ctx, repo, now)
	if err != nil {
		return fmt.Errorf("build digest: %w", err)
	}

	htmlBody, textBody, err := renderDigest(d)
	if err != nil {
		return fmt.Errorf("render digest: %w", err)
	}

	subject := fmt.Sprintf("[VigilAfrica:%s] Daily Flood Digest — %d event%s — %s",
		cfg.Environment, d.Total, plural(d.Total), d.Date)

	if err := mailer.Send(ctx, subject, htmlBody, textBody); err != nil {
		return fmt.Errorf("send digest: %w", err)
	}

	logger.Info("daily digest sent", "date", d.Date, "total", d.Total)
	return nil
}
