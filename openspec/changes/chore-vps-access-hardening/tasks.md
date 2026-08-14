# Tasks: Harden the VPS Deploy Path

**21 tasks.** Ordered so that **no task removes something a later task depends on** — the first
revision had two such defects (a forced command installed before its script existed, and a repo
checkout deleted while a later control still needed it). Both are resolved below.

⚠️ **Lockout safety:** task 1.1 exists solely so that later tasks cannot strand the maintainer
outside the box. Do it first, verify it with a second concurrent session, and do not skip it.

## 1. Credential path — highest severity, no data risk

> ⚠️ **Implementation vs rollout.** Tasks 1.1 and 1.3–1.6 need the live VPS for **migration and
> verification**, not for authoring. The scripts, sudoers drop-ins, split-user provisioning and
> `sshd_config` changes all live in [`deploy/provision.sh`](../../../deploy/provision.sh) and are
> writable from a workstation; what cannot be done there is applying them and proving they work.
> Do not read the deferral as "nothing here is implementable yet."

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
      **Repo side landed** (deliberately still unticked — see the gate below): both `ssh-keyscan`
      calls removed; `known_hosts` written and **validated** by the shared
      [`.github/scripts/write-known-hosts.sh`](../../../.github/scripts/write-known-hosts.sh), which
      rejects an empty secret, `@cert-authority`/`@revoked` records and wildcard host patterns, and
      confirms via `ssh-keygen -F` that the pin actually matches `VPS_HOST`. Both `ssh` calls set
      `StrictHostKeyChecking=yes`, an explicit `UserKnownHostsFile`, **and**
      `GlobalKnownHostsFile=/dev/null` so the secret is the exclusive trust source. Secrets moved out
      of `${{ }}` interpolation into `env:`.
      ⚠️ **Corrected after independent review — the first version's enrollment procedure was
      unsound.** It told the operator to run `ssh-keyscan` "on the VPS console". `ssh-keyscan` opens
      a *network* connection and records whatever answers **wherever it is run**, so it cannot
      authenticate the key and simply repeats the trust-on-first-use step this secret exists to
      remove. The runbook now derives the pin from `/etc/ssh/ssh_host_ed25519_key.pub` on the
      filesystem, with a console fingerprint comparison. It also no longer suggests `hostname -f`,
      which is not reliably equal to `VPS_HOST`.
      ⚠️ **This stays unticked until the control is real, not merely written.** Required:
      (a) set `VPS_HOST_KEY` in **both** environments from the filesystem-derived key;
      (b) pin a deliberately wrong key on staging and confirm the deploy **fails**;
      (c) restore the correct key and confirm a staging deploy **succeeds**.
      Until (a), every deploy *and* the `workflow_dispatch` rollback path fail closed at Configure
      SSH. That is the intended direction, but it is an outage until the secret exists. It does not
      alter the VPS itself — recovery is to set the secret.
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
      and define the privilege boundary explicitly. ⚠️ "A `sudoers` rule for the script and nothing
      else" is **not sufficient on its own**: the forced command runs as the deploy user, and the
      checkout and `.env` it reads are deploy-owned — so elevating a helper that reads
      deploy-writable compose or configuration stays root-equivalent via arbitrary images, bind
      mounts, or compose interpolation. Specify all of:
      - a **root-owned, deploy-non-writable** privileged helper, and root-owned manifests/allowlist;
      - argument parsing done **unprivileged**, before elevation;
      - the exact `sudo` argv permitted, with a reset environment (`env_reset`, `secure_path`);
      - rejection tests for alternate compose paths, added mounts, extra flags, stdin, and
        environment injection.
      Depends on 1.3 — doing it first breaks deploys.
- [ ] 1.5 **Split the deploy account in two, and retire the old one.** One user currently owns both
      environment directories, so production's reviewer gate is not a boundary at the host level.
      Create `deploy-staging` and `deploy-prod`, each owning only its own path, key, and forced
      command. ⚠️ **Creating the new principals does not remove the old one** — the original
      `deploy` account's `authorized_keys`, sudoers entry, group memberships, and file ownership can
      all survive and keep the exposure intact. Lock or delete the account, remove its sudoers entry
      and docker-group membership, transfer ownership, and **prove the old key reaches neither
      environment.** `DEPLOY_USER` is a single variable with a single default — this is a
      restructure, not a find-and-replace.
- [ ] 1.6 **Harden `sshd_config`** — `provision.sh` never touches it, leaving password auth and root
      login at image defaults. Set `PasswordAuthentication no`, `KbdInteractiveAuthentication no`,
      `PermitRootLogin no`. ⚠️ **Requires 1.1 verified first.** ⚠️ `sshd -t` checks **syntax, not
      effective policy** — OpenSSH takes the first obtained value, so an earlier cloud-image
      `Include` or a `Match` block can leave password or root login enabled while `-t` still passes.
      Verify with `sshd -T -C user=…,host=…,addr=…` for every relevant account, then **empirically**:
      key login succeeds, password login is refused, root login is refused — all proven **before**
      closing the rescue session. Document the console/rescue rollback path.
- [ ] 1.7 **Confirm the reviewer set and document key rotation.** ✅ The `production` environment's
      required-reviewer rule **has been verified to exist** via the public GitHub environments API,
      so it is no longer an unverified premise — but the *reviewer set* has not been checked, and
      `VPS_SSH_KEY` is a static secret with no expiry. Confirm both, then update
      [`vps.md:130-133`](../../../docs/deployment/vps.md) for tasks 1.1–1.6.
      **Reviewer set — CONFIRMED 2026-08-14** via `gh api repos/didi-rare/vigilafrica/environments`:
      `production` carries `required_reviewers` with exactly one reviewer, **`didi-rare`** — the sole
      maintainer, who also triggers releases. So the gate is a **self-approval confirmation step, not
      a separation-of-duties boundary**, and it constrains only the workflow path; `VPS_SSH_KEY`
      reaches the host without touching GitHub. This is now recorded in `vps.md` and **strengthens
      the case for 1.5** — the host-level account split has to be the boundary, because this is not.
      Key rotation documented in `vps.md` with the retire-the-old-key step called out, since adding
      a new key without removing the old one is the usual way rotation silently fails.
      **Left open deliberately:** the task also requires documenting tasks 1.1–1.6, and 1.1/1.3–1.6
      are not implemented. Ticking this now would document a host state that does not exist.

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
- [ ] 2.3 **Choose ONE release-authority design and define who populates it.** ⚠️ Two defects here.
      First, revision 1 required server-side `origin/release` ancestry validation (task 1.3) and
      then deleted the repo clone that made it possible. Second, revision 2 offered "a bare mirror
      **or** a digest allowlist" — an unresolved fork, and **it never said who writes the
      allowlist.** If the deploy credential can add or select entries, a stolen key approves its own
      digest and the control is theatre. Pick one:
      - **(a)** keep a **root-owned, deploy-non-writable read-only bare mirror** and verify the
        requested tag/commit against it; or
      - **(b)** define the provenance verifier, the trusted signer identity, an immutable compose
        artefact, and an **admin-only** allowlist update path over a channel the deploy credential
        cannot use.
      Either way, define **how the compose file itself reaches the host** once the checkout is gone.
- [ ] 2.4 **Remove the checkout and prune build state.** Drop `git fetch`/`git checkout` from the
      deploy path, then confirm no source tree, module cache, or **stale Docker builder layers**
      remain — switching to a remote image does not remove existing layers.
- [ ] 2.5 **Update every runbook that still describes checkout-based deployment.** ⚠️ Task 1.7's doc
      update lands *before* this group removes the checkout and the on-host build, and nothing
      afterwards revisits it. [`vps.md`](../../../docs/deployment/vps.md) still instructs cloning,
      pulling, `git checkout --force <tag>` and `--build`, and
      [`release-process.md`](../../../docs/deployment/release-process.md) describes checkout-based
      rollback. Update deployment, rollback, manual-operation and topology docs **after** 2.3's
      design is chosen, or they will actively mislead during an incident.

## 3. Container and host hardening

- [ ] 3.1 **Add `cap_drop` to `prod-db` / `staging-db`**, which get `no-new-privileges` but no
      capability drop ([`docker-compose.prod.yml:14-15`](../../../docker-compose.prod.yml)) while
      their API siblings get `cap_drop: ALL`. ⚠️ A blanket `cap_drop: ALL` will likely break the
      postgis entrypoint, which chowns the data dir on init — expect to restore `CHOWN`,
      `DAC_OVERRIDE`, `FOWNER`, `SETUID`, `SETGID`. **Verify against a cold volume**; a warm volume
      skips init and hides the failure.
- [ ] 3.2 **Add `mem_limit` and `pids_limit` to all eight services, with a numeric budget.**
      There are none today, and staging and production share one box, so a runaway staging container
      can OOM production. ⚠️ **Per-service limits alone do not achieve this** — eight individually
      reasonable limits can still sum past physical memory and permit a host-wide OOM. Measure host
      capacity first, set explicit per-service values whose **total leaves a reserve** for the OS,
      Caddy and the Docker daemon, and verify under staged memory pressure that production survives
      a staging container trying to exhaust its limit.
      ⚠️ **Corrected:** an earlier draft assumed cgroup v2 was required. That is a **rootless**
      requirement; the rootful daemon in use enforces limits on cgroup v1 too. Verify **empirically**
      — `docker info` for driver and available controllers, then read the effective limits off a
      running container — rather than inspecting the cgroup filesystem type.
- [ ] 3.3 **Separate Umami from the API network, and measure the real gateway.** `prod-api` and the
      public-facing `prod-umami` currently share `prod-internal`
      ([`docker-compose.prod.yml:62-63`](../../../docker-compose.prod.yml) and [`:117-118`](../../../docker-compose.prod.yml)), so a compromised
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

- [ ] 4.1 **Fix what the backup captures, then sync it off-box.**
      [`vps.md:156`](../../../docs/deployment/vps.md) already flags the off-box gap. ⚠️ Two further
      defects in the existing crons at [`vps.md:151-152`](../../../docs/deployment/vps.md):
      - they dump **only the `vigilafrica` database**, but each cluster also holds the separate
        **`umami` database** and the cluster-wide **roles** needed to restore either — losing the
        `umami` role makes a restore fail even where the data survived;
      - `docker compose exec … | gzip > file` runs **without `pipefail`**, so a failed dump still
        produces a valid-looking gzip and the cron reports success. A silent empty backup is worse
        than no backup, because it removes the reason to check.
      Require: globals/roles plus **both** databases, `set -o pipefail`, a non-empty/size sanity
      check, atomic publication (write to a temp name, rename on success), restrictive file
      permissions, off-box transfer with failure alerting.
- [ ] 4.2 **Restore into a scratch database and compare representative row counts — for both
      databases.** An untested backup is not a backup, and a tested restore is a prerequisite for
      any future Podman volume work.
      ⚠️ **Corrected:** an earlier draft claimed the root crons *will break* after the account
      split. They will not — root retains access to the rootful Docker socket and the compose paths
      are unchanged. **Verify them after the migration; change them only if something actually broke.**

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
- [ ] 5.2 **Measure the real peer address, narrow `TRUSTED_PROXY_CIDRS` to it, and build a gate that
      can actually detect the failure.** `172.16.0.0/12` was a broad guess never checked against the
      running stack. Log the observed peer so the value is observable rather than inferred.
      ⚠️ **"Extend the production smoke test" is not by itself an executable gate.** A GitHub runner
      presents a single client address and no endpoint exposes the resolved bucket key, so such a
      test **cannot distinguish one global bucket from correct per-client buckets** — it would pass
      through the exact regression it is meant to catch. Specify a real gate: either two controlled
      egress identities, or a server-side/in-container probe that exercises trusted and untrusted
      peers and returns a machine-checkable result.

## 6. Explicitly not in this change

- ~~Migrate to Podman or rootless Docker~~ — deferred; corrected edge cases recorded in
  [proposal.md](proposal.md) so they are not rediscovered. Revisit on a fresh VPS build, with
  pinned versions and the peer address measured first.
- ~~Multi-host or orchestrator topology~~ — this stays a single VPS.
- ~~Application features and web work~~
