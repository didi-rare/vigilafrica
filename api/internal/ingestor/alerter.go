package ingestor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/models"
)

const resendAPIURL = "https://api.resend.com/emails"

// AlertConfig holds email alert configuration read from environment variables.
type AlertConfig struct {
	ResendAPIKey        string
	AlertEmailTo        string
	AlertFromEmail      string // verified sender domain in Resend
	StalenessThresholdH int    // hours before a staleness alert fires
}

// LoadAlertConfig reads alert configuration from environment variables.
// Returns a config and a boolean indicating whether alerting is enabled.
func LoadAlertConfig() (AlertConfig, bool) {
	fromEmail := os.Getenv("ALERT_FROM_EMAIL")
	if fromEmail == "" {
		fromEmail = "VigilAfrica Alerts <alerts@vigilafrica.org>"
	}
	cfg := AlertConfig{
		ResendAPIKey:        os.Getenv("RESEND_API_KEY"),
		AlertEmailTo:        os.Getenv("ALERT_EMAIL_TO"),
		AlertFromEmail:      fromEmail,
		StalenessThresholdH: 2,
	}
	if h := os.Getenv("ALERT_STALENESS_THRESHOLD_HOURS"); h != "" {
		if v, err := strconv.Atoi(h); err == nil && v > 0 {
			cfg.StalenessThresholdH = v
		}
	}
	enabled := cfg.ResendAPIKey != "" && cfg.AlertEmailTo != ""
	if !enabled {
		slog.Warn("alerter: RESEND_API_KEY or ALERT_EMAIL_TO not set — email alerts disabled")
	}
	return cfg, enabled
}

// SendFailureAlert sends an email when an ingestion run fails.
// If Resend is unreachable, the error is logged but does not crash the caller.
func SendFailureAlert(cfg AlertConfig, run *models.IngestionRun) {
	if cfg.ResendAPIKey == "" || cfg.AlertEmailTo == "" {
		return
	}

	errMsg := ""
	if run.Error != nil {
		errMsg = *run.Error
	}
	subject := fmt.Sprintf("[VigilAfrica] Ingestion failed at %s", run.StartedAt.Format(time.RFC3339))
	body := fmt.Sprintf(
		"<p>An ingestion run failed.</p>"+
			"<ul>"+
			"<li><strong>Run ID:</strong> %d</li>"+
			"<li><strong>Started:</strong> %s</li>"+
			"<li><strong>Events fetched:</strong> %d</li>"+
			"<li><strong>Events stored:</strong> %d</li>"+
			"<li><strong>Error:</strong> %s</li>"+
			"</ul>"+
			"<p>Check your VPS logs for details.</p>",
		run.ID,
		run.StartedAt.Format(time.RFC3339),
		run.EventsFetched,
		run.EventsStored,
		errMsg,
	)

	if err := sendEmail(cfg, subject, body); err != nil {
		slog.Error("alerter: failed to send failure alert", "run_id", run.ID, "err", err)
	} else {
		slog.Info("alerter: failure alert sent", "run_id", run.ID, "to", cfg.AlertEmailTo)
	}
}

// SendStalenessAlert sends an email when no successful ingestion has occurred
// within the configured threshold window.
func SendStalenessAlert(cfg AlertConfig, lastSuccessAt time.Time, hoursStale int) {
	if cfg.ResendAPIKey == "" || cfg.AlertEmailTo == "" {
		return
	}

	subject := fmt.Sprintf("[VigilAfrica] No successful ingestion in %d hours", hoursStale)
	body := fmt.Sprintf(
		"<p>The VigilAfrica ingestion system may be stalled.</p>"+
			"<ul>"+
			"<li><strong>Last successful run:</strong> %s</li>"+
			"<li><strong>Hours since last success:</strong> %d</li>"+
			"<li><strong>Threshold:</strong> %d hours</li>"+
			"</ul>"+
			"<p>Check your VPS scheduler and logs immediately.</p>",
		lastSuccessAt.Format(time.RFC3339),
		hoursStale,
		cfg.StalenessThresholdH,
	)

	if err := sendEmail(cfg, subject, body); err != nil {
		slog.Error("alerter: failed to send staleness alert", "err", err)
	} else {
		slog.Warn("alerter: staleness alert sent",
			"last_success_at", lastSuccessAt.Format(time.RFC3339),
			"hours_stale", hoursStale,
			"to", cfg.AlertEmailTo,
		)
	}
}

// sendEmail sends a single email via the Resend API.
// Returns an error if the API call fails; does not panic.
func sendEmail(cfg AlertConfig, subject, htmlBody string) error {
	payload := map[string]any{
		"from":    cfg.AlertFromEmail,
		"to":      []string{cfg.AlertEmailTo},
		"subject": subject,
		"html":    htmlBody,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal email payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendAPIURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to build Resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.ResendAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Resend API unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Resend API returned non-2xx status: %d", resp.StatusCode)
	}
	return nil
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

// StartStalenessWatchdog launches a goroutine that periodically checks whether
// a successful ingestion has occurred within the threshold window.
// If stale, it fires a Resend alert. The goroutine exits when ctx is cancelled.
func StartStalenessWatchdog(ctx context.Context, repo database.Repository, cfg AlertConfig) {
	if cfg.ResendAPIKey == "" || cfg.AlertEmailTo == "" {
		slog.Info("watchdog: alerting disabled — staleness watchdog not started")
		return
	}

	checkInterval := 30 * time.Minute
	threshold := time.Duration(cfg.StalenessThresholdH) * time.Hour

	slog.Info("watchdog: started",
		"check_interval", checkInterval,
		"staleness_threshold_hours", cfg.StalenessThresholdH,
	)

	go func() {
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				slog.Info("watchdog: stopped")
				return
			case <-ticker.C:
				lastSuccessRun, err := repo.GetLastSuccessfulIngestionRun(ctx)
				if err != nil {
					slog.Error("watchdog: failed to query last successful ingestion run", "err", err)
					continue
				}

				var firstRun *models.IngestionRun
				if lastSuccessRun == nil {
					firstRun, err = repo.GetFirstIngestionRun(ctx)
					if err != nil {
						slog.Error("watchdog: failed to query first ingestion run", "err", err)
						continue
					}
				}

				referenceTime, ok := stalenessReferenceTime(lastSuccessRun, firstRun)
				if !ok {
					slog.Warn("watchdog: no ingestion runs found yet")
					continue
				}

				if time.Since(referenceTime) > threshold {
					hoursStale := int(time.Since(referenceTime).Hours())
					SendStalenessAlert(cfg, referenceTime, hoursStale)
				}
			}
		}
	}()
}
