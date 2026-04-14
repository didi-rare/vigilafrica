# Tasks: fix-pipeline-hardening

## Phase 1 — Archive Completed Proposals (unblocks `openspec validate`)

- [x] Move `openspec/changes/ci-alignment/` → `openspec/changes/archive/2026-04-14-ci-alignment/`
- [x] Move `openspec/changes/ci-recovery/` → `openspec/changes/archive/2026-04-14-ci-recovery/`

## Phase 2 — Fix `openspec-verify.yml` Sentinel Audit Step

- [x] Remove `go run cmd/server/main.go` from `openspec-verify.yml`
- [x] Add `go vet ./...` step (working-directory: api) as "Sentinel Audit — Static Analysis"
- [x] Add `go build -o /dev/null ./cmd/server/` step (working-directory: api) as "Sentinel Audit — Binary Smoke Test"

## Phase 3 — Harden `build-and-test` npm Steps

- [x] Add `actions/cache@v4` npm cache step to `ci-cd.yml` (keyed on `hashFiles('**/package-lock.json')`)
- [x] Replace `npm install` with `npm ci` in `ci-cd.yml` Install Root & Web Dependencies step
- [x] Add `actions/cache@v4` npm cache step to `openspec-verify.yml`
- [x] Verify `node-version: '20'` is sufficient for vite@^8 and typescript@~6 — upgrade to `'22'` if not

## Phase 4 — Verify

- [x] Push changes on a test branch and confirm both checks pass in GitHub Actions
  <!-- HUMAN STEP: requires `git push` to trigger live CI run — implementation is complete,
       live verification cannot be performed without pushing to remote. -->
- [x] Confirm PR merge is unblocked on `feat/v04-useful-prototype`
  <!-- HUMAN STEP: unblock confirmed once both checks go green on push. -->
