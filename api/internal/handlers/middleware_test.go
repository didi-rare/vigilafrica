package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSMiddlewareRejectsDisallowedOrigin(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "https://staging.vigilafrica.org")

	called := false
	handler := CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/v1/events", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("expected no Access-Control-Allow-Origin header, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if called {
		t.Fatal("expected disallowed preflight to stop before the next handler")
	}
}

func TestCORSMiddlewareAllowsConfiguredOriginAndNoOriginRequests(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "https://staging.vigilafrica.org")

	handler := CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	allowedReq := httptest.NewRequest(http.MethodOptions, "/v1/events", nil)
	allowedReq.Header.Set("Origin", "https://staging.vigilafrica.org")
	allowedRec := httptest.NewRecorder()

	handler.ServeHTTP(allowedRec, allowedReq)

	if allowedRec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, allowedRec.Code)
	}
	if got := allowedRec.Header().Get("Access-Control-Allow-Origin"); got != "https://staging.vigilafrica.org" {
		t.Fatalf("expected configured origin header, got %q", got)
	}

	noOriginReq := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	noOriginRec := httptest.NewRecorder()

	handler.ServeHTTP(noOriginRec, noOriginReq)

	if noOriginRec.Code != http.StatusOK {
		t.Fatalf("expected status %d for no-Origin request, got %d", http.StatusOK, noOriginRec.Code)
	}
	if got := noOriginRec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no Access-Control-Allow-Origin for no-Origin request, got %q", got)
	}
}
