---
id: fix-unify-client-ip-resolution
status: implemented — decision (B); four independent review rounds absorbed
branch: fix/unify-client-ip-resolution
---

# Proposal: Unify the Two Client-IP Resolution Paths (fix-unify-client-ip-resolution)

## ⚠️ Read this first: it is NOT the vulnerability it was first called

This was surfaced during the independent review of [`chore-vps-access-hardening`](../chore-vps-access-hardening/proposal.md) and initially described as "a live security defect." **Investigation does not support that.** The corrected assessment is below, because starting this work from the wrong premise would produce the wrong fix.

What is true: **there are two different client-IP resolution policies in one service, and one of them trusts forwarded headers from anybody.**

| | resolver | trusts `X-Forwarded-For` from | used by |
|---|---|---|---|
| guarded | `clientIP()` — [`middleware.go:274-298`](../../../api/internal/handlers/middleware.go) | **only a configured trusted proxy** | rate limiter, [`middleware.go:386`](../../../api/internal/handlers/middleware.go) |
| unguarded | `extractIP()` — [`context.go:74-94`](../../../api/internal/handlers/context.go) | **anyone** | `/v1/context`, [`context.go:42`](../../../api/internal/handlers/context.go) |

Each has exactly one caller — verified by grep across `api/internal/`.

## What an attacker actually gains — and it is close to nothing

Traced end to end rather than assumed:

- `/v1/context` accepts **no** `lat`/`lng`/`country` query parameter — verified, the IP is its only location input. So forging `X-Forwarded-For` is functionally *an undocumented location parameter*, not an escalation.
- What it returns is **public, read-only, unauthenticated** data: a MaxMind location plus up to 5 events within 200 km ([`context.go:61`](../../../api/internal/handlers/context.go)).
- **Rate limiting is unaffected.** `/v1/context` sits behind `RateLimitMiddleware` ([`main.go:99,111`](../../../api/cmd/server/main.go)), which uses the **guarded** resolver — so a forged header cannot be used to evade or dilute rate limits. This is the protection that actually matters, and it works today.
- The resolved value is **not logged, persisted, or forwarded** anywhere — verified: `extractIP()`'s return feeds only `geo.Lookup()`. So there is no log- or analytics-poisoning path.

**Net: the practical exploit is "you can look at events near a place other than where you are."** For a public safety map, that is arguably a feature.

## So why fix it

Three reasons, none of them "we are being exploited":

1. **It is a trap for the next change.** Two similarly-named helpers, one safe and one not, with nothing marking the difference. The next IP-dependent feature — abuse controls, per-region quotas, personalisation, an audit trail — has a 50% chance of picking the unguarded one, and *then* it is a real vulnerability. This is the whole argument.
2. **The comment on the unsafe one is actively misleading.** [`context.go:73`](../../../api/internal/handlers/context.go) says it accounts *"for reverse proxies (Vercel/Caddy)"* — describing intent, while the guarded resolver's comment correctly states the constraint. A reader comparing them cannot tell which is deliberate.
3. **It contradicts a requirement this project just adopted.** `chore-vps-access-hardening` merged a spec delta requiring that forwarded headers be honoured *"only when the immediate peer is a trusted proxy"*, on **"every endpoint that resolves a client IP, including geolocation as well as rate limiting."** The code does not meet the spec it now ships with.

## ✅ Decision: (B) — close the gap AND add an explicit location parameter

If header-spoofing is effectively the only way to ask "what is happening near X," then closing it alone would **remove a capability without replacing it.** Users on VPNs, corporate proxies, or mobile carrier NAT already get poor IP geolocation; option (A) would have left them no recourse.

**Chosen 2026-08-08:**

| | |
|---|---|
| ~~(A) Close it, accept the loss~~ | rejected — silently removes a capability real users depend on |
| **(B) Close it, add explicit `lat`/`lng` with IP as fallback** | **chosen** — turns an accidental behaviour into a designed one |

**What (B) commits this change to:**

- The public API contract changes. [`openspec/specs/vigilafrica/openapi.yaml`](../../specs/vigilafrica/openapi.yaml) is the source of truth; edit it and run `npm run sync:openapi`. ⚠️ CI hard-fails on drift between it and `api/internal/handlers/openapi.yaml` ([`ci-cd.yml:48-49`](../../../.github/workflows/ci-cd.yml)).
- A bounds-validation path, which is new surface. An out-of-range coordinate must **400**, never silently fall back to IP — a typo must not be indistinguishable from a working query.
- A source discriminator in the response, so a client can tell "we geolocated you" from "you asked about here." This is what makes the fallback honest rather than invisible, and it is the same disclosure principle `feature-events-pagination` established for truncated results.

**Consequence for scope:** this is no longer a one-file fix. It is a contract change with a validation path and a regenerated spec, and it should be reviewed as one.

## Relationship to `chore-vps-access-hardening`

This **supersedes task 5.1** of that change. Splitting it out because that change has **21 tasks, none started**, and its group 1 is infrastructure work with a different risk profile, different reviewers, and a different timeline. A one-file code fix should not wait on an SSH-hardening programme.

⚠️ Its **task 5.2 stays where it is** — measuring the real proxy peer and narrowing `TRUSTED_PROXY_CIDRS` off the broad `172.16.0.0/12` is infrastructure work, and it is what makes the guarded path *meaningfully* guarded. **Until 5.2 lands, unifying the resolvers raises the bar from "anyone" to "anything on the Docker bridge"** — which, as that proposal records, includes the public-facing Umami container. Worth doing anyway; not worth overstating.

## Also found, not in scope

[`context.go:30-31`](../../../api/internal/handlers/context.go) reads `DEV_OVERRIDE_IP` and `DEV_FORCE_LAGOS` from the environment and lets them override all IP logic in the production binary. **Verified NOT set in either compose file**, so not currently exposed. Flagged rather than fixed here; belongs with a wider config-hygiene pass.

## Out of Scope

- Narrowing `TRUSTED_PROXY_CIDRS` and measuring the real gateway — `chore-vps-access-hardening` task 5.2.
- Network separation for Umami — same change, task 3.3.
- Removing the `DEV_*` overrides.
- Any rate-limiter behaviour change; the limiter is already correct.

## Open Questions

1. ~~(A) or (B)?~~ **Settled 2026-08-08: (B).**
2. What should an explicit coordinate do when it is out of range, or in the sea? Reuse the bbox validation from `fix-ingest-bbox-validation`, or clamp?
3. ~~Should `/v1/context` disclose which source it used?~~ **Settled by (B): yes, it SHALL.** Remaining detail: the exact field name and vocabulary (`"source": "ip" | "explicit"`), which is an OpenAPI naming call for `/openspec-apply`.
4. Does an explicit coordinate outrank `DEV_OVERRIDE_IP`? The precedence table must be written down before it is coded — the existing override ordering at `context.go:29-43` is load-bearing and easy to break while moving code around it.
