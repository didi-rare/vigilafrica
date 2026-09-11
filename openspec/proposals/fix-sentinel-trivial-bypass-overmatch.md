---
id: fix-sentinel-trivial-bypass-overmatch
status: proposed
branch: tbd
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

## Verification

- [ ] Unit tests in `api/cmd/sentinel/main_test.go` (which already covers `isGovernanceRecord`,
      `isCritical`, `isAllowed`, `migrationIsExempt`) extended with the discussion case: a commit
      whose body contains the token in prose must **NOT** bypass.
- [ ] ⚠️ **Prove the gate by re-breaking what it guards.** Construct a branch with a critical file
      change and no record, confirm it FAILS; add a prose mention of the token, confirm it **still
      fails**; add the token as an intentional opt-out, confirm it passes. A test that only asserts
      the happy path would not have caught this defect.
- [ ] Run the real binary, not just the unit tests. The container invocation used to find this:

      docker run --rm -v <repo>:/repo -w /repo/api golang:1.26-alpine \
        sh -c "apk add --no-cache git; git config --global --add safe.directory /repo; go run ./cmd/sentinel"

      ⚠️ On Windows this needs `MSYS_NO_PATHCONV=1`, and a linked git worktree cannot be mounted
      directly — its `.git` is a pointer file, so clone to a standalone repo first.

## Impact

- **Affected:** `api/cmd/sentinel/main.go`, `api/cmd/sentinel/main_test.go`.
- **Blast radius:** the gate itself. A mistake here either blocks every PR or silently permits
  ungoverned changes, so the re-breaking step above is not optional.
- **Not urgent, but not cosmetic.** Nothing is currently ungoverned as a result — #266 was corrected
  once noticed — but the control cannot be relied on until this lands, and its failure mode is
  invisible.
- **Out of scope:** whether the bypass should exist at all, and whether `web/src/` is the right
  critical-path set.
