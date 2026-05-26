# Tasks: v1.3 Impact Category Expansion

## 1. Category Registry

- [ ] 1.1 Add backend category constants for `landslides` and `tempExtremes`
- [ ] 1.2 Add a supported-category helper used by ingestion, normalization, and handlers
- [ ] 1.3 Replace flood-default category normalization with explicit category mapping
- [ ] 1.4 Add unsupported-category handling that is deliberate and test-covered

## 2. Database Migration

- [ ] 2.1 Add migration expanding `events.category` constraint to `floods`, `wildfires`, `landslides`, `tempExtremes`
- [ ] 2.2 Add down migration that refuses narrowing while v1.3 category rows exist, or documents accepted cleanup
- [ ] 2.3 Verify existing flood/wildfire data survives the migration

## 3. EONET Ingestion

- [ ] 3.1 Update EONET request category parameter to include all v1.3 categories
- [ ] 3.2 Test request construction for the exact v1.3 category set
- [ ] 3.3 Test normalizer output for `landslides`
- [ ] 3.4 Test normalizer output for `tempExtremes`
- [ ] 3.5 Test unsupported EONET categories do not silently become floods

## 4. API Contract

- [ ] 4.1 Update `GET /v1/events?category=` validation to accept all v1.3 categories
- [ ] 4.2 Update API error messages to list all valid category values
- [ ] 4.3 Update `api-contract.md` request/response examples and allowed values
- [ ] 4.4 Add API tests for `landslides` and `tempExtremes` filters

## 5. Frontend

- [ ] 5.1 Extend frontend `EventCategory` union to include `landslides` and `tempExtremes`
- [ ] 5.2 Add filter dropdown options for all v1.3 categories
- [ ] 5.3 Add category-specific event-card labels and badge styles
- [ ] 5.4 Add category-specific detail-page labels and badge styles
- [ ] 5.5 Add category-specific map marker variants/glyphs/accessibility names
- [ ] 5.6 Test that new categories are not rendered as wildfire by fallback logic

## 6. Seed and Demo Data

- [ ] 6.1 Add at least one Nigeria landslide seed event
- [ ] 6.2 Add at least one Nigeria temperature-extreme seed event
- [ ] 6.3 Add at least one Ghana landslide seed event
- [ ] 6.4 Add at least one Ghana temperature-extreme seed event
- [ ] 6.5 Verify seeded v1.3 events enrich to country/ADM1 boundaries

## 7. Documentation and Verification

- [ ] 7.1 Update roadmap v1.3 section when implementation is complete
- [ ] 7.2 Update `spec.md` and `architecture.md` category references
- [ ] 7.3 Run backend tests covering category validation, normalization, ingestion, and migrations
- [ ] 7.4 Run frontend tests covering filters, cards, detail page, and map markers
- [ ] 7.5 Run `npm run spec:validate`
