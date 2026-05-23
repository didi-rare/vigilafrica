---
id: fix-api-country-filter
status: proposed
branch: fix/api-country-filter
---

# Spec: API Country-Filter Input Hardening (fix-api-country-filter)

## Context

`/openspec-review` of the running stack on 2026-05-14 surfaced an API contract asymmetry:

- Response payloads include both `country_code` (ISO alpha-2) and `country_name`
- But the `country` query param on `/v1/events` and `/v1/states` only matches `country_name` via `ILIKE`
- An ISO-shaped input like `?country=NG` returns `[]` silently
- A plausible-sounding `?country_code=NG` returns the **unfiltered** list silently (unknown param dropped)

This produces the most common shape of "API filter is broken" bug report: caller used the wrong param name or value form, got the no-filter fallback, assumed the data was wrong.

The fix is additive — accept `country_code` alongside the existing `country`, validate both, and return HTTP 400 when neither resolves. No breaking change for existing callers (web frontend, bookmarks).

Companion: [openspec/proposals/fix-api-country-filter.md](openspec/proposals/fix-api-country-filter.md).

## Decision Log

| # | Decision | Alternatives | Why |
|---|---|---|---|
| D1 | Additive `country_code` param alongside existing `country` (Option B from the proposal) | Deprecate `country` in favour of `country_code` (Option D) | Breaking change — web frontend at [web/src/api/events.ts:68](web/src/api/events.ts#L68) sends `country=Nigeria`; user-bookmarked `/?country=Nigeria` URLs would break. A breaking redesign deserves its own `feat-api-v2-country-input` proposal |
| D2 | If both `country` and `country_code` are present, `country_code` wins | Reject 400 on both-given | Code is the more authoritative ISO identifier. Web callers will only send one or the other, so the conflict path is operator-CLI territory — silently picking the canonical wins |
| D3 | Unknown values return HTTP 400 with a hint listing supported codes + names | Return `[]` (current behaviour) | The proposal's whole point: "looks broken" is "got the no-filter fallback". 400 turns silent failure into a learnable error |
| D4 | Country↔name lookup lives in a new file `api/internal/handlers/country.go`, NOT reused from `ingestor.DefaultCountries` | Reuse `ingestor.DefaultCountries` | The ingestor list carries BBox data unrelated to filter parsing. Importing ingestor from handlers creates a wrong-direction dependency. The lookup is two entries today — duplication cost is negligible vs. coupling cost |
| D5 | Case-insensitive matching for both code and name (`ng` → `Nigeria`, `nigeria` → `Nigeria`) | Strict case-sensitive | Existing `ILIKE` is already case-insensitive — preserving that prevents surprise 400s for callers using `?country=nigeria` |
| D6 | Repository signature unchanged (still takes a `country` string = canonical name) | Take a code, do the lookup in the repo | Handler-layer normalisation keeps the repository agnostic about the API surface. Today's repo uses `ILIKE` against `country_name` — already correct once the handler resolves to canonical name |
| D7 | `state` param is untouched | Apply the same allow-list treatment to state | Out of scope per the proposal. State values are populated from EONET enrichment, not from a fixed list, so allow-listing them is a separate design question |
| D8 | Unknown query params (e.g. `?countrycode=NG`) are still silently dropped | Strict unknown-param rejection | Proposal explicitly punts to a future `chore-api-strict-params`. This proposal is targeted at the country contract only |

## Components to Touch

### New files

| File | Purpose |
|---|---|
| `api/internal/handlers/country.go` | `countryCodeToName` + `countryNameToCode` lookups + `resolveCountry(url.Values) (canonicalName string, present bool, err error)` |
| `api/internal/handlers/country_test.go` | Unit tests for `resolveCountry` (code, name, case-insensitivity, both-given precedence, unknown value, no-param) |

### Modified files

| File | Change |
|---|---|
| [api/internal/handlers/events.go](api/internal/handlers/events.go) | Replace `query.Get("country")` direct read at line 48 with `resolveCountry(query)` call; emit 400 on unknown |
| [api/internal/handlers/states.go](api/internal/handlers/states.go) | Replace `r.URL.Query().Get("country")` at line 16 with `resolveCountry()` call; emit 400 on unknown |
| [api/internal/handlers/events_test.go](api/internal/handlers/events_test.go) | Add subtest table to `TestListEvents…` covering: `country=Nigeria` (existing-still-works), `country_code=NG`, `country_code=ng` (case), both-given-code-wins, `country_code=XX` → 400, `country=Atlantis` → 400 |
| `api/internal/handlers/states_test.go` (new) | Mirror coverage for `/v1/states` |
| [api/internal/handlers/openapi.yaml](api/internal/handlers/openapi.yaml) | Document `country_code` query param on `/v1/events` and `/v1/states`; document the 400-on-unknown response |
| [openspec/specs/vigilafrica/api-contract.md](openspec/specs/vigilafrica/api-contract.md) | Same documentation update at the spec layer |

### Untouched

- [api/internal/database/queries.go](api/internal/database/queries.go) `GetDistinctStatesByCountry` and `ListEvents` — D6 keeps the repository signature unchanged
- Web frontend — already sends `country=Nigeria`/`country=Ghana` which keeps working unchanged
- `country_code` field of response payloads — already present and correct, just newly usable as a filter input
- `state`, `category`, `status`, `limit`, `offset` filters — out of scope per D7

## Behaviour Contract

- **B1** — A request with `?country=Nigeria` MUST return the same filtered result as today (no regression on the canonical name input)
- **B2** — A request with `?country=nigeria` (lowercase) MUST work — case-insensitive name match (D5)
- **B3** — A request with `?country_code=NG` MUST resolve to `Nigeria` and return the same filtered events as `?country=Nigeria`
- **B4** — A request with `?country_code=ng` (lowercase code) MUST work — case-insensitive code match (D5)
- **B5** — A request with **both** `?country=Ghana&country_code=NG` MUST return Nigeria events (code wins per D2)
- **B6** — A request with `?country_code=XX` (unknown code) MUST return HTTP 400 with body `{"error":"unknown country: supported values are NG, GH (or Nigeria, Ghana)"}`
- **B7** — A request with `?country=Atlantis` (unknown name) MUST return HTTP 400 with the same body
- **B8** — A request with neither param MUST behave as today (no filter applied)
- **B9** — `/v1/states` MUST follow B1-B8 identically — same `resolveCountry` helper used by both handlers
- **B10** — The web frontend at `web/src/api/events.ts` MUST continue to work without modification — it sends `country=Nigeria`/`country=Ghana` which satisfies B1

## Phase 1 — Implementation

- [ ] Create `api/internal/handlers/country.go` with the lookup maps and `resolveCountry()`
- [ ] Update [events.go](api/internal/handlers/events.go) `ListEvents` to call `resolveCountry()`
- [ ] Update [states.go](api/internal/handlers/states.go) `StatesHandler` to call `resolveCountry()`
- [ ] Both handlers emit `respondWithError(w, http.StatusBadRequest, msg)` on `err != nil`

## Phase 2 — Tests

- [ ] Create `country_test.go` with unit tests for `resolveCountry()` covering all 8 cases in the Behaviour Contract
- [ ] Extend `events_test.go` with a `TestListEventsCountryFilter` table covering: known name, known code, code wins over name, unknown name → 400, unknown code → 400
- [ ] Create `states_test.go` with the same coverage for the states handler

## Phase 3 — Documentation

- [ ] Update [openapi.yaml](api/internal/handlers/openapi.yaml): add `country_code` query param description on both endpoints; document the 400 response schema
- [ ] Update [openspec/specs/vigilafrica/api-contract.md](openspec/specs/vigilafrica/api-contract.md): mirror the openapi changes at the spec level

## Acceptance Criteria

- [ ] All B1-B10 verified by unit tests
- [ ] `go test ./...` from `api/` is green
- [ ] `go vet ./...` clean; `go build ./...` clean
- [ ] `web` frontend tests still pass — `country=Nigeria` keeps working (B10)
- [ ] `openapi.yaml` shows `country_code` as a query param on both endpoints
- [ ] `api-contract.md` updated alongside openapi.yaml
- [ ] PR description includes the live "before / after" table from the proposal showing the four shapes (`?country=Nigeria`, `?country=NG`, `?country_code=NG`, none) and how each behaves post-fix

## Out of Scope (reaffirmed)

- Strict unknown-query-param validation across the API (future `chore-api-strict-params`)
- Renaming/deprecating the `country` input (deliberate breaking change → separate proposal)
- Filter combinations — current combinations already work, no change
- Country code aliases (e.g. accepting `nga` 3-letter) — alpha-2 only
- Applying the same allow-list treatment to `state` — out of scope per D7

## Risks

- **R1 — Breaking partial-match callers**: today `?country=Nig` returns `[]`; post-fix it returns 400. Mitigation: 400 with a clear "supported values are…" hint guides the caller faster than the silent empty response did. No legitimate frontend or bookmark uses partial names
- **R2 — Adding `country_code` to the surface increases API contract size**: each new param is a future maintenance obligation. Mitigation: D8 explicitly punts strict validation; the country contract is fully documented in openapi.yaml
- **R3 — Frontend regression**: the web app's filter dropdown sends `country=Nigeria`. Mitigation: B10 + a test verifying the canonical-name path is unchanged
- **R4 — Country list expansion**: when a third country is added (e.g. v0.8), `countryCodeToName` must be updated. Mitigation: explicit handler-layer lookup makes the addition a one-line change; also forces a parallel update to the ingestor's `DefaultCountries` which already pairs codes with names

## Verification Plan

1. Implement Phase 1 + Phase 2 on this branch
2. `go test ./internal/handlers/... -count=1 -v` from `api/` — all green including the new tests
3. `go vet ./...` and `go build ./...` clean
4. Update openapi.yaml + api-contract.md per Phase 3
5. Open PR to `development`; reviewer checks B1-B10 coverage in the test table
6. Post-merge → main → staging: `curl https://api.staging.vigilafrica.org/v1/events?country_code=NG | jq '.meta'` → confirms non-zero total; `curl …?country=Atlantis` → confirms 400

No new automated CI changes required.
