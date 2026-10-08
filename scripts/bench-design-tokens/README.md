# bench-design-tokens — screenshot diff for pure-CSS refactors

The verification harness behind
[`chore-design-tokens`](../../openspec/changes/chore-design-tokens/tasks.md).
A token migration is only a refactor if the rendered pixels do not change, and
green CI cannot tell you that: the project already shipped a UX regression with
100%-green verification once (#193), and only a screenshot caught it. So the
proof ships as a runnable script, the same way `bench-dashboard-cls/` does.

`screenshot-diff.mjs` renders the three page types — home with the lazy
dashboard mounted, `/for-partners`, and `/events/:id` — at 375 / 768 / 1280 px
against **two served builds**, the branch and a control built from the base
branch, and counts differing pixels with `pixelmatch`. It prints both the exact
count (threshold 0) and the perceptual count (pixelmatch's default 0.1), writes
every screenshot to `out/`, and writes a `*-diff.png` for anything that differs.
It exits 1 on any perceptual difference so it can gate a PR.

Determinism: every API response is route-mocked (`/health`, `/v1/context`,
`/v1/states`, `/v1/events`, `/v1/events/:id`), every non-localhost request (map
tiles, analytics) is aborted, `Date` is pinned, motion is reduced and CSS
animations are frozen by Playwright at capture time.

## Prerequisites

Playwright, `pixelmatch` and `pngjs` are deliberately **not** committed
dependencies — this is an occasional diagnostic and it would add a browser
download to every `npm ci`. Install them transiently **at the repository root**
(Node resolves the imports from there) and run from the root:

```sh
npm i --no-save --no-package-lock playwright pixelmatch pngjs
npx playwright install chromium
```

## Running it

Two builds served at once — the control arm is not optional — and **both built
with the same `VITE_API_BASE_URL`**. `/for-partners` prints the API origin, which
falls back to the page's own origin when the variable is unset, so two builds on
different ports differ by exactly the port digits. That was the first thing this
harness found, and it was the harness, not the CSS.

```sh
# control: the base branch, built into a sibling directory
git worktree add --detach ../baseline origin/development
(cd ../baseline && npm ci && cd web && npm ci && cd .. \
  && VITE_API_BASE_URL=https://api.vigilafrica.org npm run web:build)
(cd web && npx vite preview --outDir ../../baseline/web/dist --port 4174)

# branch
VITE_API_BASE_URL=https://api.vigilafrica.org npm run web:build
(cd web && npx vite preview --port 4173)

node scripts/bench-design-tokens/screenshot-diff.mjs
```

`BASELINE_URL`, `BRANCH_URL` and `OUT_DIR` are environment-overridable. `out/`
is gitignored; attach the table (and any diff PNGs) to the PR instead.

### WebGL

MapLibre needs WebGL2. Playwright's default headless shell cannot create a
WebGL2 context even with software GL, so the map throws, the app's error
boundary replaces the dashboard with an error state, and the home page never
renders an event card. Set `PW_CHANNEL=chrome` (installed Google Chrome) or
`PW_CHANNEL=chromium` (Playwright's full build) to run in a browser that can.
On this project's Windows host the full Playwright build is blocked by the same
Application Control policy that blocks Go test binaries, so `chrome` is the one
that works there.

### Noise floor

Run the baseline against itself first (`BRANCH_URL` = `BASELINE_URL`,
`OUT_DIR=…/out-noise`). The map canvas is rasterised by software GL and is not
bit-identical between two captures, so `exact` is non-zero wherever the map is
visible while `perceptual` stays 0. Compare the branch run against that floor:
the claim is "no difference beyond the self-comparison", never "exact = 0".

## Reading the result

- **exact = 0 and perceptual = 0** on every row: the refactor changed nothing
  the browser paints. This is the bar for a token migration.
- **exact > 0, perceptual = 0**: sub-threshold anti-aliasing noise. Investigate
  which rows, but it is not a layout change.
- **perceptual > 0**: open the `*-diff.png`; red pixels are where the two arms
  disagree. A one-pixel band across the page is a spacing change; glyph-shaped
  noise is a type change.
- A size mismatch means the page height changed — the clearest possible
  evidence that spacing moved.
