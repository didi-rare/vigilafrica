---
description: Kicks off a new feature by drafting a proposal and technical spec following the OpenSpec workflow.
---
# /openspec-explore

Triggered when the user wants to start working on a new feature or change.

## Objectives
1. Ask the user clarifying questions about the feature if the requirements are ambiguous.
2. Generate a unique Change ID (e.g., `feature-[verb]-[noun]`).
3. Create a proposal document in `openspec/proposals/[change-id].md`.
4. Create a capability-focused technical specification in `openspec/specs/[change-id].md`.
5. Require user approval before moving to the `/openspec-apply` phase.

## Instructions
1. Analyze the user's prompt.
2. If more detail is needed, ask the user. 
3. Otherwise, use `write_to_file` to create `openspec/proposals/[change-id].md` summarising the "Why" and "What".
4. Use `write_to_file` to create `openspec/specs/[change-id].md` detailing the technical implementation, components to touch, and acceptance criteria.
5. Present the generated spec to the user and prompt them to run `/openspec-apply` when ready.
