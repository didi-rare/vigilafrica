# Proposal: Make the Dashboard Reservation Symmetric (fix-dashboard-reservation-symmetry)

## Why

The Suspense fallback reserves height while the lazy dashboard chunk loads (`.dashboard-fallback`, `App.css`). The **resolved** dashboard has no matching floor.

A reservation only removes a layout shift if the thing that replaces it occupies the same space. Today that holds by coincidence — the dashboard happens to mount at ~1459px above 768px because `.dashboard-layout` is `height: 800px` — not by construction.

**It stops holding whenever the dashboard does not render its full layout:**

- the **API error state** (`.dashboard-state` — a small message and retry button)
- an **empty result set**
- a filtered view with few events

In those states `.dashboard-layout`'s fixed 800px never applies and the section collapses to a small block. On a viewport taller than the reservation, the page **shrinks** when the chunk resolves — a visible upward shift. The reservation has moved the shift rather than removed it.

## Origin, and a correction to the finding that prompted it

Raised by an external reviewer against `App.css:442`, which proposed either *"give the resolved dashboard the same minimum height"* or *"cap the reservation to a height the dashboard always occupies."*

⚠️ **The finding's stated premise is out of date.** It describes an uncapped `100vh` reservation that "becomes substantially taller than the resolved dashboard" on tall viewports. That cap already shipped in **#198** — the rule is `min-height: min(100vh, 1460px)`, so at 2560px tall it resolves to 1460px against a ~1459px dashboard. The described case is already handled.

**But its first remedy was genuinely missing, and it closes the mirror case the cap does not cover** — the dashboard resolving *shorter* than the reservation. That is what this change implements.

The reviewer's second remedy — capping to "a height the dashboard always occupies" — is **rejected**: the height the dashboard *always* occupies is the error state's, which is small, and reserving only that would reintroduce the original ~1459px shift that #193/#198 exist to prevent.

## What Changes

`.dashboard.section` gets the **same** floor as `.dashboard-fallback`:

```css
min-height: min(100vh, 1460px);
```

plus the same `min-height: 0` exemption below 480px, where the reservation is deliberately dropped because the taller mobile hero already pushes the next section below the fold.

The two rules must stay in step; both carry comments saying so.

## Trade-off, accepted deliberately

On a viewport taller than ~1460px whose dashboard resolves short, this leaves a blank band below the content.

That band is **exactly what the fallback already reserved**, so the user sees no movement — and a static band is preferable to a jump in a product whose dashboard is the reason people scrolled. On typical desktop heights (`100vh < 1460px`) the real dashboard exceeds the floor and it never binds.

This is the same trade-off #198 weighed, applied consistently to both sides of the swap instead of only one.

## Out of Scope

- Changing the 1460px cap or the 480px breakpoint — both are backed by the measured trial data in the archived `fix-dashboard-layout-reservation`.
- Virtualising or paginating the events list — that is [feature-events-pagination](../feature-events-pagination/proposal.md).
- Any change to what the dashboard renders.

## Verification

- [x] `npm run lint`, `stylelint`, `type-check` clean
- [x] `npm run test` — 70/70 across 9 files
- [ ] Post-deploy: confirm no shift when the dashboard resolves into its **error** state on a >1460px-tall viewport — the case this change exists for, and the one no automated check covers
- [ ] Re-check CLS on staging across **≥3 runs**, per the standing note that a single post-deploy run has twice shown a phantom regression
