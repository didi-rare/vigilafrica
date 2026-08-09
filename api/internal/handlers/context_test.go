package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/geoip"
	"vigilafrica/api/internal/models"
)

// ─── Fixtures ────────────────────────────────────────────────────────────────
//
// Every test builds its ContextConfig explicitly. Nothing here reads the
// process environment, so the suite is hermetic (§9.5): an ambient
// DEV_FORCE_LAGOS or TRUSTED_PROXY_CIDRS cannot change a result. That was a
// real defect in the first version of this file, not a hypothetical.

func testProxyConfig(t *testing.T, cidrs ...string) ProxyConfig {
	t.Helper()
	if len(cidrs) == 0 {
		cidrs = []string{"127.0.0.1/8", "::1/128", "172.16.0.0/12"}
	}
	var out []*net.IPNet
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			t.Fatalf("bad test CIDR %q: %v", c, err)
		}
		out = append(out, n)
	}
	return ProxyConfig{Trusted: out}
}

func testContextConfig(t *testing.T) ContextConfig {
	t.Helper()
	return ContextConfig{Proxy: testProxyConfig(t)}
}

func newRequest(target, remoteAddr string, headers map[string]string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

// contextTestRepo is the minimum database.Repository surface GetContext
// touches. The embedded interface is nil: any other method panics, which is
// intended — this test must not silently grow a dependency.
type contextTestRepo struct {
	database.Repository
	gotLat, gotLng float64
}

func (r *contextTestRepo) GetNearbyEvents(_ context.Context, lat, lng float64, _ float64, _ int) ([]models.Event, error) {
	r.gotLat, r.gotLng = lat, lng
	return []models.Event{}, nil
}

// stubGeo maps an IP to a location, so handler behaviour is testable without an
// mmdb file. Any unlisted IP fails the lookup, exercising the degrade path.
type stubGeo map[string]*geoip.Location

func (g stubGeo) Lookup(ip string) (*geoip.Location, error) {
	if loc, ok := g[ip]; ok {
		return loc, nil
	}
	return nil, errors.New("no location for IP")
}

func decodeContext(t *testing.T, rec *httptest.ResponseRecorder) ContextResponse {
	t.Helper()
	var resp ContextResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON (%v): %s", err, rec.Body.String())
	}
	return resp
}

// ─── Handler-level proxy behaviour ───────────────────────────────────────────

// TestGetContextIgnoresForgedHeadersFromUntrustedPeer is the regression test
// for this change, asserted at the RESPONSE level.
//
// ⚠️ An earlier version stopped at resolveLocationPlan. That was insufficient
// for exactly the reason this change exists: it would still pass if the handler
// stopped calling the correct helper — the same wiring defect being fixed. This
// version drives GetContext and inspects the decoded body.
func TestGetContextIgnoresForgedHeadersFromUntrustedPeer(t *testing.T) {
	geo := stubGeo{
		"203.0.113.9":  {Country: "Peer Country", State: "Peer State", Lat: 1, Lng: 1},
		"8.8.8.8":      {Country: "Forged Country", State: "Forged State", Lat: 2, Lng: 2},
		"41.58.100.7":  {Country: "Real Client", State: "Real State", Lat: 3, Lng: 3},
		"2001:db8::1":  {Country: "IPv6 Peer", State: "IPv6 State", Lat: 4, Lng: 4},
	}

	tests := []struct {
		name        string
		remoteAddr  string
		headers     map[string]string
		wantCountry string
		why         string
	}{
		{
			name:        "untrusted peer cannot forge via X-Forwarded-For",
			remoteAddr:  "203.0.113.9:51000",
			headers:     map[string]string{"X-Forwarded-For": "8.8.8.8"},
			wantCountry: "Peer Country",
			why:         "the peer is not a configured proxy, so the header is not evidence",
		},
		{
			name:        "untrusted peer cannot forge via X-Real-IP",
			remoteAddr:  "203.0.113.9:51000",
			headers:     map[string]string{"X-Real-IP": "8.8.8.8"},
			wantCountry: "Peer Country",
		},
		{
			name:        "trusted proxy is believed",
			remoteAddr:  "172.18.0.1:51000",
			headers:     map[string]string{"X-Forwarded-For": "41.58.100.7"},
			wantCountry: "Real Client",
			why:         "this is the real Caddy path and must keep working",
		},
		{
			name:        "trusted proxy takes the leftmost entry",
			remoteAddr:  "172.18.0.1:51000",
			headers:     map[string]string{"X-Forwarded-For": "41.58.100.7, 10.0.0.5"},
			wantCountry: "Real Client",
		},
		{
			name:        "trusted proxy, X-Real-IP only",
			remoteAddr:  "127.0.0.1:51000",
			headers:     map[string]string{"X-Real-IP": "41.58.100.7"},
			wantCountry: "Real Client",
		},
		{
			name:        "untrusted IPv6 peer cannot forge",
			remoteAddr:  "[2001:db8::1]:51000",
			headers:     map[string]string{"X-Forwarded-For": "8.8.8.8"},
			wantCountry: "IPv6 Peer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewContextHandler(&contextTestRepo{}, geo, testContextConfig(t))
			rec := httptest.NewRecorder()
			h.GetContext(rec, newRequest("/v1/context", tc.remoteAddr, tc.headers))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
			}
			resp := decodeContext(t, rec)
			if resp.Location == nil {
				t.Fatalf("location is null; want %q", tc.wantCountry)
			}
			if resp.Location.Country != tc.wantCountry {
				t.Errorf("country = %q, want %q\n  peer=%q headers=%v\n  %s",
					resp.Location.Country, tc.wantCountry, tc.remoteAddr, tc.headers, tc.why)
			}
			if resp.LocationSource != LocationSourceIP {
				t.Errorf("location_source = %q, want %q", resp.LocationSource, LocationSourceIP)
			}
		})
	}
}

// ─── Precedence ──────────────────────────────────────────────────────────────

func TestResolveLocationPlanPrecedence(t *testing.T) {
	base := testContextConfig(t)

	t.Run("explicit outranks DevForceLagos", func(t *testing.T) {
		cfg := base
		cfg.DevForceLagos = true
		h := NewContextHandler(&contextTestRepo{}, nil, cfg)
		plan, err := h.resolveLocationPlan(newRequest("/v1/context?lat=51.5&lng=-0.12", "203.0.113.9:1", nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if plan.Source != LocationSourceExplicit || plan.Explicit == nil || plan.Explicit.Lat != 51.5 {
			t.Errorf("explicit did not win: source=%q explicit=%+v", plan.Source, plan.Explicit)
		}
	})

	t.Run("explicit outranks DevOverrideIP", func(t *testing.T) {
		cfg := base
		cfg.DevOverrideIP = "8.8.8.8"
		h := NewContextHandler(&contextTestRepo{}, nil, cfg)
		plan, err := h.resolveLocationPlan(newRequest("/v1/context?lat=51.5&lng=-0.12", "203.0.113.9:1", nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if plan.Source != LocationSourceExplicit || plan.LookupIP != "" {
			t.Errorf("expected explicit to win, got source=%q lookupIP=%q", plan.Source, plan.LookupIP)
		}
	})

	t.Run("DevForceLagos suppresses any lookup", func(t *testing.T) {
		cfg := base
		cfg.DevForceLagos = true
		cfg.DevOverrideIP = "8.8.8.8"
		h := NewContextHandler(&contextTestRepo{}, nil, cfg)
		plan, err := h.resolveLocationPlan(newRequest("/v1/context", "203.0.113.9:1", nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if plan.LookupIP != "" {
			t.Errorf("lookup IP = %q, want empty — forceLagos must suppress the lookup", plan.LookupIP)
		}
		if plan.Explicit == nil || plan.Explicit.State != "Lagos" || plan.Source != LocationSourceDevOverride {
			t.Errorf("expected hardcoded Lagos as dev_override, got %+v source=%q", plan.Explicit, plan.Source)
		}
	})

	t.Run("DevOverrideIP outranks the request IP", func(t *testing.T) {
		cfg := base
		cfg.DevOverrideIP = "8.8.8.8"
		h := NewContextHandler(&contextTestRepo{}, nil, cfg)
		plan, err := h.resolveLocationPlan(newRequest("/v1/context", "203.0.113.9:1", nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if plan.LookupIP != "8.8.8.8" || plan.Source != LocationSourceDevOverride {
			t.Errorf("got lookupIP=%q source=%q", plan.LookupIP, plan.Source)
		}
	})

	t.Run("no headers falls back to the peer", func(t *testing.T) {
		h := NewContextHandler(&contextTestRepo{}, nil, base)
		plan, err := h.resolveLocationPlan(newRequest("/v1/context", "203.0.113.9:51000", nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if plan.LookupIP != "203.0.113.9" {
			t.Errorf("lookup IP = %q, want the peer", plan.LookupIP)
		}
	})

	t.Run("malformed RemoteAddr is passed through, not fatal", func(t *testing.T) {
		h := NewContextHandler(&contextTestRepo{}, nil, base)
		plan, err := h.resolveLocationPlan(newRequest("/v1/context", "not-an-address", nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if plan.LookupIP != "not-an-address" {
			t.Errorf("lookup IP = %q; a malformed peer should be passed through for the lookup to reject", plan.LookupIP)
		}
	})
}

// ─── Coordinate parsing ──────────────────────────────────────────────────────

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
		{name: "trailing junk is rejected", query: "?lat=6.5junk&lng=3.3792", wantErr: true, errHas: "lat"},
		{name: "lat above range", query: "?lat=90.1&lng=3.3792", wantErr: true, errHas: "lat"},
		{name: "lat below range", query: "?lat=-90.1&lng=3.3792", wantErr: true, errHas: "lat"},
		{name: "lng above range", query: "?lat=6.5244&lng=180.1", wantErr: true, errHas: "lng"},
		{name: "lng below range", query: "?lat=6.5244&lng=-180.1", wantErr: true, errHas: "lng"},

		// ⚠️ These are the cases that shipped broken in the first revision.
		// ParseFloat accepts them, and every comparison against NaN is false, so
		// a plain range check passes them straight through to json.Marshal —
		// which cannot encode them, AFTER the 200 has been committed.
		{name: "NaN lat is rejected", query: "?lat=NaN&lng=0", wantErr: true, errHas: "lat"},
		{name: "lowercase nan lat is rejected", query: "?lat=nan&lng=0", wantErr: true, errHas: "lat"},
		{name: "NaN lng is rejected", query: "?lat=0&lng=NaN", wantErr: true, errHas: "lng"},
		{name: "Inf lat is rejected", query: "?lat=Inf&lng=0", wantErr: true, errHas: "lat"},
		{name: "-Inf lng is rejected", query: "?lat=0&lng=-Inf", wantErr: true, errHas: "lng"},
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

// ─── Handler responses ───────────────────────────────────────────────────────

// TestGetContextRejectsInvalidCoordinates asserts the FULL response, not just
// the status. The NaN case previously returned 200 with a truncated body.
func TestGetContextRejectsInvalidCoordinates(t *testing.T) {
	for _, q := range []string{"?lat=999&lng=0", "?lat=NaN&lng=0", "?lat=0&lng=Inf", "?lat=5"} {
		t.Run(q, func(t *testing.T) {
			h := NewContextHandler(&contextTestRepo{}, nil, testContextConfig(t))
			rec := httptest.NewRecorder()
			h.GetContext(rec, newRequest("/v1/context"+q, "203.0.113.9:1", nil))

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %q)", rec.Code, rec.Body.String())
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			var body APIError
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("400 body is not valid JSON (%v): %s", err, rec.Body.String())
			}
			if body.Error == "" {
				t.Error("400 body has an empty error message")
			}
		})
	}
}

func TestGetContextDegradesGracefully(t *testing.T) {
	t.Run("no geo reader", func(t *testing.T) {
		h := NewContextHandler(&contextTestRepo{}, nil, testContextConfig(t))
		rec := httptest.NewRecorder()
		h.GetContext(rec, newRequest("/v1/context", "203.0.113.9:1", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		resp := decodeContext(t, rec)
		if resp.Location != nil || resp.LocationSource != LocationSourceUnavailable {
			t.Errorf("want null location and %q, got %+v / %q", LocationSourceUnavailable, resp.Location, resp.LocationSource)
		}
	})

	// The failed-lookup half, which the first revision claimed but never covered.
	t.Run("geo reader present but lookup fails", func(t *testing.T) {
		h := NewContextHandler(&contextTestRepo{}, stubGeo{}, testContextConfig(t))
		rec := httptest.NewRecorder()
		h.GetContext(rec, newRequest("/v1/context", "203.0.113.9:1", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		resp := decodeContext(t, rec)
		if resp.Location != nil || resp.LocationSource != LocationSourceUnavailable {
			t.Errorf("a failed lookup must degrade, got %+v / %q", resp.Location, resp.LocationSource)
		}
	})
}

func TestGetContextExplicitLocationDrivesTheQuery(t *testing.T) {
	repo := &contextTestRepo{}
	h := NewContextHandler(repo, nil, testContextConfig(t))
	rec := httptest.NewRecorder()
	h.GetContext(rec, newRequest("/v1/context?lat=6.5244&lng=3.3792", "203.0.113.9:1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	resp := decodeContext(t, rec)
	if resp.LocationSource != LocationSourceExplicit {
		t.Errorf("location_source = %q, want %q", resp.LocationSource, LocationSourceExplicit)
	}
	if resp.Location == nil || resp.Location.Lat != 6.5244 || resp.Location.Lng != 3.3792 {
		t.Errorf("returned location = %+v, want the supplied coordinates", resp.Location)
	}
	// The coordinates must actually reach the query, not merely be echoed back.
	if repo.gotLat != 6.5244 || repo.gotLng != 3.3792 {
		t.Errorf("nearby-events query used (%v, %v), want the supplied coordinates", repo.gotLat, repo.gotLng)
	}
}
