# fix-mobile-and-status-accuracy

**Branch:** `fix/mobile-and-status`
**Proposal:** `openspec/proposals/fix-mobile-and-status-accuracy.md`
**Spec:** `openspec/specs/fix-mobile-and-status-accuracy.md`

## 1. Housekeeping

- [x] 1.1 Confirm active OpenSpec docs, React standards, and accessibility scope
- [x] 1.2 Replace stale `Task.md` content with this active change checklist
- [x] 1.3 Mark proposal/spec status as in progress

## 2. Mobile Layout

- [x] 2.1 Stack the dashboard layout at mobile widths and remove the fixed height
- [x] 2.2 Let the page own scrolling on mobile instead of nested sidebar scrolling
- [x] 2.3 Make event location text wrap without card or viewport overflow
- [x] 2.4 Add sticky-nav-safe scroll margins for section anchors
- [x] 2.5 Fix event detail header overflow caught during mobile browser pass

## 3. Status Accuracy

- [x] 3.1 Update the prototype banner for v0.7 complete, v1.0 staging live
- [x] 3.2 Refresh status body copy and remove literal markdown asterisks
- [x] 3.3 Update `milestones.json` so v0.7 is complete and v1.0 is active
- [x] 3.4 Rewrite the page meta description for Nigeria and Ghana live

## 4. Map Glyphs

- [x] 4.1 Remove the failing demotiles glyph dependency
- [x] 4.2 Keep cluster-count labels rendering without glyph 404 warnings

## 5. Accessibility

- [x] 5.1 Add skip-to-main as the first focusable element
- [x] 5.2 Fix step list semantics so axe reports no `aria-allowed-role` violations
- [x] 5.3 Hide milestone status emojis from assistive tech and preserve label spacing
- [x] 5.4 Add reduced-motion handling for page and map animations

## 6. Verification

- [x] 6.1 Add focused tests for skip link, milestone emoji accessibility, list semantics, and glyph config
- [x] 6.2 Run web lint
- [x] 6.3 Run web tests
- [x] 6.4 Run web build
- [x] 6.5 Run OpenSpec validation
