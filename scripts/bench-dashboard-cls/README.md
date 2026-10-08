# bench-dashboard-cls — layout stability of the events dashboard

Two Playwright harnesses backing the layout-stability claims in
[`feature-events-pagination`](../../openspec/changes/feature-events-pagination/tasks.md)
task 4.6, and the reservation comment in `web/src/App.css`.

Committed because an independent review of this change flagged those CLS figures
as **asserted without a reproducible artifact** — the same finding that produced
the standing rule for `scripts/bench-enrichment/`: a number that drives a
decision ships with a runnable script, or it is an assertion, not evidence.

| script | answers |
|---|---|
| `measure-height.mjs` | how many pixels does the pagination bar add to the mounted dashboard, and does the `.dashboard-fallback` reservation still cover it? |
| `measure-cls.mjs` | does the change alter CLS, measured **against a control build** rather than alone? |
| `capture-fallback.mjs` | while the dashboard chunk is held back, is a loading affordance inside the viewport, and where exactly do the bar, card and spinner sit relative to the fold? (`chore-web-audit-leftovers` item 4) |
| `check-reduced-motion.mjs` | under `prefers-reduced-motion: reduce`, is the **computed** animation on each pseudo-element actually `none`? Run it against a **staging** build, the only one with the banner. (the #191 near-miss, re-checked for the staging-banner glow) |

## Prerequisites

Playwright is deliberately **not** a committed dependency — these are occasional
diagnostics and it would add a browser download to every `npm ci`. Install it
transiently **at the repository root** (Node resolves the import from there, so
installing it under `web/` will not work), and run the scripts from the root:

```sh
npm i --no-save --no-package-lock playwright
npx playwright install chromium
```

If a Chromium is already on the machine (`PLAYWRIGHT_BROWSERS_PATH` set, as in
the cloud session containers), pin the Playwright release that ships that
build instead of downloading another — `chromium-1194` is Playwright 1.56.x,
and a newer Playwright will refuse to launch an older build.

All four scripts mock every API response (`/health`, `/v1/context`, `/v1/states`,
`/v1/events`), so no API needs to be running. That is not only convenience: the
production API rejects cross-origin fetches from `localhost`, and mocking lets
the fixture be pinned at the 3,268-event continental scale rather than today's 43.

## Measuring the height delta

```sh
cd web && npm run build && npx vite preview --port 4173
# in another shell:
node scripts/bench-dashboard-cls/measure-height.mjs
```

It A/Bs **within a single page load** — measures `#dashboard`, sets
`display: none` on `.dashboard-results`, measures again — so the delta is the
bar's full contribution including its collapsed margin, with nothing else varying.

Result at the time of writing: **+64px** (mounted `#dashboard` 1459px → 1523px),
identical at 43 and 3,268 events across 1350x940, 1920x1600 and 768x1024, because
the bar's height is fixed by `min-height` and does not vary with the row count.
At 375px width it wraps to two lines and adds +99px, where no reservation applies.

## Measuring CLS

Needs **two** builds served simultaneously — the branch and a control:

```sh
# control
git worktree add /tmp/baseline origin/development
cd /tmp/baseline/web && npm ci && npm run build && npx vite preview --port 4174

# branch
cd web && npm run build && npx vite preview --port 4173

node scripts/bench-dashboard-cls/measure-cls.mjs
```

⚠️ **The control arm is not optional.** The page carries a small pre-existing
shift — the freshness banner appearing pushes `.dashboard-filters` — so a
single-arm number cannot distinguish "my change shifts the page" from "the page
already shifted". Measuring the branch alone is how a regression gets certified
as fine.

The dashboard chunk is delayed 1,200 ms (`CHUNK_DELAY_MS`) so the reservation is
genuinely on screen before the mount; without it the chunk arrives too fast to
observe the shift it exists to prevent.

### What it found

This harness caught a regression introduced by this very change: gating the
results bar on `eventsData` made it appear when the fetch resolved and shove the
800px layout down 64px.

| build | CLS | runs shifting |
|---|---|---|
| `origin/development` (control) | 0.0059 | 8/8 |
| branch, bar gated on data | **0.0122** | 5/5 |
| branch, container always rendered | **0.0054** | 8/8 |

⚠️ The residual ~0.0054 is **pre-existing** — the freshness banner, present
identically on the control — and is not addressed by this change.

⚠️ Raising the `.dashboard-fallback` cap 1460px → 1530px was measured to change
CLS by **nothing** at 1920x1600. At realistic viewports the content below the
reservation is already off-screen, and an off-screen shift is not counted. The
cap is maintained so it never falls below the real mounted height, not because it
was observed to fix a shift — do not cite it as one.

### Re-measured for `chore-web-audit-leftovers`

That change put the shared `.loading-state` card (spinner + status text)
inside the reservation and added a fixed, zero-footprint progress bar.
Measured with `VIEWPORT=<w>x<h>` (new override) at the three audit viewports
plus the harness default, 8 runs per cell, control = `origin/development` at
`3040a00`:

| viewport | control CLS | branch CLS |
|---|---|---|
| 1920x1600 | 0.0054 (8/8) | 0.0054 ×6, 0.0000 ×2 (6/8) |
| 1350x940 | 0.0001 (0/8) | 0.0001 (0/8) |
| 768x1024 | 0.0002 (0/8) | 0.0002 (0/8) |
| 375x812 | 0.0003 ×7, 0.0002 ×1 (0/8) | 0.0003 (0/8) |

No cell worse than the control. The 1920x1600 residual is still the freshness
banner (the two zero runs are timing noise in when it lands); the sub-0.001
residual elsewhere is the nav settling as fonts load, on both arms. The bar is
`position: fixed`, so it never appears as a shift source. The branch arm lists
one extra source, `P`: it is the loading card's `<p>` in the same font-swap
entry as the nav, with identical before/after rects — the control's bare text
node simply had no element to attribute to. Same value, one more name.

⚠️ CLS did not catch the margin-collapse defect this change shipped and then
fixed (the loading card's 2rem top margin collapsing through
`.dashboard-fallback`, growing the reservation 32px past its cap): the extra
height sat below the fold at every audit viewport, and an off-screen shift is
not counted. `capture-fallback.mjs`'s bounding boxes are what showed it —
measure geometry as well as CLS when the reservation changes.

## Measuring the fallback affordance

```sh
TARGET_URL=http://localhost:4173/ VIEWPORT=375x812 OUT=/tmp/fallback.png \
  node scripts/bench-dashboard-cls/capture-fallback.mjs
```

Holds the dashboard chunk back forever, screenshots the viewport and prints
the bounding boxes of `.dashboard-fallback__progress`, the fallback card and
the spinner, plus `barAboveFold` / `cardAboveFold`. It exits non-zero if the
chunk mounted anyway or an element is missing, so a printout full of `null`s
is a failure, not a pass. At 375x812 on `chore-web-audit-leftovers`: bar at
y 0–3 (in view), card at y 1064–1244 (below the fold, as before — the bar is
what the phone user sees without scrolling).

## Checking reduced motion

```sh
TARGET_URL=http://localhost:4175/ node scripts/bench-dashboard-cls/check-reduced-motion.mjs
```

Prints the computed `animation-name`, iteration count, opacity and transform
of each animated pseudo-element under both media states, and exits non-zero
if any animation that must stop under `reduce` still has a name, or if an
element is missing (run against a production build, the banner rows are
absent and the script fails — by design). The staging glow must read
`animation=none … opacity=0` under `reduce`; the progress segment must read
`animation=none … width=<viewport>` (static full-width bar).

## Notes

- Absolute CLS values depend on viewport and machine. Run both arms in the same
  session and compare them to each other; do not compare a number here against a
  Lighthouse run.
- `RUNS`, `CHUNK_DELAY_MS`, `BASELINE_URL`, `BRANCH_URL` and `TARGET_URL` are all
  environment-overridable.
