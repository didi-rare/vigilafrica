---
id: feat-filter-dropdown
status: proposed
branch: feat/filter-dropdown
---

# Proposal: Pristine Themed Filter Dropdown (feat-filter-dropdown)

## Why

The dashboard's three filters — Country, Category, State — are native
`<select class="dashboard-filter-select">` elements in `EventsDashboard.tsx`.
The **closed** control is themed (Ground Truth dark card, amber focus), but the
**open option list is rendered by the OS**, so on Windows/Chrome it drops down a
light-grey/blue native popup that clashes hard with the dark UI (reported with a
screenshot during the `feat-ground-truth-identity` live-map smoke, 2026-06-14).
CSS on the `<select>` cannot style that native popup — the only way to a
genuinely on-brand dropdown is to replace the native control with a custom,
fully-styleable listbox.

This is the follow-up deferred out of `feat-ground-truth-identity` (kept out of
that PR deliberately, as it is a functional component change, not brand work).

## What Changes

### 1. A reusable, accessible `Select` component

- New `web/src/components/Select.tsx` (+ `Select.css`): a WAI-ARIA
  **listbox**-pattern single-select — a button trigger that opens a themed popup
  of options. No dependency added; hand-rolled with the existing token system.
- **API is a drop-in for the current usage** (same value/onChange contract):
  ```ts
  type SelectOption = { value: string; label: string }
  interface SelectProps {
    id: string
    value: string
    onChange: (value: string) => void
    options: SelectOption[]
    disabled?: boolean
    'aria-label': string
  }
  ```
- **Accessibility (parity with — or better than — the native control):**
  trigger `aria-haspopup="listbox"` + `aria-expanded`; popup `role="listbox"`
  with `role="option"` + `aria-selected`; label via `aria-label` (the existing
  `sr-only` `<label htmlFor>` is preserved). Keyboard: Enter/Space/↓/↑ open;
  ↑/↓ move; Enter/Space select; Esc closes (focus returns to trigger);
  Home/End; Tab closes; type-ahead by first letter. Closes on outside-click and
  on blur. `aria-activedescendant` tracks the highlighted option.
- **Visual:** the trigger keeps the current `.dashboard-filter-select` look
  (bg-card, 1px border, `--radius-md`, amber `:focus-visible`, disabled 0.4);
  the popup is a themed panel (bg-card, border, radius, soft shadow) with option
  hover/active/selected states in amber. Tokens only — no colour literals
  (ADR-013). Open/close transition gated by `prefers-reduced-motion` (ADR-015).

### 2. Wire it into the dashboard filters

- Replace the three native `<select>`s in `EventsDashboard.tsx` (Country,
  Category, State) with `<Select>`, mapping the existing options to
  `{value,label}[]`. `onChange` calls the unchanged `handleCountryChange` /
  `handleCategoryChange` / `handleStateChange` — so URL-param state, the
  `country`→`state` cascade reset, and the `category_filter_selected` /
  `state_filter_selected` analytics events are all untouched. State select keeps
  its `disabled={availableStates.length === 0}` behaviour.

### 3. Tests

- `web/src/components/Select.test.tsx` (vitest + testing-library + vitest-axe):
  renders options, opens/closes via mouse + keyboard, selects via Enter and
  click, fires `onChange` with the option value, respects `disabled`, closes on
  Esc/outside-click, and has **zero axe violations** open and closed.
- Existing `EventsDashboard.test.tsx` updated for the new control (it currently
  drives the native `<select>` by value).

## Out of Scope

- **No multi-select, no search/combobox input, no option groups** — three short
  single-select lists don't need them.
- **No filtering behaviour change** — same params, same analytics, same cascade.
- **No new filters, routes, or API calls.**
- **No global form-control system** — this is the dashboard filter control;
  generalising to every input is a later change if ever wanted.
- **No new dependency** (no Radix/Headless/Downshift) — keeps the bundle and the
  plain-CSS posture (ADR-013).

## Capabilities

### Modified Capabilities

- `dashboard-filtering`: the Country/Category/State filters keep identical
  behaviour but render through a themed, accessible custom dropdown instead of
  the OS-native `<select>` popup, so the control is on-brand in every browser.

## Acceptance Criteria

- [ ] `Select.tsx` + `Select.css` added; no new npm dependency; no colour
      literals outside `tokens.css`.
- [ ] Drop-in API (`id`, `value`, `onChange(value)`, `options`, `disabled`,
      `aria-label`); the three dashboard filters use it with behaviour unchanged
      (URL params, `country`→`state` reset, analytics events, disabled state).
- [ ] Keyboard fully operable: open, arrow-navigate, select, Esc-close (focus
      returns to trigger), Home/End, type-ahead, Tab-closes.
- [ ] ARIA: `listbox`/`option`/`aria-selected`/`aria-expanded`/
      `aria-activedescendant`; vitest-axe reports **0 violations** open and closed.
- [ ] Themed popup: amber hover/selected states, reduced-motion-gated
      transition; matches the Ground Truth dark surface at 1440/768/390.
- [ ] Closes on outside-click and blur; opening one closes any other.
- [ ] All gates green: `tsc` build, eslint 0, stylelint clean, full vitest
      (incl. axe), plus new `Select.test.tsx`.

## Risks

- **R1 — A11y regression vs native.** Custom listboxes are easy to get subtly
  wrong (focus, SR semantics). Mitigation: follow the WAI-ARIA listbox pattern
  exactly; vitest-axe + explicit keyboard tests; manual SR spot-check.
- **R2 — Mobile/touch UX.** Native `<select>` gives OS pickers on touch.
  Mitigation: the custom listbox is touch-operable (tap trigger → tap option);
  verified at 390px. (Acceptable trade for cross-browser on-brand consistency.)
- **R3 — Behaviour drift.** Replacing the control could silently drop the
  `state` cascade reset or an analytics event. Mitigation: handlers are reused
  verbatim; `EventsDashboard.test.tsx` asserts the wiring.

## Verification Plan

1. `Select.test.tsx` green (mouse + keyboard paths, onChange value, disabled,
   Esc/outside-close, axe 0-violations open & closed).
2. `EventsDashboard.test.tsx` updated + green; full vitest suite green.
3. build + eslint + stylelint green.
4. Browser: the live dashboard at 1440/768/390 — open each filter, confirm the
   popup is dark/on-brand (no native grey/blue), hover/selected states read,
   keyboard works, reduced-motion respected, and filtering still drives the map
   + list as before.

## Origin

Deferred from `feat-ground-truth-identity` (2026-06-14): during the live-map
smoke the maintainer flagged the native `<select>` popup as off-theme and asked
for a "pristine" dropdown, to be built as its own change with an OpenSpec record
and tests rather than folded into the brand-identity PR.
