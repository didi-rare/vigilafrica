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

Sections **"Tail all logs (live)"**, **"Per-service logs"** and **"Container status"** all use
`/opt/vigilafrica/staging/docker-compose.yml` — five commands, at lines 78, 84, 85, 86 and 92 on
`development` as of 2026-08-14. (Cited by section as well as line because
`chore-vps-access-hardening` edits the same file and shifts the numbering.)

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

- `logs db` — **there is no `db` service in any Compose file** (the dev file calls it `postgres`)
- `logs caddy` — **there is no `caddy` service in any Compose file** (see item 2)
- `logs api` — `api` exists only in the *dev* file, while the running staging containers are
  `staging-api` etc. The command therefore describes a service that is not what is deployed

⚠️ **Unverified:** the exact runtime behaviour of the `logs api` case has *not* been observed on the
box. Because the deploy checks the repo out into `/opt/vigilafrica/staging`, the dev
`docker-compose.yml` **does** exist at that path, so the command will not fail on a missing file;
it resolves within the same Compose project name (`staging`, from the directory) but names a service
that is not running. The likely outcome is an error or empty output rather than misleading data —
but **that should be confirmed on the box, not assumed**, because "quietly returns nothing" and
"errors loudly" have very different consequences at 3am. Do not repeat the stronger claim that it
"returns plausible output" until someone has actually run it.

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

## Checked and found CORRECT — do not "fix" these

Audited at the same time, so the next person knows what was covered rather than re-deriving it:

- **Health-probe ports are right.** `localhost:8081` (staging) and `localhost:8080` (production)
  match the published ports in `docker-compose.staging.yml` (`127.0.0.1:8081:8080`) and
  `docker-compose.prod.yml` (`127.0.0.1:8080:8080`).
- **The external health URLs are right**, and correctly described as exercising DNS/TLS/Caddy.
- **The rollback section is right** — it defers to `release-process.md` and states rollback is a
  redeploy of a previous tag, never an in-place VPS edit. ⚠️ Note `release-process.md` itself still
  describes *checkout-based* rollback, which `chore-vps-access-hardening` task 2.5 will change; that
  is 2.5's problem, not this one's.

## Relationship to `chore-vps-access-hardening` (#231)

#231 edits this same file and fixes exactly two adjacent things: the false claim that GitHub
Environment reviewers gate direct SSH, and the unpinned `ssh` entry command. It changes **none** of
the commands above, so this proposal is the precise complement — no overlap, and no gap between them.

Verified by diffing `development` against the #231 branch for this file.

## Out of Scope

- The host-level boundary and SSH-pinning corrections — already shipped in #231.
- Anything in `chore-vps-access-hardening` groups 1–5.
- Reconciling the live Caddyfile with the example (note it; do not attempt it here).
- `release-process.md`'s checkout-based rollback — belongs to task 2.5.

## Verification

Not "read it again" — each command must be **executed** against the real stack:

- [ ] Every command in the operator runbook runs successfully on staging and returns output about
      the staging stack specifically.
- [ ] `logs`, `ps`, and the Caddy reload procedure each demonstrated, with the exact service names
      as written.
- [ ] A grep confirms no remaining reference to `/opt/vigilafrica/*/docker-compose.yml`.
