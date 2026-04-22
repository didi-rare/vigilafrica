# VigilAfrica Demo Environment 

This guide explains how to stand up a fully isolated, seeded demo environment of VigilAfrica. 
The demo environment is separate from the production infrastructure and disables live EONET polling to ensure a consistent, curated experience.

> **Hosted Demo**: TBD — see project README once deployed

---

## Prerequisites

- **Docker + Docker Compose**: To run the database and API.
- **Node.js (v18+)**: To run the React frontend.
- **Git**: To clone the repository.

```bash
# Clone the repository if you haven't already
git clone https://github.com/didi-rare/vigilafrica.git
cd vigilafrica
```

## Start the demo

1. Create your environment file:
```bash
cp .env.example .env
```
*(No need to modify `.env` – the demo override sets what it needs itself.)*

2. Start the demo backend services (Database + API + Seeder):
```bash
docker compose -f docker-compose.demo.yml up -d
```
> **Note**: On the first start, the `demo-api` will run migrations, and then the `demo-seeder` will automatically populate the database with events for Nigeria and Ghana.

3. Install frontend dependencies:
```bash
# From the root directory
npm install
```

4. Start the frontend pointing to the local demo API:
```bash
npm run web:dev
```

## Access the frontend

Open [http://localhost:5173](http://localhost:5173) in your browser.
You should immediately see the map dotted with both Flood and Wildfire events across Nigeria and Ghana.

## Stop the demo

When you're finished, bring down the services. The demo database volume will be preserved.
```bash
docker compose -f docker-compose.demo.yml down
```

## Reset demo data

If you want to clear the database and start fresh (for instance, to test idempotency or reset state):
```bash
docker compose -f docker-compose.demo.yml down -v
docker compose -f docker-compose.demo.yml up -d
```
