## Why

The CI/CD pipelines are currently failing due to a "Triple Regression": a package name conflict in the Go root, brittle unit tests failing on floating-point precision, and a missing directory context in the OpenSpec verification workflow. This recovery change stabilizes the command center by removing redundant files and making automation more robust.

## What Changes

- [DELETE] `api/scratch.go` to resolve the `package main` vs `package api` conflict.
- [MODIFY] `api/internal/normalizer/normalizer_test.go` to use semantic coordinate comparison instead of string matching.
- [MODIFY] `.github/workflows/openspec-verify.yml` to set `working-directory: api` for the Sentinel Audit step.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
None.

## Impact

- Build reliability for `VigilAfrica CI/CD`.
- Accuracy of `OpenSpec — Drift Verification`.
