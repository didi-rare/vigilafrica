package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

const (
	baseBranch  = "origin/development"
	changeDir   = "openspec/changes/"
	proposalDir = "openspec/proposals/"
	trivialFlag = "[trivial]"
)

// trivialLineRe matches a commit-message line whose entire content (ignoring
// only trailing whitespace) is the bypass token, flush against the left
// margin. This is deliberately stricter than a substring search over the
// whole message: prose that discusses or quotes the token — "the bypass
// token is `[trivial]`" — does not match, only a line whose sole content is
// the token itself. Case-insensitive to preserve the old behaviour of
// matching "[TRIVIAL]"/"[Trivial]" too.
//
// No leading whitespace is allowed, on purpose: an EARLIER draft of this
// regex allowed `^\s*`, and a commit body containing an indented usage
// example — "Example of what NOT to do:\n\n    [trivial]\n\nDon't paste
// that literally." — matched it, for the same reason prose mentions matched
// the old bare-substring check: the line's CONTENT looked like an opt-out
// even though its CONTEXT didn't mean one. Requiring column zero is
// consistent with how the token is actually used in this repository's own
// history (e61b202, 119d047 both carry it unindented, alone, at the end of
// the commit body) and rejects the indented-example shape outright.
var trivialLineRe = regexp.MustCompile(`(?im)^` + regexp.QuoteMeta(trivialFlag) + `\s*$`)

// syntheticMergeSubjectRe matches GitHub's auto-generated subject for the
// synthetic merge commit actions/checkout resolves as HEAD on a
// `pull_request` trigger ("Merge <full sha> into <full sha>") — see
// resolveAuditCommit.
var syntheticMergeSubjectRe = regexp.MustCompile(`^Merge [0-9a-f]{7,40} into [0-9a-f]{7,40}$`)

var criticalPaths = []string{
	"api/internal/",
	"api/cmd/",
	"web/src/",
}

var allowList = []string{
	"api/db/migrations/",
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "\n🛡️  Sentinel Audit Failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("\n✅ Sentinel Audit Passed: Governance requirements satisfied.")
}

func run() error {
	fmt.Printf("🔍 Starting VigilAfrica Sentinel Audit against %s...\n", baseBranch)

	// 1. Check for a deliberate, current bypass on HEAD.
	isTrivial, sha, subject, err := checkTrivial()
	if err != nil {
		return fmt.Errorf("failed to check commit history: %w", err)
	}
	if isTrivial {
		// Reported loudly and attributed to a specific commit, not printed as
		// an anonymous line nobody reading a green job would notice. This
		// bypass is not itself audited — say so, so a reviewer knows to look
		// at the commit directly rather than trust the tick.
		//
		// Plain stdout is not enough on its own: a PASSED check on GitHub
		// shows a green tick without opening the log at all, which is
		// exactly how the original bug stayed invisible. reportBypassLoudly
		// also emits a `::warning::` workflow command (surfaces as an
		// annotation on the PR, visible without opening the run) and a job
		// summary line (visible on the run's own summary page) when the
		// corresponding GitHub Actions environment is present; both are
		// no-ops outside Actions, so this is unchanged when run locally.
		message := fmt.Sprintf("Trivial bypass invoked by %s (%q). Skipping deep audit — this opt-out is NOT independently verified.", sha, subject)
		// The bypass is scoped to THIS commit's message, but the diff it
		// excuses is the WHOLE PR (baseBranch...HEAD). On a multi-commit
		// branch, an earlier commit could add an ungoverned critical file
		// while a later, unrelated commit happens to carry the token — the
		// bypass then covers a change it never reviewed. Closing that
		// properly needs auditing each commit's own diff against its own
		// message, a larger change than this fix; naming the commit count
		// here is the cheap half: it turns a silent gap into a visible one a
		// reviewer can act on. See the proposal's "Known limitation" note.
		if n, cerr := commitCount(baseBranch, "HEAD"); cerr == nil && n > 1 {
			message += fmt.Sprintf(" This PR carries %d commits — the bypass covers the WHOLE diff, not just this commit; verify the others too.", n)
		}
		fmt.Printf("⚠️  %s\n", message)
		reportBypassLoudly(message)
		return nil
	}

	// 2. Get diff
	files, err := getDiffFiles()
	if err != nil {
		return fmt.Errorf("failed to get git diff: %w", err)
	}

	criticalChanges := []string{}
	governanceChanges := []string{}

	for _, file := range files {
		if isGovernanceRecord(file) {
			governanceChanges = append(governanceChanges, file)
			continue
		}

		if isCritical(file) && !isAllowed(file) {
			criticalChanges = append(criticalChanges, file)
		}
	}

	fmt.Printf("📊 Audit results: %d critical code changes, %d governance records.\n", len(criticalChanges), len(governanceChanges))

	if len(criticalChanges) > 0 && len(governanceChanges) == 0 {
		fmt.Println("\n❌ GHOST IMPLEMENTATION DETECTED")
		fmt.Println("You have modified critical source code without proposing an OpenSpec change.")
		fmt.Println("Modified files:")
		for _, f := range criticalChanges {
			fmt.Printf("  - %s\n", f)
		}
		fmt.Println("\n👉 SOLUTION: add an OpenSpec record for this change — a proposal under")
		fmt.Println("   'openspec/proposals/' or a change record under 'openspec/changes/' —")
		fmt.Println("   or add '[trivial]' to a commit message if this is a minor maintenance task.")
		return fmt.Errorf("governance violation")
	}

	return nil
}

// commitCount returns how many commits are in the range from..to, via
// `git rev-list --count`. Used only to make the bypass message name how
// many commits it covers when there is more than one -- see its call site.
func commitCount(from, to string) (int, error) {
	cmd := exec.Command("git", "rev-list", "--count", from+".."+to)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(out.String()))
	if err != nil {
		return 0, err
	}
	return n, nil
}

func getDiffFiles() ([]string, error) {
	// Triple dot diff finds changes in current branch since it diverged from base
	cmd := exec.Command("git", "diff", baseBranch+"...HEAD", "--name-only")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	
	lines := strings.Split(out.String(), "\n")
	var files []string
	for _, l := range lines {
		if t := strings.TrimSpace(l); t != "" {
			files = append(files, t)
		}
	}
	return files, nil
}

// checkTrivial reports whether the commit under review carries a
// deliberate, current bypass opt-out, and — when it does — the short SHA
// and subject line of the commit that carried it, so the caller can report
// the skip loudly instead of silently.
//
// Only that commit's own message is examined, not the whole
// `baseBranch..HEAD` range the old implementation scanned with a bare
// substring match. Two real holes followed from that: (1) a commit that
// merely DISCUSSED the token in prose — e.g. explaining why a bypass was
// NOT appropriate here, which is exactly what happened on PR #266 —
// silently disabled the audit, because `strings.Contains` cannot
// distinguish a mention from an opt-out; (2) on a long-lived branch, an
// early, legitimate opt-out kept applying to every later commit, including
// ones that added critical code afterwards, because the whole range was
// matched. Restricting to one commit closes both: the bypass must be
// re-declared on the commit currently under review, and it must be the
// entire content of one of its lines — see trivialLineRe.
//
// "The commit under review" is resolveAuditCommit's result, not always
// literal HEAD — see there for why.
func checkTrivial() (isTrivial bool, sha string, subject string, err error) {
	ref, rerr := resolveAuditCommit()
	if rerr != nil {
		// Same fail-safe posture as below: an unresolvable ref means no
		// bypass, not a free pass.
		return false, "", "", nil
	}

	cmd := exec.Command("git", "log", "-1", ref, "--format=%h%x00%s%x00%B")
	var out bytes.Buffer
	cmd.Stdout = &out
	if runErr := cmd.Run(); runErr != nil {
		// If the ref isn't resolvable (e.g. a shallow or malformed
		// checkout), fail SAFE: report no bypass rather than silently
		// granting one, so the deep audit still runs instead of being
		// skipped by accident.
		return false, "", "", nil
	}

	parts := strings.SplitN(out.String(), "\x00", 3)
	if len(parts) != 3 {
		return false, "", "", nil
	}
	sha, subject, body := parts[0], parts[1], parts[2]

	return trivialLineRe.MatchString(body), sha, subject, nil
}

// resolveAuditCommit returns the commit whose message should be inspected
// for the bypass token, as a git ref string.
//
// Found live on this repository's own PR #273, not by local testing: on a
// `pull_request` trigger, actions/checkout's default behaviour (no `ref:`
// override in openspec-verify.yml) checks out GitHub's synthetic merge
// commit — refs/pull/<n>/merge — as HEAD. That commit's own message is
// auto-generated ("Merge <full sha> into <full sha>"), never the real
// message a contributor wrote, so reading literal HEAD makes the bypass
// token invisible even when a real commit deliberately carries it on its
// own line. Confirmed on the real run for this PR: checkout resolved
// `refs/remotes/pull/273/merge`, whose message was literally
// "Merge 6e5e0de951574603cf687608e32640f7211804ea into
// 7018b11f12695eb24810ba3d09243604dc202b28".
//
// GitHub constructs that merge commit as base-tip merged with PR-head
// (first parent base, second parent head) — documented, stable behaviour
// countless Actions workflows depend on. When HEAD has exactly two parents
// and its own subject matches that auto-generated shape, the second parent
// — the actual PR branch tip — is used instead. A direct push to
// development (this workflow's other trigger) produces no such commit, so
// plain HEAD is used unchanged; a real, human-authored merge commit would
// not match the auto-generated subject pattern and also falls through to
// plain HEAD.
func resolveAuditCommit() (string, error) {
	cmd := exec.Command("git", "log", "-1", "HEAD", "--format=%P%x00%s")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return parseAuditCommitRef(out.String()), nil
}

// parseAuditCommitRef is resolveAuditCommit's pure logic, factored out so it
// can be unit-tested without shelling out to git: given the raw
// `%P\x00%s` output for HEAD, it returns the ref that should be examined
// for the bypass token.
func parseAuditCommitRef(gitLogOutput string) string {
	line := strings.TrimRight(gitLogOutput, "\n")
	parts := strings.SplitN(line, "\x00", 2)
	if len(parts) != 2 {
		return "HEAD"
	}
	parents := strings.Fields(parts[0])
	subject := parts[1]

	if len(parents) == 2 && syntheticMergeSubjectRe.MatchString(subject) {
		return parents[1]
	}
	return "HEAD"
}

// reportBypassLoudly surfaces a bypass in two places a green check does not
// otherwise reach: a `::warning::` workflow command, which GitHub renders as
// an annotation on the PR's Checks tab (visible without opening the run
// log), and a line appended to the job summary, visible on the run's own
// summary page. Both are GitHub Actions-specific and silently do nothing
// when their environment variable is absent (e.g. a local `go run`), so this
// changes nothing outside CI.
func reportBypassLoudly(message string) {
	fmt.Printf("::warning title=Sentinel bypass invoked::%s\n", escapeWorkflowCommandValue(message))

	summaryPath := os.Getenv("GITHUB_STEP_SUMMARY")
	if summaryPath == "" {
		return
	}
	f, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return // best-effort; the workflow command above already fired
	}
	defer f.Close()
	fmt.Fprintf(f, "\n### ⚠️ Sentinel bypass invoked\n\n%s\n", message)
}

// escapeWorkflowCommandValue percent-encodes the characters GitHub's
// workflow-command parser treats specially, per its documented escaping
// rules, so a commit subject containing them cannot corrupt or truncate the
// annotation.
func escapeWorkflowCommandValue(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "\r", "%0D")
	s = strings.ReplaceAll(s, "\n", "%0A")
	return s
}

// isGovernanceRecord reports whether a changed file is an active OpenSpec
// record that satisfies the gate. Both layouts are accepted: the flat
// `openspec/proposals/*.md` layout (used by the current sprint workflow) and
// the `openspec/changes/<id>/` layout. Archived records (any path containing
// `/archive/`) do not count — archiving is a past decision, not a record for
// the change under review.
func isGovernanceRecord(path string) bool {
	if strings.Contains(path, "/archive/") {
		return false
	}
	return strings.HasPrefix(path, proposalDir) || strings.HasPrefix(path, changeDir)
}

func isCritical(path string) bool {
	for _, p := range criticalPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func isAllowed(path string) bool {
	for _, p := range allowList {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}
