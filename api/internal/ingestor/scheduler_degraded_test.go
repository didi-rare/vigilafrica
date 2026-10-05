package ingestor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"vigilafrica/api/internal/alert"
	"vigilafrica/api/internal/models"
)

// End-to-end through the PRODUCTION path (independent review, PR #280): fake
// EONET + fake GDACS + fake Resend, real runScheduledIngest -> IngestWithBudget
// -> runIngest -> resolver -> runOutcome -> persistence -> notifyIfDegraded.
// The unit tests prove each piece; this proves they are wired together, so it
// fails if the ingest loop stops counting upstream failures or the scheduler
// stops alerting.

// runLedger is an in-memory ingestion_runs table with the same semantics the
// SQL implements (previous = latest non-running row for the country, by id).
type runLedger struct {
	*mockRepo
	mu   sync.Mutex
	runs []*models.IngestionRun
}

func (l *runLedger) CreateIngestionRun(_ context.Context, startedAt time.Time, country string) (int64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	id := int64(len(l.runs) + 1)
	l.runs = append(l.runs, &models.IngestionRun{ID: id, CountryCode: country, StartedAt: startedAt, Status: models.RunStatusRunning})
	return id, nil
}

func (l *runLedger) CompleteIngestionRun(_ context.Context, id int64, s models.IngestionRunStatus, fetched, stored int, msg *string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	r := l.runs[id-1]
	r.Status, r.EventsFetched, r.EventsStored, r.Error = s, fetched, stored, msg
	return nil
}

func (l *runLedger) GetPreviousCompletedIngestionRun(_ context.Context, country string, before int64) (*models.IngestionRun, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := len(l.runs) - 1; i >= 0; i-- {
		r := l.runs[i]
		if r.ID < before && r.CountryCode == country && r.Status != models.RunStatusRunning {
			cp := *r
			return &cp, nil
		}
	}
	return nil, nil
}

func (l *runLedger) MarkIngestionRunAlerted(_ context.Context, id int64, deliveredAt time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.runs[id-1].AlertSentAt = &deliveredAt
	return nil
}

func (l *runLedger) status(id int64) models.IngestionRunStatus {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.runs[id-1].Status
}

// orphan inserts a run stuck in 'running', as a crashed process would leave.
func (l *runLedger) orphan(country string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.runs = append(l.runs, &models.IngestionRun{ID: int64(len(l.runs) + 1), CountryCode: country, Status: models.RunStatusRunning})
}

// fakeResend counts delivery attempts and can be told to fail.
type fakeResend struct {
	mu       sync.Mutex
	attempts int
	subjects []string
	fail     bool
}

func (f *fakeResend) handler(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attempts++
	b, _ := io.ReadAll(r.Body)
	f.subjects = append(f.subjects, string(b))
	if f.fail {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = w.Write([]byte(`{"id":"x"}`))
}

func (f *fakeResend) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.attempts
}

// degradedWorld serves one polygon flood from EONET while GDACS returns 500.
func degradedWorld(t *testing.T) (*alert.Client, *fakeResend) {
	t.Helper()
	const flood = `{"events":[{"id":"EONET_22248","title":"Flood in Cameroon 1104078",
		"categories":[{"id":"floods"}],
		"sources":[{"id":"GDACS","url":"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1104078"}],
		"geometry":[{"date":"2026-08-03T20:00:00Z","type":"Polygon","coordinates":[[[4.377,9.216],[4.828,9.216],[4.828,9.569],[4.377,9.569],[4.377,9.216]]]}]}]}`
	eonet := httptest.NewServer(closedQueryStub(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, flood)
	}))
	t.Cleanup(eonet.Close)
	restore := installTestServer(t, eonet)
	t.Cleanup(restore)

	gdacsStub(t, status(http.StatusInternalServerError), status(http.StatusInternalServerError))

	resend := &fakeResend{}
	srv := httptest.NewServer(http.HandlerFunc(resend.handler))
	t.Cleanup(srv.Close)
	client := alert.NewClient(alert.Config{
		ResendAPIKey: "test-key", FromEmail: "alerts@example.test", ToEmails: []string{"ops@example.test"},
		Endpoint: srv.URL, Environment: "test",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return client, resend
}

func TestScheduledIngestDegradedAlertsOncePerStreak(t *testing.T) {
	client, resend := degradedWorld(t)
	ledger := &runLedger{mockRepo: &mockRepo{}}
	ctx := context.Background()

	runScheduledIngest(ctx, ledger, client, testCountry, NewRunBudget())
	if got := ledger.status(1); got != models.RunStatusDegraded {
		t.Fatalf("run 1 status = %q, want degraded — the ingest loop is not counting upstream failures", got)
	}
	if resend.count() != 1 {
		t.Fatalf("run 1: %d alert(s) sent, want 1", resend.count())
	}
	if !strings.Contains(resend.subjects[0], "degraded") {
		t.Errorf("alert subject should say degraded, got payload %s", resend.subjects[0])
	}

	runScheduledIngest(ctx, ledger, client, testCountry, NewRunBudget())
	if resend.count() != 1 {
		t.Errorf("run 2 (still degraded): %d alert(s) total, want 1 — the streak was re-alerted", resend.count())
	}
	// Carry-forward must keep the streak's ORIGINAL delivery time: nothing was
	// sent for run 2, so stamping it "now" would make alert_sent_at a lie.
	if a, b := ledger.runs[0].AlertSentAt, ledger.runs[1].AlertSentAt; a == nil || b == nil || !a.Equal(*b) {
		t.Errorf("carry-forward delivery time = %v, want the original %v", b, a)
	}

	// ⚠️ Independent-review case: an orphaned 'running' row between two degraded
	// runs must not look like the start of a new streak.
	ledger.orphan(testCountry.Code)
	runScheduledIngest(ctx, ledger, client, testCountry, NewRunBudget())
	if resend.count() != 1 {
		t.Errorf("after an orphaned running row: %d alert(s) total, want 1 — the orphan masked the streak", resend.count())
	}
}

func TestScheduledIngestRetriesDegradedAlertAfterFailedSend(t *testing.T) {
	client, resend := degradedWorld(t)
	ledger := &runLedger{mockRepo: &mockRepo{}}
	ctx := context.Background()

	resend.fail = true
	runScheduledIngest(ctx, ledger, client, testCountry, NewRunBudget())
	if resend.count() != 1 {
		t.Fatalf("run 1: %d attempt(s), want 1", resend.count())
	}

	// ⚠️ Independent-review case: the send failed, so the streak was never
	// actually alerted. The next degraded run must retry rather than stay silent.
	resend.fail = false
	runScheduledIngest(ctx, ledger, client, testCountry, NewRunBudget())
	if resend.count() != 2 {
		t.Fatalf("run 2: %d attempt(s) total, want 2 — a failed send silenced the streak", resend.count())
	}

	runScheduledIngest(ctx, ledger, client, testCountry, NewRunBudget())
	if resend.count() != 2 {
		t.Errorf("run 3: %d attempt(s) total, want 2 — delivered streak was re-alerted", resend.count())
	}
}

// failingMarkLedger fails MarkIngestionRunAlerted, simulating a crash or DB
// error in the window between a successful send and recording it.
type failingMarkLedger struct{ *runLedger }

func (failingMarkLedger) MarkIngestionRunAlerted(context.Context, int64, time.Time) error {
	return errors.New("db unavailable")
}

func TestScheduledIngestDisabledAlertingDoesNotSilenceTheStreak(t *testing.T) {
	enabled, resend := degradedWorld(t)
	disabled := alert.NewClient(alert.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ledger := &runLedger{mockRepo: &mockRepo{}}
	ctx := context.Background()

	// Alerting unconfigured: the degraded run must NOT be recorded as alerted.
	runScheduledIngest(ctx, ledger, disabled, testCountry, NewRunBudget())
	if ledger.runs[0].AlertSentAt != nil {
		t.Fatal("a disabled alert client was recorded as having delivered the alert")
	}

	// Configured mid-outage: the first real email must go out.
	runScheduledIngest(ctx, ledger, enabled, testCountry, NewRunBudget())
	if resend.count() != 1 {
		t.Errorf("after enabling alerting mid-streak: %d alert(s), want 1 — the streak was silenced", resend.count())
	}
}

func TestScheduledIngestAlertIsAtLeastOnceWhenRecordingFails(t *testing.T) {
	// Documents the chosen trade: if recording fails after a successful send,
	// the next degraded run sends again (a duplicate), rather than risking
	// silence. Exactly-once is NOT claimed.
	client, resend := degradedWorld(t)
	ledger := failingMarkLedger{&runLedger{mockRepo: &mockRepo{}}}
	ctx := context.Background()

	runScheduledIngest(ctx, ledger, client, testCountry, NewRunBudget())
	runScheduledIngest(ctx, ledger, client, testCountry, NewRunBudget())
	if resend.count() != 2 {
		t.Errorf("unrecorded send: %d alert(s), want 2 (at-least-once)", resend.count())
	}
}

func TestScheduledIngestRealertsAfterRecovery(t *testing.T) {
	client, resend := degradedWorld(t)
	ledger := &runLedger{mockRepo: &mockRepo{}}
	ctx := context.Background()

	runScheduledIngest(ctx, ledger, client, testCountry, NewRunBudget()) // degraded: alert 1

	// GDACS answers 404: a refusal, not an outage, so the run is a success and
	// the streak ends.
	gdacsStub(t, status(http.StatusNotFound), status(http.StatusNotFound))
	runScheduledIngest(ctx, ledger, client, testCountry, NewRunBudget())
	if got := ledger.status(2); got != models.RunStatusSuccess {
		t.Fatalf("run 2 status = %q, want success (a 404 is a refusal, not an outage)", got)
	}

	gdacsStub(t, status(http.StatusInternalServerError), status(http.StatusInternalServerError))
	runScheduledIngest(ctx, ledger, client, testCountry, NewRunBudget()) // new streak: alert 2
	if resend.count() != 2 {
		t.Errorf("degraded -> success -> degraded: %d alert(s), want 2", resend.count())
	}
}

func TestScheduledIngestStreaksAreIsolatedPerCountry(t *testing.T) {
	client, resend := degradedWorld(t)
	ledger := &runLedger{mockRepo: &mockRepo{}}
	ctx := context.Background()
	other := testCountry
	other.Code = "GH"

	runScheduledIngest(ctx, ledger, client, testCountry, NewRunBudget())
	runScheduledIngest(ctx, ledger, client, other, NewRunBudget())
	if resend.count() != 2 {
		t.Errorf("degraded in two countries: %d alert(s), want 2 — one country's streak suppressed the other's", resend.count())
	}
}
