# Spec: Unify Client-IP Resolution (fix-unify-client-ip-resolution)

**Status:** Implemented — decision (B); **four** independent review rounds absorbed (all BLOCK, all findings fixed). Round-by-round record in [tasks.md](tasks.md) §7–§10.

⚠️ **Deviations from this document, recorded rather than glossed:**
1. `clientIP()` was replaced by the injected `ProxyConfig.ClientIP` method during the round-1 refactor for §2.6/§6.3. This document still names `clientIP()`; the single-resolver property it required is unchanged, and `clientIPWithTrustedProxies` is still the only implementation.
2. `GetContext` became a method on `ContextHandler` (§6.3) rather than a closure-returning function, and gained an injected `*slog.Logger` (§8.6) and a `GeoLookup` interface so handler behaviour is testable without an mmdb.
3. The response is marshalled to a buffer before the status is committed, which this document did not anticipate — see tasks.md §8.3.
**Companion:** [`proposal.md`](proposal.md) (rationale, corrected severity, design question, out-of-scope).

## Context

`api/internal/handlers` contains two client-IP resolvers with different trust policies and one caller each:

```
clientIP()   middleware.go:274   guarded    -> rate limiter        middleware.go:386
extractIP()  context.go:74       UNGUARDED  -> GET /v1/context     context.go:42
```

`extractIP()` returns the first `X-Forwarded-For` entry, else `X-Real-IP`, else `RemoteAddr`, **without consulting the peer address**. `clientIP()` consults `TRUSTED_PROXY_CIDRS` first and falls back to the peer when it is untrusted.

The work is to collapse these into one resolver, keep the rate limiter's behaviour byte-identical, and decide what `/v1/context` offers in place of the capability that closing the gap removes.

⚠️ **`clientIPWithTrustedProxies` is the reusable core.** [`middleware.go:278`](../../../api/internal/handlers/middleware.go) already takes the CIDR list as a parameter — it is written to be testable and reusable. `clientIP()` is only the env-reading wrapper. Do **not** write a third resolver.

## Components to Touch

### Modified

1. **`api/internal/handlers/context.go`**
   - Delete `extractIP()` (lines 73–94) entirely. Do not keep it as a deprecated alias — a second name for the same behaviour reintroduces the ambiguity this change exists to remove.
   - Call the shared resolver at line 42.
   - ⚠️ Preserve the `DEV_OVERRIDE_IP` / `DEV_FORCE_LAGOS` precedence at lines 29–43 exactly. It is out of scope and its ordering is load-bearing: the override wins over the request, and `forceLagos` suppresses the lookup entirely.
   - Fix the misleading comment. State the trust constraint, not the intent.

2. **`api/internal/handlers/middleware.go`**
   - Rename nothing that is exported; `clientIP`/`clientIPWithTrustedProxies` are package-private and already correctly named.
   - Add a doc comment noting this is now **the only** client-IP path in the package, so a future reader does not add a second.

3. **`openspec/specs/vigilafrica/openapi.yaml`** — **required (decision B)**. Add the explicit location parameter(s) to `/v1/context`, then run `npm run sync:openapi` to regenerate `api/internal/handlers/openapi.yaml`. ⚠️ CI fails the build if these drift ([`ci-cd.yml:48-49`](../../../.github/workflows/ci-cd.yml)).

### New

4. **`api/internal/handlers/context_test.go`** — currently no test file exists for this handler. Coverage listed below.

### Untouched

- ~~The rate limiter. Its resolution is already correct; this change must not alter it.~~
  ⚠️ **This became false and is corrected here.** The limiter WAS refactored: it now parses its trusted-proxy CIDRs once at construction instead of per request (§2.6), calls the extracted `rateLimitMiddlewareWithConfig`, and takes an injected logger (§8.6). Its *resolution behaviour* is unchanged — same CIDRs, same precedence, same buckets — which is what "must not alter it" was protecting, and there are now tests driving the production composition to prove it.
- `TRUSTED_PROXY_CIDRS` defaults and parsing — `chore-vps-access-hardening` task 5.2 owns narrowing them.

## Behaviour

### Trust policy (both paths)

- A request whose peer is **not** in `TRUSTED_PROXY_CIDRS` SHALL have `X-Forwarded-For` and `X-Real-IP` ignored on **every** endpoint, and the peer address used instead.
- A request whose peer **is** trusted SHALL continue to resolve to the first `X-Forwarded-For` entry, else `X-Real-IP`, else the peer — unchanged from today's rate-limiter behaviour.
- Rate-limit bucketing SHALL be unchanged. This is a regression guard, not a feature.

### Explicit location (decision B)

- `/v1/context` SHALL accept an explicit client-supplied location, which takes precedence over IP-derived geolocation.
- An out-of-range or unparseable coordinate SHALL return `400` with a message naming the offending parameter — **not** a silent fallback to IP, which would make a typo indistinguishable from a working query.
- The response SHALL indicate which source was used, so a client can distinguish "we geolocated you" from "you asked about here."

## Acceptance Criteria

- [x] No second client-IP resolver exists: `grep -rn "func extractIP\|extractIP(" api/` returns nothing — no declaration, no call site.
  ⚠️ **Criterion amended.** As originally written ("`grep -rn \"extractIP\" api/` returns nothing") it was false and would stay false: historical prose comments deliberately name the removed helper to record why a second resolver must not return. Marking a false criterion complete is the defect; the criterion is corrected rather than the comments deleted.
- [ ] Exactly one client-IP resolution path exists in `api/internal/handlers`, asserted by a grep-based or vet-style check, not only by review.
- [ ] `GET /v1/context` with a forged `X-Forwarded-For` from an **untrusted** peer resolves to the peer address, and the response does **not** reflect the forged location.
- [ ] `GET /v1/context` with `X-Forwarded-For` from a **trusted** peer resolves to the forwarded address — the legitimate Caddy path still works.
- [ ] Two clients through a trusted proxy still land in **distinct** rate-limit buckets (regression guard on the untouched path).
- [ ] An untrusted peer cannot forge a rate-limit identity (existing behaviour, now explicitly asserted — [`middleware_test.go:43,58`](../../../api/internal/handlers/middleware_test.go) covers the header cases but not the untrusted-peer case).
- [ ] `DEV_FORCE_LAGOS=true` still short-circuits before any lookup; `DEV_OVERRIDE_IP` still wins over the request. Both asserted, since this change moves code around them.
- [ ] `/v1/context` still returns `200` with `location: null` when the GeoIP reader is absent or the lookup fails — the graceful-degradation path at [`context.go:45-49`](../../../api/internal/handlers/context.go) must survive.
- [ ] An explicit out-of-range coordinate returns `400` naming the parameter, **not** a silent IP fallback.
- [ ] `openapi.yaml` updated and `npm run sync:openapi` run; the CI drift check passes.
- [ ] The response discriminates IP-derived from client-supplied location, so degraded geolocation is distinguishable from a deliberate query.
- [ ] An explicit location takes precedence over IP, and over `DEV_OVERRIDE_IP`? — **resolve during apply**; the precedence table must be written down before it is coded.
- [ ] `scripts/test-api.ps1` green (⚠️ Windows AppLocker blocks bare `go test` binaries here; the script is the supported path and does **not** run `-race`).

## Testing Notes

⚠️ **The test that matters must fail before the fix.** A test asserting "untrusted peer cannot forge location" written against the current code SHALL be confirmed red first — the lesson from `feature-events-pagination` task 1.2, where an ordering test passed on the old code too and proved nothing.

Table-driven over `(peer address, headers, expected resolved IP)`, exercising: untrusted peer + forged XFF; trusted peer + XFF; trusted peer + `X-Real-IP` only; neither header; malformed `RemoteAddr`; IPv6 peer and `::1`.

⚠️ `clientIPWithTrustedProxies` takes the CIDR list as an argument — use it directly rather than setting `TRUSTED_PROXY_CIDRS` in the environment, so tests do not leak state into each other.

## Risks

| Risk | Mitigation |
|---|---|
| Narrowing trust breaks legitimate geolocation if `TRUSTED_PROXY_CIDRS` does not actually match Caddy's peer | The guarded path already runs in production for rate limiting, so the CIDR list is proven to match today. Confirm on staging before production. |
| The new parameter becomes an unbounded query surface (arbitrary coordinates, no rate distinction) | It is bounded by the existing per-client rate limiter, which this change does not touch. Confirm that holds — the limiter keys on IP, not on the queried location. |
| The guarded path is only as good as the CIDR list | Acknowledged and **not** solved here — `172.16.0.0/12` currently trusts the whole Docker bridge, including `prod-umami`. Owned by `chore-vps-access-hardening` tasks 3.3 and 5.2. |

## Sentinel

Touches `api/internal/` — a **critical path**. This change record must be in the implementing PR's own diff, or the PR must carry a `[trivial]` commit tag (it should not; this is not trivial).
