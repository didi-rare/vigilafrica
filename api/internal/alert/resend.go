package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"vigilafrica/api/internal/models"
)

const defaultResendEndpoint = "https://api.resend.com/emails"

// Config holds the runtime email alert settings.
type Config struct {
	ResendAPIKey string
	FromEmail    string
	ToEmails     []string
	Endpoint     string
}

// Client sends VigilAfrica operational alerts through Resend's HTTP API.
type Client struct {
	cfg        Config
	httpClient *http.Client
	log        *slog.Logger
}

// NewClient creates a Resend-backed alert client. Empty API key or recipient
// intentionally leaves the client enabled as a no-op for local/dev runs.
func NewClient(cfg Config, logger *slog.Logger) *Client {
	if cfg.Endpoint == "" {
		cfg.Endpoint = defaultResendEndpoint
	}
	if cfg.FromEmail == "" {
		cfg.FromEmail = "VigilAfrica Alerts <alerts@vigilafrica.org>"
	}
	cfg.ToEmails = cleanRecipients(cfg.ToEmails)
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		log:        logger,
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.cfg.ResendAPIKey != "" && len(c.cfg.ToEmails) > 0
}

func ParseRecipients(value string) []string {
	if value == "" {
		return nil
	}
	return cleanRecipients(strings.Split(value, ","))
}

func cleanRecipients(values []string) []string {
	recipients := make([]string, 0, len(values))
	for _, value := range values {
		recipient := strings.TrimSpace(value)
		if recipient == "" {
			continue
		}
		recipients = append(recipients, recipient)
	}
	return recipients
}

func (c *Client) SendIngestFailure(ctx context.Context, run *models.IngestionRun) error {
	if run == nil {
		return nil
	}

	if !c.Enabled() {
		c.log.Warn("alerting disabled; skipping ingestion failure alert")
		return nil
	}

	errMsg := ""
	if run.Error != nil {
		errMsg = *run.Error
	}

	data := struct {
		RunID         int64
		CountryCode   string
		StartedAt     string
		CompletedAt   string
		EventsFetched int
		EventsStored  int
		Error         string
	}{
		RunID:         run.ID,
		CountryCode:   run.CountryCode,
		StartedAt:     run.StartedAt.Format(time.RFC3339),
		EventsFetched: run.EventsFetched,
		EventsStored:  run.EventsStored,
		Error:         errMsg,
	}
	if run.CompletedAt != nil {
		data.CompletedAt = run.CompletedAt.Format(time.RFC3339)
	}

	htmlBody, textBody, err := renderEmail(failureHTMLTemplate, failureTextTemplate, data)
	if err != nil {
		return fmt.Errorf("render failure alert: %w", err)
	}

	subject := fmt.Sprintf("[VigilAfrica] Ingestion failed for %s at %s", run.CountryCode, data.StartedAt)
	if err := c.sendEmail(ctx, subject, htmlBody, textBody); err != nil {
		return fmt.Errorf("send failure alert: %w", err)
	}
	c.log.Info("failure alert sent", "run_id", run.ID, "country", run.CountryCode, "recipient_count", len(c.cfg.ToEmails))
	return nil
}

func (c *Client) SendStalenessAlert(ctx context.Context, lastSuccessAt time.Time, threshold time.Duration) error {
	if !c.Enabled() {
		c.log.Warn("alerting disabled; skipping staleness alert")
		return nil
	}

	hoursStale := int(time.Since(lastSuccessAt).Hours())
	data := struct {
		LastSuccessAt string
		HoursStale    int
		Threshold     string
	}{
		LastSuccessAt: lastSuccessAt.Format(time.RFC3339),
		HoursStale:    hoursStale,
		Threshold:     threshold.String(),
	}

	htmlBody, textBody, err := renderEmail(stalenessHTMLTemplate, stalenessTextTemplate, data)
	if err != nil {
		return fmt.Errorf("render staleness alert: %w", err)
	}

	subject := fmt.Sprintf("[VigilAfrica] No successful ingestion in %d hours", hoursStale)
	if err := c.sendEmail(ctx, subject, htmlBody, textBody); err != nil {
		return fmt.Errorf("send staleness alert: %w", err)
	}
	c.log.Warn("staleness alert sent", "last_success_at", data.LastSuccessAt, "hours_stale", hoursStale, "recipient_count", len(c.cfg.ToEmails))
	return nil
}

func (c *Client) sendEmail(ctx context.Context, subject, htmlBody, textBody string) error {
	payload := map[string]any{
		"from":    c.cfg.FromEmail,
		"to":      c.cfg.ToEmails,
		"subject": subject,
		"html":    htmlBody,
		"text":    textBody,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal email payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.Endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.ResendAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "VigilAfrica-Alerts/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("resend request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if len(body) == 0 {
			return fmt.Errorf("resend returned status %d", resp.StatusCode)
		}
		return fmt.Errorf("resend returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func renderEmail(htmlTmpl, textTmpl string, data any) (string, string, error) {
	var htmlBody strings.Builder
	if err := template.Must(template.New("html").Parse(htmlTmpl)).Execute(&htmlBody, data); err != nil {
		return "", "", err
	}

	var textBody strings.Builder
	if err := template.Must(template.New("text").Parse(textTmpl)).Execute(&textBody, data); err != nil {
		return "", "", err
	}
	return htmlBody.String(), textBody.String(), nil
}

const failureHTMLTemplate = `<p>An ingestion run failed.</p>
<ul>
<li><strong>Run ID:</strong> {{.RunID}}</li>
<li><strong>Country:</strong> {{.CountryCode}}</li>
<li><strong>Started:</strong> {{.StartedAt}}</li>
<li><strong>Completed:</strong> {{.CompletedAt}}</li>
<li><strong>Events fetched:</strong> {{.EventsFetched}}</li>
<li><strong>Events stored:</strong> {{.EventsStored}}</li>
<li><strong>Error:</strong> {{.Error}}</li>
</ul>
<p>Check the API logs and ingestion_runs table on the VPS.</p>`

const failureTextTemplate = `An ingestion run failed.
Run ID: {{.RunID}}
Country: {{.CountryCode}}
Started: {{.StartedAt}}
Completed: {{.CompletedAt}}
Events fetched: {{.EventsFetched}}
Events stored: {{.EventsStored}}
Error: {{.Error}}`

const stalenessHTMLTemplate = `<p>The VigilAfrica ingestion system may be stalled.</p>
<ul>
<li><strong>Last successful run:</strong> {{.LastSuccessAt}}</li>
<li><strong>Hours since last success:</strong> {{.HoursStale}}</li>
<li><strong>Threshold:</strong> {{.Threshold}}</li>
</ul>
<p>Check the scheduler, outbound EONET access, and API logs.</p>`

const stalenessTextTemplate = `The VigilAfrica ingestion system may be stalled.
Last successful run: {{.LastSuccessAt}}
Hours since last success: {{.HoursStale}}
Threshold: {{.Threshold}}`
