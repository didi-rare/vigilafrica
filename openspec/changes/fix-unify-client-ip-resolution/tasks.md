# Tasks: Unify Client-IP Resolution

Promoted from `openspec/proposals/` + `openspec/specs/` at `/openspec-apply`. Rationale and the corrected severity are in [proposal.md](proposal.md); the technical spec is in [design.md](design.md).

⚠️ **`openspec/specs/vigilafrica/task.md` was NOT used** — that file declares itself `# SUPERSEDED` ("Do not use this file"). The repo's real convention for a change's task list is `openspec/changes/<id>/tasks.md`, matching `feature-events-pagination` and `chore-vps-access-hardening`.

## 0. Precedence — decided BEFORE coding

[design.md](design.md) required this table to be written down first, because the existing override ordering at `context.go:29-43` is load-bearing and easy to break while moving code around it.

| # | Condition | Location returned | `location_source` |
|---|---|---|---|
| 1 | valid `lat` **and** `lng` supplied | exactly the supplied coordinates | `explicit` |
| 2 | `DEV_FORCE_LAGOS=true` | hardcoded Lagos, no lookup | `dev_override` |
| 3 | `DEV_OVERRIDE_IP` set | mmdb lookup of that IP | `dev_override` |
| 4 | otherwise | mmdb lookup of the trusted-proxy-aware client IP | `ip` |
| — | reader absent, or lookup finds nothing | `location: null` | `unavailable` |

**Why explicit outranks the `DEV_*` overrides.** They exist to substitute for unhelpful IP geolocation on a developer's machine. An explicit request is the caller stating exactly what they want, and it is part of the public contract. If `DEV_*` won, the documented parameter would silently stop working on any box with the variable set — the same *silent* failure class this change exists to remove. A developer who wants Lagos simply omits the parameters.

⚠️ **Explicit coordinates are NOT reverse-geocoded.** No point→admin-name resolution exists in `api/internal/` (checked). `country` and `state` are returned empty for `explicit`; the coordinates drive the nearby-events query, which is all they are needed for. Stated rather than faked.

## 1. Test first — must be RED before the fix

- [x] 1.1 ⚠️ **My first attempt at this test was the exact trap the task warns about, and I caught it before running anything.** I wrote it against `clientIPWithTrustedProxies` — which was *already correct*, so it would have passed pre-fix and proved nothing. The defect was never in that function; it was that `/v1/context` did not call it. Redirected the test at `resolveLocationPlan`, the seam where a request becomes a location.
  **Defect demonstrated empirically first**, via a throwaway test run against the pre-fix tree (`scripts/test-api.ps1 -GoTestArgs '-run','TestZZDefectDemo',...`), then deleted:
  ```
  peer=203.0.113.9 (UNTRUSTED), X-Forwarded-For=8.8.8.8
    clientIP()  [rate limiter] -> "203.0.113.9"   <- correct
    extractIP() [/v1/context]  -> "8.8.8.8"       <- forged value believed
  ```
  The two resolvers returned **different answers for identical input**. That is the evidence; the rest of this change removes the difference.
- [x] 1.2 Table-driven over `(peer, headers, expected country)`, asserted through `GetContext` on the decoded response body: untrusted peer + forged `X-Forwarded-For`; untrusted peer + forged `X-Real-IP`; trusted proxy + XFF; trusted proxy taking the leftmost XFF entry; trusted proxy + `X-Real-IP` only; untrusted IPv6 peer; **trusted IPv6 loopback (`::1`)**. Malformed `RemoteAddr` is covered separately in the precedence test.
  ⚠️ **This task's wording has been reconciled twice and the earlier text was left FALSE in between** — independent review round 2 caught it. It previously said the tests call `clientIPWithTrustedProxies` directly, then said they use `t.Setenv` and read `TRUSTED_PROXY_CIDRS`, then said the malformed-`RemoteAddr` case was dropped and that `::1` was covered. **None of those describe the final implementation.** What is actually true now: every test constructs `ContextConfig`/`ProxyConfig` explicitly, nothing reads the environment, `t.Setenv` is not used anywhere in this file, malformed `RemoteAddr` **is** covered, and `::1` **is** covered.
  **Lesson recorded rather than tidied away:** a task list edited to match an earlier draft of the code becomes a false record of the work. Reconcile it with the tree at the end, not with the plan.

## 2. Unify the resolver

- [x] 2.1 Delete `extractIP()` from `context.go` **entirely** — no deprecated alias. A second name for the same behaviour reintroduces the ambiguity this change exists to remove.
- [x] 2.2 Route `/v1/context` through the shared resolver. ⚠️ **Undeclared deviation, now recorded** (review round 2): the review refactor replaced package-level `clientIP()` with the injected `ProxyConfig.ClientIP` method, so the name in this task no longer exists. The single-resolver property still holds — `clientIPWithTrustedProxies` remains the only implementation — but the plan said `clientIP()` and the code says otherwise, and that deviation went unrecorded until review caught it.
- [x] 2.3 Replace the misleading comment (`context.go:73` described intent — "accounting for reverse proxies") with one stating the trust constraint.
- [x] 2.4 Doc comment recording the single-path property — now on `ProxyConfig.ClientIP` (see the 2.2 deviation), not on `clientIP`, which was removed.
- [x] 2.5 Preserve `DEV_FORCE_LAGOS` / `DEV_OVERRIDE_IP` behaviour per the table above, and assert both with tests, since this change moves code around them.

## 3. Explicit location (decision B)

- [x] 3.1 Accept `lat` and `lng` query parameters on `/v1/context`, precedence per §0.
- [x] 3.2 Validate: both required together; parseable as floats; `lat` in [-90, 90]; `lng` in [-180, 180]. Anything else → **400 naming the offending parameter**, via the existing `respondWithError` used throughout `events.go`. ⚠️ **Never a silent fallback to IP** — a typo must not be indistinguishable from a working query.
- [x] 3.3 Add `location_source` to the response so a client can distinguish "we geolocated you" from "you asked about here". Same disclosure principle `feature-events-pagination` established for truncated results.

## 4. Contract

- [x] 4.1 Update `openspec/specs/vigilafrica/openapi.yaml` — the source of truth — with the two parameters, the `400` response, and `location_source` on `ContextResponse`.
- [x] 4.2 Run `npm run sync:openapi`. ⚠️ CI hard-fails on drift against `api/internal/handlers/openapi.yaml` (`ci-cd.yml:48-49`).
- [x] 4.3 Amend the **Situational Context API** requirement in the spec delta: it currently says the system resolves *"the caller's IP address"*, which will no longer be the only path.

## 5. Verification

- [x] 5.1 ⚠️ **Criterion AMENDED, not just annotated** (independent review, P3). It originally demanded `grep -rn "extractIP" api/` return nothing, which was false — and marking a false criterion complete is the defect, regardless of how harmless the residue is. **Amended to:** no *declaration* and no *call site* of a second resolver may exist. Verified: `grep -rn "func extractIP\|extractIP(" api/` returns nothing. Historical mentions survive in prose comments deliberately, recording why a second resolver must not be reintroduced.
- [x] 5.2 Asserted mechanically: excluding tests, `X-Forwarded-For`/`X-Real-IP` are read in exactly one function, `clientIPWithTrustedProxies`. `ProxyConfig.ClientIP` is the only caller-facing entry point to it.
- [x] 5.3 Untrusted peer + forged XFF → response does **not** reflect the forged location.
- [x] 5.4 Trusted peer + XFF → resolves to the forwarded address; the legitimate Caddy path still works.
- [x] 5.5 Two clients through one trusted proxy land in **distinct** buckets — asserted by driving `rateLimitMiddlewareWithConfig`, the production middleware itself. ⚠️ The first attempt at this test **reimplemented** the middleware instead of calling it and would have passed with production wiring broken; see §8.2.
- [x] 5.6 An untrusted peer cannot rotate `X-Forwarded-For` to escape its own bucket — asserted through the production middleware. Plus a 429 contract test (status, `Content-Type`, `Retry-After`).
- [x] 5.7 `location: null` still returned with `200` when the reader is absent or the lookup fails.
- [x] 5.8 Invalid coordinate → `400` **naming the offending parameter**, asserted across 9 handler cases. ⚠️ The earlier assertion accepted any non-empty message, which a generic "bad request" would have satisfied while breaking the contract.
- [x] 5.9 Full suite green — every package `ok`. ⚠️ **Exact command recorded, because the shorthand was wrong:** `./scripts/test-api.ps1 -GoTestArgs '-count=1','./...'`. Writing `scripts/test-api.ps1 -count=1` would suppress the script's default `./...` and test only the current directory — review round 3 caught that the record implied a wider run than the command performs. ⚠️ The final runs used native `go test -count=1 ./...` because Docker Desktop was down; `-race` cannot run on this host at all (no gcc, and the Docker path lacks a cgo toolchain), so CI (`ci-cd.yml:57`) is the only `-race` coverage. Also `go build ./...`, `go vet ./internal/handlers/`, `openspec validate fix-unify-client-ip-resolution --strict` (**valid**), `openspec validate --specs` (1 passed), and the sentinel gate. ⚠️ Windows AppLocker blocks bare `go test` binaries here; the script is the supported path and does **not** run `-race`.

## 7. Independent review round 1 — verdict BLOCK, all findings fixed

`/sol-review` (gpt-5.6-sol, `/openspec-review` framing) **blocked** this change. It was right on every count. Fixes:

- [x] 7.1 **NaN accepted as a coordinate (P2).** `strconv.ParseFloat("NaN")` succeeds and **every comparison against NaN is false**, so the range check passed it through; `json.Marshal` then failed *after* `WriteHeader(200)`, returning **200 with a truncated body** — the exact opposite of the 400 contract written in the same change. Verified independently before fixing:
  ```
  NaN    parsed=NaN   err=false   passes lat range check=true
  json encode error: json: unsupported value: NaN
  ```
  Fixed in `parseCoordinate` with `math.IsNaN`/`math.IsInf`, plus 5 parser cases and handler-level 400 assertions. (`Inf` was already caught by the range check; rejected explicitly anyway.)
- [x] 7.2 **Tests stopped at the planning helper (P2).** They would have passed if `GetContext` stopped calling it — the same wiring defect this change exists to fix. Introduced a `GeoLookup` interface so handler behaviour is testable without an mmdb, and `TestGetContextIgnoresForgedHeadersFromUntrustedPeer` now drives `GetContext` and asserts the **decoded response body**.
- [x] 7.3 **Task 5.5 was marked complete for a test that did not exist (P2).** The worst finding: a box ticked from the plan rather than the code. Written for real in `ratelimit_identity_test.go` — two clients through one trusted proxy get distinct buckets, and an untrusted peer cannot rotate `X-Forwarded-For` to escape its own.
- [x] 7.4 **§2.6 / §6.3 configuration read per request (P2).** Now `ContextConfig` + `ProxyConfig`, loaded once via `LoadContextConfig()` in `main` and injected into a `ContextHandler` struct. The rate limiter parses its CIDRs once at construction instead of per request. Behaviour is unchanged; the env reads moved to startup.
  ⚠️ Introduced a `GeoLookup` interface, so `main` must guard the **typed-nil trap**: a nil `*geoip.Reader` assigned to the interface is a *non-nil* interface value and would dereference on first lookup. Guarded explicitly and commented at the call site.
- [x] 7.5 **Tests inherited ambient env (P3).** Config injection removes the dependency entirely — no test reads the environment. **Proven** by running the package with hostile values set: `DEV_FORCE_LAGOS=true DEV_OVERRIDE_IP=8.8.8.8 TRUSTED_PROXY_CIDRS=10.0.0.0/8` → `ok`. Also added the missing failed-lookup case and restored the malformed-`RemoteAddr` case.
- [x] 7.6 **`GeoLocation` schema description was now false (P3).** It claimed the location is always MaxMind-derived; it can be caller-supplied. Rewritten to point at `location_source` and to state that `country`/`state` are empty for explicit coordinates. Resynced.
- [x] 7.7 **Literal `extractIP` criterion (P3).** Amended — see 5.1.

**Not adopted, with reason:** none. Every finding was accepted.

⚠️ Review also **cleared** the security reasoning: "no P0 understatement was found." It independently confirmed `/v1/context` was never a rate-limit bypass, the resolved IP is never logged, and — a fact this change had not credited — Caddy already overwrites both forwarded headers with `{remote_host}` (`deploy/Caddyfile.example:5-7`).

⚠️ Review could not run `-race` locally (the Docker test path lacks a cgo toolchain) and left CI's status unverified. **Closed here:** `ci-cd.yml:57` runs `go test -race ./...` and `build-and-test` passed on PR #225.

## 8. Independent review round 2 — verdict BLOCK, all findings fixed

Round 2 (`/sol-review`, `/openspec-review` framing) confirmed **no over-correction** of the round-1 fixes: *"Amending the literal `extractIP` criterion did not hide a failure — the stricter original grep is currently clean."* Security reasoning cleared again. It found seven more:

- [x] 8.1 **The spec promised "open events"; the query returns all statuses (P2).** Verified: `GetNearbyEvents` filters on `geom IS NOT NULL` and `ST_DWithin` only — **no status predicate has ever existed**. The phrase was inherited from the live requirement and copied forward unchecked; archiving would have enshrined a knowingly false spec.
  **Decision (b): amend the requirement, do not narrow the query.** Closed-event ingestion was added deliberately (#148) — a recently-closed flood nearby is still situational awareness — `status` is on every event so clients can distinguish, and filtering would have silently removed data callers see today, bundled into an unrelated change. Requirement reworded and a scenario added.
- [x] 8.2 **The rate-limit test reimplemented the middleware (P2).** `limiterFor` rebuilt an equivalent handler, so both tests would still pass if production keyed on the proxy, the peer, or a constant. ⚠️ **This is the same defect class the change exists to fix, one layer up** — a test that duplicates wiring cannot detect the wiring being wrong. Extracted `rateLimitMiddlewareWithConfig`, which `rateLimitMiddlewareFromEnv` now calls; tests drive that exact function.
- [x] 8.3 **200 committed before encoding (P3).** ⚠️ **My own comment was false.** It asserted "every float reaching this point is finite" — but `GeoLookup` results and `NearbyEvents` carry floats this handler never validates, so malformed dependency or database data could still yield a truncated 200. Now marshals to a buffer first and returns 500 on failure; covered by a test feeding `Inf`/`NaN` from the GeoIP stub.
- [x] 8.4 **§8.6 — package-global `slog` (P3).** `*slog.Logger` injected into `ContextHandler` and `LoadContextConfig`, nil → `slog.Default()`, matching `NewEventHandler`/`NewDigestHandler`.
- [x] 8.5 **New inaccuracy introduced while fixing one (P3).** The round-1 `GeoLocation` rewrite claimed IP-derived `country`/`state` are populated. `reader.go` sets `State` only when MaxMind returns a subdivision, so it can be empty. Corrected and resynced.
- [x] 8.6 **400 tests accepted any message; `::1` case absent (P3).** Both fixed — see 5.8 and 1.2.
- [x] 8.7 **The task record itself had gone false (P3).** See 1.2, 2.2, 2.4, 5.2.

## 9. Independent review round 3 — verdict BLOCK, all findings fixed

- [x] 9.1 **🚨 P1: I shipped an invalid OpenAPI contract, and every gate passed.** A multi-line `description:` block lost its indentation, so **both copies became unparseable YAML** — the document served at `/openapi.yaml` and rendered by `/docs`. `npm run sync:openapi` copies bytes without parsing; `git diff --exit-code` compared two equally-broken files; `openspec validate --specs` does not read the embedded YAML; `go build`/`vet`/`test` never touched it. **Nothing automated could see it.** Verified with PyYAML: `mapping values are not allowed here`.
  Fixed the indentation **and the blind spot**: `openapi_contract_test.go` parses the **embedded** copy — the bytes actually served — and asserts `lat`/`lng` survive as real parameters, that the `400` is declared, and that `location_source` is required. ⚠️ **Proven by re-breaking the file**: the gate fails with `yaml: line 401: mapping values are not allowed in this context`, and passes once repaired. Uses `gopkg.in/yaml.v3`, already in `go.mod` — no new dependency.
- [x] 9.2 **Undisclosed fallback location (P2).** With no resolvable location the handler still queried events around the hardcoded geographic centre of Nigeria, so a caller could receive events from a place the response never named while reporting `location: null, location_source: unavailable`. **That is the exact class this change exists to remove**, and the live spec already required an empty list. The query is now skipped entirely; asserted by a test that fails if the repository is called at all.
- [x] 9.3 **Database errors became successful empty responses (P2).** `if err == nil { … }` turned an outage into "no events near you" — indistinguishable from the real thing, and the worst possible answer for a safety product (§4.5, §6.4). Now logged and returned as a sanitized 500, with a test asserting the upstream error does not leak.
- [x] 9.4 **`design.md` was still false (P2).** It claimed the rate limiter was untouched — it was refactored for §2.6/§8.6 — and still carried the literal zero-`extractIP` criterion. Both corrected in place, with the resolution-behaviour guarantee restated and pointed at the tests that now prove it.
- [x] 9.5 **§8.6 injection was incomplete (P3).** `LoadProxyConfig`, `trustedProxyCIDRsFromEnv` and the rate-limit constructor still used package-global `slog`. All now take an injected logger, nil → `slog.Default()`.
- [x] 9.6 **The recorded test command was wrong (P2, part).** See 5.9.

⚠️ **Standing lesson from this round, worth more than the fix:** four green gates and a full passing test suite certified a contract that no parser could read. *Green gates only prove what they actually check.* The fix that matters is 9.1's parse test, not the indentation.

## 10. Independent review round 4 — verdict BLOCK, all findings fixed

- [x] 10.1 **The contract omitted the 500 the handler returns (P2) — and my own new gate passed it.** Round 3 added sanitized 500s; the OpenAPI declared only 200/400. ⚠️ **The gate I wrote in 9.1 to catch contract lies did not catch this one**, because it checked syntax and a few key names. Declared the 500, and **strengthened the gate to assert every status the handler can return is declared**, that `lat`/`lng` are real parameters with `in`/`type`/`minimum`/`maximum` matching `parseCoordinate`, and that the `location_source` enum covers every constant. **Proven by deleting the 500 again**: `/v1/context does not declare a 500 response, but the handler returns one`.
  ⚠️ **Still not full OpenAPI 3.1 schema validation** — that needs a validator dependency and is not done here. Stated rather than implied.
- [x] 10.2 **The spec delta dropped the empty-array guarantee (P2).** Round 3 changed production specifically to stop querying around an undisclosed centre, but the delta's unavailable-location scenario no longer required `nearby_events: []` — archiving it would have deleted the requirement justifying the change. Restored, plus a new scenario requiring the failed-query 500 rather than a disguised empty 200.
- [x] 10.3 **Stale metadata and 11 dead links (P2).** ⚠️ **I found these independently before the review returned, and the cause is instructive:** `proposal.md` and `design.md` were written at `openspec/proposals/` and `openspec/specs/` (two levels deep), then `git mv`'d into `openspec/changes/<id>/` (three levels deep). My "all links resolve" check ran **before the move** and was never re-run — the same shape as the 9.1 P1: a check true at the moment it ran, invalidated later, never repeated. All links repaired, `status:` and `branch:` corrected, and the round count fixed.
- [x] 10.4 **§8.6 injection was still incomplete (P3).** `rateLimitMiddlewareFromEnv` still called `slog.Default()` and `main` passed `nil`. `main` now builds scoped loggers (`subsystem=context`, `subsystem=ratelimit`) and injects them; `RateLimitMiddleware`/`GlobalRateLimitMiddleware` take a logger.
- [x] 10.5 **§10.2/§10.4 dependency governance (P3).** `gopkg.in/yaml.v3` became a direct dependency without rationale or a list update.
  **§10.2 rationale:** the embedded OpenAPI document is served to clients and must be machine-parseable; nothing in the repo verified that, and an unparseable contract shipped as a result. Go's stdlib has no YAML parser. `yaml.v3` was **already present as an indirect dependency** (v3.0.1, in `go.sum`), so this adds no new external surface — only a direct import. Alternatives considered: a full OpenAPI validator (heavier, and pulls a larger tree for a syntax guard), or an npm validator in CI (a second toolchain for a Go-embedded asset). **Test-only** — imported solely by `openapi_contract_test.go`, and must not appear in non-`_test.go` files, matching the `testcontainers-go` precedent. `docs/standards/developers-go.md` §10.4 updated.

## 6. Explicitly not in this change

- ~~Narrowing `TRUSTED_PROXY_CIDRS` off `172.16.0.0/12` and measuring the real gateway~~ — `chore-vps-access-hardening` task 5.2. ⚠️ Until that lands, this raises the bar from *anyone* to *anything on the Docker bridge*, which includes `prod-umami`.
- ~~Network separation for Umami~~ — same change, task 3.3.
- ~~Removing the `DEV_*` overrides~~ — flagged in the proposal, belongs with a config-hygiene pass.
- ~~Reverse-geocoding explicit coordinates to country/state~~ — no such resolver exists; would be new capability.
