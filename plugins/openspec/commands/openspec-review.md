---
description: Review completed work against the active OpenSpec spec and project standards.
argument-hint: [change-id]
allowed-tools: [Read, Glob, Grep, Bash]
---

# /openspec-review

Review a completed OpenSpec change.

## Arguments

The user invoked this command with: $ARGUMENTS

## Workflow

1. Read `.agents/workflows/openspec-review.md` if it exists and preserve its intent.
2. Locate the active spec in `openspec/specs/`. If there are multiple active specs and no change
   ID was provided, ask which one to review.
3. Read the active proposal, active spec, `Task.md`, and relevant files under `docs/standards/`.
4. Review as a code reviewer: lead with findings ordered by severity, grounded in file and line
   references.
5. Check:
   - Spec alignment: every acceptance criterion is met or explicitly deferred.
   - Standards alignment: relevant project standards are followed.
   - Security: no hardcoded secrets, unsafe SQL construction, tenant-boundary leaks, or
     sensitive storage mistakes.
   - Reliability: offline, sync, and idempotency behavior is addressed when relevant.
   - Tests: meaningful tests exist for changed behavior, or missing coverage is called out.
6. If there are no findings, say so clearly and mention residual risk or test gaps.
