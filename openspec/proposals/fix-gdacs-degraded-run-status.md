---
id: fix-gdacs-degraded-run-status
status: in-progress
branch: fix/gdacs-degraded-run-status
---

# Proposal: A GDACS Outage Must Not Report as a Healthy Run (fix-gdacs-degraded-run-status)

## Why

Deferred as §8.1 of `fix-eonet-polygon-transposition`:

> when geometry resolution fails systematically the run still records `success`, `/health` still
> reports `ok`, and nothing pages. The counters and warnings exist only in logs, and this change has
> already demonstrated that **a counter nobody reads is not a control**.

That change made polygon events fail CLOSED: if the GDACS polygon cannot be verified, the event is
skipped (new) or metadata-only updated (existing) rather than stored with EONET's transposed
geometry. Correct — but the same mechanism means a GDACS outage silently removes floods from the
map while every observable signal says everything is fine. On an early-warning product, missing
floods with a green dashboard is the worst failure shape.

## The deferral overstated the cost — correcting the record

§8.1 deferred this as "a new subsystem — persisted per-run status, typed failure reasons, and alert
routing". Checked against the code on 2026-10-03, **two of the three already exist**:

- **Persisted per-run status** — `ingestion_runs.status` (migration `000004`), written by
  `CompleteIngestionRun` at the end of every run.
- **Alert routing** — `alert.Client.SendIngestFailure` emails operators via Resend; the scheduler
  already calls it on failed runs.
- **`/health` already has a `degraded` state**, and the public dashboard already renders a banner for
  it.

Only the typed failure reason is genuinely new. The job is to route a new run outcome through
existing plumbing, not to build plumbing.

## The design distinction that matters: refusal vs outage

`RunBudget.resolve` returns `ok=false` for two very different situations, and they must not be
conflated:

| failure | example | meaning | degraded? |
| --- | --- | --- | --- |
| **unverifiable** | GDACS answered; no single polygon matches (or two different ones do) | the fail-closed refusal working as designed | **no** |
| **unverifiable** | GDACS returns 404 for the event | GDACS no longer knows the event | **no** |
| **upstream** | transport error, timeout, 5xx/429, unparseable body | GDACS could not answer | **yes** |
| **upstream** | per-run request/time budget exhausted | we stopped asking before getting answers | **yes** |
| **upstream** | some episode fetches failed AND no candidate matched | "no match" cannot be claimed from a partial view | **yes** |

⚠️ Getting this wrong in either direction is a real failure. Treat refusals as outages and a single
genuinely unmatchable flood keeps the system `degraded` — and the public banner up — forever, which
trains everyone to ignore it. Treat outages as refusals and nothing changes from today.

## What Changes

1. **Typed resolver failure** (`api/internal/ingestor/gdacs.go`): `resolveGDACSPolygon` and
   `RunBudget.resolve` return a `resolveFailure` (`none` / `unverifiable` / `upstream`) instead of
   a bare bool, classified per the table above. The cache stores it too.
2. **New run status `degraded`** (`models`, migration `000016`): a run whose ingestion completed but
   in which ≥1 polygon event failed for an **upstream** reason. The error column carries a
   human-readable reason with the count.
3. **Staleness watchdog keeps working**: `GetLastSuccessfulIngestionRun` counts `degraded` as a
   completed run. ⚠️ Without this, a GDACS outage would ALSO fire a *false* "ingestion has stopped"
   staleness alert after its threshold — a wrong diagnosis on top of the right one.
4. **`/health`** reports `degraded` when any country's last run is `degraded` (as it already does for
   `failure`). **`/ready` still returns 503 only for `failure`**: an upstream data provider being
   down does not make this API unable to serve, and readiness must not conflate the two.
5. **Alerting on transition only**: the scheduler emails when a country's run becomes `degraded`
   and the previous completed run for that country was not. A GDACS outage spanning hours sends one
   email, not one per scheduler cycle. (Failures keep their existing per-run behaviour — unchanged.)
6. **Public dashboard**: `degraded` caused by GDACS shows an accurate message ("some flood areas
   could not be verified … may be missing") rather than the generic "ingestion did not complete",
   and the OpenAPI `status` enum and the web type gain `degraded`.

## Out of Scope

- Changing per-run alert behaviour for `failure` (no dedupe exists there today; separate concern).
- A single-episode-fetch failure when exactly one candidate still matched: resolution proceeds as
  today. A missing episode could in principle hide a second matching ring (ambiguity). Recorded,
  not changed.

## Verification

- Unit tests for the classifier (each table row), the run-status decision, `/health` +
  `/ready` status mapping, and the transition-only alert decision.
- Migration up/down exercised against a real Postgres via the integration suite.
- ⚠️ Re-break: point the resolver at an unreachable GDACS and confirm the run goes `degraded`,
  `/health` says `degraded`, `/ready` stays 200, and exactly one alert is decided across two
  consecutive degraded runs. Then a genuinely-unmatchable polygon with GDACS healthy must stay
  `success`.
- Independent review (`gpt-5.6-sol`) before merge, per this repository's standing practice for
  correctness-critical changes.
