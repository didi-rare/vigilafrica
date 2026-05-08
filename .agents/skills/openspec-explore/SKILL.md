---
name: openspec-explore
description: Draft a new OpenSpec proposal and implementation spec in this repository. Use when the user types or asks for /openspec-explore, wants to start a new feature/change with OpenSpec, or needs a change ID plus proposal/spec documents under openspec/.
---

# openspec-explore

Use this repo-local skill as the Codex-visible entry for `/openspec-explore`.

## Workflow

1. Read `.agents/workflows/openspec-explore.md` if it exists and preserve its intent.
2. Treat `openspec/proposals/`, `openspec/specs/`, and `openspec/archive/` as the OpenSpec document roots.
3. Clarify requirements only if a useful proposal/spec cannot be drafted from the user request.
4. Choose a unique lowercase hyphenated change ID, such as `feature-record-offline-sale`.
5. Create `openspec/proposals/<change-id>.md` with `Why`, `What changes`, `Out of scope`, and `User impact`.
6. Create `openspec/specs/<change-id>.md` with `Context`, `Components to touch`, `Implementation plan`, `Acceptance criteria`, and `Verification plan`.
7. Present the change ID and ask for approval before implementation unless the user explicitly asks to continue.

## Notes

- Keep proposal and spec documents concise, capability-focused, and testable.
- If the global `openspec-workflows` skill is available, use it as the broader lifecycle reference and keep this skill focused on the Explore phase.
