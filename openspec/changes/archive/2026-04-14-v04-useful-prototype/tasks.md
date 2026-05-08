# Tasks: v0.4 Useful Prototype

- [x] Add `maxmindinc/geoipupdate` container to `docker-compose.yml` with a shared volume and `.env` references.
- [x] Implement `api/internal/geoip/reader.go` wrapper to process `GeoLite2-City.mmdb` files.
- [x] Add `GET /v1/context` endpoint routing and logic in `api/internal/handlers/context.go`.
- [x] Add PostGIS `ST_DWithin` spatial query to fetch events within 200km radius in `api/internal/database/queries.go`.
- [x] Add `maplibre-gl` dependency to `web/package.json` (ADR-001).
- [x] Refactor `EventsDashboard` UI into a split-screen CSS Grid / Flex layout.
- [x] Build the MapLibre Base Component with Dark/Satellite style inside `web/src/components/Map.tsx`.
- [x] Build neon `pulse-fire` and `pulse-flood` marker animations.
- [x] Wire frontend React Query hook to `GET /v1/context` and pass nearby events down as props.
- [x] Implement `/events/:id` basic detail routing and layout.
- [x] Dockerize the Go API service and integrate into `docker-compose.yml`.
- [x] Implement `DEV_OVERRIDE_IP` logic for localized testing on localhost.
