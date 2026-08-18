# Tasks: Harden the VPS Deploy Path

**21 tasks — 8 done, 13 open** (last counted 2026-08-18). Ordered so that **no task removes something a later task depends on** — the first
revision had two such defects (a forced command installed before its script existed, and a repo
checkout deleted while a later control still needed it). Both are resolved below.

⚠️ **Lockout safety:** task 1.1 exists solely so that later tasks cannot strand the maintainer
outside the box. Do it first, verify it with a second concurrent session, and do not skip it.

## 1. Credential path — highest severity, no data risk

> ⚠️ **Implementation vs rollout.** Tasks 1.1 and 1.3–1.6 need the live VPS for **migration and
> verification**, not for authoring. The forced-command script, sudoers drop-in, split-user
> provisioning and `sshd_config` hardening do **not exist yet** — none of them are in
> [`deploy/provision.sh`](../../../deploy/provision.sh) today — but all of them *can be written*
> into repository-controlled provisioning assets from a workstation. What cannot be done there is
> applying them to the host and proving they work. Do not read the deferral as "nothing here is
> implementable yet", and do not read this note as "the code is already written".

- [x] 1.1 **Create and verify a separate administrative account before touching anything else.**
      [`provision.sh:33-47`](../../../deploy/provision.sh) creates only deploy accounts, so hardening
      SSH or converting deploy access to a forced command with no admin path in **can lock the
      maintainer out of the VPS.** Create an admin user with its own key and sudo rights, then
      prove it by opening a **second concurrent session** while the first stays open.

      ✅ **DONE 2026-08-14.** `vigil-admin` created with its own dedicated ed25519 key (separate from
      the deploy key) and `sudo` group membership. **Verified the way the task requires:** logged in
      from a *second* terminal while the original root session stayed open, and `sudo -v` succeeded
      there. The rescue path is real, not assumed — **this unblocks 1.6.**

      ⚠️ **`passwd <admin>` is not optional.** With no password set the account is locked for
      password auth and **`sudo` fails**, leaving an "admin" account that cannot administer. This
      adds no SSH exposure — 1.6 disables password *authentication* separately; the password exists
      only for `sudo`.

      ⚠️ **Gotcha that cost a cycle:** the first attempt failed with *"No ED25519 host key is known
      … and you have requested strict checking"*. That is the **host** half failing before
      authentication is attempted, and says nothing about the account. `StrictHostKeyChecking=yes`
      **refuses an unknown host rather than enrolling it**, so the pinned `known_hosts` file must be
      written *first* — the same ordering defect independent review caught in the runbook. The
      operator instructions in
      [`staging-production-topology.md`](../../../docs/deployment/staging-production-topology.md)
      already write the file before connecting; follow them in order.
- [x] 1.2 **Pin the host key.** Add a `VPS_HOST_KEY` secret per environment and write `known_hosts`
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
      ✅ **COMPLETE — verified live on staging 2026-08-14, not self-certified.**

      `VPS_HOST_KEY` is set in **both** environments. The value was derived from
      `/etc/ssh/ssh_host_ed25519_key.pub` on the VPS console and agreed **four independent ways**:
      the maintainer's laptop `known_hosts`, `ssh-keygen -lf` on the box, recomputation from the
      transcribed string, and the fingerprint sshd actually presents on port 22 — so the `.pub` is
      demonstrably not stale.

      Three staging deploys, in an order chosen so the negative test could not be confounded with a
      wrong `VPS_HOST`:

      | # | Run | Pin | Result |
      |---|---|---|---|
      | 1 | [31796783722](https://github.com/didi-rare/vigilafrica/actions/runs/31796783722) | correct | **success** — `Pinned host key verified`, `/health` = `ed3bf23` |
      | 2 | [31797550080](https://github.com/didi-rare/vigilafrica/actions/runs/31797550080) | **valid but wrong** | **failure, exit 255** |
      | 3 | [31797616419](https://github.com/didi-rare/vigilafrica/actions/runs/31797616419) | restored | **success** |

      ⚠️ **Run 2 is the one that matters, and it failed in exactly the right place.** The decoy was a
      *well-formed* ed25519 record for the correct host — so `Configure SSH` **succeeded** (the
      validator accepted it, as designed) and the refusal came from **OpenSSH itself** at connect
      time: `REMOTE HOST IDENTIFICATION HAS CHANGED` / `Host key verification failed`, with the smoke
      test correctly **skipped**. Using a malformed decoy would only have tested our own validator,
      not host-key verification.

      Also confirmed: a failed deploy is a **no-op**, not an outage — staging stayed healthy on
      `ed3bf23` throughout run 2, because the connection is refused before anything is executed.

      Run 1 additionally confirms `VPS_HOST` is the IP `178.104.104.122`, which until then was
      inferred rather than known.
- [x] 1.3 **Forced-command deploy protocol — one atomic task.** ⚠️ The first revision split this in
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
- [x] 1.4 **Remove `deploy` from the `docker` group** ([`provision.sh:36`](../../../deploy/provision.sh))
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

      ✅ **1.3 AND 1.4 ROLLED OUT AND VERIFIED LIVE 2026-08-15**, via
      [`forced-command-migration.md`](../../../docs/deployment/forced-command-migration.md).
      Both shipped together, as this task required.

      **Refusals, proven from a workstation with the real deploy key — not reasoned about:**

      | attempt | result |
      |---|---|
      | interactive shell | `refused: this key is restricted to the deploy protocol; no shell` |
      | `id` / `docker ps` / unknown verb | refused |
      | `deploy-staging <sha>; id`, backticks, `$()` | `refused: command contains shell metacharacters` |
      | extra argument | `refused: too many arguments` |
      | production ref shape on the staging verb | `refused: production ref must be a SemVer tag` |
      | **SFTP**, **SCP `/etc/passwd`** | `Connection closed` — nothing exfiltrated |
      | **port forward → internal `:8081`** | `administratively prohibited`, `curl_http=000` |

      **Accept path proven too**, which matters as much: a live `deploy-staging ed3bf23` completed
      end to end **with `deploy` outside the `docker` group** — `vigil-deploy-run: staging now at
      ed3bf23`, staging healthy, and a container start-time diff proving **production was untouched**.

      **1.4's privilege boundary:** `deploy` is in no group but its own (`docker:x:988:` is empty);
      `/home/deploy/.ssh` is `root:root 0755` so the account cannot create the `authorized_keys2`
      that `sshd -T` revealed is also an effective key source; the trees are fresh **root-owned
      clones** rather than re-owned checkouts, because `chown -R root` over a deploy-writable
      checkout promotes `.git/hooks` and `.git/config` into root-trusted state; and sudo permits
      exactly two regex-matched argv forms, verified by `sudo -l -U deploy` plus rejection tests for
      alternate compose paths, added mounts, extra flags and `DOCKER_HOST` injection.

      ⚠️ **The sudoers wildcard trap:** `sudoers(5)` `*` matches across whitespace, so
      `vigil-deploy-run staging *` does **not** constrain argv — measured, it permitted
      `staging abc1234 --extra-flag`. The regex form (`^...$`, sudo ≥ 1.9.10; this host runs
      1.9.15p5) does. **Verify with `sudo -l -U`, never `visudo -c`** — the wildcard form parses
      perfectly and permits everything.

      ⚠️ **A bug only the host could find.** `exec 9> file 2>/dev/null` applies the redirection to
      the **whole shell, permanently**, so it silenced stderr for the entire script: every `die`
      went to `/dev/null` and deploys failed with exit 1 and no output. It survived three
      independent review rounds because it is invisible until runtime and every offline test died
      before reaching the lock line. Fixed in #239, with a regression test that deliberately gets
      *past* the lock.
- [x] 1.5 **Split the deploy account in two, and retire the old one.** One user currently owns both
      environment directories, so production's reviewer gate is not a boundary at the host level.
      Create `deploy-staging` and `deploy-prod`, each owning only its own path, key, and forced
      command. ⚠️ **Creating the new principals does not remove the old one** — the original
      `deploy` account's `authorized_keys`, sudoers entry, group memberships, and file ownership can
      all survive and keep the exposure intact. Lock or delete the account, remove its sudoers entry
      and docker-group membership, transfer ownership, and **prove the old key reaches neither
      environment.** `DEPLOY_USER` is a single variable with a single default — this is a
      restructure, not a find-and-replace.
      **DONE 2026-08-16**, runbook at [`docs/deployment/deploy-account-split.md`](../../../docs/deployment/deploy-account-split.md).
      ⚠️ **"each owning only its own path" no longer applies** — after 1.3/1.4 **neither account owns
      any path; root does**, and the helper does all the writing. Re-owning per account would undo
      1.4. What was actually split is **credentials and authority**.
      `deploy-staging` (uid 1002) and `deploy-prod` (uid 1003) exist, each in **only its own group**,
      neither in `docker`, each with its **own** key — `SHA256:eeBSUU…` / `SHA256:2gVnPK…`, distinct
      by fingerprint (`provision.sh` now refuses identical ones; a string compare would pass two
      copies of one key with different comments).
      The environment is pinned in the root-owned forced command; the **boundary** is sudoers, proven
      by `sudo -l -U`: `deploy-staging` → staging rule only, `deploy-prod` → production rule only.
      ⚠️ **The split was proven with WELL-FORMED requests the other account would accept**, so a
      refusal is separation rather than a parse error: staging key → `deploy-production v1.5.0` and
      prod key → `deploy-staging 48d0b09` both gave `refused: key is pinned to …`.
      Both environments then deployed for real through the new accounts: staging run 31957826770
      (`4edd382`, health `ok`) and production run 31957880766 (redeploy of the running `v1.5.0`, gate
      approved, health `ok`, NG+GH `success`).
      **Old account retired:** `authorized_keys` removed, out of `docker`, `usermod -L -s nologin`
      (`passwd -S` → `L`), no sudoers entry. Old key now fails at **authentication** —
      `Permission denied (publickey)` — not at the forced command.
      ⚠️ Remaining `deploy`-owned files are confined to
      `/opt/vigilafrica/.forced-command-rollback-20260815T132945Z`, which is `root:root 0700` and
      therefore unreachable; `/etc/sudoers*` carries no `deploy` reference.
      ⚠️ **`VPS_USER` and `VPS_SSH_KEY` were already per-environment secrets**, so the workflows
      needed **no change** — only four secret values. `vigil-deploy-run` was unchanged too.
      Incidental confirmation: `vigil-deploy-run` now appears **unmasked** in Actions logs, where it
      previously rendered `vigil-***-run` because `VPS_USER` was literally `deploy`.
- [x] 1.6 **Harden `sshd_config`** — `provision.sh` never touches it, leaving password auth and root
      login at image defaults. Set `PasswordAuthentication no`, `KbdInteractiveAuthentication no`,
      `PermitRootLogin no`. ⚠️ **Requires 1.1 verified first.** ⚠️ `sshd -t` checks **syntax, not
      effective policy** — OpenSSH takes the first obtained value, so an earlier cloud-image
      `Include` or a `Match` block can leave password or root login enabled while `-t` still passes.
      Verify with `sshd -T -C user=…,host=…,addr=…` for every relevant account, then **empirically**:
      key login succeeds, password login is refused, root login is refused — all proven **before**
      closing the rescue session. Document the console/rescue rollback path.
      **DONE 2026-08-16**, runbook at [`docs/deployment/sshd-hardening.md`](../../../docs/deployment/sshd-hardening.md).
      Applied as a **low-sorted drop-in** `/etc/ssh/sshd_config.d/00-vigilafrica-hardening.conf`, not
      an edit to the main file: `Include` sits at line 12 and OpenSSH takes the **first** obtained
      value, so drop-ins beat the main file and the *lowest* filename beats later ones — `00-` cannot
      be overridden by a future cloud-init `50-`. The familiar "99 = wins" convention is backwards.
      Before: `passwordauthentication yes`, `permitrootlogin without-password`
      (so root-by-key was live, and `/root/.ssh/authorized_keys` existed).
      After, confirmed by `sshd -T` and per-account `sshd -T -C` for `vigil-admin`/`deploy`/`root`:
      all three directives `no`. `kbdinteractiveauthentication` was **already** `no` at line 71.
      Proven empirically from the workstation, and re-verified independently: key login `KEY_LOGIN_OK`;
      offered methods now `publickey` alone (was `publickey,password`); `root` → `Permission denied
      (publickey)` despite holding a key; deploy forced command unchanged (`refused: unknown verb`).
      Also folded into `provision.sh` so a rebuild cannot regress it — **guarded**: it refuses to
      harden when no non-deploy account has a key-based login, since the deploy accounts have no
      shell and cannot recover a host. Rollback is `rm` of the one drop-in plus `systemctl reload ssh`.
- [x] 1.7 **Confirm the reviewer set and document key rotation.** ✅ The `production` environment's
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
      ~~**Left open deliberately:** the task also requires documenting tasks 1.1–1.6, and 1.1/1.3–1.6
      are not implemented. Ticking this now would document a host state that does not exist.~~
      **DONE 2026-08-16** — 1.1–1.6 are now all implemented and host-verified, so `vps.md` documents
      a state that actually exists. Added a **Host Access Model** section (account table, how a deploy
      runs, and a control→enforcer→task table so an incident responder can see which boundaries fail
      independently), and rewrote **Deploy-credential rotation** for two accounts and two keys.
      ⚠️ **Three stale claims corrected, found while writing it:**
      (1) *"There is no host-level boundary between staging and production today"* — false since 1.5.
      (2) The provisioning command still advertised the removed `SSH_PUBLIC_KEY`/`DEPLOY_USER`.
      (3) **The `.env` section instructed `install -o deploy -g deploy`** — actively wrong since 1.4,
      and it would hand a leaked deploy key the database password. The **host is correct**
      (`root:root 600`, verified live); only the documentation was stale, so the risk was to the next
      person provisioning a host rather than to the running one.

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

- [x] 5.1 **Unify the two IP-resolution policies behind one trusted-proxy-aware resolver.**
      ✅ **Shipped in v1.4.0 (#225) as `fix-unify-client-ip-resolution`, 47/47 — verified against the
      code on 2026-08-18, not self-certified.** `extractIP()` no longer exists. Both call sites now
      route through one resolver: `/v1/context` via `h.cfg.Proxy.ClientIP(r)`
      ([`context.go:150`](../../../api/internal/handlers/context.go)) and rate limiting via
      `proxies.ClientIP(r)` ([`middleware.go:432`](../../../api/internal/handlers/middleware.go)),
      both landing in `clientIPWithTrustedProxies` ([`middleware.go:302`](../../../api/internal/handlers/middleware.go)).
      Both tests this task requires exist and were located by name, not assumed:
      the forged-header case on **both** paths — `TestGetContextIgnoresForgedHeadersFromUntrustedPeer`
      (`context_test.go:104`) and `TestClientIPIgnoresSpoofedForwardedForFromUntrustedPeer`
      (`middleware_test.go:40`) — and the distinct-bucket case,
      `TestRateLimitBucketsAreDistinctPerForwardedClient` (`ratelimit_identity_test.go:67`),
      alongside `TestRateLimitIdentityCannotBeForgedByUntrustedPeer` (`:89`).
      That change is archived at
      `openspec/changes/archive/2026-08-18-fix-unify-client-ip-resolution/`.
      ⚠️ **This does not close group 5.** Until 5.2 narrows the CIDR list, the bar is raised only from
      *any peer* to *anything on the Docker bridge* — which includes the public-facing `prod-umami`.
      ~~Original text:~~
      `/v1/context` uses `extractIP()` ([`context.go:74-94`](../../../api/internal/handlers/context.go)),
      which honours `X-Forwarded-For` and `X-Real-IP` from **any** peer, while rate limiting uses
      `clientIP()` ([`middleware.go:386`](../../../api/internal/handlers/middleware.go)), which
      checks the peer first. Make both use one resolver. Add tests that an untrusted peer cannot
      forge a client IP on **either** path, and that two distinct clients through a trusted proxy
      land in distinct rate-limit buckets.
- [ ] 5.2 **Measure the real peer address, narrow `TRUSTED_PROXY_CIDRS` to it, and build a gate that
      can actually detect the failure.**
      🔵 **IMPLEMENTED 2026-08-18 in `feat/narrow-trusted-proxy-cidrs`; deliberately NOT ticked — it
      is unproven until it has been deployed.** The code is written and tested; the boundary it
      describes does not exist until staging and production actually run it.

      **The measurement, which is the part that was missing.** Read from inside the production API
      container's own network namespace, not inferred from the host:
      `nsenter -t <pid> -n ss -tn state established '( sport = :8080 )'` → peer
      **`[::ffff:172.19.0.1]:38090`**. So the peer is the compose bridge gateway, **172.19.0.1** for
      production and **172.18.0.1** for staging (`docker inspect`), and it arrives as an
      **IPv4-mapped IPv6 address**, not a dotted quad.

      ⚠️ **`172.16.0.0/12` was worse than "broad".** The same bridge holds `prod-umami`
      (172.19.0.3), which is publicly reachable through Caddy, and `prod-db` (172.19.0.5). Four
      addresses were trusted where one is needed, and one of them accepts traffic from the internet.

      ⚠️ **The narrowing only works because of a subtlety worth stating.** `net.IPNet.Contains`
      normalizes IPv4-mapped addresses, so `::ffff:172.19.0.1` does match `172.19.0.1/32` — verified
      by running it, and pinned by `TestIPv4MappedPeerMatchesIPv4CIDR`. A refactor that "simplifies"
      the parsing would break production while every dotted-quad unit test kept passing.

      ⚠️ **A `/32` is unsafe without a pinned subnet, and this was the real trap.** Neither network
      declared one, so Docker assigned from a dynamic pool; a recreated network could land on another
      range and the failure would be **silent** — the API stops trusting Caddy, falls back to the
      unresolvable private peer, returns a null location, and `/health` still says `ok`. Both compose
      files now pin the subnet. Verified locally that adding a pin to a live network makes Compose
      recreate the network and its containers automatically (exit 0, no manual `docker network rm`),
      and that the pin yields the intended gateway.

      **The gate, in three parts** — the task rejected "extend the smoke test" because one runner
      address cannot distinguish one global bucket from correct per-client buckets:
      1. `handlers.VerifyGatewayTrusted` at startup reads the container's own default gateway from
         `/proc/net/route` and logs **ERROR** if it is outside `TRUSTED_PROXY_CIDRS`. This is the
         drift detector, and it is server-side and machine-checkable. It logs rather than exits: a
         wrong trust list is a degradation, and refusing to boot would convert it into an outage.
      2. `deploy/verify-proxy-trust.sh <env>` speaks to the API from **two different peers** — a
         throwaway container on the bridge forging `X-Forwarded-For` (must be disbelieved) and a real
         request through Caddy (must still be believed). Checking only the first would pass with the
         trust list empty, which breaks every real client, so both directions are asserted.
      3. Unit tests covering the drifted gateway, a bridge neighbour, and the IPv4-mapped form.

      ⚠️ **`/proc/net/route` is little-endian**: `010013AC` is 172.19.0.1, not 1.0.19.172. The first
      version of the parser got this backwards and returned a plausible wrong address; it is now
      pinned by a test against the measured value.

      **Still required before this can be ticked:**
      - [ ] Deploy to staging; confirm the log line `trusted-proxy check passed` with
            `gateway=172.18.0.1`, and that `/v1/context` still returns a real location.
      - [ ] Run `deploy/verify-proxy-trust.sh staging` and get `RESULT: PASS`.
      - [ ] Repeat both on production after promotion. ⚠️ The subnet pin recreates the network, so
            the production deploy restarts the whole stack rather than just the API.

      ~~Original text:~~ can actually detect the failure.
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
