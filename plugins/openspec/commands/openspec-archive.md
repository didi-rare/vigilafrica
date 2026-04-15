---
description: Validate and archive a completed OpenSpec change.
argument-hint: [change-id]
allowed-tools: [Read, Glob, Grep, Bash, Write, Edit]
---

# /openspec-archive

Archive a completed OpenSpec change.

## Arguments

The user invoked this command with: $ARGUMENTS

## Workflow

1. Read `.agents/workflows/openspec-archive.md` if it exists and preserve its intent.
2. Confirm the active spec exists in `openspec/specs/`. If there are multiple active specs and
   no change ID was provided, ask which one to archive.
3. Confirm implementation has been reviewed or verified.
4. Run relevant tests before moving files.
5. Move `openspec/proposals/<change-id>.md` to `openspec/archive/proposal-<change-id>.md` if it
   exists.
6. Move `openspec/specs/<change-id>.md` to `openspec/archive/spec-<change-id>.md`.
7. Check whether the root product spec or docs need a small update.
8. Report what was archived and what verification ran.

Do not archive if acceptance criteria are materially unmet unless the user explicitly approves
archiving with known gaps.
