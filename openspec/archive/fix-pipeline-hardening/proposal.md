## Why

Two GitHub Actions checks block every PR merge on `feat/v04-useful-prototype` (and will continue to block all future PRs until fixed):

1. **`OpenSpec — Drift Verification / openspec-verify`** — Fails after 29s because the "VigilAfrica Sentinel Audit" step runs `go run cmd/server/main.go`, which is the production HTTP server. The server immediately calls `os.Exit(1)` when `DATABASE_URL` is not set. CI has no database. This step has never been able to succeed.

2. **`VigilAfrica CI/CD / build-and-test`** — Fails after 41s, most likely due to non-deterministic `npm install` against bleeding-edge version ranges (`react@^19.2.4`, `vite@^8.0.4`, `typescript@~6.0.2`) that fail to resolve or produce peer-dependency conflicts in the GitHub Actions `ubuntu-latest` environment.

A secondary but compounding cause of `openspec-verify` failures: the `ci-alignment` and `ci-recovery` change proposals have all tasks marked complete (`[x]`) but have not been archived. The `openspec validate --specs` drift check treats fully-completed but unarchived proposals as unresolved governance drift, which causes the step to exit with a non-zero code before the broken Sentinel Audit step is even reached.

## What Changes

- **[REMOVE / REPLACE]** `openspec-verify.yml` step "VigilAfrica Sentinel Audit": Replace `go run cmd/server/main.go` with `go vet ./...` + `go build -o /dev/null ./cmd/server/` — these verify code correctness and binary compilation without requiring a database or starting a server.

- **[ARCHIVE]** `openspec/changes/ci-alignment` → `openspec/changes/archive/2026-04-14-ci-alignment`

- **[ARCHIVE]** `openspec/changes/ci-recovery` → `openspec/changes/archive/2026-04-14-ci-recovery`

- **[MODIFY]** `ci-cd.yml` and `openspec-verify.yml`: Switch `npm install` to `npm ci` for deterministic, lockfile-bound dependency installation.

- **[ADD]** npm dependency cache using `actions/cache@v4` keyed on `package-lock.json` hash.

- **[VERIFY]** Node version compatibility: confirm `node-version: '20'` is sufficient for `vite@^8.0.4` and `typescript@~6.0.2`, or upgrade to `node-version: '22'` if not.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
None — this is a pipeline governance hardening change only.

## Impact

- `.github/workflows/openspec-verify.yml` (Sentinel Audit step replacement; npm install → ci)
- `.github/workflows/ci-cd.yml` (npm install → ci; npm cache; Node version verification)
- `openspec/changes/ci-alignment/` → archived
- `openspec/changes/ci-recovery/` → archived
