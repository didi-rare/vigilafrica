# Design: fix-pipeline-hardening

## Problem Diagnosis

### Failure 1: `openspec-verify` — Broken Sentinel Audit Step

**Current (broken) step in `.github/workflows/openspec-verify.yml`:**
```yaml
- name: VigilAfrica Sentinel Audit
  working-directory: api
  run: go run cmd/server/main.go
```

`api/cmd/server/main.go` is the production HTTP server. It contains:
```go
dbURL := os.Getenv("DATABASE_URL")
if dbURL == "" {
    slog.Error("DATABASE_URL is not set")
    os.Exit(1)          // ← CI exits with code 1 here, every time
}
```

There is no `DATABASE_URL` in the CI environment. The binary compiles (~25s) then exits immediately with code 1. This is why the job fails at exactly 29s.

This step was introduced in `ci-recovery` as a "working-directory" fix but the intent (running a dedicated audit/sentinel binary) was never fulfilled — the `governance-sentinel` change that would build `cmd/sentinel/main.go` has all tasks unchecked and is still pending.

**Resolution:**
Replace with two steps that verify compile-time correctness and static analysis without requiring any runtime infrastructure:

```yaml
- name: Sentinel Audit — Static Analysis
  working-directory: api
  run: go vet ./...

- name: Sentinel Audit — Binary Smoke Test
  working-directory: api
  run: go build -o /dev/null ./cmd/server/
```

`go vet` catches misused format verbs, unreachable code, suspicious composite literals, and other correctness issues. `go build -o /dev/null` verifies the binary compiles cleanly — the same check already in `ci-cd.yml` line 43. Neither step requires a database.

**Future path:** When `governance-sentinel` is complete, this step can be upgraded to `go run cmd/sentinel/main.go --audit` for rich structural governance checks.

---

### Failure 2: `openspec-verify` — Drift Validation Flags Completed but Unarchived Proposals

The `openspec validate --specs` step exits non-zero when it finds proposals in `openspec/changes/` that are fully complete but not archived. Both `ci-alignment` and `ci-recovery` have every task marked `[x]` and must be moved to `openspec/changes/archive/`.

**Archive targets:**
```
openspec/changes/ci-alignment/     → openspec/changes/archive/2026-04-14-ci-alignment/
openspec/changes/ci-recovery/      → openspec/changes/archive/2026-04-14-ci-recovery/
```

The `governance-sentinel` change (all tasks `[ ]`) is genuinely in-progress and must NOT be archived.

---

### Failure 3: `build-and-test` — Non-Deterministic `npm install`

**Current:**
```yaml
- name: Install Root & Web Dependencies
  run: |
    npm install
    cd web && npm install
```

`npm install` re-resolves all semver ranges on every run. The `web/package.json` pins bleeding-edge ranges:
- `react@^19.2.4`
- `vite@^8.0.4`
- `typescript@~6.0.2`
- `maplibre-gl@^5.23.0`

Range resolution against npm's public registry under `ubuntu-latest` is non-deterministic. A newly published patch version, peer-dep conflict, or transient registry error causes silent failures. This also applies to `openspec-verify.yml`'s `npm install -g @fission-ai/openspec@latest`.

**Resolution:**
```yaml
- name: Cache npm dependencies
  uses: actions/cache@v4
  with:
    path: ~/.npm
    key: ${{ runner.os }}-npm-${{ hashFiles('**/package-lock.json') }}
    restore-keys: |
      ${{ runner.os }}-npm-

- name: Install Root & Web Dependencies
  run: |
    npm ci
    cd web && npm ci
```

`npm ci` installs exactly what is in `package-lock.json`, fails loudly on any mismatch, and is significantly faster than `npm install`. The cache layer avoids re-downloading packages on every run.

**Node version check:**
Vite 8.x and TypeScript 6.x require Node ≥ 20 LTS. The current `node-version: '20'` is at the boundary. If CI errors include `ERR_OSSL_EVP_UNSUPPORTED` or `Digital Envelope Routines` errors, upgrade to `node-version: '22'`.

---

## Affected Files

| File | Change |
|------|--------|
| `.github/workflows/openspec-verify.yml` | Replace Sentinel Audit step; add npm cache |
| `.github/workflows/ci-cd.yml` | `npm install` → `npm ci`; add npm cache; verify Node version |
| `openspec/changes/ci-alignment/` | Move to `openspec/changes/archive/2026-04-14-ci-alignment/` |
| `openspec/changes/ci-recovery/` | Move to `openspec/changes/archive/2026-04-14-ci-recovery/` |

## Acceptance Criteria

- [ ] `OpenSpec — Drift Verification / openspec-verify (pull_request)` passes green on next PR
- [ ] `VigilAfrica CI/CD / build-and-test (pull_request)` passes green on next PR
- [ ] `go vet ./...` exits 0 in the Sentinel Audit step
- [ ] `go build -o /dev/null ./cmd/server/` exits 0 in the Sentinel Audit step
- [ ] `openspec validate --specs` exits 0 (no unresolved drift)
- [ ] `npm ci` installs cleanly in both root and `web/`
- [ ] PR merging is unblocked on `feat/v04-useful-prototype`

## Not In Scope

- Completing the `governance-sentinel` change (separate ongoing work)
- Adding database integration tests or a Postgres service to CI
- Pinning exact versions in `web/package.json` (low-risk; defer to a separate chore)
- Any feature changes to the API or frontend
