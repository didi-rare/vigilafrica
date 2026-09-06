# Tasks: Stop Ingesting Transposed EONET Polygon Geometry

⚠️ **This task list was REWRITTEN on 2026-09-06 after an independent adversarial
review (`gpt-5.6-sol`) returned BLOCK on the first implementation.** The review
found three P0s. Two of them were things the author had asserted the opposite of
in prose, in comments, and in a PR description. They are recorded in §6 rather
than quietly corrected, because the pattern — a confident claim no test asserted
— is the reusable lesson.

⚠️ **Verify every location claim against GDACS, never against our own database.**
Our rows are the thing under suspicion. GDACS is upstream of EONET, so it is the
origin rather than a second opinion.

## 1. Stop the bleeding

- [x] 1.1 **Reject impossible coordinates at normalization.** `validLonLat` +
      `polygonCoordinatesPlausible` in `normalizer.go`. A latitude outside ±90° is
      definitionally impossible, so the guard can never reject valid data. Every
      position is walked, nesting is depth-bounded, and MultiPolygon is handled
      explicitly rather than silently dropped.
      ⚠️ **It rejects structurally malformed trees too.** The first version
      returned *true* for empty coordinates and for a position like `["x", 200]`
      — the latter was traversed as if it were nested structure, so its values
      were never range-checked. RFC 7946 §3.1.1 defines a position as at least
      two numbers; anything else is not a position.
      ⚠️ **This guard cannot catch our own events.** Their reversed longitudes are
      under 90, so they stay numerically plausible. It caught 14 of 40 sampled
      events; ours were in the other 26. That gap is the whole reason for §2.

- [x] 1.2 **Fail closed by SKIPPING unresolved polygon geometry.** Superseded the
      original "quarantine" plan: nothing unverified is written at all, so there
      is nothing to quarantine and no flag for a query to forget to filter on.
      ⚠️ **The first implementation claimed to do this and did the opposite.** On
      a resolution failure `geoJSON` still held EONET's polygon and execution fell
      through to `UpsertEvent`. Because the upsert is keyed on `source_id`, **one
      GDACS network error would overwrite a corrected row with the transposed
      polygon again**, silently, while the run reported success.
      Pinned by `TestRunIngest_GDACSFailureNeverOverwritesWithTransposedGeometry`,
      which fails if the resolver call is removed from the ingest path.

- [x] 1.3 **Replace the counter that nobody read.** `EventsUnverifiedGeom` counted
      events *stored* without verification and incremented correctly for both bad
      events — and nobody reads run summaries. It is replaced by
      `EventsGeomUnresolved`, whose meaning is inverted: a rising value now means
      events are **missing**, not wrong. `EventsGeomResolved` sits beside it, so a
      run where resolution silently stops working is visible as one counter
      falling to zero while the other climbs.

## 2. Get the real geometry

- [x] 2.1 **Resolve the matching GDACS episode, and keep the polygon.**
      `ingestor/gdacs.go`.
      ⚠️ **The episode is load-bearing and the first implementation got it wrong.**
      It used the event-level centroid, which is whatever GDACS considers current.
      For 1104105 that is **episode 5 in the Niger Delta**, while EONET published
      **episode 2 in the north** — so the "fix" would have moved the flood ~700km
      rather than correcting it. Episodes are now matched on vertex count **and**
      extent correspondence; a vertex-count match alone is not proof, since two
      episodes can share one.
      ⚠️ **The polygon is preserved, not reduced to a centroid.**
      `GetNearbyEvents` runs `ST_DWithin` against `geom`, which for a polygon
      measures to the **nearest edge**. Collapsing a flood to its centre would
      drop it for users inside the flood but far from the middle — a silent
      reduction in who gets warned. A representative centroid is stored separately
      in `latitude`/`longitude`, which is what lets these events appear on the map
      **for the first time**: the frontend drops null-coordinate events, and
      polygons never had coordinates.

- [x] 2.2 **Fail closed when GDACS is unreachable.** No fallback to the EONET
      geometry, ever. Covered for 500s, non-JSON, missing episodes, empty bodies,
      cancelled contexts, an exhausted call budget and an expired time budget.

- [x] 2.3 **Do NOT blanket-swap polygon coordinates.** Not done, and recorded so it
      is not re-proposed. `extentsCorrespond` accepts **both** orientations, so the
      day upstream is repaired the direct match wins and this keeps working — the
      property a blanket swap would not have.

- [x] 2.4 **Bound the added network work.** A per-run budget of 40 calls **and a
      90-second wall-clock ceiling**, with per-episode cancellation checks and a
      cache keyed on source URL + vertex count.
      ⚠️ **The call cap alone was not enough, and the wall clock is the control
      that matters.** `schedulerLockTTL` is 5 minutes, sized against a documented
      ~3-minute worst case with an explicit note to revisit above ~4 minutes. 40
      calls at the client timeout is ~13 minutes — the lock would expire mid-run
      and a second replica could ingest concurrently.

- [x] 2.5 **Pin the request origin.** `CheckRedirect` refuses to follow redirects.
      ⚠️ Go follows cross-host redirects by default, so the claim that an upstream
      value "can influence which event is fetched, never which host" was **not
      true end to end** until this was added. The host allowlist validated the URL
      we were *given*; nothing validated where the request *ended up*.
      The event-type check is now a real allowlist — `^[A-Z]{2}$` accepted `ZZ`
      while the comment claimed it constrained hazard codes.

## 3. Repair what is already stored

- [x] 3.1 **Migration `000015`.** Restores the episode-matched GDACS polygon for
      both rows, generated from live GDACS rather than hand-typed.
      **Outcomes, measured against real PostGIS, not predicted:**
      - `EONET_22248` → **Cameroon**, `state_name` NULL. Its extent still
        intersects the Nigeria bbox because that box overhangs the border, so the
        row legitimately stays — correctly labelled via the ADM0 fallback.
      - `EONET_23208` → **Jigawa**, northern Nigeria.
      ⚠️ **That is the THIRD answer this event has had, and the first two were
      wrong.** Production served *Adamawa* (transposed). The first fix would have
      written *Delta* (event-level centroid, episode 5). *Jigawa* is what the
      episode EONET actually published resolves to. The lesson is that "correct
      the transposition" and "ask GDACS where the event is" are different
      questions with different answers.
      ⚠️ GDACS publishes 4 decimal places to EONET's 6, so restored rings are
      accurate to ~11m rather than ~1m. Immaterial at flood scale, recorded so
      nobody later reads it as corruption.

- [x] 3.2 **Re-audited the whole production table 2026-09-06 — the defect is
      confined to Polygon geometry.** Paginated all 45 rows rather than trusting a
      single page, then cross-checked every point against GDACS.

      | | |
      | --- | --- |
      | geometry types | **Polygon 2, Point 43** — no other non-point geometry |
      | rows with NULL lat/lng | 2 — exactly the two known polygons |
      | point events sourced from GDACS | **43 of 43**, so all were checkable |
      | points compared | 40 (3 transient fetch errors) |
      | stored vs GDACS distance | median **0.41 km**, max **8.2 km** |
      | distance if those points were transposed | min **71 km**, median **1058 km** |
      | points plausibly transposed | **0** |

      ⚠️ **Do not read the residual drift as a second defect.** 14 points differ
      from GDACS's centroid by more than 1 km. That is EONET's snapshot against
      GDACS's *current* centroid for an evolving event — the same episode drift
      that made the polygon fix hard, at harmless scale. The two populations do
      not overlap: real drift tops out at 8 km, transposition starts at 71 km.

      ⚠️ Points were verified because "they are points, so they are fine" is an
      assumption, not evidence — the round-1 review said as much. They are fine,
      and now that is measured.

## 4. Prove it

- [x] 4.1 **`verify-transposition.mjs` re-derives every number from live sources.**
      Census, plus a pairwise episode-matched comparison.
      ⚠️ **The first version overstated what it proved.** It read `geometry[0]`
      while production selects the most recent dated geometry; it scanned only
      episodes 1–4, omitting the episode 5 its own implementation relied on; it
      treated a vertex-count match as equality; it accepted two unrelated ids from
      the command line without checking they referred to the same event; and it
      reported "no EONET record found" as ordinary output with exit 0 — so *no
      comparison performed* was indistinguishable from *comparison passed*.
      All fixed: the GDACS id is bound through the event's own source metadata,
      all episodes are enumerated, extents must correspond, and incomplete proof
      exits non-zero.

- [ ] 4.2 **Deploy to staging, then production, and verify.** `/v1/events?
      category=floods` must show no Kwara flood; the two corrected events must
      carry Polygon geometry AND coordinates; `events_geom_resolved` should be
      non-zero on a run that ingests a polygon.

## 5. Security

- [x] 5.1 **The GDACS source URL is upstream data and is never fetched.** The host
      is validated against `gdacs.org`, only `eventtype` (allowlisted) and
      `eventid` (digits) are extracted, the request is rebuilt against a constant
      base URL, and redirects are refused. Pinned by tests including the
      `gdacs.org.evil.example.com` suffix-confusion case, a foreign URL that must
      never reach the network, and a cross-host redirect that must not be followed.

## 6. What the review caught, and why the tests did not

Recorded because the pattern is more reusable than the bugs.

All three P0s were in code with a **green suite**, and each contradicted a
confident claim in a comment, a commit message, or the PR description:

| claim | reality |
| --- | --- |
| "fails closed by design" | failed **open**; a GDACS error re-stored the transposed polygon |
| "corrects the transposition" | substituted a **different episode**, moving one flood ~700km |
| "never which host" | Go followed **cross-host redirects** by default |

⚠️ **The tests missed all three for one reason: they tested the helper, not the
caller.** `TestResolveGDACSCentroid/fails_closed` asserted a boolean return while
production stored the suspect polygon three lines later. Deleting the resolver
call from `eonet.go` entirely would have left every test passing.

The new tests assert **caller-level outcomes** — what reached `UpsertEvent`, and
what the migration did to a seeded transposed row — so removing the fix fails
them.
