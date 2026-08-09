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
- [x] 1.2 Table-driven over `(peer, headers, expected IP)`: untrusted peer + forged XFF; untrusted peer + forged `X-Real-IP`; trusted peer + XFF; trusted proxy taking the leftmost XFF entry; trusted peer + `X-Real-IP` only; no headers; IPv6 loopback peer; untrusted IPv6 peer.
  ⚠️ **Deviation from this task as originally written, recorded rather than glossed.** It said to call `clientIPWithTrustedProxies` directly and avoid the environment. Following 1.1, the tests target `resolveLocationPlan` instead — and that function reads `TRUSTED_PROXY_CIDRS` internally, so the env var is unavoidable. Used `t.Setenv`, which the stdlib restores per-test and refuses to run under `t.Parallel()`, so the isolation the original wording wanted still holds — by a different mechanism. Testing the correct-but-uncalled function was the whole trap.
  ⚠️ **Dropped from the original list: malformed `RemoteAddr`.** `resolveLocationPlan` returns it verbatim (via `remoteAddrIP`'s `SplitHostPort` error path), which is existing `clientIP` behaviour this change does not alter; asserting it here would test the untouched path, not this change.

## 2. Unify the resolver

- [x] 2.1 Delete `extractIP()` from `context.go` **entirely** — no deprecated alias. A second name for the same behaviour reintroduces the ambiguity this change exists to remove.
- [x] 2.2 Route `/v1/context` through the shared `clientIP()` / `clientIPWithTrustedProxies`. Do **not** write a third resolver.
- [x] 2.3 Replace the misleading comment (`context.go:73` described intent — "accounting for reverse proxies") with one stating the trust constraint.
- [x] 2.4 Add a doc comment on `clientIP` recording that it is now the **only** client-IP path in the package.
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
- [x] 5.2 Asserted mechanically: `grep -rn "X-Forwarded-For\|X-Real-IP" api/internal/ --include=*.go` (excluding tests) matches **only** `middleware.go:293,302`, both inside `clientIPWithTrustedProxies`.
- [x] 5.3 Untrusted peer + forged XFF → response does **not** reflect the forged location.
- [x] 5.4 Trusted peer + XFF → resolves to the forwarded address; the legitimate Caddy path still works.
- [x] 5.5 Rate-limit bucketing unchanged — two clients through a trusted proxy still land in distinct buckets (regression guard on the untouched path).
- [x] 5.6 An untrusted peer cannot forge a rate-limit identity (existing behaviour, now explicitly asserted).
- [x] 5.7 `location: null` still returned with `200` when the reader is absent or the lookup fails.
- [x] 5.8 Out-of-range coordinate → `400` naming the parameter.
- [x] 5.9 `scripts/test-api.ps1` green — every package `ok`, handlers `0.029s`. Also `go build ./...`, `go vet ./internal/handlers/`, `openspec validate fix-unify-client-ip-resolution --strict` (**valid**), `openspec validate --specs` (1 passed), and the sentinel gate. ⚠️ Windows AppLocker blocks bare `go test` binaries here; the script is the supported path and does **not** run `-race`.

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

## 6. Explicitly not in this change

- ~~Narrowing `TRUSTED_PROXY_CIDRS` off `172.16.0.0/12` and measuring the real gateway~~ — `chore-vps-access-hardening` task 5.2. ⚠️ Until that lands, this raises the bar from *anyone* to *anything on the Docker bridge*, which includes `prod-umami`.
- ~~Network separation for Umami~~ — same change, task 3.3.
- ~~Removing the `DEV_*` overrides~~ — flagged in the proposal, belongs with a config-hygiene pass.
- ~~Reverse-geocoding explicit coordinates to country/state~~ — no such resolver exists; would be new capability.
