# API Docs Exposure Policy

VigilAfrica's public API documentation is acceptable for staging and public demo environments while the API only exposes unauthenticated public natural-event data.

Before any private, admin, operator, or partner endpoints are added:

- set `API_DOCS_ENABLED=false` in production-like runtime environments that should not expose `/docs` or `/openapi.yaml`;
- keep `/docs` and `/openapi.yaml` available only in local development, staging, or an authenticated/private operator surface;
- update this policy and the release checklist with the chosen production behavior.

The API handlers fail closed for docs exposure when `API_DOCS_ENABLED` is `false`, `0`, or `off`.
