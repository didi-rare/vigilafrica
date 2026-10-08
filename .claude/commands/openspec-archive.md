---
description: Validates and closes out a completed feature by archiving its OpenSpec documents.
---
# /openspec-archive

Triggered when the user determines the feature implementation is complete and verified.

## Objectives
1. Validate that the implemented codebase matches the defined spec in `openspec/specs/`.
2. Ensure tests pass and acceptance criteria are met.
3. Move the proposal and spec files into the `openspec/archive/` folder.
4. Verify if the overarching product specification document requires an update.

## Instructions
1. Review the completed tasks against the active `openspec/specs/[change-id].md`.
2. Execute any relevant tests using `run_command` (if configured in the project).
3. If everything is successfully validated, use `run_command` to `mv` the proposal from `openspec/proposals/[change-id].md` to `openspec/archive/proposal-[change-id].md` and the spec from `openspec/specs/[change-id].md` to `openspec/archive/spec-[change-id].md`.
4. Inform the user that the feature is officially closed out and archived.
