---
id: feat-staging-banner-prominence
status: proposed
branch: feat/staging-banner-prominence
---

# Proposal: Make Staging Banner More Visible (feat-staging-banner-prominence)

## Why

After `fix-staging-vite-env-flag` landed, the `<StagingBanner>` component in [web/src/App.tsx:90-102](web/src/App.tsx#L90-L102) finally renders on `staging.vigilafrica.org`. But in browser screenshot review the user found it too subtle — a thin amber text strip that reads as decorative rather than a "you are NOT on production" warning. The whole purpose of the banner is to make the staging context impossible to miss; the current rendering doesn't pull its weight.

## What Changes

Two small visual enhancements to `.staging-banner` styling (no behaviour or scope change):

1. **Left-side coloured stripe** — a 4px solid amber vertical bar on the banner's leading edge, drawn via a `::before` pseudo-element so the existing flex layout isn't affected
2. **Subtle pulse animation** — a slow `box-shadow` glow on the stripe (2.5s ease-in-out, infinite). Visible enough to catch the eye on first load, low-amplitude enough to not become an irritant on prolonged viewing
3. **Inline icon** — add `AlertTriangle` from `lucide-react` (already a project dependency, used elsewhere in [App.tsx](web/src/App.tsx)) before the banner text so the warning is iconic + textual
4. **Respect `prefers-reduced-motion`** — disable the pulse for users who have requested reduced motion

## Out of Scope

- Changing banner copy or the underlying gate condition (`VITE_ENV === 'staging'`)
- Making the banner sticky on scroll — current static position is appropriate; a sticky banner becomes visual debt on long pages
- Replacing the amber palette with red/orange "alarm" colours — amber matches the existing token system and the staging colour story; the goal is "noticeable" not "alarming"
- Production banner — `vigilafrica.org` continues to render no banner
- A separate banner for `local` or `demo` environments — out of scope per the existing `VITE_ENV === 'staging'` gate

## Origin

Surfaced in browser review by the maintainer on 2026-05-23 immediately after `fix-staging-vite-env-flag` deployed to staging. Screenshot showed the banner rendering correctly but reading as a thin orange text strip rather than a prominent notice.
