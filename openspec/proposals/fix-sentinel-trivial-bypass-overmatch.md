---
id: fix-sentinel-trivial-bypass-overmatch
status: in-progress
branch: fix/sentinel-trivial-bypass-overmatch
---

# Proposal: The Sentinel's Bypass Token Matches Anywhere, So Discussing It Disables the Gate (fix-sentinel-trivial-bypass-overmatch)

## Why

The sentinel is this repository's governance control (ADR-010): a PR touching `api/internal/`,
`api/cmd/` or `web/src/` must carry an OpenSpec record, or opt out with a bypass token in a commit
message. See [[reference_sentinel_gate_active]].

⚠️ **The bypass is matched as a bare substring over the entire commit range, so a commit that merely
MENTIONS the token disables the audit.**

```go
// api/cmd/sentinel/main.go
const trivialFlag = "[trivial]"

func checkTrivial() (bool, error) {
	cmd := exec.Command("git", "log", baseBranch+"..HEAD", "--format=%B")
	...
	return strings.Contains(strings.ToLower(out.String()), trivialFlag), nil
}
```

`run()` then short-circuits before looking at any file:

```
ℹ️  Trivial bypass detected in commit history. Skipping deep audit.
✅ Sentinel Audit Passed: Governance requirements satisfied.
```

## How it was found — accidentally, which is the point

On 2026-09-09, PR #266 (`fix/maplibre-xss-advisory`) legitimately failed the gate: three files under
`web/src/` with no governance record. A proposal was written rather than taking the bypass, and the
commit message explained **why the bypass was inappropriate** — naming the token to do so.

That sentence triggered the bypass. The gate reported **PASS** and never evaluated the proposal it
had just demanded. Reproduced locally by running the real binary in the Go container against a
standalone clone:

| commit message | sentinel output |
| --- | --- |
| names the token while rejecting it | `Trivial bypass detected — skipping deep audit` → PASS |
| same change, token not named | `3 critical code changes, 1 governance records` → PASS |

Both are green, but only the second actually audited anything. **The failure is silent and fails
open** — the operator sees a tick either way.

⚠️ This was caught only because the binary was run locally out of curiosity about an unrelated
AppLocker problem. Nothing in CI distinguishes the two passes: the bypass line is informational, not
a warning, and no one reads a green job's log.

## Who this affects

Any commit message that discusses the mechanism, including:

- a doc change explaining the governance workflow to contributors;
- a review note arguing that a change is or is not minor;
- **this proposal's own implementation PR**, which will need to name the token to fix it;
- a revert, whose auto-generated message quotes the original.

It cannot be exploited by an outsider — it needs commit access — but that is not the risk. The risk
is a maintainer silently losing the control while believing they still have it, which is precisely
the class of failure [[reference_green_gates_prove_only_what_they_check]] records.

## What Changes

Tighten the match so intent is required, not mention. Options, cheapest first:

1. **Require the token on its own line** (`^\s*\[trivial\]\s*$` per commit, multiline). Simple, and
   discussion in prose no longer matches.
2. **Require it in the SUBJECT line only** — the first line of some commit in the range. Matches how
   [[reference_trivial_tag_placement]] already describes usage, and is the easiest rule to explain.
3. **Require it in the HEAD commit's subject**, so the opt-out is a deliberate, current act rather
   than something inherited from an old commit in a long-lived branch.

⚠️ Option 3 also closes a second, unverified hole worth checking during implementation: a
long-running branch that legitimately used the bypass early keeps it applied to **every later
commit**, including ones that add critical code afterwards. **UNVERIFIED** — plausible from the code,
not yet demonstrated.

Whichever is chosen, the bypass should be **reported loudly**: printing the commit that carried it
turns an invisible skip into a reviewable fact.

### Resolution: option 1 + the HEAD-only half of option 3, not option 2/3's subject-line restriction

Implemented as: the token must be the **entire content of some line** in **HEAD's own commit
message** (`^\s*\[trivial\]\s*$`, case-insensitive, multiline) — not merely present anywhere in the
`baseBranch..HEAD` range, and not inherited from an earlier commit further back on the branch.

This takes option 3's real security property (a deliberate, *current* act, closing the
branch-inheritance hole) without its subject-line restriction. Checked against actual repository
history before choosing: every real `[trivial]` commit found (`e61b202`, `119d047`) carries it as
the **last line of the body**, not the subject — `reference_trivial_tag_placement` documents "commit
message," never "subject line" specifically. Restricting to the subject would have broken the
established, working convention for no additional security benefit; restricting to *a line, in
HEAD's message* gets the same guarantee option 3 wanted while staying compatible with it.

The previously-unverified branch-inheritance hole is now **confirmed real** (not just plausible):
demonstrated below by re-breaking the fix with a constructed 3-commit branch.

### ⚠️ Round 2 — independent review (`gpt-5.6-sol`) found three more real defects

Self-review of round 1 called the fix complete. It was not. An adversarial pass — same standard as
[[project_codex_review_calibration]] — read the actual live CI run for this PR's own commit and
found the round-1 fix would not have worked in real CI at all:

1. **P0 — HEAD is not what round 1 assumed.** On a `pull_request` trigger, `actions/checkout`'s
   default (no `ref:` override, and none was added) checks out GitHub's *synthetic merge commit* —
   `refs/pull/<n>/merge` — as HEAD. That commit's message is auto-generated
   (`"Merge <sha> into <sha>"`), never the real message a contributor wrote. Round 1's `git log -1
   HEAD` would have read that generated message on every real PR, making a real, deliberately-placed
   token **invisible in every real run** — confirmed on this PR's own live CI: checkout resolved
   `refs/remotes/pull/273/merge`, subject literally `"Merge 6e5e0de… into 7018b11…"`. This would not
   have been a residual attack surface; it would have broken the escape hatch entirely, for everyone,
   silently, the first time anyone actually relied on it. **Fixed:** `resolveAuditCommit` detects the
   two-parent, auto-generated-subject shape and reads the second parent (the real PR tip) instead.
2. **P0 — indentation reopened the same class of hole.** Round 1's regex allowed `^\s*` before the
   token, so an indented documentation example — `"Example of what NOT to do:\n\n    [trivial]\n\n
   Don't paste that literally."` — matched, for the identical reason a prose mention matched the
   *original* bug: the line's content looked like an opt-out even though its context didn't mean one.
   **Fixed:** the token must start at column zero (`^\[trivial\]\s*$`) — still exactly how the real
   commits (`e61b202`, `119d047`) use it.
3. **P1 — reported to stdout only.** A passed check shows a green tick without opening the log,
   which is how the *original* bug stayed invisible for as long as it did. **Fixed:**
   `reportBypassLoudly` now also emits a `::warning::` GitHub annotation (visible on the PR without
   opening the run) and a job-summary line; both are no-ops outside Actions.

Two more findings were investigated and are recorded, not fixed:

- Go's `\s` is ASCII-only (rejects a pasted NBSP) — **not fixed**: this fails CLOSED (audit still
  runs), the safe direction, so it is a minor false-negative-on-the-opt-out annoyance, not a hole.
- **Known limitation, deliberately not fixed here:** the bypass is scoped to the reviewed commit's
  *message*, but the diff it excuses is the *whole PR*. On a multi-commit branch, an earlier commit
  can add an ungoverned critical file while a later, unrelated commit happens to carry `[trivial]` —
  confirmed real with a constructed 2-commit branch, and this repo's own history contains real
  non-squash merges of 3, 4, and 7 commits, so this is a reachable shape, not a hypothetical one.
  Properly closing it means auditing each commit's own diff against its own message — a materially
  bigger change than "match the token more precisely," and out of scope for this proposal by its own
  original framing. **Mitigated, not closed:** the bypass message now names the commit count when
  there's more than one, turning a silent gap into a visible prompt to check the rest by hand. A
  follow-up proposal is the right vehicle for the full fix, if it's judged worth the redesign.

Every fix was proved by re-running the real binary, not just re-reading the diff — see the updated
scenario table below.

## Verification

- [x] Unit tests in `api/cmd/sentinel/main_test.go`: `TestTrivialLineRe` (10 cases — the 8 from round
      1 plus the indentation regression), `TestParseAuditCommitRef` (6 cases, including the exact
      synthetic-merge shape observed live on PR #273), `TestEscapeWorkflowCommandValue`. All existing
      tests (`isGovernanceRecord`, `isCritical`, `isAllowed`, `migrationIsExempt`) still pass
      unmodified.
- [x] ⚠️ **Proved the gate by re-breaking what it guards**, using the real binary (not just unit
      tests) against a standalone clone with `origin/development` pointed at a base carrying the
      final fix. Seven constructed scenarios, each diffing a real `web/src/` change against that
      base — E and G are the round-2 regression proofs, built to model what round 1's testing missed:

      | scenario | shape | sentinel result |
      | --- | --- | --- |
      | A — no record, no token | plain commit | ❌ FAIL — correct |
      | B — no record, token named in prose | "Not using the `[trivial]` bypass here…" | ❌ FAIL — confirmed separately that the *original* bare-substring check returns `true` (wrongly bypasses) on this exact range |
      | C — no record, token as its own line, plain commit | real opt-out, no merge involved | ✅ PASS, reported |
      | D (sanity) — real governance record, no token | — | ✅ PASS via the normal path, unaffected |
      | **E — token as its own line, but checked out as GitHub's synthetic PR-merge HEAD** | models the real CI topology round 1 missed | ✅ PASS — resolves through the merge commit to the real PR tip and reports its actual SHA/subject, not the merge commit's |
      | **F — token indented as a quoted doc example** | the round-2 indentation hole | ❌ FAIL — correct |
      | **G — 2-commit branch: commit 1 adds an ungoverned critical file, commit 2 (unrelated) carries the token** | the known, unfixed limitation | ✅ PASS, but now says *"This PR carries 2 commits — the bypass covers the WHOLE diff, not just this commit; verify the others too."* |

      Container invocation used (`MSYS_NO_PATHCONV=1`, standalone clone since a linked worktree's
      `.git` is a pointer file, `core.longpaths=true` needed for this repo's deep archived paths on
      Windows):

      docker run --rm -v <repo>:/repo -w /repo/api golang:1.26-alpine \
        sh -c "apk add --no-cache git; git config --global --add safe.directory /repo; go run ./cmd/sentinel"

- [x] Ran the real binary against this PR's own actual commit (not a constructed scenario): reported
      `2 critical code changes, 1 governance records`, passed via the normal governance path — the
      updated `openspec/proposals/` record in this same diff, not the bypass.

## Impact

- **Affected:** `api/cmd/sentinel/main.go`, `api/cmd/sentinel/main_test.go`, plus `CONTRIBUTING.md`
  and `openspec/specs/vigilafrica/decisions.md` — both described the bypass's contract loosely
  ("commits containing `[trivial]` in the message"), which is exactly the ambiguity that caused the
  bug; updated to state the precise placement, HEAD-only, and multi-commit semantics.
- **Blast radius:** the gate itself. A mistake here either blocks every PR or silently permits
  ungoverned changes, so the re-breaking step above is not optional.
- **Not urgent, but not cosmetic.** Nothing is currently ungoverned as a result — #266 was corrected
  once noticed — but the control cannot be relied on until this lands, and its failure mode is
  invisible.
- **Out of scope:** whether the bypass should exist at all, and whether `web/src/` is the right
  critical-path set.
