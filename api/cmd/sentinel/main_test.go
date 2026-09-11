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

// TestTrivialLineRe is the regression test for the actual defect: the old
// implementation matched `[trivial]` as a bare substring anywhere in the
// commit range, so a commit that merely DISCUSSED the token disabled the
// audit. This never shipped with the literal token in its final wording —
// caught and reworded before merge — but the discussion case below is a
// faithful reconstruction of what the earlier, un-reworded message said, and
// it MUST NOT match.
func TestTrivialLineRe(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "real usage: standalone line at the end of the body",
			body: "fix(web): stop page content showing through the sticky nav\n\n" +
				"The nav is position: sticky over a translucent background...\n\n" +
				"[trivial]\n",
			want: true,
		},
		{
			name: "standalone line with trailing whitespace only",
			body: "chore: bump a dependency\n\n[trivial]  \n",
			want: true,
		},
		{
			// The second real defect, found by independent review: an earlier
			// version of this regex allowed leading whitespace too, which matched
			// an INDENTED usage example -- e.g. a doc commit showing contributors
			// what the token looks like without intending to invoke it. Requiring
			// column zero closes this the same way requiring a whole line closed
			// the prose-mention case.
			name: "THE SECOND DEFECT: token indented as a quoted/example line",
			body: "docs: explain the bypass token\n\n" +
				"Example of what NOT to do:\n\n" +
				"    [trivial]\n\n" +
				"Do not paste that literally.\n",
			want: false,
		},
		{
			name: "case-insensitive, preserving old behaviour",
			body: "chore: bump a dependency\n\n[TRIVIAL]\n",
			want: true,
		},
		{
			// The actual defect. Reconstructed from the real earlier wording
			// of what became commit ae4b3ea (reworded before it landed) —
			// the token is named in the middle of a sentence explaining why
			// the bypass was NOT used, and the old substring match could not
			// tell the difference between that and an opt-out.
			name: "THE DEFECT: token discussed in prose, not on its own line",
			body: "docs(openspec): record the maplibre 6 migration\n\n" +
				"The sentinel gate correctly refused this PR: three files under\n" +
				"web/src changed with no governance record. Note on wording: an\n" +
				"earlier version of this message named the bypass token literally\n" +
				"([trivial]) while explaining why it was not used.\n",
			want: false,
		},
		{
			name: "token quoted inline with other text on the same line",
			body: "chore: tidy up\n\nRun with the `[trivial]` tag if this recurs.\n",
			want: false,
		},
		{
			name: "token as a substring of a longer bracketed word",
			body: "chore: tidy up\n\n[trivially] skip this one\n",
			want: false,
		},
		{
			name: "no mention at all",
			body: "fix(api): correct an off-by-one in pagination\n\nFixes #123.\n",
			want: false,
		},
		{
			name: "empty message",
			body: "",
			want: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := trivialLineRe.MatchString(c.body); got != c.want {
				t.Errorf("trivialLineRe.MatchString(%q) = %v, want %v", c.body, got, c.want)
			}
		})
	}
}

// TestParseAuditCommitRef is the regression test for the third real defect,
// found only by checking this PR's OWN live CI run against GitHub, not by
// local testing: on a pull_request trigger, actions/checkout resolves HEAD
// to GitHub's synthetic merge commit ("Merge <sha> into <sha>"), never the
// real commit a contributor wrote -- confirmed on PR #273 itself, whose real
// commit was 6e5e0de and whose checked-out HEAD subject was literally
// "Merge 6e5e0de951574603cf687608e32640f7211804ea into
// 7018b11f12695eb24810ba3d09243604dc202b28". Reading HEAD directly there
// would make a real, deliberately-placed bypass token invisible in every
// real PR run.
func TestParseAuditCommitRef(t *testing.T) {
	cases := []struct {
		name  string
		input string // raw "%P\x00%s" git log output
		want  string
	}{
		{
			// The exact shape observed live on PR #273.
			name: "THE DEFECT: real GitHub synthetic PR merge commit",
			input: "7018b11f12695eb24810ba3d09243604dc202b28 6e5e0de951574603cf687608e32640f7211804ea\x00" +
				"Merge 6e5e0de951574603cf687608e32640f7211804ea into 7018b11f12695eb24810ba3d09243604dc202b28\n",
			want: "6e5e0de951574603cf687608e32640f7211804ea",
		},
		{
			name:  "ordinary single-parent commit (a direct push, or any normal commit)",
			input: "abc1234\x00fix(api): correct an off-by-one\n",
			want:  "HEAD",
		},
		{
			// A real, human-authored merge (e.g. this repo's own promotion PRs)
			// has two parents but not the auto-generated subject -- must NOT be
			// redirected.
			name:  "real two-parent merge, human-authored subject",
			input: "aaa1111 bbb2222\x00Merge pull request #270 from didi-rare/development\n",
			want:  "HEAD",
		},
		{
			// Three or more parents (an octopus merge) is not the two-parent
			// shape GitHub's PR merge ref uses -- must NOT be redirected even if
			// the subject happens to resemble the pattern.
			name:  "three parents, not the PR-merge shape",
			input: "aaa1111 bbb2222 ccc3333\x00Merge bbb2222 into aaa1111\n",
			want:  "HEAD",
		},
		{
			name:  "malformed git output (no NUL separator)",
			input: "not what we expected",
			want:  "HEAD",
		},
		{
			name:  "empty output",
			input: "",
			want:  "HEAD",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseAuditCommitRef(c.input); got != c.want {
				t.Errorf("parseAuditCommitRef(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

func TestEscapeWorkflowCommandValue(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"plain text", "plain text"},
		{"100% sure", "100%25 sure"},
		{"line1\nline2", "line1%0Aline2"},
		{"line1\r\nline2", "line1%0D%0Aline2"},
	}
	for _, c := range cases {
		if got := escapeWorkflowCommandValue(c.in); got != c.want {
			t.Errorf("escapeWorkflowCommandValue(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
