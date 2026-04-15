---
description: Draft an OpenSpec proposal and implementation spec for a new change.
argument-hint: [change request]
allowed-tools: [Read, Glob, Grep, Write, Edit]
---

# /openspec-explore

Start a new OpenSpec change for this repository.

## Arguments

The user invoked this command with: $ARGUMENTS

## Workflow

1. Read `.agents/workflows/openspec-explore.md` if it exists and preserve its intent.
2. Clarify requirements only if the request is too ambiguous to draft a useful proposal.
3. Choose a unique lowercase hyphenated change ID, such as `feature-record-offline-sale`.
4. Create `openspec/proposals/<change-id>.md` with:
   - Why
   - What changes
   - Out of scope
   - User impact
5. Create `openspec/specs/<change-id>.md` with:
   - Context
   - Components to touch
   - Implementation plan
   - Acceptance criteria
   - Verification plan
6. Present the change ID and ask for approval before implementation unless the user explicitly
   asks to continue.
