---
id: fix-staging-soft-launch
status: archived
branch: fix/staging-soft-launch
merged_pr: https://github.com/didi-rare/vigilafrica/pull/33
archived_on: 2026-04-26
---

# Proposal: Staging Soft-Launch Hardening (fix-staging-soft-launch)

## Why

During the v1.0 staging soft-launch (governed by `chore-vps-v1-launch`), two real-world gaps surfaced that block a clean handover but are **out of scope for the launch chore itself**:

1. The frontend's connection-error state on the dashboard is a dead end — when `fetch()` fails (DNS unresolved, API down, CORS, 5xx), users see a static "Failed to connect to VigilAfrica Command Center" message with no retry control and no diagnostic information. This was observed against `vigilafrica-staging.vercel.app` while `api.staging.vigilafrica.org` was still NXDOMAIN.
2. There is no runbook for inspecting VPS logs during/after a deploy. `docs/deployment/staging-production-topology.md` describes the *topology* but not the operator commands. Today the only way to diagnose a staging failure is to invent the SSH/docker-compose incantation from scratch.
3. The Namecheap DNS records required for `staging.vigilafrica.org` and `api.staging.vigilafrica.org` are referenced by `chore-vps-v1-launch` but never enumerated as a concrete record-by-record checklist — the second screenshot from the soft-launch (DNS_PROBE_FINISHED_NXDOMAIN on `staging.vigilafrica.org`) is a direct consequence.

## What Changes

This is a UX + ops-doc change. **No backend code, no schema, no API contract changes.**

1. **Dashboard error-state refresh** — `web/src/components/EventsDashboard.tsx`: replace the static error block with a component that
   - shows a "Retry" button that re-runs the React Query fetch
   - surfaces the resolved API base URL and the underlying error message (HTTP status if available; `TypeError`/network-failure otherwise)
   - keeps the existing copy and aria semantics
2. **Vitest coverage** — `web/src/components/EventsDashboard.test.tsx`: assert the retry button appears in the error state and triggers a refetch.
3. **VPS log runbook** — new section in `docs/deployment/staging-production-topology.md` (or a new `docs/deployment/operator-runbook.md` if the topology doc gets too long) covering: SSH access, `docker compose logs` per-service, `/health` probe via curl, container status (`ps`), and quick rollback pointer back to `release-process.md`.
4. **Namecheap DNS checklist** — addendum in the same runbook listing the exact records (host, type, value, TTL) for `staging.vigilafrica.org`, `api.staging.vigilafrica.org`, `vigilafrica.org`, `api.vigilafrica.org`, plus the verification command (`dig +short` / `nslookup`).

## In Scope — Operator Action

DNS record creation in Namecheap is now part of this change (taken over from `chore-vps-v1-launch`). The records are tracked as a checklist with sign-off in §8 of the spec. The two API `A` records (`api`, `api.staging`) were already created during Phase 2 of `chore-vps-v1-launch` and are noted as done; remaining records (frontend CNAME / apex) are owner-action under this change.

## Out of Scope

- Provisioning the VPS, configuring the Vercel projects — those remain `chore-vps-v1-launch` deliverables.
- Any backend `/health` enhancement or new diagnostic endpoint.
- Frontend redesign of the dashboard layout.

## Capabilities

### Modified Capabilities

- `web-frontend` (dashboard) — error state must offer a recovery action and disclose enough to diagnose connectivity failures.
- `deployment-ops` — runbook completeness: log inspection + DNS records become first-class operator references.

## Impact

- **Files modified**: `web/src/components/EventsDashboard.tsx`, `web/src/components/EventsDashboard.test.tsx`, `docs/deployment/staging-production-topology.md`
- **Files added**: possibly `docs/deployment/operator-runbook.md` (decided during apply phase based on size)
- **No API or schema changes**
- **No new dependencies**
