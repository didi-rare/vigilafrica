---
name: openspec-archive
description: Archive a verified OpenSpec change in this repository. Use when the user types or asks for /openspec-archive, wants to close out a completed change, or needs proposals/specs moved into archive after verification.
---

# openspec-archive

Use this repo-local skill as the Codex-visible entry for `/openspec-archive`.

## Workflow

1. Read `.agents/workflows/openspec-archive.md` if it exists and preserve its intent.
2. Confirm the active spec exists in `openspec/specs/`. If there are multiple active specs and no change ID was provided, ask which one to archive.
3. Run relevant tests before moving files.
4. Move `openspec/proposals/<change-id>.md` to `openspec/archive/proposal-<change-id>.md` if it exists.
5. Move `openspec/specs/<change-id>.md` to `openspec/archive/spec-<change-id>.md`.
6. Report what was archived and what verification ran.
