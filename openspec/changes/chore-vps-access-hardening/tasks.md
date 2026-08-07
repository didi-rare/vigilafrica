# Tasks: Harden the VPS Deploy Path

**20 tasks.** Ordered so that **no task removes something a later task depends on** — the first
revision had two such defects (a forced command installed before its script existed, and a repo
checkout deleted while a later control still needed it). Both are resolved below.

⚠️ **Lockout safety:** task 1.1 exists solely so that later tasks cannot strand the maintainer
outside the box. Do it first, verify it with a second concurrent session, and do not skip it.

## 1. Credential path — highest severity, no data risk

- [ ] 1.1 **Create and verify a separate administrative account before touching anything else.**
      [`provision.sh:33-47`](../../../deploy/provision.sh) creates only deploy accounts, so hardening
      SSH or converting deploy access to a forced command with no admin path in **can lock the
      maintainer out of the VPS.** Create an admin user with its own key and sudo rights, then
      prove it by opening a **second concurrent session** while the first stays open.
- [ ] 1.2 **Pin the host key.** Add a `VPS_HOST_KEY` secret per environment and write `known_hosts`
      from it; delete the `ssh-keyscan` lines at
      [`deploy-production.yml:51`](../../../.github/workflows/deploy-production.yml) and
      [`deploy-staging.yml:20`](../../../.github/workflows/deploy-staging.yml). Verify by pinning a
      deliberately wrong key once and confirming the deploy **fails** rather than warning.
      Independent of everything else — safe to land on its own.
- [ ] 1.3 **Forced-command deploy protocol — one atomic task.** ⚠️ The first revision split this in
      two and would have broken deployment: it installed
      `restrict,command="/usr/local/bin/vigil-deploy"` in task 1.1 while the script and its argument
      validation were deferred, **and** both workflows currently send remote shell programs
      ([`deploy-production.yml:59-66`](../../../.github/workflows/deploy-production.yml),
      [`deploy-staging.yml:28-35`](../../../.github/workflows/deploy-staging.yml)), which a forced
      command would override. All of the following ship together or none do:
      - the `vigil-deploy` script itself, with **server-side** validation of what it is asked to deploy;
      - `restrict,command=...` prepended in `authorized_keys` (currently a bare key at
        [`provision.sh:41-42`](../../../deploy/provision.sh));
      - **both workflows** converted to the fixed command protocol;
      - `SSH_ORIGINAL_COMMAND` parsed against an allowlist and **rejected** when unexpected;
      - attacker-controlled stdin discarded rather than executed.
- [ ] 1.4 **Remove `deploy` from the `docker` group** ([`provision.sh:36`](../../../deploy/provision.sh))
      and replace with a `sudoers` rule for the task-1.3 script and nothing else. Depends on 1.3 —
      doing it first breaks deploys, doing it without 1.3's validation is a thin wrapper around root.
- [ ] 1.5 **Split the deploy account in two.** One user currently owns both environment
      directories, so production's reviewer gate is not a boundary at the host level. Create
      `deploy-staging` and `deploy-prod`, each owning only its own path, its own key, its own forced
      command. ⚠️ `DEPLOY_USER` is a single variable with a single default — this is a restructure,
      not a find-and-replace.
- [ ] 1.6 **Harden `sshd_config`** — `provision.sh` never touches it, leaving password auth and root
      login at image defaults. Set `PasswordAuthentication no`, `KbdInteractiveAuthentication no`,
      and `PermitRootLogin no`. ⚠️ **Requires 1.1 verified first.** Run `sshd -t` before reloading,
      keep the existing session open during the reload, and document the console/rescue rollback path.
- [ ] 1.7 **Verify the production environment protection rule actually exists**, and document key
      rotation. [`vps.md:130-133`](../../../docs/deployment/vps.md) asserts production requires a
      reviewer, but the workflow YAML only names the environment — the rule lives in GitHub settings
      and is **unverified external state**. Check it live (`gh api`), then update
      [`vps.md`](../../../docs/deployment/vps.md) for tasks 1.1–1.6. `VPS_SSH_KEY` is currently a
      static secret with no expiry.

## 2. Move the build off production

- [ ] 2.1 **Decide where `APP_VERSION` is stamped, before moving the build.** It is a *build arg*
      baked into `/health.version` ([`api/Dockerfile:19-20`](../../../api/Dockerfile)). Both smoke
      tests assert on it ([`deploy-production.yml:73-74`](../../../.github/workflows/deploy-production.yml),
      `deploy-staging.yml:43-44`). Moving the build without moving the stamp breaks both.
- [ ] 2.2 **Build and push to GHCR from CI — completely specified.** The workflows currently grant
      no `packages: write`, perform no registry login, and define no pull path for the VPS. A
      newly published package can default to **private**, so anonymous pulls would fail. Specify:
      job permissions, CI registry auth, **explicit package visibility** (verified, not assumed —
      the repo being public does not settle it), a read-only VPS pull credential if the package
      stays private, and the fact that images are addressed **by digest**, not "tagged by digest."
- [ ] 2.3 **Replace the repo-based release check with a server-side digest allowlist.** ⚠️ The first
      revision required server-side `origin/release` ancestry validation (task 1.3) and then deleted
      the repo clone that made it possible — a stolen key could have requested any digest. Either
      keep a **read-only bare mirror** for ancestry checks, or move to signed provenance plus a
      server-side allowlist of approved release digests. Also define **how the compose file itself
      reaches the host** once the checkout is gone.
- [ ] 2.4 **Remove the checkout and prune build state.** Drop `git fetch`/`git checkout` from the
      deploy path, then confirm no source tree, module cache, or **stale Docker builder layers**
      remain — switching to a remote image does not remove existing layers.

## 3. Container and host hardening

- [ ] 3.1 **Add `cap_drop` to `prod-db` / `staging-db`**, which get `no-new-privileges` but no
      capability drop ([`docker-compose.prod.yml:14-15`](../../../docker-compose.prod.yml)) while
      their API siblings get `cap_drop: ALL`. ⚠️ A blanket `cap_drop: ALL` will likely break the
      postgis entrypoint, which chowns the data dir on init — expect to restore `CHOWN`,
      `DAC_OVERRIDE`, `FOWNER`, `SETUID`, `SETGID`. **Verify against a cold volume**; a warm volume
      skips init and hides the failure.
- [ ] 3.2 **Add `mem_limit` and `pids_limit` to all eight services.** There are none today, and
      staging and production share one box, so a runaway staging container can OOM production.
      ⚠️ **Corrected:** an earlier draft assumed cgroup v2 was required. That is a **rootless**
      requirement; the rootful daemon in use enforces limits on cgroup v1 too. Verify **empirically**
      — `docker info` for driver and available controllers, then read the effective limits off a
      running container — rather than inspecting the cgroup filesystem type.
- [ ] 3.3 **Separate Umami from the API network, and measure the real gateway.** `prod-api` and the
      public-facing `prod-umami` currently share `prod-internal`
      ([`docker-compose.prod.yml:101-130`](../../../docker-compose.prod.yml)), so a compromised
      Umami container can reach the API and forge `X-Forwarded-For` from a peer inside the trusted
      `172.16.0.0/12` range. Put API and Umami on separate application networks with the database
      attached to both; assign a **fixed subnet/gateway** and record the actual address.
- [ ] 3.4 **Replace the unpinned Docker install and verify the loopback guarantee.**
      [`provision.sh:19-21`](../../../deploy/provision.sh) pipes `get.docker.com` into a root shell;
      Docker documents that script as **not recommended for production**. Install a pinned version
      from Docker's signed apt repository. Then **record the live Docker version**: the
      loopback-publishing guarantee this stack relies on holds only on **≥ 28.0.0** — below that,
      same-L2 hosts can reach ports published to localhost. Also set `ufw default deny incoming`
      explicitly; [`provision.sh:49-52`](../../../deploy/provision.sh) only runs three `ufw allow`
      lines and `ufw --force enable`, so today the policy is inherited rather than set.
- [ ] 3.5 **Audit effective Postgres privileges, not just the `umami` role.** ⚠️ Revoking grants
      from `umami` alone does not establish isolation: PostgreSQL grants **`CONNECT` and `TEMPORARY`
      to `PUBLIC` by default on databases.** Audit `PUBLIC` plus schema, table, sequence and default
      privileges, and state the intended `CONNECT` policy explicitly.

## 4. Resilience

- [ ] 4.1 **Sync backups off-box.** [`vps.md:156`](../../../docs/deployment/vps.md) already flags
      this and it is still open.
- [ ] 4.2 **Restore a backup into a scratch database and assert the row count matches**, then
      re-check the crons at [`vps.md:151-152`](../../../docs/deployment/vps.md).
      ⚠️ **Corrected:** an earlier draft claimed the root crons *will break* after the account
      split. They will not — root retains access to the rootful Docker socket and the compose paths
      are unchanged. **Verify them after the migration; change them only if something actually broke.**
      An untested backup is not a backup, and a tested restore is a prerequisite for any future
      Podman volume work.

## 5. Client-IP resolution ⚠️ sentinel-gated

The only group touching `api/internal/`. **Surfaced by independent review** — a live defect today,
not migration preparation.

- [ ] 5.1 **Unify the two IP-resolution policies behind one trusted-proxy-aware resolver.**
      `/v1/context` uses `extractIP()` ([`context.go:74-94`](../../../api/internal/handlers/context.go)),
      which honours `X-Forwarded-For` and `X-Real-IP` from **any** peer, while rate limiting uses
      `clientIP()` ([`middleware.go:386`](../../../api/internal/handlers/middleware.go)), which
      checks the peer first. Make both use one resolver. Add tests that an untrusted peer cannot
      forge a client IP on **either** path, and that two distinct clients through a trusted proxy
      land in distinct rate-limit buckets.
- [ ] 5.2 **Measure the real peer address, narrow `TRUSTED_PROXY_CIDRS` to it, and make the failure
      visible.** `172.16.0.0/12` was a broad guess and has never been checked against the running
      stack. Log the observed peer so the value is observable rather than inferred, then extend the
      production smoke test beyond `status` and `version` — as written it would pass cleanly through
      a total collapse of per-client rate limiting.

## 6. Explicitly not in this change

- ~~Migrate to Podman or rootless Docker~~ — deferred; corrected edge cases recorded in
  [proposal.md](proposal.md) so they are not rediscovered. Revisit on a fresh VPS build, with
  pinned versions and the peer address measured first.
- ~~Multi-host or orchestrator topology~~ — this stays a single VPS.
- ~~Application features and web work~~
