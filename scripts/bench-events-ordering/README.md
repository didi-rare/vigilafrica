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
PostGIS 15-3.4. **Two independent runs, both reported** — read them as a range:

| query shape | without | with | index in plan? |
|---|---|---|---|
| page 1, no filter | 2.11 / 2.15 ms | **0.09 / 0.15 ms** | ✅ yes |
| page 1, `country=Nigeria` (the real access pattern) | 3.31 / 4.16 ms | **0.22 / 0.36 ms** | ✅ yes |
| deep page, `offset 3200` | 3.96 / 3.72 ms | 3.38 / 3.41 ms | ❌ **no** |

The plan for page 1 changes from `Seq Scan` + top-N heapsort to a plain
`Index Scan`, and a **second independent signal in the same output agrees**:
shared buffer hits fall from **128 to 51**. Index size: 152 kB.

## What the numbers do *not* say

- **The absolute saving is ~2–4 ms per request.** That is a large ratio on a
  small number. The handler's `SELECT COUNT(*)` seq-scans regardless and remains
  the dominant cost of the endpoint. This index is added because it is real,
  twice-measured and near-free — **not** because the endpoint was slow.
- **Deep pagination is not helped.** Past ~`offset 3200` the planner correctly
  reverts to `Seq Scan` + quicksort and never reads the index. That is an
  inherent offset-pagination cost, accepted deliberately in the proposal.
- **The fixture is synthetic.** 3,268 rows, `event_date` spread over 730 days
  (~4.5 rows per date), Nigeria ~40% of rows. It is representative of *volume and
  tie density*, not of real EONET clustering.

## Notes

- **Self-cleaning.** Everything runs in one transaction ending in `ROLLBACK` —
  synthetic rows (`source = 'bench-events-ordering'`), the candidate index and
  the two helper functions are all discarded, even if the script aborts midway.
  Safe to re-run.
- `geom` is left NULL on the fixtures so `trg_enrich_event_location()`
  short-circuits; `country_name`/`state_name` are set directly. Seeding does not
  pay for spatial enrichment this script is not measuring.
- Index usage is read from `EXPLAIN (FORMAT JSON)`, **not** from
  `pg_stat_user_indexes`. Those counters flush at transaction end and the view is
  held stable within a transaction, so it would report 0 scans however the
  planner actually behaved.
- Timings are a **median of 40 runs after one untimed warm-up**; `min` is printed
  alongside so the spread is visible. A single reading at this scale is noise.
