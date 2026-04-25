package alert

import (
	"context"
	"log/slog"
	"time"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/models"
)

type WatchdogConfig struct {
	CheckInterval      time.Duration
	StalenessThreshold time.Duration
}

func (cfg WatchdogConfig) withDefaults() WatchdogConfig {
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 15 * time.Minute
	}
	if cfg.StalenessThreshold <= 0 {
		cfg.StalenessThreshold = 2 * time.Hour
	}
	return cfg
}

func StartStalenessWatchdog(ctx context.Context, repo database.Repository, client *Client, cfg WatchdogConfig, logger *slog.Logger) {
	if client == nil || !client.Enabled() {
		slog.Info("watchdog: alerting disabled; staleness watchdog not started")
		return
	}
	if logger == nil {
		logger = slog.Default()
	}

	cfg = cfg.withDefaults()
	logger.Info("watchdog: started", "check_interval", cfg.CheckInterval, "staleness_threshold", cfg.StalenessThreshold)

	go func() {
		ticker := time.NewTicker(cfg.CheckInterval)
		defer ticker.Stop()
		var lastAlertReference time.Time

		for {
			select {
			case <-ctx.Done():
				logger.Info("watchdog: stopped")
				return
			case <-ticker.C:
				referenceTime, ok := latestIngestionReference(ctx, repo, logger)
				if !ok {
					continue
				}

				if !shouldSendStalenessAlert(time.Now(), referenceTime, cfg.StalenessThreshold, lastAlertReference) {
					if time.Since(referenceTime) <= cfg.StalenessThreshold {
						lastAlertReference = time.Time{}
					}
					continue
				}

				if err := client.SendStalenessAlert(ctx, referenceTime, cfg.StalenessThreshold); err != nil {
					logger.Error("watchdog: failed to send staleness alert", "err", err)
					continue
				}
				lastAlertReference = referenceTime
			}
		}
	}()
}

func latestIngestionReference(ctx context.Context, repo database.Repository, logger *slog.Logger) (time.Time, bool) {
	lastSuccessRun, err := repo.GetLastSuccessfulIngestionRun(ctx)
	if err != nil {
		logger.Error("watchdog: failed to query last successful ingestion run", "err", err)
		return time.Time{}, false
	}

	var firstRun *models.IngestionRun
	if lastSuccessRun == nil {
		firstRun, err = repo.GetFirstIngestionRun(ctx)
		if err != nil {
			logger.Error("watchdog: failed to query first ingestion run", "err", err)
			return time.Time{}, false
		}
	}

	referenceTime, ok := stalenessReferenceTime(lastSuccessRun, firstRun)
	if !ok {
		logger.Warn("watchdog: no ingestion runs found yet")
	}
	return referenceTime, ok
}

func stalenessReferenceTime(lastSuccessRun, firstRun *models.IngestionRun) (time.Time, bool) {
	if lastSuccessRun != nil {
		if lastSuccessRun.CompletedAt != nil {
			return *lastSuccessRun.CompletedAt, true
		}
		return lastSuccessRun.StartedAt, true
	}

	if firstRun != nil {
		return firstRun.StartedAt, true
	}

	return time.Time{}, false
}

func shouldSendStalenessAlert(now, referenceTime time.Time, threshold time.Duration, lastAlertReference time.Time) bool {
	if now.Sub(referenceTime) <= threshold {
		return false
	}

	if !lastAlertReference.IsZero() && lastAlertReference.Equal(referenceTime) {
		return false
	}

	return true
}
