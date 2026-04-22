# Spec: Backend Database Integration Tests (chore-backend-db-tests)

## Context
VigilAfrica uses PostgreSQL with the PostGIS extension for managing geospatial data and enriching events with administrative boundaries. Because these spatial queries (`ST_SetSRID`, `ST_MakePoint`, `ST_Contains`) execute inside the database engine, traditional SQL mocking libraries cannot accurately test our core logic. To ensure stability for v1.0, we must write real integration tests for the `database` package.

## Components to Touch
1.  `api/go.mod` (Add `testcontainers-go` dependency)
2.  `api/internal/database/testutil_test.go` (New test helper file)
3.  `api/internal/database/postgres_test.go` (New file for integration tests)

## Implementation Plan
1.  **Dependencies:** Run `go get github.com/testcontainers/testcontainers-go/modules/postgres` in the `api/` directory.
2.  **Test Helper (`testutil_test.go`):** 
    *   Create a function that uses testcontainers to start a `postgis/postgis:15-3.3` container.
    *   Map the container's randomized port to access the DB.
    *   Automatically apply the migrations from `api/db/migrations` to schema up the test DB.
    *   Return a `*database.PostgresRepo` instance and a teardown function.
3.  **Test Cases (`postgres_test.go`):**
    *   `TestUpsertEvent`: Verify inserting a new event and updating an existing one based on `SourceID`.
    *   `TestGetNearbyEvents`: Insert mock events with known coordinates, query a central point with a radius, and assert distance calculations.
    *   `TestCreateAndCompleteIngestionRun`: Verify the run lifecycle and status updates.

## Acceptance Criteria
- [ ] `api/go.mod` contains the `testcontainers-go` dependencies.
- [ ] `api/internal/database` test coverage reaches a minimum of 70%.
- [ ] Tests execute successfully in both local environments and CI via `go test`.
- [ ] Running tests leaves no orphaned Docker containers running.

## Verification Plan
1.  Run `go test -v -cover ./internal/database/` locally.
2.  Verify output reports successful test passes and lists coverage > 70%.
3.  Run `docker ps` after tests to ensure the ephemeral PostGIS container was cleanly terminated.
