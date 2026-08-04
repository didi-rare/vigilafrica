# spike-lagos-geocoding — Days 4–5 harness

Reproduces the geocoding measurements in
[`openspec/proposals/spike-lagos-report-geocoding.md`](../../openspec/proposals/spike-lagos-report-geocoding.md).

Kept in the repo so the **not viable** verdict is reproducible rather than
self-certified. No dependencies beyond Node 18+ (`fetch` is built in).

| script | what it does |
|---|---|
| `geocode.mjs` | Resolves the 17 retrieved locality mentions through **two independent gazetteers** — Nominatim (OpenStreetMap) and Open-Meteo (GeoNames). Writes `geocode-results.json`. |
| `score.mjs` | Clopper–Pearson exact binomial CIs for the four-way split, plus the two sensitivity scenarios. |
| `mitigate.mjs` | Evaluates four post-processing rules against the bars, to test rather than assume whether a cheap filter rescues the result. |

```sh
node geocode.mjs    # ~35s — rate-limited to 1 req/s per Nominatim usage policy
node score.mjs
node mitigate.mjs
```

## Two things to know before re-running

- **`geocode.mjs` hits live third-party APIs.** Results can drift as OSM and
  GeoNames are edited. `geocode-results.json` is the **2026-08-04 snapshot** the
  proposal's numbers were computed from; keep it if you want to reproduce the
  published figures exactly.
- **`score.mjs` and `mitigate.mjs` embed the adjudicated classification**
  (exact / close / wrong / failed) rather than deriving it. That judgement is
  human and is argued in the proposal — including a sensitivity analysis showing
  the verdict survives the one call likely to be contested ("Lekki"). Changing a
  label means changing the argument, so it is written where it can be reviewed.

Nominatim is queried with a `User-Agent` per its usage policy. Both services are
free tiers; do not raise the request rate.
