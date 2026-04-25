---
description: Draft an OpenSpec proposal and implementation spec for a new change.
---

Use this Codex-facing workflow as the `/openspec-explore` backing prompt.

If `.agent/workflows/opsx-explore.md` exists, preserve its intent. Prefer that file as the
repo-specific stance and use this file as the Codex wrapper.

## Workflow

1. Clarify requirements only if the request is too ambiguous to draft a useful proposal.
2. Choose a unique lowercase hyphenated change ID, such as `feature-record-offline-sale`.
3. Create `openspec/proposals/<change-id>.md` with:
   - Why
   - What changes
   - Out of scope
   - User impact
4. Create `openspec/specs/<change-id>.md` with:
   - Context
   - Components to touch
   - Implementation plan
   - Acceptance criteria
   - Verification plan
5. Present the change ID and ask for approval before implementation unless the user explicitly
   asks to continue.
