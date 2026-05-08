# Spec: Secondary Data Oracle (feature-secondary-oracle)

**Status:** Deferred to v1.1+ (Post-Launch)

## Context
EONET is highly reliable but represents a single point of failure. Adding a secondary oracle will require standardizing a single internal `models.Event` representation that can be mapped from multiple diverse upstream APIs.

## Components to Touch
1. `api/internal/ingestor/gdacs.go` (New file)
2. `api/internal/normalizer/gdacs_normalizer.go` (New file)
3. `api/internal/ingestor/scheduler.go` (Update to poll multiple sources)

## Implementation Plan
*(To be detailed post-v1.0)*
1.  Evaluate GDACS API or similar global disaster feeds.
2.  Implement a new `fetch` and `normalize` pipeline similar to the existing EONET flow.
3.  Ensure the deduplication logic in `database.UpsertEvent` correctly handles overlapping events from different sources (e.g., using spatial proximity matching rather than strict `SourceID` matching, as different APIs will use different IDs for the same physical wildfire).

## Acceptance Criteria
- [ ] TBD.

## Verification Plan
- [ ] TBD.
