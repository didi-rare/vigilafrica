package ingestor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestParseGDACSReferenceRejectsForeignHosts is the SSRF guard.
//
// The source URL arrives inside upstream data, so it is attacker-influenced in
// the same sense any third-party feed is. We never fetch it; we extract two
// validated components and rebuild the request against a constant host. These
// cases pin that: anything not on gdacs.org must be refused outright.
func TestParseGDACSReferenceRejectsForeignHosts(t *testing.T) {
	rejected := []string{
		"https://evil.example.com/report.aspx?eventtype=FL&eventid=1104078",
		"https://gdacs.org.evil.example.com/report.aspx?eventtype=FL&eventid=1104078",
		"http://www.gdacs.org/report.aspx?eventtype=FL&eventid=1104078", // not https
		"https://www.gdacs.org/report.aspx?eventtype=FL",                // no id
		"https://www.gdacs.org/report.aspx?eventid=1104078",             // no type
		"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=abc",    // non-numeric id
		"https://www.gdacs.org/report.aspx?eventtype=FLOOD&eventid=1",   // type not 2 letters
		"https://www.gdacs.org/report.aspx?eventtype=../&eventid=1",
		"",
		"not a url at all",
	}
	for _, u := range rejected {
		if _, ok := parseGDACSReference(u); ok {
			t.Errorf("expected %q to be rejected", u)
		}
	}
}

func TestParseGDACSReferenceAcceptsRealSourceURLs(t *testing.T) {
	// Exactly the form stored on the affected production rows.
	cases := map[string]gdacsReference{
		"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1104078": {EventType: "FL", EventID: "1104078"},
		"https://gdacs.org/report.aspx?eventtype=EQ&eventid=42":          {EventType: "EQ", EventID: "42"},
		"https://www.gdacs.org/report.aspx?eventtype=fl&eventid=1104105": {EventType: "FL", EventID: "1104105"},
	}
	for u, want := range cases {
		got, ok := parseGDACSReference(u)
		if !ok {
			t.Fatalf("expected %q to parse", u)
		}
		if got != want {
			t.Errorf("%q -> %+v, want %+v", u, got, want)
		}
	}
}

func TestResolveGDACSCentroid(t *testing.T) {
	t.Run("returns the authoritative centroid", func(t *testing.T) {
		// The real payload shape for GDACS 1104078 — the flood that EONET placed
		// in Kwara. GDACS puts it at lon 9.414, lat 4.6027, in Cameroon.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("eventid") != "1104078" {
				t.Errorf("unexpected eventid %q", r.URL.Query().Get("eventid"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"geometry":{"type":"Point","coordinates":[9.414,4.6027]},"properties":{"country":"Cameroon","iso3":"CMR"}}`))
		}))
		defer srv.Close()
		defer swapGDACSURL(srv.URL)()

		lon, lat, ok := resolveGDACSCentroid(context.Background(),
			"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1104078")
		if !ok {
			t.Fatal("expected resolution to succeed")
		}
		if lon != 9.414 || lat != 4.6027 {
			t.Errorf("got lon=%v lat=%v, want 9.414 / 4.6027", lon, lat)
		}
		// The point of the whole change: this must NOT be the transposed value
		// that placed the event in Nigeria.
		if lon == 4.6027 {
			t.Error("returned the transposed coordinates")
		}
	})

	t.Run("refuses a transposed payload rather than trusting a second upstream", func(t *testing.T) {
		// If GDACS itself ever served an impossible latitude we must not accept it.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"geometry":{"type":"Point","coordinates":[36.5,136.9]}}`))
		}))
		defer srv.Close()
		defer swapGDACSURL(srv.URL)()

		if _, _, ok := resolveGDACSCentroid(context.Background(),
			"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1"); ok {
			t.Error("expected latitude 136.9 to be refused")
		}
	})

	t.Run("fails closed on transport and shape problems", func(t *testing.T) {
		for name, handler := range map[string]http.HandlerFunc{
			"500":            func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) },
			"not json":       func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<html>")) },
			"no geometry":    func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"properties":{}}`)) },
			"not a point":    func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"geometry":{"type":"Polygon","coordinates":[]}}`)) },
			"short coords":   func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"geometry":{"type":"Point","coordinates":[1]}}`)) },
			"empty response": func(w http.ResponseWriter, _ *http.Request) {},
		} {
			t.Run(name, func(t *testing.T) {
				srv := httptest.NewServer(handler)
				defer srv.Close()
				defer swapGDACSURL(srv.URL)()

				if _, _, ok := resolveGDACSCentroid(context.Background(),
					"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=1"); ok {
					t.Errorf("%s: expected failure, got success", name)
				}
			})
		}
	})

	t.Run("a non-GDACS source URL never triggers a request", func(t *testing.T) {
		called := false
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
		defer srv.Close()
		defer swapGDACSURL(srv.URL)()

		if _, _, ok := resolveGDACSCentroid(context.Background(), "https://evil.example.com/x?eventtype=FL&eventid=1"); ok {
			t.Error("expected rejection")
		}
		if called {
			t.Error("a foreign source URL must not reach the network at all")
		}
	})
}

// swapGDACSURL points the resolver at a test server and returns a restore func.
func swapGDACSURL(u string) func() {
	prev := gdacsEventDataURL
	gdacsEventDataURL = u
	return func() { gdacsEventDataURL = prev }
}
