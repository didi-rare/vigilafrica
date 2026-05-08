---
description: Review completed work against the active OpenSpec spec and project standards.
---

Use this Codex-facing workflow as the `/openspec-review` backing prompt.

There is no repo-local `.agent/workflows/opsx-review.md` today, so this file is the canonical
review wrapper for Codex.

## Workflow

1. Locate the active spec in `openspec/specs/`. If there are multiple active specs and no
   change ID was provided, ask which one to review.
2. Read the active proposal, active spec, `Task.md`, and relevant files under `docs/standards/`.
3. Review as a code reviewer: lead with findings ordered by severity, grounded in file and line
   references.
4. Check:
   - Spec alignment: every acceptance criterion is met or explicitly deferred.
   - Standards alignment: relevant project standards are followed.
   - Security: no hardcoded secrets, unsafe SQL construction, tenant-boundary leaks, or
     sensitive storage mistakes.
   - Reliability: offline, sync, and idempotency behavior is addressed when relevant.
   - Tests: meaningful tests exist for changed behavior, or missing coverage is called out.
5. If there are no findings, say so clearly and mention residual risk or test gaps.
