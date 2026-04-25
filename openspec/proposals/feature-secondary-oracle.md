# Proposal: Secondary Data Oracle (feature-secondary-oracle)

**Status:** Deferred to v1.1+ (Post-Launch)

## Why
Currently, VigilAfrica relies entirely on NASA EONET. If EONET goes down, experiences an extended outage, or changes its API format, VigilAfrica will have no events to ingest. To ensure high availability of alerting data, a secondary oracle is needed.

## What Changes
We will integrate a secondary global disaster feed, such as the Global Disaster Alert and Coordination System (GDACS), to serve alongside EONET. The ingestor will merge and deduplicate events from both feeds before storing them in PostgreSQL.

## Out of Scope (for v1.0)
- **This entire feature is out of scope for the v1.0 public launch.** We will rely on EONET as the single source of truth for the MVP and accept the upstream risk.

## User Impact
When eventually implemented, users will experience fewer "data gaps" and potentially faster alerting if one upstream source is quicker to report an event than the other.
