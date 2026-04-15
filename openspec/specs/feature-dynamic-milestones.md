---
change_id: feature-dynamic-milestones
status: spec
created_date: 2026-04-15
author: Claude Code
---

# Spec: Dynamic Milestone Tracking

## Objective

Replace the hardcoded `MILESTONES` array in `web/src/App.tsx` with a build-time generated
`milestones.json` file that is derived automatically from the locked OpenSpec milestone roadmap.
The UI always reflects the true project state without manual edits to the component.

---

## Architecture Decision

**Approach: Build-time static JSON generation**

A Node.js script (`scripts/generate-milestones.js`) runs as part of the web build.
It reads the locked milestone index in `openspec/specs/vigilafrica/roadmap.md` and outputs `web/src/data/milestones.json`.
The frontend imports this static JSON at bundle time — no API calls, no runtime latency.

| Approach | Verdict |
|---|---|
| Hardcoded array (current) | ❌ Manual, stale |
| Build-time static JSON | ✅ Selected — zero runtime cost, auto-synced |
| GitHub API at runtime | ❌ Rate limits, latency, key required |
| Dedicated API endpoint | ❌ Overengineered for this data volume |

---

## Milestone State Convention

The script determines milestone state from the locked milestone index in
`openspec/specs/vigilafrica/roadmap.md`.

| Roadmap status | Output state |
|---|---|
| `✅ Complete` | `complete: true` |
| `🔄 Active` | `active: true` |
| `🔴 Planned` | `active: false`, `complete: false` |

Milestone labels and display order are still derived from **`milestones.config.json`**
(root-level), while release state comes from the OpenSpec roadmap. This keeps the UI synced
with maintainer-approved milestone governance without rewriting historical checklist items.

---

## Files to Create / Modify

### New: `milestones.config.json` (project root)

Single source of truth for milestone labels and display order:

```json
{
  "milestones": [
    { "version": "v0.1", "label": "v0.1 · Foundation" },
    { "version": "v0.2", "label": "v0.2 · First real data flow" },
    { "version": "v0.3", "label": "v0.3 · Localization engine" },
    { "version": "v0.4", "label": "v0.4 · Map + near-me experience" },
    { "version": "v0.5", "label": "v0.5 · Alert engine" }
  ]
}
```

### New: `scripts/generate-milestones.js` (project root)

```js
// Reads roadmap milestone state + milestones.config.json
// Outputs web/src/data/milestones.json
// Run via: node scripts/generate-milestones.js
```

Logic:
1. Load `milestones.config.json`
2. Read the milestone index in `openspec/specs/vigilafrica/roadmap.md`
3. Map `✅ Complete` → `complete: true`
4. Map `🔄 Active` → `active: true`
5. Map `🔴 Planned` → `active: false, complete: false`
6. Write output to `web/src/data/milestones.json`

### New: `web/src/data/milestones.json` (generated, committed)

```json
[
  { "label": "v0.1 · Foundation", "active": false, "complete": true },
  { "label": "v0.2 · First real data flow", "active": false, "complete": true },
  { "label": "v0.3 · Localization engine", "active": false, "complete": true },
  { "label": "v0.4 · Map + near-me experience", "active": false, "complete": true },
  { "label": "v0.5 · Alert engine", "active": true, "complete": false }
]
```

### Modified: `root package.json` — add `prebuild:web` script

```json
{
  "scripts": {
    "generate:milestones": "node scripts/generate-milestones.js",
    "prebuild:web": "node scripts/generate-milestones.js"
  }
}
```

### Modified: `.github/workflows/ci-cd.yml` — add milestone generation step

```yaml
- name: Generate milestone data
  run: node scripts/generate-milestones.js
```

Place this BEFORE the `Build Web` step so the generated JSON is available during bundling.

### Modified: `web/src/App.tsx`

Remove the hardcoded `MILESTONES` constant (lines 46–51).
Replace with a static import:

```typescript
import MILESTONES from './data/milestones.json'
```

The shape of each item (`label`, `active`, `complete`) is preserved and the milestone list
rendering logic remains unchanged. The milestone-related status copy in the prototype banner and
roadmap paragraph may also be updated so the surrounding text stays consistent with the generated
milestone state.

---

## Acceptance Criteria

- [ ] `milestones.config.json` exists at project root with all milestones listed
- [ ] `scripts/generate-milestones.js` runs with `node scripts/generate-milestones.js` and produces valid JSON
- [ ] `web/src/data/milestones.json` is generated and committed
- [ ] `web/src/App.tsx` no longer contains a hardcoded `MILESTONES` array
- [ ] The UI renders the same milestone list with correct `active`/`complete` states
- [ ] `npm run build` (web) succeeds after the generator runs
- [ ] CI `ci-cd.yml` runs the generator before the web build step
- [ ] When a future milestone changes state in `openspec/specs/vigilafrica/roadmap.md` (for example from `🔄 Active` to `✅ Complete`), re-running the script automatically reflects that state in the generated JSON — no manual App.tsx edit required

---

## Out of Scope

- Changing the visual design of the milestone tracker
- Runtime fetching (script is build-time only)
- Modifying the OpenSpec directory naming conventions
- Adding fields beyond `label`, `active`, `complete`




