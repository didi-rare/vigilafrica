---
change_id: feature-dynamic-milestones
status: proposal
created_date: 2026-04-14
author: Claude Code
---

# Proposal: Dynamic Milestone Tracking

## Problem

The Project Status section on the landing page (`web/src/App.tsx`, lines 46–51) contains a hardcoded milestones array:

```typescript
const MILESTONES = [
  { label: 'v0.1 · Foundation', active: false, complete: true },
  { label: 'v0.2 · First real data flow', active: false, complete: true },
  { label: 'v0.3 · Localization engine', active: true, complete: false },
  { label: 'v0.4 · Map + near-me experience', active: false, complete: false },
]
```

**Issues:**
1. **Stale data** — This array does not reflect the maintainer-approved release state. `v0.1` through `v0.4` are release-complete, while `v0.5` is the active milestone.
2. **Maintenance burden** — Every time we complete a milestone or start a new one, a developer must edit the component.
3. **Source-of-truth mismatch** — The real milestone release state should live in OpenSpec governance, specifically the locked milestone index in `openspec/specs/vigilafrica/roadmap.md`, but the UI has its own copy.
4. **Scalability** — As the project grows to v0.5, v0.6, etc., this hardcoded approach won't scale.

## Solution

Fetch the milestone state dynamically from the OpenSpec governance documents:

1. **Backend endpoint** (API) or **static generation** (build-time) that reads the locked milestone index in `openspec/specs/vigilafrica/roadmap.md` to determine:
   - Which milestones are release-complete (`✅ Complete`)
   - Which milestone is active (`🔄 Active`)
   - Which milestones remain planned (`🔴 Planned`)

2. **Frontend component** (`EventsDashboard` or new `MilestoneTracker` component) that:
   - Calls the endpoint or reads static JSON at build-time
   - Renders the milestones list in sync with OpenSpec governance

3. **Sync guarantee** — The milestone tracker is now always in sync with the OpenSpec roadmap release state.

## Impact

- ✅ **UI always reflects actual project state** — No stale milestones
- ✅ **Reduced manual maintenance** — Milestone progress updates automatically when the locked roadmap index is updated
- ✅ **Scalable** — Supports unlimited future milestones without code changes
- ✅ **Trustworthy** — Single source of truth (OpenSpec structure)

## Out of Scope

- Changing the OpenSpec directory structure
- Modifying the visual design of the milestone tracker
- Adding new milestone metadata (beyond label, active, complete)




