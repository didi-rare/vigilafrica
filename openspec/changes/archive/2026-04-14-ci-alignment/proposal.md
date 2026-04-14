## Why

The recent deployment of the Sentinel v0.4 (Useful Prototype) containerization led to multiple governance and CI pipeline failures. The drift was caused by out-of-sync configurations in the CI workflows (`go.mod` tidy discrepancies on host vs container, out-of-date Go version 1.26 vs 1.25, and outdated API entry path names) along with an OpenSpec structural mismatch after the v0.4 archive was relocated without syncing master specs. This change resolves those mismatches and aligns the repository state with our actual implementation.

## What Changes

- Fix CI/CD pipeline versions: Set Go version to 1.25 in `.github/workflows/ci-cd.yml` and `.github/workflows/openspec-verify.yml`.
- Fix structural entry point path in `openspec-verify.yml` to standard `api/cmd/server/main.go`.
- Synchronize missing `go.mod` checksums into the repository by running `go mod tidy` in the `api` root.
- Synchronize the governance states to ensure no drift validation errors after the v0.4 archival.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
None.

## Impact

- `.github/workflows/ci-cd.yml` (Go version alignment)
- `.github/workflows/openspec-verify.yml` (Go version and script entry point alignment)
- `api/go.mod` and `api/go.sum`
- OpenSpec repository governance tracking (drift resolution)
