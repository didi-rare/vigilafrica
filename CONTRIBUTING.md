# Contributing to VigilAfrica

Thank you for your interest in contributing. VigilAfrica is an open-source natural event awareness platform built for African communities. This guide covers everything you need to go from zero to a working local environment and your first pull request.

---

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Prerequisites](#prerequisites)
- [Local Setup](#local-setup)
- [Project Structure](#project-structure)
- [Development Workflow](#development-workflow)
- [Running Tests](#running-tests)
- [OpenSpec Governance](#openspec-governance)
- [Pull Request Guidelines](#pull-request-guidelines)
- [Getting Help](#getting-help)

---

## Code of Conduct

All contributors are expected to follow the [Code of Conduct](CODE_OF_CONDUCT.md). Please read it before contributing.

---

## Prerequisites

| Tool | Version | Notes |
|---|---|---|
| Go | 1.26 | `go version` to verify |
| Node.js | 18+ | For the React frontend |
| Docker Desktop | Latest | Required for PostgreSQL + PostGIS |
| Git | Any recent | — |

Optional but recommended:
- `psql` CLI — useful for inspecting the database directly
- `golangci-lint` — for local lint checks before pushing

---

## Local Setup

### 1. Clone the repository

```bash
git clone https://github.com/didi-rare/vigilafrica.git
cd vigilafrica
```

### 2. Install root dependencies (OpenSpec CLI + frontend tooling)

```bash
npm install
```

### 3. Copy and configure environment variables

```bash
cp .env.example .env
```

Edit `.env` and set at minimum:

```env
DATABASE_URL=postgres://vigilafrica:vigilafrica@localhost:5432/vigilafrica
```

For email alerting (optional for local dev), also set:
```env
RESEND_API_KEY=re_...
ALERT_EMAIL_TO=your@email.com
```

### 4. Start PostgreSQL with PostGIS

```bash
docker-compose up -d
```

This starts PostgreSQL 15 with PostGIS 3 on port 5432. Credentials match the example `DATABASE_URL` above.

### 5. Start the API server

The server runs migrations automatically on startup.

```bash
npm run api:dev
# or directly:
cd api && go run ./cmd/server/
```

You should see JSON log output:
```json
{"time":"...","level":"INFO","msg":"VigilAfrica API starting","addr":":8080","version":"0.5.0"}
```

### 6. Verify the API is healthy

```bash
curl http://localhost:8080/health
```

Expected response:
```json
{"status":"ok","version":"0.5.0","last_ingestion":null}
```

### 7. (Optional) Seed sample data

If you want events without waiting for the scheduler to run:

```bash
psql $DATABASE_URL -f api/db/seeds/sample_events_nigeria.sql
```

### 8. Start the frontend

```bash
npm run web:dev
```

Open [http://localhost:5173](http://localhost:5173).

---

## Project Structure

```
vigilafrica/
├── api/                        # Go API server (ADR-007)
│   ├── cmd/
│   │   ├── server/             # Entry point: main.go
│   │   └── ingest/             # One-shot ingestion binary
│   ├── db/
│   │   ├── migrations/         # Numbered SQL migrations (ADR-009)
│   │   └── seeds/              # Sample data for local dev
│   └── internal/
│       ├── database/           # Repository interface + pgx queries
│       ├── handlers/           # HTTP handlers + middleware
│       ├── ingestor/           # EONET fetch, scheduler, alerter
│       ├── models/             # Shared data types
│       ├── normalizer/         # EONET → internal model
│       └── geoip/              # MaxMind reader wrapper
├── web/                        # React + Vite frontend
│   └── src/
│       ├── api/                # Typed API client functions
│       ├── components/         # Shared UI components
│       └── pages/              # Route-level page components
├── openspec/                   # Governance specs and ADRs
│   ├── specs/vigilafrica/      # Roadmap, decisions, product spec
│   └── changes/                # Per-PR change records (Sentinel gate)
└── docs/
    └── deployment/             # VPS and production deployment guides
```

---

## Development Workflow

### Branches

| Branch | Purpose |
|---|---|
| `main` | Production-ready, protected |
| `development` | Integration branch — merge feature branches here |
| `feat/*` | Feature branches — branch from `development` |
| `fix/*` | Bug fix branches |

### Typical flow

```bash
git checkout development
git pull
git checkout -b feat/my-feature
# ... make changes ...
git push -u origin feat/my-feature
# open PR targeting development
```

### Sentinel CI Gate

Any change to `api/internal/*` or `api/cmd/*` **must** be accompanied by a file in `openspec/changes/` or the commit message must include `[trivial]`. The CI will reject PRs without this.

Create a change record by copying the template:
```bash
cp openspec/changes/_template.md openspec/changes/feat-my-feature.md
# edit it to describe what you changed and why
```

---

## Running Tests

```bash
# Go: unit tests
cd api && go test ./...

# Go: build check
cd api && go build ./...

# Go: vet (catch common mistakes)
cd api && go vet ./...

# Frontend: type check
cd web && npm run type-check
```

All tests must pass before a PR can be merged.

---

## OpenSpec Governance

VigilAfrica uses [OpenSpec](https://github.com/fission-ai/openspec) for feature governance. The active roadmap and all architectural decisions live in `openspec/specs/vigilafrica/`. Do not add scope to a milestone or make technology decisions without updating the spec and (where required) adding an ADR.

Key documents:
- `openspec/specs/vigilafrica/roadmap.md` — milestone scope (locked)
- `openspec/specs/vigilafrica/decisions.md` — ADR log
- `openspec/specs/vigilafrica/product.md` — feature definitions

---

## Pull Request Guidelines

1. **Target `development`**, not `main`
2. **One feature per PR** — keep diffs reviewable
3. **Include a change record** in `openspec/changes/` for any `api/internal/*` or `api/cmd/*` changes
4. **Tests must pass** — `go test ./...` and `go vet ./...` clean
5. **No hardcoded credentials** — use environment variables only
6. **No ORM** — raw `pgx` queries in `internal/database/` only (ADR-009)
7. **All SQL in the repository layer** — never in handlers

PR description template:
```markdown
## What
Brief description of the change.

## Why
Which spec requirement or bug this addresses.

## Test plan
- [ ] go build ./... clean
- [ ] go test ./... passes
- [ ] Manual test steps...
```

---

## Getting Help

- **Questions about the codebase**: Open a [GitHub Discussion](https://github.com/didi-rare/vigilafrica/discussions) or [Issue](https://github.com/didi-rare/vigilafrica/issues)
- **Security vulnerabilities**: Do not open a public issue — email the maintainer directly via GitHub profile
- **Roadmap questions**: Check `openspec/specs/vigilafrica/roadmap.md` first
