# fix-sentinel-trivial-bypass-overmatch

**Branch:** `fix/sentinel-trivial-bypass-overmatch` (PR #273)
**Proposal:** [openspec/proposals/fix-sentinel-trivial-bypass-overmatch.md](openspec/proposals/fix-sentinel-trivial-bypass-overmatch.md)
**Origin:** merged PR #267 (docs-only); this PR implements it

## Round 1 — the proposal's own bug

- [x] `checkTrivial` examines only the commit under review, not the whole
      `baseBranch..HEAD` range
- [x] Token must be the entire content of a line (not a substring anywhere
      in a sentence)
- [x] Bypass reported with the commit's SHA and subject, not an anonymous line
- [x] Unit tests (`TestTrivialLineRe`) including the actual defect
      (reconstructed from the real pre-rewording draft of `ae4b3ea`)
- [x] Re-broke what the gate guards with the real binary against a
      standalone clone (4 scenarios)

## Round 2 — independent review (`gpt-5.6-sol`) found round 1 incomplete

Self-review called round 1 done. It was not — an adversarial pass reading
this PR's own live CI run found round 1 would not have worked in real CI.

- [x] **P0 fixed:** `actions/checkout`'s default behaviour on `pull_request`
      checks out GitHub's synthetic merge commit as HEAD, whose message is
      auto-generated ("Merge \<sha\> into \<sha\>") — confirmed live on this
      PR's own run. Round 1's bypass would have been invisible in every real
      PR. `resolveAuditCommit` now detects that shape and reads the real PR
      tip (the second parent) instead.
- [x] **P0 fixed:** round 1's regex allowed leading whitespace, so an
      indented documentation example matched — the same class of hole as
      the original bug, via indentation instead of prose. Token must now
      start at column zero.
- [x] **P1 fixed:** the bypass is now reported via a GitHub `::warning::`
      annotation and job summary line, not stdout only.
- [x] **Investigated, not fixed:** ASCII-only `\s` rejects NBSP — fails
      closed (safe direction), left as is.
- [x] **Known limitation, recorded not closed:** the bypass is scoped to one
      commit's message but excuses the whole PR diff — confirmed real
      (not hypothetical) with a constructed 2-commit branch, and this
      repo's own history has real 3/4/7-commit merges. A full fix needs
      per-commit-diff auditing, a bigger redesign, out of this proposal's
      scope. Mitigated: the bypass message now names the commit count when
      >1, so it's a visible prompt to check by hand instead of a silent gap.
- [x] Unit tests: `TestTrivialLineRe` extended (indentation case),
      `TestParseAuditCommitRef` (6 cases incl. the exact PR #273 shape),
      `TestEscapeWorkflowCommandValue`
- [x] Re-broke what the gate guards again, now covering what round 1
      missed: E (synthetic-merge HEAD → still resolves correctly), F
      (indentation → now fails), G (multi-commit gap → now visible)
- [x] Ran the real binary against this PR's own actual commit: 2 critical
      changes, 1 governance record, passes via the normal path

## Documentation

- [x] `CONTRIBUTING.md` and `openspec/specs/vigilafrica/decisions.md` both
      described the bypass loosely ("commits containing `[trivial]` in the
      message") — exactly the ambiguity that caused the bug. Updated to
      state the precise contract.
- [x] `openspec/proposals/fix-sentinel-trivial-bypass-overmatch.md`:
      status → `in-progress`, Resolution + round-2 sections recording what
      was chosen, why, and the full verification table

## Not done / deliberately out of scope

- Per-commit-diff auditing (the multi-commit limitation above) — recorded
  as a follow-up candidate, not attempted here
- Whether the bypass mechanism should exist at all — unchanged
- Whether `web/src/` is the right critical-path set — unchanged
