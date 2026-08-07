# Proposal: Harden the VPS Deploy Path (chore-vps-access-hardening)

**Status:** Proposed (0/21 tasks)

⚠️ **Revised twice after independent review.** Two adversarial review passes (`gpt-5.6-sol`) found **7 P1 defects** in the first revision and **6 more** in the second. Corrections are marked ⚠️ inline rather than silently applied, because two of them were themselves wrong the first time.

**The most instructive:** revision 2 "corrected" the source-IP claim to say pasta preserves client addresses. That over-corrected — pasta is the default *network mode* for rootless containers, but Compose creates **bridge** networks, and rootless bridge publishing still defaults to `rootlessport`, which does **not** preserve source IPs. Revision 1 was closer to right. See the Podman section.

Every external runtime claim here has been checked against primary documentation. Where a claim could not be verified without host access, it is marked unverified rather than asserted.

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
   ✅ The gating rule itself **has now been verified to exist** via the public GitHub environments
   API — it is not merely asserted in [`vps.md:130-133`](../../../docs/deployment/vps.md). That
   makes the shared account the *only* thing defeating it, which raises rather than lowers this
   exposure's rank. Task 1.7 still confirms the reviewer set and rotation policy.
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

⚠️ **Corrected:** earlier revisions said daemonless "fixes none of exposures 1–4." **That is too
strong.** Exposure 1 *is* root escalation through the rootful Docker socket, and a completed
rootless migration removes that mechanism outright. What it does **not** fix is exposure 2 (host-key
verification), 3 (on-host builds), 4 (shared account across environments) or 5 (unguarded
`extractIP`) — four of the five, including the highest-ranked one.

So the honest statement is: **rootless would close one exposure that group 1 also closes, more
cheaply and with no data migration.** Deferral rests on cost and operational risk, not on the
migration being useless. Several supporting arguments were also wrong:

### ⚠️ The source-IP argument — corrected, and now narrower

Original claim: *"rootless port forwarding does not preserve client source IPs, so `isTrustedProxy`
returns false and `/v1/context` geolocates everyone identically."* **Two errors.**

1. **Wrong code path.** `/v1/context` never calls `clientIP()`, so the trusted-proxy check cannot
   affect it (see exposure 5). Only **rate limiting** would collapse into a single bucket.
2. **Right about the mechanism, then over-corrected about pasta.** Revision 2 claimed "pasta — the
   default since Podman 5.0 — preserves it," which **misapplies the default to the wrong topology.**
   Podman documents that pasta is the default *network mode* for rootless containers — but Compose
   creates **user-defined bridge networks**, and *"for rootless bridge networks, port forwarding
   uses `rootlessport` by default,"* which *"does not preserve client source IPs."* Switching to
   pasta forwarding requires `rootless_port_forwarder="pasta"` in `containers.conf` and is
   documented as **experimental**, *"subject to change."*

**What is actually true for this stack:** these compose files define bridge networks, so a rootless
migration would land on the **non-preserving** `rootlessport` path by default, and the peer address
the API observes would change. The escape hatch exists but is experimental. Whether the resulting
address happens to fall inside the configured trusted range is still **unmeasured** — but the
concern is real by default, not merely possible. The failure would be silent: `/health` returns 200
and the production smoke test checks only `status` and `version`.
The equivalent rootless-Docker behaviour remains **unverified**.

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
(`prod-api` at [`docker-compose.prod.yml:62-63`](../../../docker-compose.prod.yml), `prod-umami` at
[`:117-118`](../../../docker-compose.prod.yml) — the earlier `101-130` citation showed only Umami's
membership, not the API's). A compromised Umami
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
