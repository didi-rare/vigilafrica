---
description: Validate and archive a completed OpenSpec change.
---

Use this Codex-facing workflow as the `/openspec-archive` backing prompt.

If `.agent/workflows/opsx-archive.md` exists, preserve its intent. Prefer that file as the
repo-specific archive workflow and use this file as the Codex wrapper.

## Workflow

1. Confirm the active spec exists in `openspec/specs/`. If there are multiple active specs and
   no change ID was provided, ask which one to archive.
2. Confirm implementation has been reviewed or verified.
3. Run relevant tests before moving files.
4. Move `openspec/proposals/<change-id>.md` to `openspec/archive/proposal-<change-id>.md` if it
   exists.
5. Move `openspec/specs/<change-id>.md` to `openspec/archive/spec-<change-id>.md`.
6. Check whether the root product spec or docs need a small update.
7. Report what was archived and what verification ran.

Do not archive if acceptance criteria are materially unmet unless the user explicitly approves
archiving with known gaps.
