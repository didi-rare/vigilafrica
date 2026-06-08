package main

import "testing"

func TestIsGovernanceRecord(t *testing.T) {
	cases := []struct {
		name string
		path string
		want bool
	}{
		{"flat proposal", "openspec/proposals/feat-for-partners-page.md", true},
		{"nested change record", "openspec/changes/feature-impact-categories/proposal.md", true},
		{"archived change record", "openspec/changes/archive/governance-sentinel/design.md", false},
		{"archived proposal tree", "openspec/archive/proposal-chore-css-tokens.md", false},
		{"unrelated spec file", "openspec/specs/vigilafrica/decisions.md", false},
		{"critical source file", "api/internal/handlers/events.go", false},
		{"web source file", "web/src/pages/ForPartners.tsx", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isGovernanceRecord(c.path); got != c.want {
				t.Errorf("isGovernanceRecord(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}

func TestIsCritical(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"api/internal/database/queries.go", true},
		{"api/cmd/sentinel/main.go", true},
		{"web/src/pages/ForPartners.tsx", true},
		{"web/index.html", false},
		{"docs/standards/developers-go.md", false},
		{"docker-compose.yml", false},
		{"api/db/migrations/001_init.sql", false}, // not a critical prefix; also allow-listed defensively
	}
	for _, c := range cases {
		if got := isCritical(c.path); got != c.want {
			t.Errorf("isCritical(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestIsAllowed(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"api/db/migrations/001_init.sql", true},
		{"api/internal/handlers/events.go", false},
		{"web/src/pages/ForPartners.tsx", false},
	}
	for _, c := range cases {
		if got := isAllowed(c.path); got != c.want {
			t.Errorf("isAllowed(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// A migration file is exempt from the gate: it does not match a critical prefix
// (so it never requires a record), and it is also on the allow-list as a
// defensive belt-and-suspenders should the critical-path set ever broaden.
func TestMigrationIsExempt(t *testing.T) {
	const path = "api/db/migrations/001_init.sql"
	if isCritical(path) {
		t.Errorf("migration %q should not match a critical prefix", path)
	}
	if !isAllowed(path) {
		t.Errorf("migration %q should be allow-listed", path)
	}
}
