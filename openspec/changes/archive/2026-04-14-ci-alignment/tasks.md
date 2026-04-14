# Tasks: CI Alignment

- [x] Update `go-version` to `1.25` in `.github/workflows/ci-cd.yml`
- [x] Update `go-version` to `1.25` in `.github/workflows/openspec-verify.yml`
- [x] Fix Sentinel entry point from `api/cmd/sentinel/main.go` to `api/cmd/server/main.go` in `.github/workflows/openspec-verify.yml`
- [x] Run `go mod tidy` in the `api` root directory
- [x] Sync the `v04-useful-prototype` finalized code specs with the master OpenSpec registry by running `./openspec sync` or addressing the manual drift validations natively.
