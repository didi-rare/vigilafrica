# Tasks: Stop Ingesting Transposed EONET Polygon Geometry

**9 tasks.** Ordered so the cheap absolute gate lands before the resolver that needs network access,
and so nothing user-facing is corrected until the correction itself is verified against GDACS.

⚠️ **Verify every location claim against GDACS, never against our own database.** Our rows are the
thing under suspicion. `project_lagos_geocoding_spike_verdict` is the standing reminder that two
sources agreeing can still both be wrong; here GDACS is upstream of EONET, so it is the origin, not a
second opinion.

## 1. Stop the bleeding

- [ ] 1.1 **Reject impossible latitudes at normalization.** Any coordinate pair whose latitude falls
      outside ±90° is not a heuristic failure, it is definitionally invalid. Reject the event and
      count it. ⚠️ Apply to **every** vertex of a polygon, not just the first — the transposition is
      uniform, but a partial corruption would be worse and must not slip through.
      **Gate:** a table test built from the real EONET payloads for `EONET_23872` (Japan, "lat"
      136.77–137.76) and `EONET_20881` (Lagos, valid) — the first rejected, the second accepted.

- [ ] 1.2 **Quarantine unverifiable geometry instead of storing it as fact.** Polygons currently skip
      containment ([`eonet.go:417-422`](../../../api/internal/ingestor/eonet.go)) and are stored
      anyway. Add a persisted flag and exclude flagged rows from `/v1/events` and `/v1/context`.
      ⚠️ **Storing and hiding is deliberate** — dropping the data would destroy the evidence needed to
      fix it later, and we already have the `events_unverified_geom` precedent for keeping what we
      cannot verify. The change is that it must no longer *render as a confident location*.

- [ ] 1.3 **Promote `events_unverified_geom` from a log counter to something that can fail.** It
      incremented correctly for both bad events and nobody read it. ⚠️ **A counter nobody reads is not
      a control** — the whole reason this reached production. Either alert on a non-zero value or
      surface it in `/health` alongside `last_ingestion`.

## 2. Get the real geometry

- [ ] 2.1 **Resolve GDACS-sourced geometry from GDACS.** `source_url` already carries the event id:
      `https://www.gdacs.org/report.aspx?eventtype=FL&eventid=<id>`. The geometry endpoint is
      `https://www.gdacs.org/gdacsapi/api/polygons/getgeometry?eventtype=FL&eventid=<id>&episodeid=<n>`
      and returns `Point_Centroid`, `Poly_Affected` and `Poly_Global` features.
      ⚠️ **Episode selection is load-bearing and got this wrong once already.** For 1104105 the
      EONET polygon matches **episode 2**, not episode 1 — episode 1 is a different place entirely
      (lon 6.77–6.86, lat 6.05–6.14). Match on **vertex count** to identify the right episode rather
      than assuming `episodeid=1`.

- [ ] 2.2 **Decide and document the failure policy when GDACS is unreachable.** ⚠️ Do not silently
      fall back to the EONET geometry — that reinstates the defect under a network blip. The event
      stays quarantined (1.2) until geometry is resolved.

- [ ] 2.3 **Do NOT blanket-swap polygon coordinates.** Recorded as a rejected option so it is not
      re-proposed: it is correct today and silently corrupts every polygon the moment EONET fixes
      their feed, reintroducing invented locations with no signal.

## 3. Repair what is already stored

- [ ] 3.1 **Backfill the two affected production rows** once 2.1 lands: `EONET_22248` (must become
      Cameroon, and therefore leave the Nigeria/Ghana result set entirely) and `EONET_23208` (must
      move from Adamawa to its real northern-Nigeria location).
      ⚠️ Re-running enrichment is **not** sufficient — the trigger was never wrong. The stored
      `geom` itself must be replaced.

- [ ] 3.2 **Re-audit the whole table, not just floods.** 2 of 45 production events carry `Polygon`
      geometry and both are wrong, so the sample is 2/2. ⚠️ **Do not assume wildfires are safe
      because they are points** — verify the count of non-point geometry across the table rather
      than inferring it from the current 45-row snapshot.

## 4. Prove it

- [ ] 4.1 **A committed, runnable harness that re-derives the finding from live sources**, per the
      standing rule that a number driving a decision ships with the script that produced it. It must
      fetch both EONET and GDACS for a given event id and print the two coordinate ranges side by
      side, so the claim can be re-checked rather than trusted.
      **Acceptance:** running it against 1104078 reproduces the 1311-vertex exact match with reversed
      axes, and against 1103997 (Point) shows agreement.

- [ ] 4.2 **Verify on staging before production**, then confirm in production that `/v1/events?
      category=floods` no longer returns a Kwara flood and that the remaining floods' coordinates
      agree with GDACS.
