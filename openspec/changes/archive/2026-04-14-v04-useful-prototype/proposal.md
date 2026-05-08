# Proposal: v0.4 Useful Prototype

## Background
VigilAfrica has successfully implemented the localization engine in v0.3, enabling the ingestion of EONET API data and translating coordinates to Nigerian state features using PostGIS. To make the interface a "Useful Prototype" (v0.4), we need to close the gap between data ingestion and user experience by providing localized visualization and an intelligent "Near-Me" context.

## Goals
1.  **Context API (`GET /v1/context`)**: Deliver IP-to-Location contextual intelligence. It should locate the user geographically using MaxMind GeoLite2 and immediately present the 5 most relevant nearby events (within ~200km radius).
2.  **Interactive Sentinel Map**: Add a unified MapLibre GL JS half-screen "Radar Dashboard" component. It will feature "Dark/Satellite" hybrid mapping and animate events as radar pulses (Orange for Fire, Shimmering Blue for Floods) to match the Industrial Sentinel design.
3.  **Detailed Analysis (F-015)**: Implement robust routing and visual layout for specific event detail pages (`/events/:id`).

## Proposed Solution
- **GeoIP Backend**: Deploy `maxmindinc/geoipupdate` as a separate sidecar container in `docker-compose.yml` to routinely fetch and synchronize the MaxMind database into a shared volume, keeping the main Go API process light and preventing stale mappings. `api/internal/geoip` will load it for X-Forwarded-For resolution.
- **Frontend Map View**: Update `EventsDashboard` into a split-screen design, embedding `MapLibre` mapping adjacent to the event timeline list. Hovering on list items will correlate to map tooltips and active state states.
- **Sentinel Governance**: Ensure we honor all the SDD checks encoded in `ADR-010`.

## Out of Scope
- GPS HTML5 GeoLocation API (browser prompts).
- Complete Africa coverage (Strictly Nigeria per ADR-004).
- Production-grade rate limiting and caching (Deferred to v0.5).
