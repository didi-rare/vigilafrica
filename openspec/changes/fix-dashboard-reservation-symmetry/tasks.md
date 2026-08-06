# Tasks: Make the Dashboard Reservation Symmetric

## 1. Implementation

- [x] 1.1 Add `min-height: min(100vh, 1460px)` to `.dashboard.section`, identical to `.dashboard-fallback`
- [x] 1.2 Mirror the `max-width: 480px` → `min-height: 0` exemption so both sides waive together
- [x] 1.3 Cross-reference both rules in comments — they must stay in step, and they live in different files

## 2. Verification

- [x] 2.1 `npm run lint` clean
- [x] 2.2 `stylelint src/**/*.css` clean
- [x] 2.3 `npm run type-check` clean
- [x] 2.4 `npm run test` — 70/70 across 9 files
- [ ] 2.5 Post-deploy: confirm no shift when the dashboard resolves into its **error** state on a >1460px-tall viewport — the case this change exists for
- [ ] 2.6 Re-check CLS on staging across **≥3 runs** (a single post-deploy run has twice shown a phantom regression)

## 3. Deliberately not done

- [ ] 3.1 ~~Cap the reservation to "a height the dashboard always occupies"~~ — the reviewer's alternative remedy. **Rejected:** that height is the error state's, which is small, so it would reintroduce the ~1459px shift #193/#198 exist to prevent.
- [ ] 3.2 ~~Change the 1460px cap or the 480px breakpoint~~ — both are backed by measured trial data in the archived `fix-dashboard-layout-reservation`.

## 4. Known gap

- [ ] 4.1 No automated check covers this. The existing tests render components but assert nothing about layout height, and CI has no visual or CLS gate — which is why #193 shipped a UX regression with 100% green verification. Worth a headless height-comparison test if this area is touched again.
