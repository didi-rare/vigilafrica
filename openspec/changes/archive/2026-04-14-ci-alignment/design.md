## Context

The CI pipelines and validation mechanisms currently flag drift and mismatched metadata following the recent v0.4 release. Specifically, the repository structure shifted, the container environment was upgraded to Go 1.25 (while the CI pipeline expected Go 1.26 or wasn't cleanly tidied), and the sentinel entry point path changed.

## Goals / Non-Goals

**Goals:**
- Align CI environment (Go 1.25) to mirror local container deployment.
- Resolve GitHub Actions OpenSpec drift failures due to incorrect hardcoded paths.
- Execute `go mod tidy` on the host layer to resolve metadata checksum warnings.
- Synchronize OpenSpec to clear the "Missing Active Proposals" archive governance failure.

**Non-Goals:**
- Refactoring the core server logic.
- Adding net-new functionality.

## Decisions

- **Sync CI Go Version**: We will explicitly pin `ci-cd.yml` and `openspec-verify.yml` to Go version 1.25 to prevent arbitrary `go mod tidy` discrepancies that occur when the runner has a newer parser.
- **Update Drift Validation Path**: Modify `openspec-verify.yml` from `api/cmd/sentinel/main.go` to `api/cmd/server/main.go` to match ADR-007.

## Risks / Trade-offs

- Running `go mod tidy` locally might pull newer patch versions of indirect dependencies. This will be captured in `go.sum` and committed.
