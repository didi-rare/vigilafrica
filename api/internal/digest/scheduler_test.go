package digest

import (
	"context"
	"strings"
	"testing"
	"time"

	"vigilafrica/api/internal/models"
)

type fakeMailer struct {
	enabled bool
	sends   []sentEmail
}

type sentEmail struct{ subject, html, text string }

func (m *fakeMailer) Enabled() bool { return m.enabled }

func (m *fakeMailer) Send(_ context.Context, subject, htmlBody, textBody string) error {
	m.sends = append(m.sends, sentEmail{subject, htmlBody, textBody})
	return nil
}

func TestNextRun(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		hour int
		want time.Time
	}{
		{
			name: "before scheduled hour today",
			now:  time.Date(2026, 6, 2, 5, 0, 0, 0, time.UTC),
			hour: 6,
			want: time.Date(2026, 6, 2, 6, 0, 0, 0, time.UTC),
		},
		{
			name: "exactly at scheduled hour rolls to tomorrow",
			now:  time.Date(2026, 6, 2, 6, 0, 0, 0, time.UTC),
			hour: 6,
			want: time.Date(2026, 6, 3, 6, 0, 0, 0, time.UTC),
		},
		{
			name: "after scheduled hour rolls to tomorrow",
			now:  time.Date(2026, 6, 2, 7, 30, 0, 0, time.UTC),
			hour: 6,
			want: time.Date(2026, 6, 3, 6, 0, 0, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextRun(tt.now, tt.hour); !got.Equal(tt.want) {
				t.Fatalf("nextRun = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSendDigestSendsOneEmail(t *testing.T) {
	mailer := &fakeMailer{enabled: true}
	lister := &fakeLister{events: []models.Event{
		floodEvent("Benue flood", "Nigeria", "Benue"),
		floodEvent("Oyo flood", "Nigeria", "Oyo"),
	}}
	now := time.Date(2026, 6, 2, 6, 0, 0, 0, time.UTC)
	cfg := SchedulerConfig{Hour: 6, Environment: "test"}

	if err := SendDigest(context.Background(), lister, mailer, cfg, now, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mailer.sends) != 1 {
		t.Fatalf("sends = %d, want 1", len(mailer.sends))
	}
	got := mailer.sends[0]
	if !strings.Contains(got.subject, "[VigilAfrica:test]") {
		t.Errorf("subject missing env label: %q", got.subject)
	}
	if !strings.Contains(got.subject, "2 events") {
		t.Errorf("subject missing count: %q", got.subject)
	}
	if !strings.Contains(got.subject, "2026-06-02") {
		t.Errorf("subject missing date: %q", got.subject)
	}
	if !strings.Contains(got.html, "Benue flood") {
		t.Errorf("html missing event title: %q", got.html)
	}
	if !strings.Contains(got.text, "Always confirm with local authorities") {
		t.Errorf("text missing disclaimer: %q", got.text)
	}
}

func TestSendDigestEmptyDayStillSends(t *testing.T) {
	mailer := &fakeMailer{enabled: true}
	lister := &fakeLister{events: nil}
	now := time.Date(2026, 6, 2, 6, 0, 0, 0, time.UTC)

	if err := SendDigest(context.Background(), lister, mailer, SchedulerConfig{Hour: 6, Environment: "staging"}, now, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mailer.sends) != 1 {
		t.Fatalf("sends = %d, want 1 (daily cadence sends even when empty)", len(mailer.sends))
	}
	if !strings.Contains(mailer.sends[0].subject, "0 events") {
		t.Errorf("subject = %q, want '0 events'", mailer.sends[0].subject)
	}
	if !strings.Contains(mailer.sends[0].html, "No flood events recorded today") {
		t.Errorf("html missing empty-state copy: %q", mailer.sends[0].html)
	}
}

func TestSendDigestDisabledMailerNoSend(t *testing.T) {
	mailer := &fakeMailer{enabled: false}
	lister := &fakeLister{events: nil}
	now := time.Date(2026, 6, 2, 6, 0, 0, 0, time.UTC)

	if err := SendDigest(context.Background(), lister, mailer, SchedulerConfig{Hour: 6, Environment: "test"}, now, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mailer.sends) != 0 {
		t.Fatalf("sends = %d, want 0 when mailer disabled", len(mailer.sends))
	}
	if lister.got.Category != "" {
		t.Errorf("expected the builder to be skipped entirely when disabled; lister was queried")
	}
}
