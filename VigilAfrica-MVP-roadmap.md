# VigilAfrica MVP Roadmap

## Product principle

VigilAfrica should prove its value with the smallest believable product first.

The project’s edge is not simply showing natural events on a map. Its real advantage is making geospatial event data locally meaningful for African users by translating raw signals into familiar administrative areas and usable context.

---

## Recommended MVP scope

**Country:** Nigeria  
**Event types:** Floods and wildfires  
**Regions:** Country + state first, LGA second  
**Primary users:** NGOs, local media, civic responders, logistics planners

Why this scope works:

- Nigeria gives the project a large and relevant first use case
- floods are socially important and easy to understand
- state-level enrichment is enough to prove value before going deeper
- this is small enough to build, test, and demo credibly

---

## v0.1 — Something real exists

### Goal
Turn the repository from concept-first to proof-first.

### Build
- a real landing page instead of starter frontend content
- a minimal Go API with one health endpoint and one mock events endpoint
- one documented event JSON shape
- one architecture diagram in the README
- one screenshot in the repo

### Suggested deliverables
- `GET /health`
- `GET /events`
- hardcoded or sample event payloads for one country
- frontend page showing a list of nearby or sample events
- README updated to clearly label the project as prototype

### Success criteria
- a new visitor can understand the project quickly
- the app no longer looks like a scaffold
- contributors can run both web and API locally

---

## v0.2 — First end-to-end data flow

### Goal
Prove real ingestion.

### Build
- EONET fetcher or ingestion command
- normalization into a simple internal event model
- storage of normalized events in PostgreSQL
- API endpoint serving ingested events
- a basic frontend list or map view from live stored data

### Suggested internal event model
- event id
- source
- title
- category
- geometry type
- latitude/longitude or geometry reference
- source timestamp
- country
- status
- raw payload reference

### Success criteria
- a real upstream event enters the system
- it gets normalized and stored
- the frontend displays it

---

## v0.3 — Localization engine

### Goal
Deliver the project’s actual unique value.

### Build
- PostGIS-backed enrichment
- administrative boundary matching
- event-to-country and event-to-state/province mapping
- support for one country deeply rather than many shallowly

### Recommended focus
- Nigeria first
- flood + wildfire first
- state first, then LGA

### Success criteria
- one event can be shown as “near X state” instead of only coordinates
- the UI can filter by country/state
- the README can show before/after localization examples

---

## v0.4 — Useful prototype for real users

### Goal
Make the product usable by a small external audience.

### Build
- homepage with event summary cards
- filters by event type and region
- “near you” country/state fallback
- simple map or geographic visualization
- event detail view
- error states and empty states

### Optional additions
- basic IP-based country lookup using MaxMind
- manual region picker as the default if IP lookup is noisy

### Success criteria
- a non-technical user can understand what is happening nearby
- one NGO, journalist, or civic-tech reviewer could reasonably test it

---

## v0.5 — Operational prototype

### Goal
Make it maintainable.

### Build
- scheduled ingestion job
- deduplication logic
- basic observability
- simple admin metrics
- rate limiting and caching
- deployment hardening

### Must-fix technical hygiene
- align Go version between `go.mod` and CI
- make sure `api/main.go` exists or change the scripts/workflow
- add contributor setup instructions
- add seed/sample dataset

### Success criteria
- the prototype can run repeatedly without manual cleanup
- contributors can understand how to set up and test the system

---

## v0.6 — Country expansion model

### Goal
Make expansion systematic.

### Build
- a country onboarding template
- boundary dataset standards
- naming conventions
- source quality validation rules
- fallback logic for incomplete geometry/admin mapping

### Delivery model
Instead of trying to operationalize all African countries at once, define:
- Tier 1 countries
- Tier 2 countries
- Tier 3 backlog

### Success criteria
- adding a new country becomes a repeatable process
- data quality and enrichment rules are documented

---

## v1.0 — Credible public launch

### Goal
Launch a version that is genuinely useful and defensible.

### A credible v1.0 could include
- 1–3 countries supported well
- 2–4 event categories supported well
- localized event enrichment working consistently
- stable API and basic frontend experience
- documented data pipeline
- screenshots, demo environment, and contributor guide
- public roadmap and governance docs

### Suggested launch message
**Localized natural event awareness for selected African countries, starting with high-priority regions and event types.**

---

## Immediate repo fixes

These are the highest-leverage changes before adding more features:

1. Replace the starter frontend with a real VigilAfrica landing page
2. Make the backend entry point real, or stop referencing `api/main.go` until it exists
3. Align the Go version between `go.mod` and CI
4. Update the README structure section so it matches the actual repo contents
5. Add proof assets such as a screenshot, API sample, and architecture diagram

---

## Final guidance

VigilAfrica should not try to win by claiming full African coverage first.

It should win by proving this:

**We make natural event data locally meaningful in a way existing global feeds do not.**

Once that is working in one country with one or two event classes, the broader expansion story becomes much more credible to contributors, partners, and funders.
