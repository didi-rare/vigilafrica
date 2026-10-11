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
- ~~A single-episode-fetch failure when exactly one candidate still matched: resolution proceeds as
  today.~~ **Withdrawn in round 4** (see below): accepting geometry from a partial scan was a
  fail-open path, not an accepted gap.

## Round 4 — independent review (`gpt-5.6-sol`, 2026-10-11) verdict BLOCK; all eight findings fixed

⚠️ Two of the findings were *introduced by this change* and two were pre-existing behaviour this
change had explicitly accepted. The reviewer was right on all four; the earlier rounds had reviewed
the degraded plumbing and not the properties it rests on.

| # | Finding | Fix |
|---|---|---|
| P0 | A partial GDACS episode scan (one episode errored, or the scan hit the episode cap) with **exactly one** matching ring resolved as OK. The episode never seen may hold a second, different ring, which a complete scan refuses as ambiguous — so this was a fail-open path in the fail-closed resolver. A test pinned it ("resolves, as before"). | `resolveGDACSPolygon`: any incomplete scan is `resolveUpstream`, regardless of candidates. Both pinned tests flipped. Consequence: an event with more than `maxGDACSEpisodes` episodes now always degrades the run (none has exceeded 34 in a year). |
| P1 | Malformed feature entries (`{"features":[{}]}`, an affected polygon with unreadable or empty coordinates) were skipped as if they were valid non-matches, so a schema-broken GDACS body ended as a refusal and the run stayed `success` — the exact silent outage this change removes. | A feature with neither class nor geometry type, or an affected polygon whose coordinates do not parse or are empty, marks the scan incomplete → upstream. A well-formed feature of another class, or a ring with a different vertex count, stays a valid non-match. Five new table rows. |
| P1 | Per-event database write failures (upsert, metadata-only update) were logged and dropped, so a run could record `success` — or merely `degraded` — over lost rows. Pre-existing; this change made it more misleading by adding a milder status it could hide behind. | `IngestResult.StorageErrors`; any non-zero value is `failure`, outranks `degraded`, and propagates out of `IngestWithBudget` as an error so the scheduler's failure alert fires. End-to-end test: failing upserts → `failure` row + one failure alert. |
| P1 | A failed `CompleteIngestionRun` was logged while `IngestWithBudget` returned success and built an in-memory run as if persisted; the row stayed `running`, `/health` kept the previous run, nothing alerted. Pre-existing. | The completion error becomes the run's error (failure) and surfaces through the scheduler's failure alert; `CompleteIngestionRun` now fails unless exactly one row was updated (integration test on a non-existent id). |
| P1 | Both deploy smoke tests run `jq -e '.status == "ok" …'` on `/health`, so any legitimate degraded streak would fail every production and staging deploy — the opposite of this change's own claim that degraded is not unreadiness. **Introduced here.** | Smoke tests now gate on `GET /ready` (503 only for a real failure) and accept `ok` or `degraded` from `/health` alongside the version check. |
| P2 | `api-contract.md` said nullable fields are never omitted, then that telemetry is omitted on database failure, and its 200 example omitted the required `last_ingestion`. | One policy: `last_ingestion` always present (null when unknown), `last_ingestion_by_country` omitted when empty — written down as the single documented exception; example now matches the handler and OpenAPI. |
| P2 | Degraded alerting returned silently when `RESEND_API_KEY`/recipients were unset; the failure path warns, the degraded path did not (§8.3). | Warn log on the degraded path; a test asserts it. |
| P3 | Migration 000016 lacked the §11.12 milestone / data-quality header; the down file had no header. | Headers added. |

Also tightened the web test for a country failure alongside a degraded run: it now pins the exact
regional-failure copy and asserts the GDACS banner is absent, rather than matching a substring both
banners could satisfy.

## Round 5 — second independent pass on the round-4 fixes (`gpt-5.6-sol`, 2026-10-11) verdict BLOCK; all seven findings fixed

The second pass confirmed six of the eight round-4 fixes and found one **over-correction** and six
new issues. ⚠️ The over-correction is the instructive one: round 4 classified an affected polygon
with *explicitly empty* coordinates as an outage, which would have degraded every run for as long
as GDACS kept answering "no polygon" — a permanent false outage introduced while closing a
fail-open path.

| # | Finding | Fix |
|---|---|---|
| P1 (over-correction) | Round 4's malformed-feature guard required **both** class and geometry type to be missing, so `{"Class":"Poly_Affected"}` with no geometry type, or a Polygon with no class, still passed as a valid non-match. | A feature missing **either** discriminator marks the scan incomplete. Two new table rows, plus one for unreadable nested coordinates. |
| P1 | Explicit empty coordinates (`[]`) were classified upstream. RFC 7946 §3.1 defines an empty coordinates array as null geometry: GDACS saying "no polygon" is a complete answer. | Empty coordinates are a non-match (refusal); only unreadable coordinates mark the scan incomplete. The round-4 row is flipped with the RFC citation. |
| P2 | `resolveTyped` cached `resolveUpstream` alongside real answers. The budget is shared across countries in a run, so one transient 500 for an event in both NG and GH queries was replayed as a fact for the second country, and a malformed episode poisoned every later lookup. | Answers are cached; outages never are. Test: first lookup 500 → upstream and uncached; second lookup → resolved. **Trade-off, stated:** a dead upstream is now asked again on every lookup instead of once per run; the shared run budget bounds what that costs and an exhausted budget still fails closed (the pre-existing budget test is split into a cached refusal and a retried-but-bounded outage). |
| P2 | When ingestion failed **and** the run record could not be updated, only the first error survived into the returned error, the in-memory run and the alert. | Both errors are joined (`%w; additionally … %w`). Test asserts the alert carries both. |
| P2 | The round-4 contract example showed populated `started_at` / `events_fetched` / `events_stored`, but the handler redacts them to `null` on the public API (nothing enables the unredacted form today), OpenAPI left every run-record member optional, `status` was nullable in Go, OpenAPI and TypeScript although it is never null. | Example shows the real (redacted) public shape and says so; OpenAPI `LastIngestion` lists its six always-present keys as `required` and `status` is not nullable; Go `Status` is a plain string; the TypeScript union drops `null`. Both OpenAPI copies re-synced and re-parsed. |
| P3 | The disabled-alerting warning test swapped `slog.Default()` process-wide (§9.5). | The warning is emitted through the alert client's injected logger (`Client.Logger()`, §8.6); the test captures that logger directly. |
| P3 | The storage-error end-to-end test covered only the upsert path; removing the `StorageErrors` increment from the metadata-only path left every test green. | New end-to-end test: existing polygon event, GDACS 500, metadata update fails → `failure` row whose error names both the lost write and the GDACS outage, and one failure alert. |

## Round 6 — third independent pass (`gpt-5.6-sol`, 2026-10-11) verdict BLOCK; all three findings fixed

The third pass confirmed six of the seven round-5 fixes and the merge against `development`, and
found three more.

| # | Finding | Fix |
|---|---|---|
| P1 | JSON `null` (or an absent `coordinates` key) unmarshals into the same zero-length slice the round-5 code checks for an explicit `[]`, so a malformed affected polygon with `coordinates: null` was classified as a complete non-match and left the run healthy. | `null`/absent coordinates (nil slice) are unreadable → incomplete → upstream; an explicit `[]` (non-nil empty slice) stays a non-match. Four more pinned shapes: `null`, missing key, `[[]]` (unreadable), `MultiPolygon` and `Point` under `Poly_Affected` (well-formed non-matches). |
| P1 | A failed `CreateIngestionRun` was logged and the run went on to return **success** with no row — `/health` stale, no alert, and a degraded streak's recovery invisible. The round-2 design had accepted this ("a database hiccup cannot suppress the degraded alert") but only covered the degraded case. | Ingestion still proceeds (events are worth storing), but the creation error is joined into the outcome, so the run is a **failure** that the scheduler alerts on, rebuilt from the in-memory result. End-to-end test: run row fails, events store, one failure alert naming the unrecorded run. |
| P2 | The TypeScript `LastIngestion` omitted `country_code` and allowed `null` per-country values; the contract prose still said fields are never omitted. | `country_code?: string` added; map values typed non-null (the handler never emits one); the Null Handling rule now names both omissions. |

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
