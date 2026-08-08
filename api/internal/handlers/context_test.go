package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/models"
)

func newRequest(target, remoteAddr string, headers map[string]string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

// TestResolveLocationPlanIgnoresForgedHeadersFromUntrustedPeer is the
// regression detector for this change.
//
// ⚠️ It targets resolveLocationPlan — the seam where a request becomes a
// location — NOT clientIPWithTrustedProxies. That distinction is the whole
// point: clientIPWithTrustedProxies was always correct, so a test aimed at it
// would have passed before the fix and proved nothing. The defect was that
// /v1/context never called it.
//
// The pre-fix behaviour was demonstrated empirically before this was written:
// with peer 203.0.113.9 (untrusted) and X-Forwarded-For: 8.8.8.8, the two
// resolvers disagreed — clientIP() returned "203.0.113.9" while the deleted
// extractIP() returned "8.8.8.8".
func TestResolveLocationPlanIgnoresForgedHeadersFromUntrustedPeer(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "127.0.0.1/8,::1/128,172.16.0.0/12")

	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		wantIP     string
		why        string
	}{
		{
			name:       "untrusted peer cannot forge via X-Forwarded-For",
			remoteAddr: "203.0.113.9:51000",
			headers:    map[string]string{"X-Forwarded-For": "8.8.8.8"},
			wantIP:     "203.0.113.9",
			why:        "peer is not a configured proxy, so the header is not evidence",
		},
		{
			name:       "untrusted peer cannot forge via X-Real-IP",
			remoteAddr: "203.0.113.9:51000",
			headers:    map[string]string{"X-Real-IP": "8.8.8.8"},
			wantIP:     "203.0.113.9",
			why:        "the same rule applies to X-Real-IP",
		},
		{
			name:       "trusted proxy is still believed",
			remoteAddr: "172.18.0.1:51000",
			headers:    map[string]string{"X-Forwarded-For": "41.58.100.7"},
			wantIP:     "41.58.100.7",
			why:        "this is the real Caddy path and must keep working",
		},
		{
			name:       "trusted proxy takes the FIRST X-Forwarded-For entry",
			remoteAddr: "172.18.0.1:51000",
			headers:    map[string]string{"X-Forwarded-For": "41.58.100.7, 10.0.0.5, 172.18.0.1"},
			wantIP:     "41.58.100.7",
			why:        "the originating client is leftmost",
		},
		{
			name:       "trusted proxy, X-Real-IP only",
			remoteAddr: "127.0.0.1:51000",
			headers:    map[string]string{"X-Real-IP": "41.58.100.7"},
			wantIP:     "41.58.100.7",
		},
		{
			name:       "no headers falls back to the peer",
			remoteAddr: "203.0.113.9:51000",
			wantIP:     "203.0.113.9",
		},
		{
			name:       "IPv6 loopback peer is trusted",
			remoteAddr: "[::1]:51000",
			headers:    map[string]string{"X-Forwarded-For": "41.58.100.7"},
			wantIP:     "41.58.100.7",
			why:        "IPv6 host/port splitting is easy to get wrong",
		},
		{
			name:       "untrusted IPv6 peer cannot forge",
			remoteAddr: "[2001:db8::1]:51000",
			headers:    map[string]string{"X-Forwarded-For": "8.8.8.8"},
			wantIP:     "2001:db8::1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := resolveLocationPlan(newRequest("/v1/context", tc.remoteAddr, tc.headers))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if plan.Source != LocationSourceIP {
				t.Fatalf("source = %q, want %q", plan.Source, LocationSourceIP)
			}
			if plan.LookupIP != tc.wantIP {
				t.Errorf("lookup IP = %q, want %q\n  peer=%q headers=%v\n  %s",
					plan.LookupIP, tc.wantIP, tc.remoteAddr, tc.headers, tc.why)
			}
		})
	}
}

// TestResolveLocationPlanPrecedence pins the order decided in tasks.md §0.
// These assertions exist because the DEV_* ordering is load-bearing and this
// change moved code around it.
func TestResolveLocationPlanPrecedence(t *testing.T) {
	t.Run("explicit outranks DEV_FORCE_LAGOS", func(t *testing.T) {
		t.Setenv("DEV_FORCE_LAGOS", "true")
		plan, err := resolveLocationPlan(newRequest("/v1/context?lat=51.5&lng=-0.12", "203.0.113.9:1", nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if plan.Source != LocationSourceExplicit {
			t.Fatalf("source = %q, want %q", plan.Source, LocationSourceExplicit)
		}
		if plan.Explicit == nil || plan.Explicit.Lat != 51.5 {
			t.Errorf("explicit location not honoured: %+v", plan.Explicit)
		}
	})

	t.Run("explicit outranks DEV_OVERRIDE_IP", func(t *testing.T) {
		t.Setenv("DEV_OVERRIDE_IP", "8.8.8.8")
		plan, err := resolveLocationPlan(newRequest("/v1/context?lat=51.5&lng=-0.12", "203.0.113.9:1", nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if plan.Source != LocationSourceExplicit || plan.LookupIP != "" {
			t.Errorf("expected explicit to win, got source=%q lookupIP=%q", plan.Source, plan.LookupIP)
		}
	})

	t.Run("DEV_FORCE_LAGOS suppresses any lookup", func(t *testing.T) {
		t.Setenv("DEV_FORCE_LAGOS", "true")
		t.Setenv("DEV_OVERRIDE_IP", "8.8.8.8")
		plan, err := resolveLocationPlan(newRequest("/v1/context", "203.0.113.9:1", nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if plan.LookupIP != "" {
			t.Errorf("lookup IP = %q, want empty — forceLagos must suppress the lookup", plan.LookupIP)
		}
		if plan.Explicit == nil || plan.Explicit.State != "Lagos" {
			t.Errorf("expected hardcoded Lagos, got %+v", plan.Explicit)
		}
		if plan.Source != LocationSourceDevOverride {
			t.Errorf("source = %q, want %q", plan.Source, LocationSourceDevOverride)
		}
	})

	t.Run("DEV_OVERRIDE_IP outranks the request IP", func(t *testing.T) {
		t.Setenv("DEV_OVERRIDE_IP", "8.8.8.8")
		plan, err := resolveLocationPlan(newRequest("/v1/context", "203.0.113.9:1", nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if plan.LookupIP != "8.8.8.8" {
			t.Errorf("lookup IP = %q, want the override", plan.LookupIP)
		}
		if plan.Source != LocationSourceDevOverride {
			t.Errorf("source = %q, want %q", plan.Source, LocationSourceDevOverride)
		}
	})
}

// TestParseExplicitLocation covers the decision-(B) parameters. An invalid
// coordinate must be an error naming the parameter, never a silent fallback to
// IP — a typo must not be indistinguishable from a working query.
func TestParseExplicitLocation(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantOK  bool
		wantErr bool
		lat     float64
		lng     float64
		errHas  string
	}{
		{name: "absent is not an error", query: "", wantOK: false},
		{name: "valid pair", query: "?lat=6.5244&lng=3.3792", wantOK: true, lat: 6.5244, lng: 3.3792},
		{name: "negative pair", query: "?lat=-33.9249&lng=18.4241", wantOK: true, lat: -33.9249, lng: 18.4241},
		{name: "zero pair is present, not absent", query: "?lat=0&lng=0", wantOK: true, lat: 0, lng: 0},
		{name: "boundaries are inclusive", query: "?lat=90&lng=180", wantOK: true, lat: 90, lng: 180},
		{name: "lat without lng", query: "?lat=6.5244", wantErr: true, errHas: "lng"},
		{name: "lng without lat", query: "?lng=3.3792", wantErr: true, errHas: "lat"},
		{name: "empty lat value", query: "?lat=&lng=3.3792", wantErr: true, errHas: "lat"},
		{name: "unparseable lat", query: "?lat=abc&lng=3.3792", wantErr: true, errHas: "lat"},
		{name: "trailing junk is rejected, not truncated", query: "?lat=6.5junk&lng=3.3792", wantErr: true, errHas: "lat"},
		{name: "lat above range", query: "?lat=90.1&lng=3.3792", wantErr: true, errHas: "lat"},
		{name: "lat below range", query: "?lat=-90.1&lng=3.3792", wantErr: true, errHas: "lat"},
		{name: "lng above range", query: "?lat=6.5244&lng=180.1", wantErr: true, errHas: "lng"},
		{name: "lng below range", query: "?lat=6.5244&lng=-180.1", wantErr: true, errHas: "lng"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			loc, ok, err := parseExplicitLocation(newRequest("/v1/context"+tc.query, "203.0.113.9:1", nil))

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error naming %q, got nil (ok=%v)", tc.errHas, ok)
				}
				if !strings.Contains(err.Error(), tc.errHas) {
					t.Errorf("error %q does not name the offending parameter %q", err.Error(), tc.errHas)
				}
				if ok {
					t.Error("ok must be false when validation fails — a bad value must never fall back to IP")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && (loc.Lat != tc.lat || loc.Lng != tc.lng) {
				t.Errorf("got (%v, %v), want (%v, %v)", loc.Lat, loc.Lng, tc.lat, tc.lng)
			}
		})
	}
}

// contextTestRepo is the minimum database.Repository surface GetContext
// touches. The embedded interface is nil: any other method would panic, which
// is the intent — this test must not silently depend on more of the repository.
type contextTestRepo struct{ database.Repository }

func (contextTestRepo) GetNearbyEvents(_ context.Context, _, _ float64, _ float64, _ int) ([]models.Event, error) {
	return []models.Event{}, nil
}

// TestGetContextRejectsBadCoordinates asserts the handler surfaces a bad
// coordinate as 400 rather than quietly geolocating by IP.
func TestGetContextRejectsBadCoordinates(t *testing.T) {
	h := GetContext(contextTestRepo{}, nil)

	rec := httptest.NewRecorder()
	h(rec, newRequest("/v1/context?lat=999&lng=0", "203.0.113.9:1", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "lat") {
		t.Errorf("body %q does not name the offending parameter", rec.Body.String())
	}
}

// TestGetContextDegradesGracefullyWithoutGeoReader pins the pre-existing
// contract: no reader means a null location with HTTP 200, never a 5xx.
func TestGetContextDegradesGracefullyWithoutGeoReader(t *testing.T) {
	h := GetContext(contextTestRepo{}, nil)

	rec := httptest.NewRecorder()
	h(rec, newRequest("/v1/context", "203.0.113.9:1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"location":null`) {
		t.Errorf("expected a null location, got %s", body)
	}
	if !strings.Contains(body, string(LocationSourceUnavailable)) {
		t.Errorf("expected location_source %q, got %s", LocationSourceUnavailable, body)
	}
}

// TestGetContextExplicitLocationIsReported asserts an explicit query is
// reported as such, so a client can tell it from IP geolocation.
func TestGetContextExplicitLocationIsReported(t *testing.T) {
	h := GetContext(contextTestRepo{}, nil)

	rec := httptest.NewRecorder()
	h(rec, newRequest("/v1/context?lat=6.5244&lng=3.3792", "203.0.113.9:1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, string(LocationSourceExplicit)) {
		t.Errorf("expected location_source %q, got %s", LocationSourceExplicit, body)
	}
	if !strings.Contains(body, "6.5244") {
		t.Errorf("expected the supplied latitude in the response, got %s", body)
	}
}
