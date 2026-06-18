---
id: feature-ai-digest-narrative
status: proposed
branch: feat/ai-digest-narrative
---

# Proposal: AI Digest Narrative (feature-ai-digest-narrative)

## Why

`feature-daily-flood-digest` gives partners a structured view of the day's flood
events — JSON at `GET /v1/digest/today.json` and a daily email — grouped by
country → state. It is accurate and machine-readable, but it is not *prose*. A
focal point at NRCS or Code for Africa still has to read a nested list and form
their own one-sentence picture of the day. The ask that keeps recurring in
partnership conversations is "just tell me what today looks like in a line or
two."

This proposal adds the smallest credible version of that: a short,
plain-language brief — **headline + lede + per-state lines** — generated once a
day from the *already-computed* `digest.Digest`, grounded strictly in that data,
and surfaced as a new `narrative` field on the existing digest endpoint. It is a
read-only summarization layer; it changes no data, no enrichment, and no email
cadence.

The hard requirement is that this must not weaken VigilAfrica's
"supplementary, never sole source" posture. The design therefore puts the model
in a tightly fenced box: it never fetches anything, it can only summarize the
`Digest` passed to it, and **every** trust-bearing element around the prose
(the disclaimer, the empty-day message, the validation, the fallback) stays
deterministic Go. The model touches only the prose.

This is captured today as the "Future enhancement (backlog): AI digest
narrative" appendix in
[`feature-daily-flood-digest.md`](feature-daily-flood-digest.md); this proposal
crystallizes it.

## What Changes

### New package — `api/internal/narrative` (Go)

1. **Narrative types** — `Narrative{ Headline, Lede string; RegionLines []RegionLine; Disclaimer string; Provenance Provenance }`,
   `RegionLine{ State, Summary string }`, and
   `Provenance{ Model string; GeneratedAt time.Time; SourceHash string; Generator string }`
   where `Generator` is `"ai"` or `"template"`. `Disclaimer` is always set by Go,
   never by the model (see Guardrails).

2. **Orchestrator** — `Build(ctx, gen Generator, d digest.Digest) Narrative`. The
   single entry point. It (a) short-circuits to a deterministic template when
   `d.Total == 0` or the generator is disabled, (b) otherwise calls the
   generator, (c) runs the deterministic hallucination cross-check, (d) on any
   error or validation failure falls back to the template, and (e) **always**
   stamps the fixed disclaimer and provenance before returning. `Build` never
   returns an error — a narrative always exists.

3. **Generator interface** — `Generator interface { Enabled() bool; Generate(ctx, digest.Digest) (Narrative, error) }`.
   Narrowing the dependency keeps `Build` testable with a fake and keeps the
   Anthropic SDK out of the orchestrator's tests.

4. **Anthropic generator** — `api/internal/narrative/anthropic.go`: a `Client`
   calling the Anthropic Messages API over stdlib `net/http` + `encoding/json`
   (mirroring the existing Resend integration — no SDK dependency; see
   *Decisions*). `Enabled()` is true only when `ANTHROPIC_API_KEY` is set.
   `Generate` makes exactly one call requesting **structured JSON output** so the
   model returns exactly `{ headline, lede, region_lines: [{state, summary}] }`
   and cannot ramble or add fields. Low effort / small max-tokens / temperature 0
   (deterministic summarization, no deep reasoning). No streaming, no prompt
   caching (once/day → the 5-min cache TTL never hits), no batch.

5. **Deterministic template** — `templateNarrative(d)` produces a fixed,
   data-only narrative ("N flood events across M states today: …", or "No new
   flood events today.") used for empty days, when the model is disabled, and as
   the failure fallback. This is the floor the feature degrades to.

6. **Validation** — `validate(n, d, knownStates)`: every state and country named
   in the structured output must appear in the source `Digest`; `region_lines`
   may not reference a state with zero events; and the prose (headline + lede) is
   scanned for any supported-state name **absent** from today's digest. The
   supported-state gazetteer (`knownStates`) is fetched once by the caller
   (`GetDistinctStatesByCountry`) and passed in, so `validate` stays pure and
   testable (no DB access inside `narrative`). Any violation → reject → template.
   Cheap, deterministic, decisive.

7. **Daily scheduler** — `StartNarrativeScheduler(ctx, repo, gen, store, cfg, logger)`,
   mirroring `digest.StartDigestScheduler`: a goroutine spawned from `main`, fires
   once per day at `DIGEST_NARRATIVE_HOUR` (UTC), builds today's digest, generates
   + validates the narrative, and persists it. Exits on `ctx.Done()`. When the
   generator is disabled it logs and does not start.

### Persistence (one small table)

8. **`digest_narratives` table** (migration `000011_create_digest_narratives`):
   `date DATE PRIMARY KEY`, `source_hash TEXT`, `generator TEXT`, `model TEXT`,
   `generated_at TIMESTAMPTZ`, and the narrative body as `JSONB`. Keyed by UTC
   day so there is exactly one current narrative per day; `source_hash` lets us
   skip regeneration when the day's data has not changed and regenerate when it
   has.

9. **Repository methods** — extend `database.Repository`
   ([db.go:20](../../api/internal/database/db.go)) with
   `GetNarrative(ctx, date string) (NarrativeRecord, bool, error)` and
   `UpsertNarrative(ctx, NarrativeRecord) error`, where `NarrativeRecord` is a
   **primitive DTO defined in `database`** (`Date, SourceHash, Generator, Model`
   strings; `GeneratedAt time.Time`; `Body []byte` JSONB). The repository never
   imports `narrative` — that would cycle (`database → narrative → digest →
   database`, since `digest` already imports `database`); keeping persistence in
   primitives respects the §1.4 dependency direction. The `narrative` package
   owns the `Body` ⇄ `Narrative` (un)marshal and defines its own narrow `Store`
   interface over these two methods (same pattern as `digest.EventLister`).

### Endpoint (additive)

10. **`narrative` field on `GET /v1/digest/today.json`** — the handler
    ([handlers/digest.go](../../api/internal/handlers/digest.go)) builds today's
    digest as now, then attaches the narrative: read the persisted row for
    today; on a miss (first request before the scheduler ran) generate lazily —
    bounded by a short `context.WithTimeout` derived from `r.Context()` so a slow
    model call never holds the request goroutine (§3.5/§6.8) — persist, and serve;
    if the model is disabled, times out, or fails, attach the template.
    The narrative is emitted as a sibling `narrative` object in the response via a
    small wrapper struct that embeds `digest.Digest` (avoids a `digest →
    narrative` import cycle). An empty day still returns `200` with a template
    narrative — never 4xx/5xx, never a blocked response.

### Wiring & config

11. **`api/cmd/server/main.go`** — construct the narrative `Client` + store, call
    `StartNarrativeScheduler`, and pass the narrative provider into
    `NewDigestHandler`. Add `loadNarrativeConfigFromEnv()` alongside the existing
    `loadDigest*FromEnv` helpers.

12. **New env vars** — `ANTHROPIC_API_KEY` (Go API only; empty → AI disabled →
    template always), `DIGEST_NARRATIVE_MODEL` (default: the current latest Opus,
    pinned to a named constant; `claude-haiku-4-5` is the documented cost
    downgrade), `DIGEST_NARRATIVE_HOUR` (UTC, default `5` — an hour before the
    `DIGEST_SCHEDULE_HOUR` default of `6` so the email can include it later).

13. **Shared disclaimer** — export the existing `disclaimer` constant in
    `api/internal/digest/render.go` as `digest.Disclaimer` so the narrative reuses
    the exact same string the email and site already show (single source of truth;
    no import cycle since `narrative` imports `digest`).

### Documentation

14. `docs/operations/daily-digest.md` — a "narrative" section: what it is, the
    guardrails, the env vars, the regenerate-on-hash-change behaviour, and how to
    disable it (unset `ANTHROPIC_API_KEY`).
15. `.env.example` — document the three new vars with safe placeholders.
16. `openspec/specs/vigilafrica/api-contract.md` + `openapi.yaml` (then
    `npm run sync:openapi`) — add the additive `narrative` object to the digest
    response shape.

## Narrative shape (JSON)

The digest response gains a `narrative` sibling (existing fields unchanged):

```json
{
  "date": "2026-06-18",
  "generated_at": "2026-06-18T05:00:04Z",
  "total": 3,
  "by_country": [ "… unchanged …" ],
  "narrative": {
    "headline": "3 flood events across 2 Nigerian states today",
    "lede": "New flooding was recorded in Benue and Kogi, with Benue accounting for two of the three events.",
    "region_lines": [
      { "state": "Benue", "summary": "Two flood events, both near Makurdi." },
      { "state": "Kogi",  "summary": "One flood event reported today." }
    ],
    "disclaimer": "VigilAfrica is an awareness and visualization tool, not an official emergency alert system. …",
    "provenance": {
      "model": "claude-opus-4-6",
      "generated_at": "2026-06-18T05:00:04Z",
      "source_hash": "sha256:…",
      "generator": "ai"
    }
  }
}
```

The `provenance.model` value above is illustrative — the actual default is the
current latest Opus, overridable via `DIGEST_NARRATIVE_MODEL`. On an empty day or
any degradation, `narrative.generator` is `"template"`, the prose is the
deterministic fallback, and `provenance.model` is empty.

## The Claude call

- **Model:** `DIGEST_NARRATIVE_MODEL`, default the current latest Opus (named
  constant, bumped at implementation time); `claude-haiku-4-5` is a fine cost
  downgrade at this volume. Cost is pennies/day either way (~1 call/day/country).
- **Structured output:** the model returns exactly
  `{ headline, lede, region_lines:[{state, summary}] }` — no disclaimer field, no
  free-form keys.
- **System prompt (fixed):** "You summarize a pre-computed flood-awareness
  digest. Use ONLY the events, states, counts, and dates in the data. Never
  invent a location, count, or event. If there are zero events, say there were no
  new flood events today. Give no safety or evacuation advice."
- **User content:** the `Digest` JSON + the date + the country set.
- **No tool-use, no retrieval.** The `Digest` is the only source of truth, passed
  in the prompt. This is the single most important design choice — the model can
  only summarize, never invent.

## Scheduling & regeneration

- One goroutine started in `main`, same lifecycle as the digest/watchdog
  schedulers (spawned in `main`, exits on `ctx.Done()`), firing daily at
  `DIGEST_NARRATIVE_HOUR` UTC.
- `source_hash` = SHA-256 of the digest's stable content (date + total +
  `by_country`, excluding `generated_at`). The scheduler and the lazy path both
  skip the model when a persisted narrative already matches today's hash, and
  regenerate when the data changed.
- **Single-replica assumption** (same as the digest): no cross-replica lock for
  v1. A `scheduler_locks`-based guard (migration `000009`,
  `TryAcquireSchedulerLock`) is the documented upgrade if the deployment ever
  scales out. The lazy-on-request path is idempotent on `(date, source_hash)`, so
  a rare double-generation is harmless (last write wins, same content).

## Guardrails (what protects the "supplementary, never sole source" posture)

| Concern | Control | Where |
| --- | --- | --- |
| Hallucinated place/count | Deterministic cross-check: output names ⊆ source `Digest`; no zero-event states; prose scanned for absent state names | Go (`validate`) |
| Disclaimer dropped/reworded | Fixed `digest.Disclaimer` appended by Go; not in the output schema | Go |
| Model/API failure | Template fallback; the digest never blocks on the AI | Go (`Build`) |
| Empty day | Short-circuit to template; the model is never called | Go (`Build`) |
| Auditability | Provenance (model + timestamp + source hash + generator) stored per day | DB |

## Security & Secrets

| Secret / config | Where it lives | Where it does NOT live |
| --- | --- | --- |
| `ANTHROPIC_API_KEY` | VPS `.env` (gitignored), Go API only | The repo, the frontend, any client bundle |
| `DIGEST_NARRATIVE_MODEL` / `_HOUR` | VPS `.env` | Source literals; `.env.example` carries placeholders |

`ANTHROPIC_API_KEY` is the one genuinely new moving part. It is read only by the
Go API; the frontend never sees it and makes no Anthropic calls. Absent key →
the feature degrades to templates with no error.

## Out of Scope

- **Multilingual narratives** (Hausa / Yoruba / Twi …) — deferred to a follow-up.
  That is where Batches@50% starts to pay off (5 langs × 2 countries = 10/day);
  English-only first proves the pipeline + guardrails.
- **Narrative in the daily email** — the email keeps its current deterministic
  body for v1. Including the headline/lede in the email is a trivial follow-up
  once the JSON narrative is trusted.
- **Other categories / a combined narrative** — floods only, matching the digest.
- **On-demand per-request generation as the primary path** — generation is
  scheduled + persisted; the request path only lazily backfills a missing day.
- **Safety, evacuation, or advisory text** — explicitly forbidden in the system
  prompt; the feature summarizes, it does not advise.
- **Streaming, prompt caching, Batches, tool-use / retrieval** — none for v1.

## Capabilities

### New Capabilities

- `digest-narrative`: `GET /v1/digest/today.json` gains a `narrative` object — an
  AI-generated, data-grounded plain-language brief with a deterministic template
  fallback.

### Modified Capabilities

- `digest-today-json`: response is extended additively with the `narrative`
  field; all existing fields and consumers are unaffected.

## Acceptance Criteria

- [ ] `GET /v1/digest/today.json` returns the existing shape plus a `narrative`
      object on every response, including empty days (`200`, never 4xx/5xx).
- [ ] With `ANTHROPIC_API_KEY` set and events present, `narrative.generator` is
      `"ai"`, prose is grounded, and provenance records model + timestamp +
      source hash.
- [ ] With `ANTHROPIC_API_KEY` unset, every response carries a `"template"`
      narrative; no Anthropic call is made; no error surfaces.
- [ ] Empty day → `"template"` narrative with the "no new flood events today"
      message; the model is not called.
- [ ] The hallucination cross-check rejects any output naming a state/country not
      in the source `Digest`, or a zero-event state, and the response falls back
      to the template.
- [ ] `narrative.disclaimer` is byte-identical to `digest.Disclaimer`; the model
      output schema has no disclaimer field.
- [ ] A model/API error or timeout never blocks or errors the endpoint — it
      degrades to the template.
- [ ] The narrative is generated at most once per `(date, source_hash)`; an
      unchanged day is served from storage without a new model call.
- [ ] No literal secret appears in committed files (`git grep`);
      `ANTHROPIC_API_KEY` exists only in `.env` / `.env.example` placeholder.
- [ ] `scripts/test-api.ps1` (unit + `-Integration`), `npm run build`,
      `go vet ./...`, and the `govulncheck` + image-pin CI gates stay green; the
      OpenAPI in-sync check passes after `npm run sync:openapi`.
- [ ] No regression in `GET /v1/digest/today.json` p95 latency when a narrative
      is already persisted (served from storage, no model call on the hot path).

## Risks

- **R1 — Hallucination slips past validation.** A model could phrase prose that
  is technically grounded but misleading. *Mitigation:* structured output
  constrains shape; the cross-check rejects unknown names; the fixed disclaimer
  states the tool is supplementary; provenance makes every narrative auditable.
- **R2 — New external dependency (Anthropic API) on the daily path.** *Mitigation:*
  the model is never on the request hot path once persisted; all failures
  degrade to the deterministic template; the endpoint never blocks on the AI.
- **R3 — Cost creep.** *Mitigation:* one call/day/country, regenerate only on
  hash change, low effort + small max-tokens. Multilingual (the real cost driver)
  is out of scope.
- **R4 — Secret exposure.** *Mitigation:* `ANTHROPIC_API_KEY` is Go-API-only,
  gitignored, `git grep`-verified; the frontend never calls Anthropic.
- **R5 — Model-ID drift.** *Mitigation:* model is a single named constant +
  env override (`DIGEST_NARRATIVE_MODEL`), not scattered literals.
- **R6 — Single-replica scheduling.** Same assumption as the digest; documented,
  with the `scheduler_locks` upgrade path noted.

## Verification Plan

1. Unit: `Build` table tests — empty day, disabled generator, successful AI path,
   validation-failure fallback — asserting generator, disclaimer, and provenance
   each time (fake `Generator`, injected clock).
2. Unit: `validate` table tests — unknown state/country, zero-event state,
   absent-state-name in prose, all-clean — assert accept/reject.
3. Unit: `templateNarrative` produces the deterministic strings for empty and
   non-empty digests.
4. Handler test: `GET /v1/digest/today.json` returns the digest + a `narrative`
   object; persisted-hit path makes no model call; cache-miss lazy path persists.
5. Integration (`scripts/test-api.ps1 -Integration`): seed today's floods, run the
   scheduler step against a fake generator, assert one `digest_narratives` row,
   correct `source_hash`, and idempotency on a second run with unchanged data.
6. Secrets: `git grep` shows no `ANTHROPIC_API_KEY` literal; `.env.example` has a
   placeholder only.
7. Manual: with a real key in local `.env`, hit the endpoint on a seeded day and
   confirm grounded prose, the exact disclaimer, and `generator: "ai"`; unset the
   key and confirm `generator: "template"` with no error.

## Decisions (resolving the backlog open questions)

1. **Generate at ingestion (persist) vs on-demand.** → **Persist via a daily
   scheduler**, with a lazy-on-first-request backfill. Keeps the request path
   fast; one call/day/country.
2. **Field on `today.json` vs separate `/narrative` endpoint.** → **Field on the
   existing endpoint** — one fetch for partners.
3. **English-only v1.** → **Yes** (de-risk; multilingual deferred).
4. **Opus vs Haiku.** → **Configurable**, default the current latest Opus, Haiku
   documented as the cost downgrade. Both fine at this volume.
5. **`ANTHROPIC_API_KEY` placement.** → **Go API only**, `.env` (gitignored),
   placeholder in `.env.example` — same discipline as `RESEND_API_KEY` /
   `DIGEST_TO`.
6. **Anthropic client: SDK vs stdlib.** → **Stdlib `net/http` + `encoding/json`**,
   no `anthropic-sdk-go`. Honours §10.1 (stdlib-first) and follows the direct
   §10.4 precedent that Resend email calls the REST API over stdlib with no SDK.
   No new `go.mod` dependency, no §10.2 record or §10.4 list change required. The
   one cost — hand-building the structured-output request/parse — is small for a
   single fixed call shape.

## Origin

Backlog appendix in `feature-daily-flood-digest.md`, captured from the 2026-06-18
design conversation and crystallized here. Builds directly on the
`feature-daily-flood-digest` digest builder + scheduler infrastructure and the
v0.3 events repository. First Anthropic/LLM integration in the codebase (via
stdlib `net/http`, no SDK — see Decisions); deliberately fenced to a read-only,
data-grounded summarization role.
