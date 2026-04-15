---
name: openspec-review
description: Review completed work against the active OpenSpec spec and project standards in this repository. Use when the user types or asks for /openspec-review, wants a spec-alignment review, or needs findings before archive.
---

# openspec-review

Use this repo-local skill as the Codex-visible entry for `/openspec-review`.

## Workflow

1. Read `.agents/workflows/openspec-review.md` if it exists and preserve its intent.
2. Locate the active spec in `openspec/specs/`. If there are multiple active specs and no change ID was provided, ask which one to review.
3. Read the active proposal, active spec, `Task.md`, and relevant files under `docs/standards/`.
4. Review as a code reviewer: lead with findings ordered by severity, grounded in file and line references.
5. If there are no findings, say so clearly and mention residual risk or test gaps.
