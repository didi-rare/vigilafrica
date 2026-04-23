# Proposal: Developer-Driven API Load Testing (chore-k6-performance-tests)

## Why
As VigilAfrica approaches a public launch (v1.0), we need confidence that our Go REST API and PostgreSQL database can handle realistic public traffic. Without load testing, we risk the API collapsing under moderate concurrent usage, especially when executing spatial boundary checks.

## What Changes
We will introduce a local, developer-driven performance testing suite using [k6](https://k6.io/). A dedicated load-testing script will simulate concurrent virtual users (VUs) querying the `/v1/events` API with realistic filters, establishing a baseline performance metric (e.g., 95th percentile response time under 200ms).

## Out of Scope
- Automated execution in CI/CD (GitHub Actions) due to resource variability and flakiness.
- Load testing the upstream NASA EONET API.
- Load testing the ingestion background jobs.

## User Impact
No direct impact on end-users. For maintainers, it provides an essential tool to verify that new database indexes or API changes do not degrade read performance before they are merged.
