---
description: Implement the active OpenSpec change with a tracked task checklist.
---

Use this Codex-facing workflow as the `/openspec-apply` backing prompt.

If `.agent/workflows/opsx-apply.md` exists, preserve its intent. Prefer that file as the
repo-specific implementation workflow and use this file as the Codex wrapper.

## Workflow

1. Locate the active spec in `openspec/specs/`. If there are multiple active specs and no
   change ID was provided, ask which one to apply.
2. Read the active proposal, active spec, and relevant files under `docs/standards/`.
3. Create or update `Task.md` in the repository root with an actionable checklist.
4. Implement the change in small, verifiable steps.
5. Update checklist statuses as tasks complete.
6. Run relevant tests or verification. If a required check cannot run, explain why.
7. Tell the user the change is ready for review and suggest `/openspec-review` or
   `/openspec-archive` as appropriate.
