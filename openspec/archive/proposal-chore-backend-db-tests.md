# Proposal: Backend Database Integration Tests (chore-backend-db-tests)

## Why
Currently, the `api/internal/database` package has 0% test coverage. Because VigilAfrica relies heavily on PostgreSQL and PostGIS for critical spatial enrichment logic (e.g., determining which state a set of coordinates belongs to), using in-memory SQL mocks is insufficient and provides false confidence. We need a reliable way to test actual database interactions and spatial joins in our CI and local development workflows.

## What Changes
We will introduce integration tests for the database package using `testcontainers-go`. This library automatically spins up an ephemeral Docker container running `postgis/postgis:15-3.3` at the start of a test run, applies our SQL migrations, executes the tests against the real database engine, and tears the container down afterward.

## Out of Scope
- Frontend React component testing (to be handled in a separate spec).
- Testing of `api/internal/handlers` (HTTP API tests can be added later once the core DB layer is tested).
- SQL mocking strategies (like `sqlmock`).

## User Impact
No direct impact on end-users. For maintainers and contributors, this provides a massive boost in confidence that core features like ingestion, deduplication, and near-me context queries are functioning correctly, preventing regressions before public launch.
