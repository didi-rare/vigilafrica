# Tasks: Stop Ingesting Transposed EONET Polygon Geometry

⚠️ **THREE independent adversarial reviews (`gpt-5.6-sol`), all BLOCK.**
Round 1 found three P0s. Round 2, run against the rewrite, found three more —
including an **over-correction of a round-1 fix** and a defect the round-1 fix had
**moved rather than removed**. That is the whole argument for reviewing a fix as
a new claim rather than treating it as settled. §6 and §7 record both rounds. The review
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


## 7. Round 2 — what reviewing the fix caught

The rewrite was itself wrong in three ways. None would have been found by
re-reading it, because each was a *consequence* of a round-1 fix rather than an
oversight.

- [x] 7.1 **The skip over-corrected.** Round 1 stored bad geometry; the rewrite
      skipped the event entirely — and with it the title, status, category and
      dates, none of which depend on coordinates. ⚠️ **A flood that had since
      CLOSED would have stayed `open` in our data for as long as GDACS was
      unreachable.** On an early-warning product that is worse than a wrong
      location, and worse than round 1. Fixed with `UpdateEventMetadata`, which
      refreshes an existing row's non-geometry fields while leaving `geom`
      untouched; a genuinely new event with unverifiable geometry is still not
      invented. Pinned by
      `TestRunIngest_UnresolvedGeometryStillRefreshesMetadata`, both branches.

- [x] 7.2 **The episode match was still not proof.** Round 1's defect was matching
      on vertex count; the rewrite added a bounding-box check and called it
      settled. ⚠️ **An envelope is not a shape** — a concave outline, a ring with
      holes, a square symmetric under transposition, or the same ring reordered
      all satisfy it. Now every vertex is compared, and **exactly one** candidate
      episode must match: zero or several is an ambiguity we refuse rather than
      resolve by guessing. The tolerance also dropped from 1e-3 (~111m, roughly
      20x the rounding it was meant to absorb) to 2e-4, derived from GDACS's 4dp
      precision. Pinned by `TestPositionsCorrespond`, including the identical-
      envelope-different-shape case.

- [x] 7.3 **The budget fix moved the defect.** The 90-second ceiling was created
      inside `processEONETBody`, which runs **twice per country** (open and closed
      queries). The real ceiling was 180s per country and 360s for NG+GH —
      overrunning both `schedulerLockTTL` and the standalone ingestor's 2-minute
      deadline, i.e. the exact failure the budget existed to prevent. One budget
      per `Ingest` call now spans both responses. Pinned by
      `TestRunIngest_GDACSBudgetIsSharedAcrossBothResponses`.

- [x] 7.4 **The marker could sit outside its own polygon.** `positionsCentroid`
      averaged boundary vertices, so it was weighted by sampling density and could
      land outside a concave shape — a flood along a river bend is exactly that.
      Replaced with `representativePoint`, which mirrors `ST_PointOnSurface` and is
      inside by construction. ✅ Sanity check: for 1104078 it computes
      (9.413907, 4.602700), matching GDACS's own published centroid `[9.414,
      4.6027]` to 4dp. Pinned by `TestRepresentativePointIsInsideConcavePolygon`.

- [x] 7.5 **Four smaller corrections.** The GDACS source is now selected wherever
      it appears rather than assuming `Sources[0]` (40/40 GDACS-first today is an
      observation, not an API contract); `geom_type` is set to what is actually
      stored rather than what EONET declared; `bboxesIntersect` was renamed
      `envelopesOverlap` and documented as the deliberate permissive
      approximation it is, with the precise test left to PostGIS; and
      **MultiPolygon is now rejected explicitly** rather than half-supported —
      EONET has never been observed emitting one, and a claim of support that no
      test exercises is worse than an honest refusal.

## 8. Deliberately NOT done, and why

- [ ] 8.1 **Persisted degraded run status and alerting.** ⚠️ Real gap, honestly
      stated: when geometry resolution fails systematically the run still records
      `success`, `/health` still reports `ok`, and nothing pages. The counters and
      warnings exist only in logs, and this change has already demonstrated that
      **a counter nobody reads is not a control**. Deferred because it is a new
      subsystem — persisted per-run status, typed failure reasons, and alert
      routing — not because it is unimportant. Until it lands, a GDACS outage
      degrades coverage quietly.

- [ ] 8.2 **MultiPolygon end-to-end support.** Rejected rather than half-built.
      Supporting it properly means the resolver, the `geometry_type` enum in
      `openapi.yaml`, and both web consumers, for a geometry EONET has never sent.
      The normalizer now refuses it explicitly instead of silently dropping it, so
      if one ever arrives it will be visible rather than invisible.


## 9. Round 3 — and the first review finding that was itself wrong

Round 3 returned BLOCK with stopping signal "false". Six findings were real and
are fixed below. ⚠️ **One was not, and is recorded because a review is evidence,
not authority.**

- [x] 9.1 **`event_date` and `raw_payload` ARE derived from the geometry.** The
      round-2 metadata path refreshed them, justified in §7.1 by the claim that
      "dates do not depend on coordinates". **That claim was false**: `event_date`
      is parsed from the selected geometry snapshot's own `date` field
      (`normalizer.go`), and `raw_payload` contains that geometry verbatim.
      Writing either beside the OLD geometry asserts that an old polygon belongs
      to a new observation — and the digest selects by `event_date`, so a stale
      extent could surface as a current flood. Both are now excluded. Proven
      against real PostGIS by `TestUpdateEventMetadataPreservesGeometryAndDate`,
      which checks `ST_Equals` on the geometry and that the ORIGINAL date survives.

- [x] 9.2 **"Exactly one" counted feature records, not distinct geometries.**
      GDACS can serve the same ring under two episodes, which reported ambiguity
      where there was none and refused an event whose geometry was perfectly
      clear. Candidates are now deduplicated by geometry; genuine ambiguity — two
      DIFFERENT corresponding rings — is still refused, and the lowest episode id
      wins so the result does not depend on map iteration order.

- [x] 9.3 **The budget was per COUNTRY, not per run.** Scope has now been wrong
      three times: per response, then per `Ingest`, then per country —
      `runAllCountries` loops over `DefaultCountries`, doubling it again for
      NG+GH. One `RunBudget` is created in each run loop and threaded through
      every country.

- [x] 9.4 **The cap counted resolutions, not HTTP requests.** One resolution
      issues one event request plus up to `maxGDACSEpisodes` geometry requests, so
      a nominal 40 permitted ~840. The budget is now spent inside `gdacsGetJSON`,
      where requests are actually made, and renamed `maxGDACSRequestsPerRun`.

- [x] 9.5 **GDACS preference used `strings.Contains` on the whole URL.**
      Introduced in round 2. A permitted NASA/USGS link carrying `gdacs.org` in
      its path or query outranked a real GDACS source later in the list, and the
      resolver then rejected it on hostname — turning safe input into silent
      geometry loss. Now compares the parsed hostname with the same predicate as
      the resolver.

- [x] 9.6 **"MultiPolygon is explicitly rejected" was false.** Removing the branch
      left it falling through to the generic no-geometry path, making an arrival
      indistinguishable from an event that simply had none — the silent drop §7.5
      claimed to have fixed. It now returns an error naming the type.

- [x] 9.7 ❌ **REVIEW FINDING DISPROVEN — do not "fix" this.** Round 3 claimed
      `representativePoint` returns a marker inside a hole when rings are
      flattened, with a specific counterexample (outer `[-10,10]²`, hole
      `[-9,9]²` starting at `(-9,1)`). **Run against the real function it returns
      (9.5, 0.0) — in the annulus, on the surface.** Four further hole
      configurations also pass, because the connector edges add crossings in pairs
      and even-odd parity survives. Both real GDACS `Poly_Affected` polygons are
      single-ring (1311 and 28 vertices), so holes do not arise in ingested data
      either. Recorded by `TestRepresentativePointWithFlattenedHoles`.
      ⚠️ This does not prove the function correct for every ring — it proves the
      counterexample wrong. **A reviewer with a strong track record still produces
      findings that must be checked before they are acted on.**
