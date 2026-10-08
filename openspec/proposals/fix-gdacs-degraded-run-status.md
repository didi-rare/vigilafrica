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
| **upstream** | episode count exceeds the scan cap AND no candidate matched | episodes past the cap were never seen — a partial view by construction | **yes** |
| **upstream** | a `200` that lacks the fields an answer must carry (`{}`, null `properties`, missing `features`) | a maintenance page or error envelope, not GDACS saying "no episodes" | **yes** |

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
   `failure`). **`/ready` returns 503 for `failure` or an unreadable database, never for `degraded`**: an upstream data provider being
   down does not make this API unable to serve, and readiness must not conflate the two.
5. **One alert per degraded streak in normal operation — at-least-once, not exactly-once — keyed on delivery**: `ingestion_runs.alert_sent_at` records when
   a degraded alert was actually delivered. After each degraded run the scheduler compares against
   the previous **completed** run for that country (never a `running` row). It suppresses the email
   only if that run was degraded **and its alert was delivered**, and then carries the flag forward.
   A failed send leaves the flag unset, so the next degraded run retries. A GDACS outage spanning
   hours sends one email, not one per cycle. (Failures keep their existing per-run behaviour.)
6. **Public dashboard**: `degraded` caused by GDACS shows an accurate message ("some flood areas
   could not be verified … may be missing") rather than the generic "ingestion did not complete",
   and the OpenAPI `status` enum and the web type gain `degraded`.

## Round 2 — independent review (`gpt-5.6-sol`) returned BLOCK; all findings verified, all fixed

Each finding was checked against the code before acting; none were taken on trust.

- **P0 — schema-less `200`s classified as refusals.** Confirmed with a probe before fixing: `{}`,
  `{"properties":null}`, a maintenance envelope, and `{}` from every geometry episode all came back
  `unverifiable`, which would have re-hidden an outage. Now `upstream`.
- **P0 — the episode cap was a partial scan classified as complete.** My own rule ("no match is only
  a refusal from a complete scan"), not applied to the cap. Now `upstream`.
- **P1 — a failed send silenced the whole streak.** The first design deduplicated on the previous
  run's *status*, so a failed Resend call left the streak looking alerted forever. Now keyed on
  delivery (`alert_sent_at`) and retried.
- **P1 — an orphaned `running` row masked the streak**, re-sending the alert. The lookup now reads
  the latest *completed* run, by id, after the run finishes.
- **P1 — `/ready` returned 200 when the database could not be queried.** Pre-existing (errors were
  only logged), fixed because it is the same function: `/ready` now 503s; `/health` stays 200.
- **P2 — wording overstated the cause and effect.** `upstream` also covers 429/5xx, malformed bodies
  and budget exhaustion, and existing events keep their last verified outline (stale, not missing).
  Corrected in the email, run message, OpenAPI, UI, and here.
- **P2 — `api-contract.md` still described the v0.1 `/health`** ("always ok"). Stale since v0.5;
  updated because this change alters that behaviour.
- **P2 — tests proved the pieces but not the wiring.** Added an end-to-end test driving the real
  `runScheduledIngest` through fake EONET, GDACS and Resend across consecutive runs, plus an
  integration test of the dedupe SQL.

Reviewer could not run the integration or web suites in its sandbox; both were run here and pass.

**Recorded, not changed:** the scheduler's five-minute lease is not renewed, so in principle an
EONET backoff longer than the lease could let two runs overlap. Pre-existing and independent of this
change; with `alert_sent_at` the worst case is one duplicate email.

## Round 3 — second independent review returned BLOCK; it caught an over-correction of mine

Round-1 verdicts: 3 FIXED, 3 NOT FIXED, 1 **OVER-CORRECTED**. Verified before acting:

- **P1, over-correction — the episode cap.** Round 1 marked a scan truncated at
  `maxGDACSEpisodes = 20` as incomplete, i.e. an outage. Verified live: GDACS flood 1104053 (Italy)
  declared **34 episodes** on 2026-10-03. Any such event would have been degraded on every run for
  its whole lifetime, keeping the public banner up while GDACS answered normally. **Fix:** the cap is
  a malformed-count backstop, not a scan limit — raised to 100; the per-run request and time
  budgets bound the real work. A count above 100 is still treated as malformed → incomplete.
- **P1 — schema-less 200s, inner level.** `{"properties":{}}` and `{"properties":{"episodes":null}}`
  still read as "zero episodes". Now `upstream` unless `episodes` or `episodeid` is actually present.
- **P1 — disabled alerting recorded as delivered.** `SendIngestFailure` returns nil when
  unconfigured, so the streak was marked alerted and configuring alerting mid-outage sent nothing.
  Now checked before any dedupe.
- **P2 — "once per streak" was really at-least-once.** A failed record-after-send (or a crash in
  that window) re-sends. **Not engineered away; stated honestly** in code and tested: the opposite
  ordering (record, then send) could silence a real outage, and a duplicate is the recoverable error.
- **P2 — migration not replayable** (developers-go.md §11.4). Now `IF EXISTS` / `IF NOT EXISTS`; a
  wrong constraint name is instead caught by the integration test that writes a `degraded` run.
- **P2 — `/ready` absent from OpenAPI; public/email copy said "until it recovers"**, which is wrong
  when the cause is our own request budget. Both corrected.

Tests added for every item, and each new guard re-broken and confirmed to fail: cap back to 20,
inner-schema check removed, disabled-alerting check removed, migration made non-replayable.

## Round 4 — third independent review; one real defect, one accepted limitation

- **P1 — `/health` read in-progress runs (fixed).** Every scheduled run inserts a `running` row
  first, and both health queries took the latest row of any status, so for the duration of EVERY
  run `/health` reported the in-progress run and a degraded or failed state read as `ok`; a crash
  orphan hid it indefinitely. Pre-existing for `failure`, but it undercut the signal this change
  adds. Both queries now read the latest **completed** run.
- **P2 — `alert_sent_at` was not a true delivery time (fixed).** Carry-forward stamped `NOW()`
  though nothing was sent. It now copies the streak's original delivery time. The remaining
  "once per streak" wording now says at-least-once.
- **P3 — `gofmt` (fixed).**
- **P1 — budget exhaustion can still degrade a long event every run (ACCEPTED, recorded).** The cap
  is 100 but the whole NG+GH run has 60 GDACS requests, and one event costs 1 + its episode count,
  so a ≥60-episode event (or several long ones together) would exhaust the budget on every run.
  Measured before deciding: over the past year the longest NG event reached 8 episodes and GH 3, and
  the live NG/GH feed on 2026-10-05 carried two polygon floods of 6 and 3 episodes. Crucially, the
  `degraded` status would be **true** in that case — the flood genuinely is not being verified —
  whereas before this change it was silently skipped forever. The real fix (persisting the matched
  episode across runs so a long event is scanned once) is a separate change, worth doing only if
  long events ever appear here. A test now proves a 34-episode event resolves within the
  **production** budget rather than an unbounded one.

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
