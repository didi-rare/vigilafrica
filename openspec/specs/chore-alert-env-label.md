---
id: chore-alert-env-label
status: proposed
branch: chore/alert-env-label
---

# Spec: Environment Label in Alert Email Subjects (chore-alert-env-label)

## Context

After the 2026-05-11 staging incident, the maintainer could not tell at a glance whether a Resend page was a staging or production alarm — both deployments share the prefix `[VigilAfrica]` in [api/internal/alert/resend.go:118](api/internal/alert/resend.go#L118) and [resend.go:148](api/internal/alert/resend.go#L148). Staging noise blended with production noise and eroded the signal value of paging.

The frontend already detects environment via `VITE_ENV=staging|production` (see [openspec/proposals/fix-staging-vite-env-flag.md](openspec/proposals/fix-staging-vite-env-flag.md) and [web/vite.config.ts:13-14](web/vite.config.ts#L13-L14)). This spec introduces the backend equivalent — `APP_ENV` — and wires it into alert subjects.

Companion: [openspec/proposals/chore-alert-env-label.md](openspec/proposals/chore-alert-env-label.md).

## Decision Log

| # | Decision | Alternatives | Why |
|---|---|---|---|
| D1 | Env var named `APP_ENV` | `ENVIRONMENT`, `DEPLOY_ENV`, reuse `VITE_ENV` | `APP_ENV` is the established backend convention (Laravel, Symfony, Rails) and mirrors `VITE_ENV` without colliding with Vite's reserved prefix. `ENVIRONMENT` is verbose; `DEPLOY_ENV` is uncommon. |
| D2 | Default to literal string `"unknown"` when `APP_ENV` is unset | Default to `"production"` (safer), default to `"local"`, fail to start | `"unknown"` makes misconfiguration *visible* in the next alert — defaulting to `"production"` would silently mask a staging deploy that forgot to set the var. Fail-to-start is too brittle for a side-channel field. |
| D3 | Prefix format `[VigilAfrica:<env>]` | `[STAGING] [VigilAfrica] ...`, `[VigilAfrica] [staging] ...` | Single bracket scans cleanly in an inbox, keeps the app name first (matches the existing convention), and the colon is a low-noise separator. |
| D4 | Lowercase env value in subject | Uppercase (`STAGING`) | Matches the input value (`staging`/`production` are already lowercase per `VITE_ENV` convention). No normalisation/branching needed. |
| D5 | Add `Environment` to `alert.Config`, not a package-level global | Read `os.Getenv` inside subject builders | Keeps the alert package free of env-var coupling; `loadAlertConfigFromEnv()` in main.go remains the single place that reads the environment. |

## Components to Touch

### Modified files

| File | Change |
|---|---|
| [api/internal/alert/resend.go](api/internal/alert/resend.go) | Add `Environment string` field to `Config`. In `NewClient`, normalise empty value to `"unknown"`. Update both `Sprintf` subject lines (resend.go:118, resend.go:148) to use the new prefix format. |
| [api/cmd/server/main.go](api/cmd/server/main.go) | In `loadAlertConfigFromEnv()` (line 136), populate `Environment` via `envOrDefault("APP_ENV", "unknown")`. |
| [api/internal/alert/resend_test.go](api/internal/alert/resend_test.go) | Update `TestClientSendIngestFailurePostsToResend` to set `Environment: "staging"` and assert `payload["subject"]` starts with `"[VigilAfrica:staging]"`. Add a new test `TestClientSubjectUsesEnvironmentPrefix` covering: (a) populated env yields `[VigilAfrica:<env>]`, (b) empty/missing env yields `[VigilAfrica:unknown]`. Cover both `SendIngestFailure` and `SendStalenessAlert`. |
| [.env.example](.env.example) | Add a new `APP_ENV` entry in the alerting section (around line 50) with comment explaining valid values (`local`, `staging`, `production`) and that it tags alert subjects. |
| [docker-compose.staging.yml](docker-compose.staging.yml) | Add `- APP_ENV=staging` to the API service `environment` block (around line 51, near `RESEND_API_KEY`). Hardcoded — not `${APP_ENV:-staging}` — to prevent a misconfigured `.env` from labelling staging alerts as production. |
| [docker-compose.prod.yml](docker-compose.prod.yml) | Add `- APP_ENV=production`. Hardcoded for the same reason. |
| [docker-compose.yml](docker-compose.yml) | Add `- APP_ENV=${APP_ENV:-local}` to the API service so local dev defaults to `local` but is overridable via `.env`. |
| [docker-compose.demo.yml](docker-compose.demo.yml) | Add `- APP_ENV=demo` if the demo compose file launches the API service (verify during execution; skip if it doesn't run the API). |
| [docs/deployment/vps.md](docs/deployment/vps.md), [docs/deployment/resend-setup.md](docs/deployment/resend-setup.md) | Append a line documenting `APP_ENV` alongside the other alert env vars. |
| [CONTRIBUTING.md](CONTRIBUTING.md) (line ~74) | Add `APP_ENV` to the local-dev env example block. |
| [openspec/specs/vigilafrica/decisions.md](openspec/specs/vigilafrica/decisions.md) (ADR-011, line 337) | Annotate the original subject line with a "superseded by chore-alert-env-label" amendment that references the new `[VigilAfrica:<env>] …` format. Edit-in-place would falsify the ADR's dated record; the amendment preserves history while pointing readers at current behaviour. |

### Deliberately untouched

- [docs/security/priority-fixes.md](docs/security/priority-fixes.md) "Already Verified During Audit" entries (lines 45-46) still quote the old `[VigilAfrica] Ingestion failed for NG at 2026-04-29T12:36:52Z` subject. Those rows describe the *literal inbox screenshots* captured during the audit on that date; editing them would falsify the audit log. They are documentation of past events, not living strings.

### Untouched

Frontend (`web/`), database, ingestion logic, watchdog config. The frontend's `VITE_ENV` and the backend's `APP_ENV` remain independent — they should agree per deploy but are not derived from each other.

## Behaviour Contract

- **B1** — When `APP_ENV=staging`, both alert subjects MUST begin with the literal prefix `[VigilAfrica:staging]` followed by a single space, e.g. `[VigilAfrica:staging] Ingestion failed for NG at 2026-05-11T10:00:00Z`.
- **B2** — When `APP_ENV=production`, the prefix MUST be `[VigilAfrica:production]`.
- **B3** — When `APP_ENV` is unset, empty after trimming, or the variable is missing entirely, the prefix MUST be `[VigilAfrica:unknown]` — never silently fall back to `production`.
- **B4** — `APP_ENV` MUST NOT be normalised, lowercased, or validated against an allow-list inside the alert package. Whatever value `loadAlertConfigFromEnv` resolves is the value that appears in the subject. (Out-of-band misuse like `APP_ENV=staging-eu` would render `[VigilAfrica:staging-eu]` — intentional.)
- **B5** — No other alert payload field (body, recipient list, From address) MAY be conditional on `APP_ENV`. The label is a subject-line nudge only; routing remains the responsibility of `ALERTS_TO`.
- **B6** — Setting `APP_ENV` MUST NOT change any non-alerting behaviour of the API. The variable exists solely for human consumption in alert subjects in this change.

## Phase 1 — Code Change

- [x] Add `Environment string` field to `alert.Config` in [api/internal/alert/resend.go](api/internal/alert/resend.go)
- [x] In `NewClient`, set `cfg.Environment = "unknown"` if `cfg.Environment == ""` (mirrors the existing defaulting pattern for `Endpoint` / `FromEmail`)
- [x] Replace both subject `Sprintf` calls to use `[VigilAfrica:%s]` with `c.cfg.Environment` as the first arg
- [x] In [api/cmd/server/main.go](api/cmd/server/main.go) `loadAlertConfigFromEnv()`, set `Environment: envOrDefault("APP_ENV", "unknown")`

## Phase 2 — Deploy Wiring

- [x] Add hardcoded `APP_ENV=staging` to [docker-compose.staging.yml](docker-compose.staging.yml)
- [x] Add hardcoded `APP_ENV=production` to [docker-compose.prod.yml](docker-compose.prod.yml)
- [x] Add overridable `APP_ENV=${APP_ENV:-local}` to [docker-compose.yml](docker-compose.yml)
- [x] Inspect [docker-compose.demo.yml](docker-compose.demo.yml); add `APP_ENV=demo` only if it launches the API service
- [x] Grep `deploy/` for any provisioning scripts that bake env at startup; thread `APP_ENV` through if found — `deploy/` only contains `provision.sh` and `Caddyfile.example`; neither bakes alert env vars, so no further changes needed

## Phase 3 — Tests + Docs

- [x] Update [api/internal/alert/resend_test.go:55](api/internal/alert/resend_test.go#L55) to assert the new prefix
- [x] Add `TestClientSubjectUsesEnvironmentPrefix` covering staging/production/empty cases for both subject builders (4 subtests including whitespace→unknown)
- [x] Update [.env.example](.env.example) with `APP_ENV` entry + comment
- [x] Update [docs/deployment/vps.md](docs/deployment/vps.md), [docs/deployment/resend-setup.md](docs/deployment/resend-setup.md), [CONTRIBUTING.md](CONTRIBUTING.md) to mention `APP_ENV`
- [ ] Manual smoke: temporarily lower `ALERT_STALENESS_THRESHOLD_HOURS` on staging, confirm received email subject reads `[VigilAfrica:staging]`, revert threshold *(deferred to deploy)*

## Acceptance Criteria

- [x] `go test ./api/internal/alert/...` passes with new and updated assertions (verified 2026-05-22; all 7 tests + 4 subtests green)
- [ ] On staging deploy, a forced staleness alert arrives with subject prefix `[VigilAfrica:staging]` *(verify post-deploy)*
- [ ] On production deploy, a real or forced staleness alert arrives with subject prefix `[VigilAfrica:production]` *(verify post-deploy)*
- [x] `git grep -nE "\[VigilAfrica\] (Ingestion|No successful)"` returns zero matches in `api/` source (no stale literal prefix in code). Remaining matches are: this proposal+spec pair (intentional — quotes old format for context), [decisions.md:337](openspec/specs/vigilafrica/decisions.md#L337) (now annotated with a supersession amendment pointing to this change), and [priority-fixes.md:45-46](docs/security/priority-fixes.md#L45-L46) (deliberately preserved audit log of screenshots taken on 2026-04-29).
- [x] `APP_ENV` appears in `.env.example` and is documented in the deployment guides
- [x] No other env-var consumer reads `APP_ENV` in this PR (the field is alert-only; broader use is a future change)

## Out of Scope (reaffirmed)

- Routing different environments to different recipient lists (`ALERTS_TO` already handles that per-deploy)
- Slack, SMS, or PagerDuty integration
- Severity levels in the subject (`[ERROR]`, `[WARN]`)
- Using `APP_ENV` for any non-alerting code path (feature flags, log labels, etc.) — a follow-up if/when needed
- Reconciling `APP_ENV` with `VITE_ENV` (they remain independent — the deploy is responsible for keeping them in sync)

## Risks

- **R1**: A future deploy forgets to set `APP_ENV` and alerts ship as `[VigilAfrica:unknown]`. **Mitigation**: hardcoding the value in `docker-compose.staging.yml` and `docker-compose.prod.yml` (rather than `${APP_ENV:-…}`) makes the env tag tied to the compose file, not to `.env`. The `unknown` fallback is the intentional visible signal if someone bypasses compose.
- **R2**: Email clients with thread-by-subject grouping (Gmail) may split previously-grouped alert threads when the prefix changes. **Mitigation**: accept it — the whole point is for staging and production to no longer thread together. Note in the PR description so the maintainer expects two new threads after deploy.
- **R3**: Subject longer than ~70 chars on mobile previews. **Mitigation**: the worst-case `[VigilAfrica:production] Ingestion failed for NG at 2026-05-11T10:00:00Z` is 73 chars — borderline but acceptable. No truncation logic needed.

## Verification Plan

1. Unit tests cover the three subject cases (staging / production / unknown) for both alert types
2. Local: `APP_ENV=staging docker compose -f docker-compose.staging.yml up api` → trigger a forced failure → confirm subject
3. Staging deploy: temporarily lower staleness threshold to force an alert; confirm subject; revert
4. Production deploy: no forced trigger — rely on the staging smoke + unit tests. First real production alert will validate.

No new automated CI changes required — existing Go test suite covers the new assertions.
