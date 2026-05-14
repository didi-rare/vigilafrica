# Design: v1.1 Impact Category Expansion

## Understanding Summary

- Build v1.1 after v1.0 production launch is complete.
- Add NASA EONET `landslides` and `tempExtremes` as first-class supported
  categories.
- Preserve `floods` and `wildfires` behavior.
- Keep NASA EONET as the only upstream source.
- Avoid broad all-category support until the product proves which categories
  matter most.
- Capture `severeStorms` and `drought` as v1.2 follow-on scope, not v1.1 work.

## Assumptions

- Category expansion applies to all supported countries at the time of
  implementation.
- The 60-minute ingestion cycle remains the default.
- EONET may return sparse data for one or both new categories in some supported
  countries, so demo seed data must not depend on live upstream volume.
- No private user data, auth, subscription state, or alert-recipient data is
  introduced by this change.

## Recommended Approach

Create a small shared supported-category registry rather than adding more
scattered conditionals. The registry should define the v1.1 category IDs,
human-readable labels, and any presentation metadata needed by the frontend.

Backend behavior:

- EONET request construction includes
  `floods,wildfires,landslides,tempExtremes`.
- `models.EventCategory` includes explicit constants for all four categories.
- The normalizer maps EONET category IDs explicitly.
- Unsupported categories are skipped or rejected deliberately; they must not
  silently default to `floods`.
- API category validation accepts all four categories and rejects unsupported
  values with an error message that lists the valid set.
- Database migrations expand the `events.category` constraint to the v1.1 set.

Frontend behavior:

- `EventCategory` TypeScript types include all four category IDs.
- Category filter options render all supported categories.
- Event cards, detail pages, and map markers use category-specific labels,
  classes, glyphs, and accessible names.
- Unknown categories should have an explicit fallback only if the API contract
  allows them; v1.1 should prefer strict supported-category behavior.

Seed/demo behavior:

- Nigeria seed data includes at least one `landslides` and one `tempExtremes`
  record.
- Ghana seed data includes at least one `landslides` and one `tempExtremes`
  record.
- Seed records remain idempotent and geographically valid for enrichment.

## Migration Strategy

Add a forward migration that replaces the current category `CHECK` constraint
with the v1.1 supported set:

- `floods`
- `wildfires`
- `landslides`
- `tempExtremes`

The down migration must not silently corrupt data. It should either:

- fail clearly if v1.1 category rows still exist, with an operator cleanup note;
  or
- delete/transform v1.1 category rows only if a documented rollback decision
  explicitly accepts that data loss.

The preferred behavior is to fail clearly until an operator removes v1.1 rows.

## Testing Strategy

- Unit test EONET request construction includes all v1.1 categories.
- Unit test normalization for `landslides` and `tempExtremes`.
- Unit test unsupported EONET categories do not default to floods.
- API tests confirm all v1.1 category filters are accepted.
- API tests confirm unsupported category filters are rejected.
- Migration tests confirm the expanded category constraint accepts v1.1 rows.
- Frontend tests confirm all four filters render and generate correct API
  query params.
- Frontend tests confirm event cards, detail badges, and map markers render
  v1.1 categories distinctly.
- Seed/demo verification confirms Nigeria and Ghana contain representative
  v1.1 rows after local seed setup.
- `npm run spec:validate` passes.

## Decision Log

| Decision | Alternatives Considered | Rationale |
|---|---|---|
| Make this v1.1, after v1.0 | Fold into v1.0 | v1.0 remains a production quality gate, not new feature scope. |
| Add `landslides` and `tempExtremes` first | Add all EONET categories | Focused value with bounded implementation and testing. |
| Reserve `severeStorms` and `drought` for v1.2 | Include all four in v1.1 | `drought` likely needs different UX expectations; keep v1.1 sharp. |
| Use explicit supported-category mapping | Keep flood default fallback | Prevents silent misclassification as category coverage expands. |
| Expand DB constraint via migration | Remove DB category constraint | Keeps persisted event taxonomy controlled and auditable. |
