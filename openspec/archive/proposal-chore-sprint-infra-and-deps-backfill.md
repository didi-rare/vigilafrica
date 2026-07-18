---
id: chore-sprint-infra-and-deps-backfill
status: proposed
branch: chore/openspec-sprint-backfill
---

# Record: Sprint Infra + Dependency Backfill (chore-sprint-infra-and-deps-backfill)

> **Retroactive / as-built governance record.** This documents four changes that
> already shipped during the 2026-05-29 → 2026-06-05 partnership-readiness sprint
> without an OpenSpec record. It proposes no new work — it exists so the spec
> history matches the git history. None of the four was individually
> Sentinel-eligible (no `api/internal/**` or `api/cmd/**` source diff), which is
> why they are consolidated here rather than each carrying its own record.

## Why

During the partnership-readiness sprint a cluster of small dependency,
infrastructure, and tooling changes shipped directly through
`development → main` without an OpenSpec record. Individually each one is a
routine maintenance change — a dev-only test runner, a toolchain patch bump, a
security dependency bump, a one-line healthcheck fix — and none of them touched
product behaviour or the API surface. But the project's governance posture is
that every merged change should be traceable to a spec, and four merges currently
are not.

This is a single **as-built governance backfill**: it records what shipped and
why, after the fact, so the spec history matches the git history. It exists
purely for traceability, audit, and so future readers (and any partner/grant
due-diligence pass) can reconstruct the reasoning behind these merges from the
spec tree rather than from commit archaeology.

None of the four changes individually required a Sentinel change-record: none
carried an `api/internal/**` or `api/cmd/**` source diff, and the parts that
touch the API repo are build-only (CI workflow YAML, Dockerfile base-image
digest) or dev-only (a PowerShell test runner). They are consolidated here
precisely because no single one of them met the bar for its own proposal.

## What Changed

Four merged changes, in ship order:

### 1. PR #107 — Docker-based Go test runner (AppLocker workaround)

Commit `c9def2f`, merged via #107 (2026-06-03).

- Adds `scripts/test-api.ps1` (new file) — runs the Go suite inside the
  digest-pinned Linux Go container instead of as native host binaries.
- **Why:** on the Windows dev host, Application Control (AppLocker) intermittently
  blocks natively-compiled `go test` binaries; the `GOTMPDIR` mitigation is
  unreliable. Running the binaries inside the container sidesteps the policy
  entirely.
- Runner behaviour: unit tests by default; `-Integration` adds `-tags=integration`
  and mounts the Docker socket so `testcontainers-go` can start sibling Postgres
  containers; reads the digest-pinned `GO_IMAGE` from `api/Dockerfile` so the
  runner never drifts from the build image; persists build/module caches in named
  volumes; forwards extra args.
- **Scope:** one new dev-only script. No `api/internal/**` or `api/cmd/**` diff.
  Not Sentinel-eligible.

### 2. PR #108 — Go toolchain bump to 1.26.4 (June 2026 stdlib CVE batch)

Commits `c40c249` + `c81f1b5`, merged via #108 (2026-06-03).

- `.github/workflows/ci-cd.yml` and `.github/workflows/openspec-verify.yml`:
  `setup-go` `go-version` `'1.26'` → `'1.26.4'` (exact patch pin; still satisfies
  `api/go.mod` `go 1.26`).
- `api/Dockerfile`: `GO_IMAGE` digest bumped to a go1.26.4 image, so the
  production build and the local Docker test runner get the same patched toolchain
  CI uses.
- `docs/standards/developers-go.md`: adds §10.11 codifying the lesson —
  `setup-go` `go-version` (every workflow) and `api/Dockerfile` `GO_IMAGE` must
  resolve to the same `go1.X.Y`, or the prod build silently drifts behind CI; a
  govulncheck hit on a stdlib advisory means bump the toolchain in both places,
  not suppress; confirm the release via go.dev/dl, not a single registry probe
  (decision-log #10).
- **Why:** the Go vuln DB picked up a batch of stdlib advisories and govulncheck
  started failing on every PR. go1.26.4 fixes them (incl. html/template XSS
  relevant to the digest + alert emails). The build was also silently behind a
  few patches.
- **Verified:** in the go1.26.4 container, `govulncheck ./...` reports 0 called
  vulnerabilities and `go test ./...` passes. Unblocked the govulncheck gate that
  was failing #106 and #107.
- **Scope:** CI workflow YAML, a Dockerfile base-image digest, and a docs
  standard. Build-only and docs-only — no `api/internal/**` or `api/cmd/**` source
  diff. Not Sentinel-eligible.

### 3. PR #110 — `react-router-dom` 7.17.0 security bump

Commit `b5aa7fa`, merged via #110 (2026-06-05).

- `web/package.json`: `react-router-dom` → `^7.17.0`; `web/package-lock.json`
  updated accordingly.
- **Why:** `npm audit` flagged two high-severity advisories in `react-router`
  7.0.0–7.14.2 (pulled via `react-router-dom ^7.14.1`): GHSA-49rj-9fvp-4h2h
  (vendored `turbo-stream` arbitrary constructor invocation → unauth RCE) and
  GHSA-8x6r-g9mw-2r78 (DoS via unbounded path expansion in `__manifest`). Both
  fixed in 7.15.0+; bumped to `^7.17.0` (latest 7.x, semver-minor).
- **Verified:** `npm audit` clean; `tsc` build + vitest (37/37) green.
- **Scope:** frontend dependency manifest + lockfile only. No API diff. Not
  Sentinel-eligible.

### 4. PR #111 — Umami healthcheck probes IPv4 (127.0.0.1)

Commit `ddb40fd`, merged via #111 (2026-06-05).

- `docker-compose.yml`, `docker-compose.staging.yml`, `docker-compose.prod.yml`:
  the `umami` service healthcheck probe URL `http://localhost:3000/api/heartbeat`
  → `http://127.0.0.1:3000/api/heartbeat` (all three files), with an explanatory
  comment.
- **Why:** in-container `localhost` resolves to IPv6 `::1`, but Umami (Next.js
  standalone) binds IPv4 `0.0.0.0` only, so the probe got `connection refused`
  and Docker reported a false `unhealthy` on a fully working container.
- **Impact today:** cosmetic — plain Compose does not act on health status and
  nothing `depends_on` umami being healthy. The fix removes the misleading status
  and prevents the same false-unhealthy on prod.
- **Scope:** three Compose files, one URL each. No application source diff. Not
  Sentinel-eligible.

## Sentinel Determination

For completeness, the explicit reason each change is recorded here (in a
consolidated chore backfill) rather than as its own Sentinel change-record:

| PR | Files touched | `api/internal/**` or `api/cmd/**` diff? | Classification |
| --- | --- | --- | --- |
| #107 | `scripts/test-api.ps1` (new) | No | Dev tooling only |
| #108 | 2 CI workflows, `api/Dockerfile` (digest), `docs/standards/developers-go.md` | No | Build-only + docs |
| #110 | `web/package.json`, `web/package-lock.json` | No | Frontend dep bump |
| #111 | 3 `docker-compose*.yml` healthchecks | No | Infra config one-liner |

No merge carried a Go application-source diff; the API-repo changes are confined
to build config (CI YAML, Dockerfile base-image digest) and a docs standard. The
frontend change is a dependency bump with no source edit. The infra change is
Compose config only. Therefore none crossed the Sentinel threshold individually,
and this consolidated chore is the correct home for the governance record.

## Out of Scope

- **Re-litigating any of the four decisions.** This record documents what
  shipped; it does not propose reverting, re-bumping, or re-architecting anything.
- **Backfilling records for changes that already have a proposal** (e.g. the
  analytics/Umami introduction itself is covered by `chore-analytics-and-feedback`).
  This record covers only the four uncovered infra/dep/tooling merges.
- **A general retroactive sweep of the full git history.** Scoped strictly to the
  2026-05-29 → 2026-06-05 sprint cluster named above.
- **Adding new CI gates or Sentinel automation** to catch future uncovered merges.
  A reasonable follow-up (a `web/`-aware governance gate was flagged in the review
  that produced this record), but not bundled here.

## Capabilities

This record introduces no new product capability and modifies no existing one. It
documents maintenance changes to:

- `dev-tooling`: a containerised Go test runner for the Windows dev host.
- `build-toolchain`: the pinned Go toolchain version across CI and the API
  Dockerfile.
- `frontend-dependencies`: the `react-router-dom` version.
- `infra-compose`: the Umami container healthcheck probe.

## Acceptance Criteria

This is an as-built backfill; "acceptance" means the record faithfully matches
what merged. Verifiable against git:

- [x] `scripts/test-api.ps1` exists on `main` and matches commit `c9def2f`.
- [x] `setup-go` `go-version` is `'1.26.4'` in both `.github/workflows/ci-cd.yml`
      and `.github/workflows/openspec-verify.yml`; `api/Dockerfile` `GO_IMAGE`
      resolves to go1.26.4; `docs/standards/developers-go.md` §10.11 present — all
      per `c40c249` + `c81f1b5`.
- [x] `web/package.json` pins `react-router-dom` at `^7.17.0` with a matching
      `web/package-lock.json`, per `b5aa7fa`.
- [x] All three `docker-compose*.yml` umami healthchecks probe
      `http://127.0.0.1:3000/api/heartbeat`, per `ddb40fd`.
- [x] No recorded change carries an `api/internal/**` or `api/cmd/**` source diff
      (Sentinel-not-required determination holds).

## Origin

Surfaced 2026-06-05 while reconciling the spec tree against the sprint git
history: four merges (#107, #108, #110, #111) shipped during the
partnership-readiness sprint without an OpenSpec record. Consolidated into one
chore-class backfill because each is individually below the bar for its own
proposal and none is Sentinel-eligible. Authored as a governance/traceability
record only — no behavioural change.
