# bench-events-ordering — is a composite index worth adding for the paginated ordering?

Answers task 1.6 of
[`feature-events-pagination`](../../openspec/changes/feature-events-pagination/tasks.md):
after `ListEvents` changed to

```sql
ORDER BY event_date DESC NULLS LAST, id DESC
```

does the existing `idx_events_event_date` suffice, or is a composite index
warranted? The task says **measure before adding** — the `area_m2` work shipped
an index `EXPLAIN` never read.

```sh
docker compose start postgres
docker exec -i vigilafrica-db psql -U vigilafrica -d vigilafrica -q -v ON_ERROR_STOP=1 \
  < scripts/bench-events-ordering/bench.sql
```

## Verdict: add it — but only in the exact `NULLS LAST` form

⚠️ **`idx_events_event_date` cannot serve this ordering, and it is not obvious
why.** It is declared `ON events(event_date DESC)`, and Postgres defaults `DESC`
to **NULLS FIRST**. The query asks for **NULLS LAST**, so the physical order does
not match — and the index has no `id` terminator either. The only candidate that
works is the exact form shipped in migration `000014`:

```sql
CREATE INDEX idx_events_event_date_id ON events (event_date DESC NULLS LAST, id DESC);
```

Median of 40 runs, 3,268 synthetic events (the continental-scale projection),
PostGIS 15-3.4. **Four independent runs** — three by the author, one by an
independent reviewer on different hardware. Absolute timings varied by ~50%
between machines, so **read the range and quote the pessimistic end**:

| query shape | without | with | speed-up | index in plan? |
|---|---|---|---|---|
| page 1, no filter | 1.41 – 2.15 ms | **0.06 – 0.15 ms** | ~15–25× | ✅ yes |
| page 1, `country=Nigeria` (the real access pattern) | 2.12 – 4.16 ms | **0.13 – 0.36 ms** | ~12–16× | ✅ yes |
| deep page, `offset 3200` | 2.20 – 3.96 ms | 2.21 – 3.41 ms | **none** | ❌ no |

The plan for page 1 changes from `Seq Scan` + top-N heapsort to a plain
`Index Scan`, and a second signal in the same output agrees: shared buffer hits
fall (**128 → 51** on one machine, **64 → 50** on another — the absolute figures
track table fill, the direction does not). Index size: 152 kB.

⚠️ **Deep pagination is not merely unhelped — on two of four runs it was
marginally SLOWER with the index present** (2.198 → 2.211 ms; 2.241 → 2.461 ms).
The planner correctly declines to use the index there and reverts to
`Seq Scan` + quicksort; the difference is within noise, but do not describe deep
pages as improved.

## What the numbers do *not* say

- **The absolute saving is ~1.3–3.8 ms per request.** That is a large ratio on a
  small number. This index is added because it is real, repeatedly measured and
  near-free — **not** because the endpoint was slow.
- ⚠️ **"The `COUNT(*)` is the dominant cost" is only true for FILTERED
  requests**, and an earlier version of this README asserted it as a blanket fact
  the harness had never measured. It now measures both count shapes, and they
  disagree with each other:

  | statement | median |
  |---|---:|
  | `COUNT(*)`, no filter | **0.17 ms** — cheap; the index scan dominates nothing |
  | `COUNT(*)`, `country=Nigeria` | **1.61 ms** — now ~12× the indexed list query (0.13 ms) |

  So after this index, a filtered request is dominated by its count, and an
  unfiltered one is not. Neither statement should be generalised to the other.
- **The fixture is synthetic.** 3,268 rows, `event_date` spread over 730 days
  (~4.5 rows per date), Nigeria ~40% of rows. It is representative of *volume and
  tie density*, not of real EONET clustering.

## Notes

- **Self-cleaning.** Everything runs in one transaction ending in `ROLLBACK` —
  synthetic rows (`source = 'bench-events-ordering'`), the candidate index and
  the two helper functions are all discarded, even if the script aborts midway.
  Safe to re-run.
- ⚠️ **Runs correctly whether or not migration `000014` is applied.** It drops
  the candidate index before measuring arm A (inside the transaction, so the real
  index comes back on `ROLLBACK`). Without that drop, once the migration exists
  arm A silently measured the *indexed* plan and reported it as the baseline, and
  the script then aborted with `relation "idx_events_event_date_id" already
  exists`. Both reproduced by independent review — the original script only
  worked on a database predating the migration, which is not where anyone would
  rerun it.
- `geom` is left NULL on the fixtures so `trg_enrich_event_location()`
  short-circuits; `country_name`/`state_name` are set directly. Seeding does not
  pay for spatial enrichment this script is not measuring.
- Index usage is read from `EXPLAIN (FORMAT JSON)`, **not** from
  `pg_stat_user_indexes`. Those counters flush at transaction end and the view is
  held stable within a transaction, so it would report 0 scans however the
  planner actually behaved.
- Timings are a **median of 40 runs after one untimed warm-up**; `min` is printed
  alongside so the spread is visible. A single reading at this scale is noise.
