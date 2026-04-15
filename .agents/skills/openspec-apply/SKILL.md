---
name: openspec-apply
description: Implement an approved OpenSpec change in this repository. Use when the user types or asks for /openspec-apply, wants to continue implementation, or needs task-driven progress against an active proposal/spec pair.
---

# openspec-apply

Use this repo-local skill as the Codex-visible entry for `/openspec-apply`.

## Workflow

1. Read `.agents/workflows/openspec-apply.md` if it exists and preserve its intent.
2. Locate the active spec in `openspec/specs/`. If there are multiple active specs and no change ID was provided, ask which one to apply.
3. Read the active proposal, active spec, and relevant files under `docs/standards/`.
4. Create or update `Task.md` in the repository root with an actionable checklist.
5. Implement the change in small, verifiable steps.
6. Update checklist statuses as tasks complete.
7. Run relevant tests or verification. If a required check cannot run, explain why.
8. Tell the user the change is ready for review and suggest `/openspec-review` or `/openspec-archive` as appropriate.
