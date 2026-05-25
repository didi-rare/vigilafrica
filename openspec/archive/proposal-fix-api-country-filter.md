---
id: fix-api-country-filter
status: proposed
branch: fix/api-country-filter
---

# Proposal: API Country-Filter Input Hardening (fix-api-country-filter)

## Why

A `/openspec-review` of the running stack on 2026-05-14 surfaced three related issues with the country filter on `/v1/events` and `/v1/states`:

1. **Filter input is country *name* not ISO code.** [api/internal/handlers/states.go:16](api/internal/handlers/states.go#L16) and [events.go:48](api/internal/handlers/events.go#L48) read `Query().Get("country")` and match via `country_name ILIKE $1` in [database/queries.go:485-501](api/internal/database/queries.go#L485-L501). So `?country=Nigeria` works, `?country=NG` returns empty. This is inconsistent with the rest of the API: response payloads include both `country_code` (ISO) and `country_name`, so the field a caller can read is not the field they can filter by.
2. **Unknown query params silently dropped.** A request like `?country_code=NG` (using a plausible-sounding param the API doesn't recognise) returns the unfiltered list with no 400, no warning header. The most common shape of "filter looks broken" is actually "caller used a slightly-wrong param name and got the no-filter fallback".
3. **`country=NG` returns empty with no hint.** `ILIKE 'NG'` against `Nigeria` is no match. The caller gets `{"states":[]}` or `{"data":[]}` with no signal that they probably meant `Nigeria` or `country_code=NG`.

Live evidence from a local session:

| Request | Result |
|---|---|
| `?country=Nigeria` | 8 NG states ✓ |
| `?country=NG` | `[]` — ILIKE 'NG' ≠ 'Nigeria' |
| `?country_code=NG` | **all 15 states** — unknown param dropped, no-filter branch fires |
| no params | all 15 states (correct fallback) |

## What Changes

This proposal carries the **Option B** shape from the earlier exploration: additive ISO support + targeted 400 for unknown country values. Path **D** (deprecate `country` in favour of `country_code` only) is not chosen here because it would break the web frontend at [web/src/api/events.ts:68](web/src/api/events.ts#L68) and any user-bookmarked `/?country=Nigeria` URLs. If a breaking redesign is later preferred, it should be a dedicated `feat-api-v2-country-input` proposal.

1. Accept `country_code` (ISO alpha-2) as input on `/v1/events` and `/v1/states`, alongside the existing `country` (name) param
2. Map `NG → Nigeria`, `GH → Ghana` internally via a small lookup
3. If both `country` and `country_code` are present, prefer `country_code` and ignore `country`
4. If `country=XYZ` or `country_code=XX` resolves to no known country, return HTTP 400 with `{"error":"unknown country: supported values are NG, GH (or Nigeria, Ghana)"}`
5. Update [api/internal/handlers/openapi.yaml](api/internal/handlers/openapi.yaml) and [openspec/specs/vigilafrica/api-contract.md](openspec/specs/vigilafrica/api-contract.md) to document both inputs
6. Add API tests covering: name, code, both-given precedence, unknown value, no-param fallback

## Out of Scope

- Strict unknown-query-param validation across the API (a future `chore-api-strict-params` if desired — touches every handler + middleware)
- Renaming/deprecating the `country` input (deliberate, see above)
- Filter combinations (`country` + `state` + `category` already work; this proposal doesn't touch them)
- Country code aliases (e.g. accepting `nga` 3-letter) — alpha-2 only

## User Impact

- Callers who knew the ISO convention (`country_code=NG`) get their filter to actually apply
- Callers who typo a country code get a clear 400 instead of a silent no-op
- Existing callers using `country=Nigeria` keep working — no behaviour change

## Origin

Surfaced during the `/openspec-review` of `chore-css-tokens` (2026-05-14), specifically while smoke-testing the running stack. The bug-hunter investigation confirmed the filter itself isn't broken — the API contract has an asymmetry (response uses `country_code`/`country_name`; input uses `country` only) that produces the user-perceived "filter is broken" symptom.
