package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestStatesCountryFilter covers B9 (states handler mirrors B1-B8) from the
// fix-api-country-filter spec. Reuses listEventsTestRepo from events_test.go
// because it satisfies the full database.Repository interface and captures
// the country arg passed into GetDistinctStatesByCountry.
func TestStatesCountryFilter(t *testing.T) {
	tests := []struct {
		name              string
		query             string
		wantStatus        int
		wantStatesCountry string // expected arg into the repository
		wantBody          string // substring expected on 400
	}{
		{name: "canonical name", query: "country=Nigeria", wantStatus: http.StatusOK, wantStatesCountry: "Nigeria"},
		{name: "lowercase name", query: "country=nigeria", wantStatus: http.StatusOK, wantStatesCountry: "Nigeria"},
		{name: "ISO code", query: "country_code=NG", wantStatus: http.StatusOK, wantStatesCountry: "Nigeria"},
		{name: "lowercase code", query: "country_code=ng", wantStatus: http.StatusOK, wantStatesCountry: "Nigeria"},
		{name: "both — code wins", query: "country=Ghana&country_code=NG", wantStatus: http.StatusOK, wantStatesCountry: "Nigeria"},
		{name: "unknown code", query: "country_code=XX", wantStatus: http.StatusBadRequest, wantBody: "unknown country"},
		{name: "unknown name", query: "country=Atlantis", wantStatus: http.StatusBadRequest, wantBody: "unknown country"},
		{name: "no params", query: "", wantStatus: http.StatusOK, wantStatesCountry: ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			repo := &listEventsTestRepo{}
			handler := StatesHandler(repo)
			req := httptest.NewRequest(http.MethodGet, "/v1/states?"+tt.query, nil)
			rec := httptest.NewRecorder()

			handler(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantStatus == http.StatusBadRequest {
				if !strings.Contains(rec.Body.String(), tt.wantBody) {
					t.Errorf("body = %q, want substring %q", rec.Body.String(), tt.wantBody)
				}
				if repo.statesCalled {
					t.Error("expected repository not to be called when input is rejected")
				}
				return
			}
			if !repo.statesCalled {
				t.Fatal("expected repository to be called on a 200 path")
			}
			if repo.lastStatesCountry != tt.wantStatesCountry {
				t.Errorf("repo received country=%q, want %q", repo.lastStatesCountry, tt.wantStatesCountry)
			}
			// Sanity: 200 responses must be valid JSON with a "states" key.
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("response body is not valid JSON: %v (body: %s)", err, rec.Body.String())
			}
			if _, ok := body["states"]; !ok {
				t.Errorf("response body missing 'states' key: %s", rec.Body.String())
			}
		})
	}
}
