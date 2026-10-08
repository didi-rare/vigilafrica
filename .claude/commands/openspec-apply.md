---
description: Implements the active technical spec by breaking it into tasks and writing the code.
---
# /openspec-apply

Triggered when the user wants to begin coding an approved OpenSpec document.

## Objectives
1. Read the active spec from `openspec/specs/`.
2. Break the spec down into a verifiable task list in the project's root `Task.md` (or `task.md` artifact).
3. Systematically implement the required changes.
4. Update checkboxes in the task list as progress is made.

## Instructions
1. Determine the active change ID. If ambiguous, ask the user or check the most recent un-archived spec in `openspec/specs/`.
2. Use `view_file` to thoroughly read the spec.
3. Update `Task.md` using `replace_file_content` or by creating a task artifact to reflect the actionable steps.
4. Execute the code changes (create/modify files, write tests) systematically.
5. After finishing implementation, inform the user to review the changes and run `/openspec-archive` when they are satisfied.
