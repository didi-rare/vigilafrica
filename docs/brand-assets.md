# Brand Assets — Provenance & Regeneration

All identity assets derive from the **Epicenter Contours** mark (ADR-016).
Vector source of truth for the mark geometry: `web/src/components/BrandMark.tsx`
(app) and `web/public/favicon.svg` (favicon cut — dashed outer contour dropped
below 24px per the ADR).

| Asset | Source / regeneration |
| --- | --- |
| `web/public/favicon.svg` | Hand-authored vector (the favicon cut). Edit directly. |
| `web/public/favicon.ico` | 16/32/48 PNG renders of `favicon.svg`, wrapped as PNG-frame ICO (Vista+/all modern browsers). |
| `web/public/apple-touch-icon.png` | 180×180 canvas render: navy `#050714` ground + the full mark (with dashed contour). |
| `web/public/social-card.png` | 1200×630 canvas render: navy ground, 56px graticule grid + amber meridian, mono LIVE FEED readout, mark + Space Grotesk wordmark + Plex Sans tagline. |
| `docs/screenshots/readme-banner.png` | 1280×320 canvas render, same composition condensed. |

**Regeneration:** the raster assets are drawn with the Canvas API in a browser
running the app (so the real self-hosted fonts — Space Grotesk / IBM Plex — are
used), exported via `canvas.toDataURL('image/png')`. Colours are the token
values: amber `#f59e0b` (`--amber-500`), slate `#94a3b8` (`--slate-400`), navy
`#050714` (`--navy-950`). If the mark or palette changes, re-render at the same
dimensions and replace the binaries in place (filenames are load-bearing —
`social-card.png` is referenced by the OG/Twitter meta in `web/index.html`).
