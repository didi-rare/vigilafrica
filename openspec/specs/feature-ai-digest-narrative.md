# Spec: AI Digest Narrative (feature-ai-digest-narrative)

**Status:** Proposed — builds on `feature-daily-flood-digest` (already implemented in `api/internal/digest`).
**Companion:** [`openspec/proposals/feature-ai-digest-narrative.md`](../proposals/feature-ai-digest-narrative.md) (rationale, design decisions, risks, resolved open questions).

## Context

The digest builder already produces the canonical day-shape:
`digest.Digest{ Date, GeneratedAt, Total, ByCountry[] → States[] → Events[] }`
([digest.go](../../api/internal/digest/digest.go)), served by
`DigestHandler.GetTodayDigest` and emailed by `digest.SendDigest`. This spec adds
a read-only summarization layer that turns that struct into a short
plain-language `Narrative`, surfaced as an additive `narrative` field on
`GET /v1/digest/today.json`.

The model is fenced to prose only: it receives the `Digest` in-prompt as the sole
source of truth (no tool-use, no retrieval), returns structured output, and is
wrapped by deterministic Go for validation, the disclaimer, the empty-day path,
and the failure fallback. This is the first Anthropic integration in the codebase
(stdlib `net/http`, no SDK — §10.1/§10.4) and is governed by the Sentinel gate
(it touches `api/internal/`, `api/cmd/`).

## Components to Touch

### New files

1. `api/internal/narrative/narrative.go` — the types (`Narrative`, `RegionLine`,
   `Provenance`), the `Generator` and `Store` interfaces, the
   `Build(ctx, gen, d, knownStates)` orchestrator, `templateNarrative(d)`,
   `validate(n, d, knownStates)`, and `sourceHash(d)` (SHA-256 over `Date` +
   `Total` + `ByCountry`, excluding `GeneratedAt`). `RegionLines` is always an
   allocated slice (`make([]RegionLine, 0)`), never nil, so it JSON-encodes to
   `[]` not `null` (§5.7).
2. `api/internal/narrative/anthropic.go` — `Client` calling the Anthropic
   Messages API over **stdlib `net/http` + `encoding/json`** (no SDK; mirrors the
   Resend integration, §10.1/§10.4): `Enabled()` (key present),
   `Generate(ctx, digest.Digest) (Narrative, error)` requesting structured JSON
   output, fixed system prompt, low effort / small max-tokens / temperature 0, no
   streaming / caching / batch / tools. The outbound call is bounded by
   `context.WithTimeout` (§3.5).
3. `api/internal/narrative/scheduler.go` — `StartNarrativeScheduler(ctx, repo, gen, cfg, logger)`
   daily goroutine + an exported `GenerateAndStore(ctx, repo, gen, now, logger)`
   for manual trigger and tests (mirrors `digest.SendDigest`).
4. `api/internal/narrative/narrative_test.go`, `anthropic_test.go`,
   `scheduler_test.go` — see Verification Plan.
5. `api/db/migrations/000011_create_digest_narratives.{up,down}.sql` — see Schema.

### Modified files

1. `api/internal/digest/render.go` — export `disclaimer` → `Disclaimer` so
   `narrative` reuses the exact string (no import cycle: `narrative` imports
   `digest`, not vice-versa). Update in-package references.
2. `api/internal/database/db.go` — add a primitive `NarrativeRecord` DTO
   (`Date, SourceHash, Generator, Model string`; `GeneratedAt time.Time`;
   `Body []byte`) and two `Repository` methods
   ([db.go:20](../../api/internal/database/db.go)):
   `GetNarrative(ctx, date string) (NarrativeRecord, bool, error)` and
   `UpsertNarrative(ctx, NarrativeRecord) error`; implement on `pgRepo` (JSONB
   `body`, `ON CONFLICT (date) DO UPDATE`). **`database` must not import
   `narrative`** — that would cycle (`database → narrative → digest → database`,
   §1.4) — so the repository speaks primitives + JSONB and the `narrative`
   package owns the `Body` ⇄ `Narrative` (un)marshal. `narrative.Store` is the
   narrow two-method view over these (declared in `narrative`, satisfied by
   `pgRepo`).
3. `api/internal/handlers/digest.go` — `NewDigestHandler` gains a narrative
   provider (`store` + `gen`); `GetTodayDigest` attaches the narrative via a
   wrapper that embeds `digest.Digest`:

   ```go
   type todayResponse struct {
       digest.Digest
       Narrative narrative.Narrative `json:"narrative"`
   }
   ```

   Read persisted narrative for `d.Date`; on miss or stale `source_hash`,
   `narrative.Build(...)` (bounded by a `context.WithTimeout` off `r.Context()`,
   §3.5/§6.8) + persist; serve. Never errors on the narrative path.
4. `api/cmd/server/main.go` — `loadNarrativeConfigFromEnv()`; construct the
   `narrative.Client` + pass the repo as the store; `StartNarrativeScheduler(...)`;
   pass provider into `NewDigestHandler`.
5. **No new `go.mod` dependency** — the Anthropic call uses stdlib `net/http` +
   `encoding/json` (§10.1, mirroring the Resend integration per the §10.4 note).
   Confirm `go mod tidy` adds nothing; govulncheck + image-pin gates stay green.
6. `.env.example` — `ANTHROPIC_API_KEY`, `DIGEST_NARRATIVE_MODEL`,
   `DIGEST_NARRATIVE_HOUR` with safe placeholders.
7. `openspec/specs/vigilafrica/api-contract.md` + `openspec/specs/vigilafrica/openapi.yaml`
   (then `npm run sync:openapi`) — additive `narrative` object.
8. `docs/operations/daily-digest.md` — narrative section (what, guardrails, env,
   regenerate-on-hash-change, how to disable).

## Schema

```sql
-- up (000011_create_digest_narratives.up.sql)
CREATE TABLE digest_narratives (
    date         DATE PRIMARY KEY,            -- UTC day; one current narrative per day
    source_hash  TEXT        NOT NULL,        -- sha256 of the digest's stable content
    generator    TEXT        NOT NULL,        -- 'ai' | 'template'
    model        TEXT        NOT NULL DEFAULT '',
    generated_at TIMESTAMPTZ NOT NULL,
    body         JSONB       NOT NULL,         -- {headline, lede, region_lines[], disclaimer, provenance}
    CONSTRAINT digest_narratives_generator_chk CHECK (generator IN ('ai','template'))
);
```

```sql
-- down (000011_create_digest_narratives.down.sql)
DROP TABLE IF EXISTS digest_narratives;
```

`date` PK gives idempotent upsert and at-most-one current row per day; storing
`source_hash` lets both the scheduler and the lazy path skip the model when the
day's data is unchanged.

## Config (new env vars)

- `ANTHROPIC_API_KEY` — Go API only. Empty → `Generator.Enabled() == false` →
  template always, no Anthropic call, no error.
- `DIGEST_NARRATIVE_MODEL` — default the current latest Opus (named constant
  `defaultNarrativeModel`, bumped at implementation time); `claude-haiku-4-5` is
  the documented cost downgrade.
- `DIGEST_NARRATIVE_HOUR` — UTC hour for the daily scheduler. Default `5` (one
  hour before the `DIGEST_SCHEDULE_HOUR` default of `6`, so a later email
  enhancement can include it). Parsed with the existing `envHourOfDay` helper.

No frontend env changes — the narrative arrives inside the existing digest fetch.

## The model call (anthropic.go)

- **Structured output:** request exactly `{ headline, lede, region_lines:[{state, summary}] }`
  via the Messages API (a single output-schema / tool definition in the request
  body, unmarshalled with `encoding/json`). No disclaimer field — Go owns it.
- **Transport:** stdlib `net/http` POST to the Messages endpoint with the
  `x-api-key` / `anthropic-version` headers; `context.WithTimeout` bounds the call.
- **System prompt (constant):** "You summarize a pre-computed flood-awareness
  digest. Use ONLY the events, states, counts, and dates in the data. Never invent
  a location, count, or event. If there are zero events, say there were no new
  flood events today. Give no safety or evacuation advice."
- **User content:** marshalled `Digest` JSON + date + country set.
- **Knobs:** temperature 0, small max-tokens, low effort; no stream/cache/batch/tools.

## Guardrails (deterministic, in Go)

1. **Empty-day / disabled short-circuit** — `Build` returns `templateNarrative(d)`
   without calling the model when `d.Total == 0` or `!gen.Enabled()`.
2. **Hallucination cross-check** — `validate(n, d, knownStates)`: the set of
   `region_lines[].state` ⊆ digest state set and each references a state with ≥1
   event; country names ⊆ digest countries; headline + lede contain no
   supported-state name absent from today's digest. `knownStates` (the
   `GetDistinctStatesByCountry` gazetteer) is fetched by the caller and passed in
   so `validate` does no DB access and stays pure. Violation → reject → template.
3. **Disclaimer** — `Build` always sets `n.Disclaimer = digest.Disclaimer`; the
   model output schema cannot carry or alter it.
4. **Graceful degradation** — any `Generate` error, timeout, or validation failure
   → `templateNarrative(d)` with `Generator = "template"`. `Build` never returns
   an error.
5. **Provenance** — `Build` stamps `{Model, GeneratedAt, SourceHash, Generator}`
   on every narrative (model empty for templates).

## Generation timing

- `StartNarrativeScheduler` fires daily at `DIGEST_NARRATIVE_HOUR` UTC, calls
  `GenerateAndStore` (build digest → `Build` → upsert). Disabled generator → logs
  and does not start (mirrors `StartDigestScheduler`).
- Lazy path in the handler: on a persisted miss or a `source_hash` mismatch for
  today, `Build` + upsert inline, then serve. Idempotent on `(date, source_hash)`,
  so a rare concurrent double-generation is harmless.
- **Single-replica assumption** documented; `scheduler_locks` /
  `TryAcquireSchedulerLock` (migration `000009`) is the scale-out upgrade.

## Implementation Plan (single PR, internally ordered)

1. `narrative` types + `Build` + `templateNarrative` + `validate` + `sourceHash`,
   with a fake `Generator` (no real client yet) — fully unit-tested.
2. Migration `000011` + repo `NarrativeRecord` DTO +
   `GetNarrative`/`UpsertNarrative` + `Store` interface.
3. `anthropic.go` (stdlib `net/http` Messages call, structured JSON output,
   system prompt, `context.WithTimeout`).
4. `scheduler.go` + `main.go` wiring + `loadNarrativeConfigFromEnv`.
5. Handler wrapper + lazy backfill.
6. `.env.example`, `api-contract.md` + `openapi.yaml` (`npm run sync:openapi`),
   `docs/operations/daily-digest.md`, and the `digest.Disclaimer` export.

## Acceptance Criteria

- [ ] `GET /v1/digest/today.json` returns the existing shape plus a `narrative`
      object on every response, empty days included (`200`, never 4xx/5xx).
- [ ] `ANTHROPIC_API_KEY` set + events present → `narrative.generator == "ai"`,
      grounded prose, provenance with model + timestamp + `source_hash`.
- [ ] `ANTHROPIC_API_KEY` unset → every response carries a `"template"` narrative;
      no Anthropic call; no error.
- [ ] Empty day → `"template"` "no new flood events today"; model not called.
- [ ] `validate` rejects any output naming a non-digest state/country or a
      zero-event state → template fallback.
- [ ] `narrative.disclaimer` is byte-identical to `digest.Disclaimer`; output
      schema has no disclaimer field.
- [ ] Model/API error or timeout never blocks or errors the endpoint.
- [ ] At most one model call per `(date, source_hash)`; unchanged day served from
      storage.
- [ ] Migration applies and `down` drops the table cleanly; no effect on existing
      tables.
- [ ] `git grep` shows no `ANTHROPIC_API_KEY` literal; `.env.example` placeholder
      only.
- [ ] `scripts/test-api.ps1` (unit + `-Integration`), `npm run build`,
      `go vet ./...`, `govulncheck`, image-pin, and OpenAPI-in-sync CI gates green.

## Verification Plan

- [ ] **Unit (`narrative`):** `Build` table tests (empty / disabled / AI-success /
      validation-fail) asserting generator, disclaimer, provenance; `validate`
      table tests (unknown name, zero-event state, prose leak, clean);
      `templateNarrative` strings; `sourceHash` stability (same content → same
      hash; changed content → different hash; `GeneratedAt` excluded).
- [ ] **Unit (handler):** persisted-hit makes no model call; cache-miss lazy path
      builds + persists; response embeds digest + `narrative`.
- [ ] **Integration (`scripts/test-api.ps1 -Integration`):** seed today's floods;
      run `GenerateAndStore` with a fake generator → one `digest_narratives` row,
      correct `source_hash`; rerun unchanged → no new generation (hash match);
      change data → regenerates.
- [ ] **Resilience:** generator returns an error / invalid output → endpoint
      serves the template, status `200`, `generator == "template"`.
- [ ] **Race:** the `narrative` package (scheduler + lazy handler path share the
      `Store`) runs green under `go test -race` (§7.9/§9.8).
- [ ] **Secrets:** `git grep` clean; `.env.example` placeholder only.
- [ ] `go vet ./...`, `go test ./...` (Docker runner), and `npm run build` green;
      OpenAPI in-sync check passes.
