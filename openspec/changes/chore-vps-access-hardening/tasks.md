# Tasks: Harden the VPS Deploy Path

Ordered by severity ÷ cost. Group 1 closes the three named exposures and carries **no data risk**.
Groups 2–4 touch running services and want a staging rehearsal first. Group 5 is the only work that
trips the sentinel gate.

## 1. Credential path — do this first, no data risk

- [ ] 1.1 Restrict the deploy key in `authorized_keys`. [`provision.sh:41-42`](../../../deploy/provision.sh)
      appends the raw public key with no options. Prefix it with
      `restrict,command="/usr/local/bin/vigil-deploy"` so a leaked key can invoke one script and
      nothing else — no shell, no port forwarding, no agent forwarding.
      **This is the single highest-value line in the change**: it converts "leaked key → shell on the
      box" into "leaked key → one validated script".
- [ ] 1.2 Pin the host key. Add a `VPS_HOST_KEY` secret per environment and write `known_hosts`
      from it; delete the `ssh-keyscan` lines at
      [`deploy-production.yml:51`](../../../.github/workflows/deploy-production.yml) and
      [`deploy-staging.yml:20`](../../../.github/workflows/deploy-staging.yml).
      Verify by pinning a deliberately wrong key once and confirming the deploy **fails** rather
      than warning.
- [ ] 1.3 Split the deploy account in two. [`provision.sh:33-47`](../../../deploy/provision.sh) creates
      one `deploy` user owning both environment directories, which makes production's
      required-reviewer gate meaningless at the host level. Create `deploy-staging` and
      `deploy-prod`, each owning only its own path, each with its own key and its own
      `authorized_keys` command restriction.
      ⚠️ `DEPLOY_USER` is currently a single variable with a single default — the script needs
      restructuring, not a find-and-replace.
- [ ] 1.4 Remove `deploy` from the `docker` group ([`provision.sh:36`](../../../deploy/provision.sh)) and
      replace with a narrow `sudoers` rule for the task-1.1 script **and nothing else**.
      ⚠️ Pair this with server-side argument validation in the same task: a script that takes an
      arbitrary tag and runs `git checkout $tag && compose up` is a thin wrapper around root.
      Re-validate the `origin/release` ancestry **on the VPS** — the check at
      [`deploy-production.yml:38-43`](../../../.github/workflows/deploy-production.yml) runs on the
      runner and is bypassed entirely by anyone using the key directly.
      **1.4 without its validation half is not an improvement. Do not split them across PRs.**
- [ ] 1.5 Harden `sshd_config` in `provision.sh` — it currently installs fail2ban and opens ufw for
      OpenSSH but never touches the daemon config, leaving password auth and root login at whatever
      the base image ships. Set `PasswordAuthentication no`, `PermitRootLogin no`,
      `KbdInteractiveAuthentication no`. Document a key-rotation cadence in
      [`vps.md`](../../../docs/deployment/vps.md); `VPS_SSH_KEY` is currently a static secret with no
      expiry.
- [ ] 1.6 Update [`docs/deployment/vps.md`](../../../docs/deployment/vps.md) to match 1.1–1.5 —
      the secrets table at `:130-133` and the provisioning section at `:24-48` both go stale.

## 2. Move the build off production

- [ ] 2.1 Build and push `api` to GHCR from CI, tagged by digest. Nothing here needs BuildKit-only
      syntax — [`api/Dockerfile`](../../../api/Dockerfile) is plain multi-stage.
- [ ] 2.2 Replace `build:` with `image:` in [`docker-compose.prod.yml`](../../../docker-compose.prod.yml)
      and [`docker-compose.staging.yml`](../../../docker-compose.staging.yml).
      ⚠️ `APP_VERSION` is a **build arg** stamped into `/health.version`
      ([`api/Dockerfile:19-20`](../../../api/Dockerfile)). Once building moves to CI it must be stamped
      there, or the smoke tests at `deploy-production.yml:73-74` and `deploy-staging.yml:43-44`
      break. Decide this before 2.1, not after.
- [ ] 2.3 Reduce the server-side deploy to `pull` + `up -d`. Drop `git fetch`/`git checkout` from the
      deploy path so the production host no longer needs a repo clone or git credentials.
- [ ] 2.4 Extend `scripts/check-image-pins.js` to cover the new first-party image ref, so the
      digest-pinning guarantee is not quietly lost for the one image we build ourselves.
- [ ] 2.5 Confirm no Go toolchain, module cache, or source tree remains on the production host.

## 3. Container hardening

- [ ] 3.1 Add `cap_drop` to `prod-db` / `staging-db`, which currently get `no-new-privileges` but no
      capability drop ([`docker-compose.prod.yml:14-15`](../../../docker-compose.prod.yml)) while their
      API siblings get `cap_drop: ALL`.
      ⚠️ A blanket `cap_drop: ALL` will **likely break the postgis entrypoint**, which chowns the
      data dir on first init — expect to add back `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETUID`,
      `SETGID`. Verify against a **cold volume**, not a warm one; a warm volume skips init and hides
      the failure.
- [ ] 3.2 Add `mem_limit` and `pids_limit` to all eight services across both stacks. There are no
      resource limits anywhere today, and staging and production share one box — a runaway staging
      container can OOM production. This is an availability defect on a single-host topology.
      **Assumption:** the host is cgroup v2 (needed for these to apply). Confirm with
      `stat -fc %T /sys/fs/cgroup` before trusting the limits.
- [ ] 3.3 Verify the `umami` Postgres role has **no grants** on the `vigilafrica` database.
      `prod-umami` is the largest attack surface (public dashboard, Next.js) and shares
      `prod-internal` with `prod-db`. Unverified — this is a check, not a known defect. If grants
      exist, revoke; consider whether Umami needs its own network.

## 4. Resilience

- [ ] 4.1 Sync backups off-box. [`vps.md:156`](../../../docs/deployment/vps.md) already flags this
      ("Sync backups off-box before calling production resilient") and it is still open.
- [ ] 4.2 Restore a backup into a scratch database and assert the row count matches production.
      **An untested backup is not a backup** — and this is a hard prerequisite for any future
      Podman migration, whose volume remap step needs dump/restore.
- [ ] 4.3 Re-point the backup crons ([`vps.md:151-152`](../../../docs/deployment/vps.md)) at whichever
      account owns the containers after task 1.3/1.4. They currently run as root via
      `docker compose exec` and will break.

## 5. Trusted-proxy correctness guard ⚠️ sentinel-gated

The only group touching `api/internal/`. Worth doing on its own merits — it is also what makes a
future rootless migration safe rather than a silent regression.

- [ ] 5.1 Assert `clientIP()` keys per-client. Add a test that two requests from distinct client IPs
      through a trusted proxy land in **distinct** rate-limit buckets, and that an untrusted peer
      cannot forge `X-Forwarded-For`. Today the only coverage is
      [`middleware_test.go:43,58`](../../../api/internal/handlers/middleware_test.go).
- [ ] 5.2 Log the observed peer address once at startup (or behind a debug flag) so the value
      `TRUSTED_PROXY_CIDRS` must contain is **observable rather than inferred** on any future runtime
      change.
- [ ] 5.3 Extend the production smoke test beyond `status` and `version`. As written
      ([`deploy-production.yml:73-74`](../../../.github/workflows/deploy-production.yml)) it would pass
      cleanly through the exact failure described in the proposal's Podman section.

## 6. Explicitly not in this change

- [ ] 6.1 ~~Migrate to Podman or rootless Docker~~ — deferred, with the edge cases recorded in
      [proposal.md](proposal.md) so they are not rediscovered. Revisit on a fresh VPS build.
- [ ] 6.2 ~~Narrow `TRUSTED_PROXY_CIDRS` off `172.16.0.0/12`~~ — necessary given the current
      topology; accepted and documented rather than fixed.
- [ ] 6.3 ~~Multi-host / orchestrator topology~~
