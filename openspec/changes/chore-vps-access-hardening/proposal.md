# Proposal: Harden the VPS Deploy Path (chore-vps-access-hardening)

**Status:** Proposed (0/21 tasks)

## Why

A review of the committed deployment surface — [`deploy/provision.sh`](../../../deploy/provision.sh),
both deploy workflows, and the two compose files — found that **the deployment
credential, not the container runtime, is the weakest link.**

Three exposures, in severity order:

1. **A leaked `VPS_SSH_KEY` grants host root.** [`provision.sh:36`](../../../deploy/provision.sh)
   adds `deploy` to the `docker` group. Docker documents that group membership as
   root-equivalent — anyone holding the key can mount the host filesystem into a
   privileged container.
2. **Host keys are never verified.** Both workflows run
   `ssh-keyscan -H "$VPS_HOST" >> ~/.ssh/known_hosts`
   ([`deploy-production.yml:51`](../../../.github/workflows/deploy-production.yml),
   [`deploy-staging.yml:20`](../../../.github/workflows/deploy-staging.yml)) — trust-on-first-use
   re-established on **every run**, which is not verification. An attacker able to
   MITM or DNS-hijack the runner harvests the key and the session.
3. **Production builds on the production host.** `docker compose up -d --build`
   ([`deploy-production.yml:65`](../../../.github/workflows/deploy-production.yml)) runs a full Go
   toolchain and `go mod download` on prod, under the root daemon.

And a fourth, found while checking whether the first three were actually closed:

4. **One `deploy` account owns both environments.** [`provision.sh:33-47`](../../../deploy/provision.sh)
   creates a single user and gives it `/opt/vigilafrica/staging` *and*
   `/opt/vigilafrica/production`. Staging auto-deploys on every push to `main` with no
   reviewer; production requires one ([`vps.md:130-133`](../../../docs/deployment/vps.md)).
   At the host level they are the same account — so **production's required-reviewer gate
   is not a real boundary.**

Related: the tag-ancestry check at
[`deploy-production.yml:38-43`](../../../.github/workflows/deploy-production.yml) runs on the
GitHub runner. Someone using a stolen key over SSH never executes that workflow, so it is a
workflow-integrity control, **not** a key-compromise control. It has to be re-validated
server-side to count for anything.

## What is already good, and is not being changed

Recorded so this proposal is not mistaken for a general indictment:

- Every published port binds `127.0.0.1` ([`docker-compose.prod.yml:59`](../../../docker-compose.prod.yml),
  [`:116`](../../../docker-compose.prod.yml)). Docker publishes ports by DNAT in `PREROUTING`,
  which never traverses ufw's `INPUT` chain — so a `0.0.0.0` publish would be reachable
  regardless of `ufw status`. Loopback binding behind the host Caddy is the documented
  mitigation, and it is already in place.
- The API container runs `read_only`, `cap_drop: ALL`, `no-new-privileges`, `tmpfs /tmp`
  ([`docker-compose.prod.yml:64-70`](../../../docker-compose.prod.yml)) on a non-root image user
  ([`api/Dockerfile:29-35`](../../../api/Dockerfile)).
- All images are digest-pinned with CI enforcement (`scripts/check-image-pins.js`).
- ufw default-deny, fail2ban, unattended-upgrades ([`provision.sh:49-57`](../../../deploy/provision.sh)).

## Decision: do NOT migrate to Podman, and record why

Podman was evaluated on the premise that daemonless is safer than a root daemon. The premise
is sound in general and **rejected for this stack right now**, for reasons that should not have
to be rediscovered:

| | Verdict |
|---|---|
| Closes the root-daemon exposure | ✅ genuinely |
| Cheaper than the tasks below | ❌ far more expensive, and riskier |
| Fixes exposures 1–4 | ❌ **none of them** — they are credential-path defects, not runtime defects |

### ⚠️ The edge case that would bite silently

[`middleware.go:278-298`](../../../api/internal/handlers/middleware.go) honours `X-Forwarded-For`
only when the **direct peer** matches `TRUSTED_PROXY_CIDRS` (default
`127.0.0.1/8,::1/128,172.16.0.0/12` — the `172.16.0.0/12` entry exists because Caddy's connection
arrives from the Docker bridge gateway).

Rootless port forwarding does not present that address. Rootless bridge networks default to
`rootlessport`, a userspace proxy that does **not** preserve client source IPs; preserving them
requires `rootless_port_forwarder = "pasta"` in `containers.conf`. Either way the observed peer
changes.

Consequence: `isTrustedProxy` returns false → `clientIP()` falls back to the proxy's own address →
**every request in the world shares one rate-limit bucket**, and `/v1/context` geolocates everyone
identically. Nothing crashes, `/health` still returns 200, and the production smoke test
([`deploy-production.yml:73-74`](../../../.github/workflows/deploy-production.yml)) checks only
`status` and `version` — so it passes clean.

⚠️ **This applies to rootless Docker identically.** It is a property of rootless port forwarding,
not of Podman. Rootless is therefore not the "cheap" version of this migration in either engine.

### Other Podman edge cases specific to this stack

- **Healthchecks are systemd transient timers** using `--user` in rootless mode. No systemd user
  session → no timers → no health status → `depends_on: condition: service_healthy`
  (used by `prod-api` and `prod-umami` against `prod-db`) never satisfies and the stack hangs.
  Compounded by `podman-compose`'s history of not enforcing `service_healthy`
  (containers/podman-compose#1119, #1183; addressed in 1.3.0 via PR #1082).
- **`restart: unless-stopped` has no supervisor.** Needs `loginctl enable-linger deploy` plus
  Quadlet units or `podman-restart.service`. Without linger, containers die when the SSH session
  ends — which the deploy workflow does on every run.
- **`vigil-prod-data` needs a UID remap.** Written by the postgis image's UID 999; rootless maps
  that into the subuid range, and Postgres refuses to start unless the data dir is `0700`/`0750`.
  On live data this means dump/restore, not an in-place `podman unshare chown` — and there is no
  tested off-box restore today (see task 4).
- **Two compose implementations with different bugs.** `podman compose` (subcommand, shells out to
  an external provider against the Podman socket) vs `podman-compose` (independent Python
  reimplementation). One must be chosen and pinned.

Not blockers: the Dockerfile is plain multi-stage with no BuildKit-specific syntax; all published
ports are >1024 and Caddy is a host package, so the privileged-port restriction never applies;
`cap_drop`/`no-new-privileges`/`read_only`/`tmpfs` all translate; there is no Swarm, so
"overlay networks unsupported" is irrelevant.

**Assumption:** cgroup v2 is already the default on the host's base image (Debian 12 / Ubuntu 22.04+),
which rootless needs for resource limits. **Unverified — I could not reach the box.** If it is
cgroup v1, task 3.2's limits are silently ignored under any rootless runtime.

**Revisit Podman on a fresh VPS build**, with Quadlet units rather than compose, and with the peer
address **measured** on the box before cutover rather than assumed. Task 5 exists to make that
measurement cheap and to make the failure visible when it happens.

## What Changes

Infrastructure and CI only, in six groups — see [tasks.md](tasks.md). Sequenced so that the
cheapest, highest-severity, zero-data-risk items land first, and nothing is built on top of an
unverified step.

## Out of Scope

- Migrating to Podman or rootless Docker. Deferred with reasons above.
- Cursor-based anything, application features, web work.
- Multi-host / orchestrator topology. This stays a single VPS.
- `TRUSTED_PROXY_CIDRS` trusting the whole `172.16.0.0/12` Docker pool. It is **necessary** given
  the current topology and bounded by loopback publishing plus network separation — recorded as
  understood-and-accepted, not fixed here.

## ⚠️ Sequencing note

This competes directly with [feature-continental-coverage](../../proposals/feature-continental-coverage.md)
for the next slot. Nothing here is a prerequisite for that work; group 1 is ~a day and is worth
doing regardless, but groups 2–4 are a real diversion. **That trade is the maintainer's call, not
this document's.**

## Verification

- [ ] A key restricted per task 1.1 cannot obtain an interactive shell — attempted and denied
- [ ] Deploy fails closed against a wrong host key (task 1.2), verified by deliberately pinning a bad one
- [ ] The staging key cannot affect production paths, and vice versa (task 1.3)
- [ ] The server-side deploy script rejects a tag not reachable from `origin/release` (task 1.4)
- [ ] No Go toolchain or `go mod` cache is present on the production host after task 2
- [ ] Both stacks come up clean from cold with the new caps and limits (task 3), on staging first
- [ ] A backup restores into a scratch database and the row count matches (task 4)
- [ ] The rate limiter keys per client IP, asserted by a test, and the prod smoke test would now
      catch it if it stopped doing so (task 5)
