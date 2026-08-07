# bench-enrichment — reproducible benchmark for migration `000013`

Measures the enrichment tie-break optimisation in
[`perf-boundary-area-precompute`](../../openspec/changes/perf-boundary-area-precompute/proposal.md):
ordering candidate boundary polygons by a **stored** `area_m2` instead of
recomputing `ST_Area(geom::geography)` per candidate row on every insert.

Committed so the number is **auditable rather than asserted** — an independent
review of the original change noted the claimed speedup had no artifact anyone
could rerun, and its own measurement came in ~30% lower.

```sh
docker compose start postgres
docker exec -i vigilafrica-db psql -U vigilafrica -d vigilafrica -q -v ON_ERROR_STOP=1 \
  < scripts/bench-enrichment/bench.sql
```

Requires migrations applied through `000013`; the script refuses to run otherwise.

## It measures the same workload twice, on purpose

| method | what it does | use it for |
|---|---|---|
| **A** | `LATERAL` lookup against `admin_boundaries` | isolating the `ORDER BY` — **flattering** |
| **B** | real `INSERT`s through the real trigger | **the number to quote** |

Method A omits plpgsql overhead and the ADM0 fallback branch. Two independent
runs on different hardware both put B **below** A:

| run | A | B |
|---|---|---|
| author, 795 polygons / 2,385 points | 15.4× | **13.3×** |
| independent reviewer, 795 / 2,385 | 15.2× | **11.5×** |

**Report the range (~11–13×), not a single figure.** The original record claimed
16.2× — a method-A number quoted as though it were method B.

## What it also checks

Speed is worthless if the labels change, so every run asserts:

- every probe point receives an **identical** `(adm_name, country_name)` under both variants — the run prints `differing`, which must be **0**
- `area_m2` equals `ST_Area(geom::geography)` **exactly** for every row, max abs diff **0**

## Notes

- **Self-cleaning.** Synthetic boundaries use `country_code = 'ZZ'` and synthetic
  events use `source = 'bench-enrichment'`; both are removed at the end and the
  production trigger is restored. Safe to re-run.
- **`table_size` inflates across repeated runs** from dead tuples left by the
  delete/reinsert cycle. `VACUUM FULL admin_boundaries` for a true figure — do
  not quote the printed size after several runs.
- The fixture replicates and translates the real 53 ADM1 polygons to ~795,
  approximating Africa-wide ADM1 count. It is representative of *volume*, not of
  real continental geometry complexity.
