---
id: chore-operator-runbook-accuracy
status: proposed
branch: tbd
---

# Proposal: Fix the Operator Runbook Commands That Do Not Work (chore-operator-runbook-accuracy)

## Why

Two independent adversarial reviews of PR #231 (host-key pinning) surfaced defects in
[`docs/deployment/staging-production-topology.md`](../../docs/deployment/staging-production-topology.md)
that are **outside that change's scope** but real, pre-existing, and verified against the repository.

They matter more than ordinary doc drift because of *when* they are read. This is the **incident
runbook** — the commands are reached for when production is misbehaving, under time pressure, by
someone who will trust them. Every one of the commands below either fails or silently inspects the
wrong thing.

⚠️ These were **deliberately not fixed in #231** to keep a security change focused. Recorded here so
they do not become another `chore-post-v11-deferred-b6` — real work that lived only in a review
comment and stayed invisible for versions.

## What Changes

### 1. Every operator command targets the wrong Compose file

The runbook uses `/opt/vigilafrica/staging/docker-compose.yml` at lines 98, 104, 105, 106 and 112.
That is the **development** Compose file. The staging stack is deployed from
`docker-compose.staging.yml` (see `deploy-staging.yml`), and production from
`docker-compose.prod.yml`.

Verified service names:

| File | Services |
|---|---|
| `docker-compose.yml` (dev) | `postgres`, `api`, `geoipupdate`, `umami` |
| `docker-compose.staging.yml` | `staging-db`, `staging-api`, `staging-geoipupdate`, `staging-umami` |
| `docker-compose.prod.yml` | `prod-db`, `prod-api`, `prod-geoipupdate`, `prod-umami` |

So the documented commands fail in three distinct ways:

- `logs db` — **there is no `db` service in any file** (dev calls it `postgres`)
- `logs caddy` — **there is no `caddy` service in any file** (see item 2)
- `logs api` — resolves against the *dev* file rather than the running staging/production project,
  so at best it inspects the wrong Compose project and at worst reports nothing while appearing to work

The last is the dangerous one: it produces plausible output instead of an error.

**Fix:** use the environment-specific file and its real service names, and make the
"replace `staging` with `production`" instruction explicit about the file *and* the service prefix,
since both change together.

### 2. Caddy is described as a Compose service; it is a host service

Line 133 states Caddy "auto-reloads on config change inside the compose stack". Caddy is **not
defined in any of the three Compose files**. [`deploy/provision.sh`](../../deploy/provision.sh)
installs it from the Cloudsmith apt repository and runs it under systemd
(`systemctl enable --now caddy`).

The reload procedure during an incident is therefore a host-level one
(`systemctl reload caddy` / `caddy validate`), not anything Compose does.

⚠️ Related known drift already recorded in memory: the **live Caddyfile differs from
`deploy/Caddyfile.example`**. Whatever this section ends up saying should acknowledge that, or the
next person will diff the two and assume the example is authoritative.

## Impact

- **Risk:** low to make, but it is incident-time documentation, so the value is disproportionate.
- **Blast radius:** documentation only. No workflow, container or host change.
- **Size:** one small PR. No dependency on `chore-vps-access-hardening`, though it touches the same
  file that task 1.7 and 2.5 will eventually revisit — worth doing **before** those, so they are not
  editing text that is already wrong.

## Out of Scope

- The host-level boundary and SSH-pinning corrections — already shipped in #231.
- Anything in `chore-vps-access-hardening` groups 1–5.
- Reconciling the live Caddyfile with the example (note it; do not attempt it here).

## Verification

Not "read it again" — each command must be **executed** against the real stack:

- [ ] Every command in the operator runbook runs successfully on staging and returns output about
      the staging stack specifically.
- [ ] `logs`, `ps`, and the Caddy reload procedure each demonstrated, with the exact service names
      as written.
- [ ] A grep confirms no remaining reference to `/opt/vigilafrica/*/docker-compose.yml`.
