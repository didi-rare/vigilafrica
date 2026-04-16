# VigilAfrica 🌍

**VigilAfrica** is an open-source effort to make natural event data more understandable and locally relevant across Africa.

The project translates raw geospatial event signals — floods, wildfires, storms, drought-related activity — into administrative areas people actually recognise: countries, states, provinces, and local government areas.

Instead of asking users to interpret coordinates and satellite metadata, VigilAfrica is being built to answer a simpler question:

**What is happening near me?**

---

## Why this project exists

Across Africa, environmental and natural event data is often available globally, but not always presented in ways that are easy for local communities, responders, researchers, journalists, or civic organisations to act on.

VigilAfrica bridges that gap by combining:

- global event feeds such as NASA EONET
- African administrative boundary context (starting with Nigeria)
- simple, location-aware user experiences

---

## Current status

> 🚧 **Early prototype — v0.1 in active development.** Not yet usable for real event monitoring.

This repository contains the foundation for a monorepo-based implementation with:

- a **Go backend** for ingestion and API services
- a **React + Vite + TypeScript frontend**
- **OpenSpec-based project governance** — locked specs with drift detection on every PR
- CI/CD workflow scaffolding for development, staging, and production branches

---

## Local development

### Prerequisites

- Go 1.26+
- Node.js 20+
- Docker + Docker Compose (for PostgreSQL with PostGIS — required from v0.2+)

### Setup

```bash
# Clone the repository
git clone https://github.com/didi-rare/vigilafrica.git
cd vigilafrica

# Copy environment variables
cp .env.example .env
# Edit .env with your local values (DATABASE_URL is not required for v0.1)

# Install frontend dependencies
npm install
cd web && npm install && cd ..

# Start the frontend dev server
npm run web:dev
# → http://localhost:5173

# Start the Go API server (separate terminal)
npm run api:dev
# → http://localhost:8080

# Check the API is running
curl http://localhost:8080/health
# → {"status":"ok","version":"0.1.0"}
```

### Run Go tests

```bash
cd api
go test ./...
```

---

## Architecture

VigilAfrica follows a **Poll → Enrich → Serve** pattern:

1. **Poll** — fetch raw events from NASA EONET (floods + wildfires, Nigeria)
2. **Enrich** — match coordinates to Nigerian state names using PostGIS
3. **Serve** — deliver localized event data via REST API → React frontend

```
NASA EONET → Go Backend → PostgreSQL + PostGIS → REST API → React (Vercel)
                                ↑
                    MaxMind GeoLite2 (local IP lookup)
```

**Full architecture**: See [`openspec/specs/vigilafrica/architecture.md`](openspec/specs/vigilafrica/architecture.md)

---

## Technology stack

| Layer      | Technology                        |
|------------|-----------------------------------|
| Backend    | Go 1.26                           |
| Frontend   | React 19 + Vite + TypeScript      |
| Database   | PostgreSQL 15 + PostGIS 3         |
| Maps       | MapLibre GL JS                    |
| IP Lookup  | MaxMind GeoLite2 (local)          |
| Frontend hosting | Vercel                      |
| Backend hosting  | VPS (Docker + Caddy)        |
| Governance | OpenSpec                          |

---

## Repository structure

```
/api              Go backend (API, ingestor, enrichment)
/web              React frontend (dashboard, map)
/openspec         Locked project specifications and ADRs
.github/          CI/CD and OpenSpec drift detection workflows
docker-compose.yml  Local dev: PostgreSQL + PostGIS
openspec.yaml     OpenSpec project configuration
```

---

## Roadmap

| Milestone | Theme                        | Status     |
|-----------|------------------------------|------------|
| v0.1      | Foundation (this milestone)  | 🔄 In progress |
| v0.2      | First real data flow         | Planned    |
| v0.3      | Localization engine          | Planned    |
| v0.4      | Map + near-me experience     | Planned    |
| v0.5      | Operational prototype        | Planned    |
| v0.6      | Country expansion model      | Planned    |
| v0.7      | Second country stable        | Planned    |
| v0.8      | Pre-demo setup               | Planned    |
| v1.0      | Credible public launch       | Planned    |

Full roadmap with acceptance criteria: [`openspec/specs/vigilafrica/roadmap.md`](openspec/specs/vigilafrica/roadmap.md)

---

## Branch strategy

| Branch        | Environment | Purpose              |
|---------------|-------------|----------------------|
| `development` | Local/dev   | Active development   |
| `main`        | Staging     | Pre-production       |
| `releases`    | Production  | Live                 |

OpenSpec drift detection runs on every push to `development` and every PR to all branches.

---

## Contributing

VigilAfrica is intended to be community-driven over time. As the prototype matures, contribution guides and issue templates will be expanded.

For now, contributions, feedback, and collaboration ideas are welcome through **GitHub Issues**.

---

## License

VigilAfrica is licensed under the **Apache License 2.0** — open for collaboration while remaining friendly to public-interest, research, and commercial reuse.

---

Maintained by **[@didi-rare](https://github.com/didi-rare)**. For collaboration or project discussions, open a GitHub Issue.
