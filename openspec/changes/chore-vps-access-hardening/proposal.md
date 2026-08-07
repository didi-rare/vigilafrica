# Proposal: Harden the VPS Deploy Path (chore-vps-access-hardening)

**Status:** Proposed (0/20 tasks)

⚠️ **Revised after independent review.** The first revision of this document was reviewed by an independent model (`gpt-5.6-sol`) which found **7 P1 defects**, including a false code-path claim, two task-ordering defects that would have broken deployment, and an internal contradiction about network isolation. Every external runtime claim has since been checked against primary documentation, and **four of them were simply wrong** — corrections are marked ⚠️ inline. What survived is recorded here; what did not is recorded as a correction rather than silently deleted.

## Why

A review of the committed deployment surface — [`deploy/provision.sh`](../../../deploy/provision.sh),
both deploy workflows, and the two compose files — found that **the deployment
credential, not the container runtime, is the weakest link.**

1. **A leaked `VPS_SSH_KEY` grants host root.** [`provision.sh:36`](../../../deploy/provision.sh)
   adds `deploy` to the `docker` group. Docker documents that group membership as
   root-equivalent — anyone holding the key can mount the host filesystem into a
   privileged container.
2. **Host keys are never verified.** Both workflows run
   `ssh-keyscan -H "$VPS_HOST" >> ~/.ssh/known_hosts`
   ([`deploy-production.yml:51`](../../../.github/workflows/deploy-production.yml),
   [`deploy-staging.yml:20`](../../../.github/workflows/deploy-staging.yml)) — trust-on-first-use
   re-established on **every run**, which is not verification.
   ⚠️ **Corrected impact:** an earlier draft said an attacker "harvests the key". That is wrong.
   SSH public-key authentication never transmits the private key, and the client's signature is
   bound to the session identifier, so it cannot be replayed against the real host. The real
   impact is **host impersonation**: the attacker can read and alter the deployment session,
   and feed back false results. Serious, but not key theft.
3. **Production builds on the production host.** `docker compose up -d --build`
   ([`deploy-production.yml:65`](../../../.github/workflows/deploy-production.yml)) executes the
   build on prod. ⚠️ **Corrected scope:** the Go toolchain and module cache live in **Docker
   builder layers**, not as a host-installed toolchain — so the exposure is production-side build
   *execution*, plus a source checkout and a build cache. Switching to a remote image does not by
   itself remove existing builder layers; they must be pruned.
4. **One `deploy` account owns both environments.** [`provision.sh:33-47`](../../../deploy/provision.sh)
   creates a single user owning both `/opt/vigilafrica/staging` and `/opt/vigilafrica/production`.
   **This is the highest-ranked exposure**, because it collapses the boundary between an
   environment that deploys automatically and one that is meant to be gated.
   ⚠️ The gating itself is **unverified external state**: only [`vps.md:130-133`](../../../docs/deployment/vps.md)
   claims production requires a reviewer. The workflow YAML merely names the `production`
   environment; protection rules live in GitHub settings and could not be checked from the repo.
   Task 1.7 verifies it before anything relies on it.
5. **`/v1/context` trusts forwarded headers from anyone.** `extractIP()`
   ([`context.go:74-94`](../../../api/internal/handlers/context.go)) reads `X-Forwarded-For` and
   `X-Real-IP` with **no trusted-proxy check**, and it is the only IP path that endpoint uses
   ([`context.go:42`](../../../api/internal/handlers/context.go)). The trusted-proxy-aware
   `clientIP()` is used **only** by the rate limiter
   ([`middleware.go:386`](../../../api/internal/handlers/middleware.go)).
   **Two different IP-resolution policies coexist in one service.** Surfaced by the independent
   review; it is a live issue today and independent of any runtime migration.
6. **Docker is installed by piping a mutable URL into a root shell.**
   [`provision.sh:19-21`](../../../deploy/provision.sh) runs `curl -fsSL https://get.docker.com | sh`,
   unpinned. Docker's own documentation states the convenience script **"isn't recommended for
   production environments."** It also means the installed Docker version is unknown, which
   matters for the loopback guarantee below.

Related: the tag-ancestry check at
[`deploy-production.yml:38-43`](../../../.github/workflows/deploy-production.yml) runs on the
GitHub runner. Someone using a stolen key over SSH never executes that workflow, so it is a
workflow-integrity control, **not** a key-compromise control.

## What is good — with the caveats the review forced

- The API container runs `read_only`, `cap_drop: ALL`, `no-new-privileges`, `tmpfs /tmp`
  ([`docker-compose.prod.yml:64-70`](../../../docker-compose.prod.yml)) on a non-root image user
  ([`api/Dockerfile:29-35`](../../../api/Dockerfile)). No caveat; this is correct.
- All images are digest-pinned with CI enforcement (`scripts/check-image-pins.js`).
- Every published port binds `127.0.0.1` ([`docker-compose.prod.yml:59`](../../../docker-compose.prod.yml),
  [`:116`](../../../docker-compose.prod.yml)), which is the documented mitigation for Docker
  publishing ports via `PREROUTING` DNAT that never traverses ufw's `INPUT` chain.
  ⚠️ **Conditional, not absolute.** Docker documents: *"In releases older than 28.0.0, hosts
  within the same L2 segment (for example, hosts connected to the same network switch) can reach
  ports published to localhost."* Because Docker is installed unpinned (item 6), **the live
  version is unverified** and this guarantee is unproven on the actual host. Task 3.4 checks it.
- ufw is **enabled** with three services allowed ([`provision.sh:49-52`](../../../deploy/provision.sh)),
  and fail2ban and unattended-upgrades are installed.
  ⚠️ **Corrected:** an earlier draft called this "default-deny" and cited these lines. Those lines
  never set a default policy — the script runs `ufw allow` three times then `ufw --force enable`.
  The effective policy is **inherited from the image**, not established here.

## Decision: do NOT migrate to Podman — for narrower reasons than first claimed

The daemonless premise is sound and **fixes none of exposures 1–4**, which are credential-path
defects. That conclusion survives review. Several supporting arguments did not:

### ⚠️ The source-IP argument — corrected, and now narrower

Original claim: *"rootless port forwarding does not preserve client source IPs, so `isTrustedProxy`
returns false and `/v1/context` geolocates everyone identically."* **Two errors.**

1. **Wrong code path.** `/v1/context` never calls `clientIP()`, so the trusted-proxy check cannot
   affect it (see exposure 5). Only **rate limiting** would collapse into a single bucket.
2. **Wrong about pasta.** Source-IP behaviour is forwarder-dependent, not universal:
   `slirp4netns` + `port_handler=rootlesskit` rewrites the source (container sees `10.0.2.100`);
   **pasta — the default since Podman 5.0 — preserves it**; `slirp4netns` + `port_handler=slirp4netns`
   also preserves it. And source preservation *from the host's own loopback* is not enabled by
   default and needs explicit `--map-gw`/`--map-host-loopback`.

**What is actually true:** in a host-Caddy-to-loopback-published-port topology the peer address a
rootless container observes is **unpredictable from documentation and must be measured**. The
failure mode is real and silent if it occurs — `/health` returns 200 and the production smoke
test checks only `status` and `version` — but it is not the certainty first claimed.
The equivalent rootless-Docker outcome is **unverified**.

### ⚠️ Other Podman claims, corrected

- **`restart: unless-stopped`.** Original: *"has no supervisor."* **Overstated.** Podman's restart
  policy does restart containers after ordinary exits without systemd. The genuine gap is
  **logout and reboot persistence**, which needs `podman-restart.service` or lingering + Quadlet.
- **`depends_on: service_healthy`.** Original: *"addressed in 1.3.0 via PR #1082."* **Backwards.**
  PR #1082 added the feature in 1.3.0; issue **#1183 reports it broken in 1.3.0** (containers
  started before conditions were evaluated), and **PR #1184** is the fix. Any adoption must pin an
  exact provider version and test cold start.
- **Volume UID remap.** Original: *"on live data this means dump/restore, not an in-place remap."*
  **Unsupported as stated.** Podman documents `podman unshare` volume mounting and ownership
  remapping. Dump/restore is the *safer, testable* path — not the only viable one.
- **Healthchecks as systemd transient timers** requiring a user session: this claim stands.

Not blockers: the Dockerfile is plain multi-stage with no BuildKit-specific syntax; all published
ports are >1024 and Caddy is a host package; `cap_drop`/`no-new-privileges`/`read_only`/`tmpfs`
all translate; there is no Swarm.

**Revisit only on a fresh VPS build**, with Quadlet rather than compose, pinned versions, and the
peer address **measured** first. Task 5 makes that failure detectable either way.

## ⚠️ The trusted-proxy range is NOT adequately bounded — corrected

An earlier draft placed this out of scope as "necessary, and bounded by loopback publishing plus
network separation." **That contradicted this document's own finding**, and the independent review
caught it: `prod-api` and the public-facing `prod-umami` share the `prod-internal` network
([`docker-compose.prod.yml:101-130`](../../../docker-compose.prod.yml)). A compromised Umami
container can reach the API directly and **forge `X-Forwarded-For` from a peer inside the trusted
`172.16.0.0/12` range**. Umami is the largest attack surface here (public dashboard, Next.js).

The actual Caddy-to-container gateway address has also **never been measured**; `172.16.0.0/12`
was adopted as a broad guess. This is now in scope — see tasks 3.3 and 5.2.

## What Changes

Infrastructure and CI only, in six groups — see [tasks.md](tasks.md). Sequenced so the cheapest,
highest-severity, zero-data-risk items land first, and **so that no task removes something a
later task depends on** (a defect in the first revision).

## Out of Scope

- Migrating to Podman or rootless Docker. Deferred with corrected reasons above.
- Application features, web work, multi-host topology.

## ⚠️ Sequencing note

This competes with [feature-continental-coverage](../../proposals/feature-continental-coverage.md).
Group 1 is worth doing regardless; groups 2–4 are a real diversion. **The trade is the maintainer's
call, not this document's.**

## Verification

- [ ] A restricted key cannot obtain an interactive shell, and an unexpected `SSH_ORIGINAL_COMMAND` is rejected — attempted and denied
- [ ] Deploy fails closed against a deliberately wrong pinned host key
- [ ] The staging key cannot affect production paths, and vice versa
- [ ] The server-side deploy control rejects an unapproved release reference **after** the repo checkout is removed
- [ ] `/v1/context` no longer honours `X-Forwarded-For` from an untrusted peer, asserted by a test
- [ ] Rate limiting keys per client IP, asserted by a test, and the prod smoke test now fails if it stops doing so
- [ ] A compromised-Umami simulation cannot forge a trusted client IP against the API
- [ ] Both stacks come up clean from cold with the new caps and limits, on staging first
- [ ] A backup restores into a scratch database and the row count matches
- [ ] Docker version and effective container resource limits are recorded from the live host, not assumed
