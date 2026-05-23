package handlers

import (
	"errors"
	"net/url"
	"testing"
)

func TestResolveCountry(t *testing.T) {
	tests := []struct {
		name         string
		country      string
		countryCode  string
		wantCanonical string
		wantPresent  bool
		wantErr      bool
	}{
		{name: "no params", wantCanonical: "", wantPresent: false},
		{name: "canonical name", country: "Nigeria", wantCanonical: "Nigeria", wantPresent: true},
		{name: "lowercase name", country: "nigeria", wantCanonical: "Nigeria", wantPresent: true},
		{name: "ghana name", country: "Ghana", wantCanonical: "Ghana", wantPresent: true},
		{name: "uppercase code", countryCode: "NG", wantCanonical: "Nigeria", wantPresent: true},
		{name: "lowercase code", countryCode: "ng", wantCanonical: "Nigeria", wantPresent: true},
		{name: "ghana code", countryCode: "GH", wantCanonical: "Ghana", wantPresent: true},
		{name: "both — code wins", country: "Ghana", countryCode: "NG", wantCanonical: "Nigeria", wantPresent: true},
		{name: "unknown code", countryCode: "XX", wantErr: true},
		{name: "unknown name", country: "Atlantis", wantErr: true},
		{name: "partial name (was previously empty)", country: "Nig", wantErr: true},
		{name: "whitespace-only treated as unset", country: "   ", wantCanonical: "", wantPresent: false},
		{name: "padded name", country: "  Nigeria  ", wantCanonical: "Nigeria", wantPresent: true},
		{name: "padded code", countryCode: "  NG  ", wantCanonical: "Nigeria", wantPresent: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			q := url.Values{}
			if tt.country != "" {
				q.Set("country", tt.country)
			}
			if tt.countryCode != "" {
				q.Set("country_code", tt.countryCode)
			}

			canonical, present, err := resolveCountry(q)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got canonical=%q present=%v", canonical, present)
				}
				if !errors.Is(err, errUnknownCountry) {
					t.Errorf("expected errUnknownCountry, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if canonical != tt.wantCanonical {
				t.Errorf("canonical = %q, want %q", canonical, tt.wantCanonical)
			}
			if present != tt.wantPresent {
				t.Errorf("present = %v, want %v", present, tt.wantPresent)
			}
		})
	}
}
