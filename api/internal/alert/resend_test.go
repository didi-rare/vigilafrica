package alert

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vigilafrica/api/internal/models"
)

func TestClientSendIngestFailurePostsToResend(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer re_test" {
			t.Fatalf("expected Authorization header, got %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	errText := "upstream unavailable"
	client := NewClient(Config{
		ResendAPIKey: "re_test",
		FromEmail:    "alerts@vigilafrica.org",
		ToEmail:      "maintainer@example.com",
		Endpoint:     server.URL,
	}, nil)

	err := client.SendIngestFailure(context.Background(), &models.IngestionRun{
		ID:            42,
		CountryCode:   "NG",
		StartedAt:     time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC),
		Status:        models.RunStatusFailure,
		EventsFetched: 3,
		EventsStored:  1,
		Error:         &errText,
	})
	if err != nil {
		t.Fatalf("SendIngestFailure returned error: %v", err)
	}

	if payload["from"] != "alerts@vigilafrica.org" {
		t.Fatalf("unexpected from: %v", payload["from"])
	}
	if payload["subject"] == "" || !strings.Contains(payload["subject"].(string), "Ingestion failed") {
		t.Fatalf("unexpected subject: %v", payload["subject"])
	}
	if !strings.Contains(payload["html"].(string), "upstream unavailable") {
		t.Fatalf("html body did not contain error: %v", payload["html"])
	}
}

func TestClientNoOpsWhenMissingAPIKey(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	client := NewClient(Config{ToEmail: "maintainer@example.com", Endpoint: server.URL}, nil)
	err := client.SendStalenessAlert(context.Background(), time.Now().Add(-3*time.Hour), 2*time.Hour)
	if err != nil {
		t.Fatalf("expected no-op without error, got %v", err)
	}
	if called {
		t.Fatal("expected missing API key to skip HTTP call")
	}
}

func TestClientReturnsErrorForResendFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(Config{
		ResendAPIKey: "re_test",
		ToEmail:      "maintainer@example.com",
		Endpoint:     server.URL,
	}, nil)

	err := client.SendStalenessAlert(context.Background(), time.Now().Add(-3*time.Hour), 2*time.Hour)
	if err == nil {
		t.Fatal("expected Resend failure to return an error")
	}
}
