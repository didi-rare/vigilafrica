---
id: fix-staging-soft-launch
status: archived
branch: fix/staging-soft-launch
merged_pr: https://github.com/didi-rare/vigilafrica/pull/33
archived_on: 2026-04-26
---

# Spec: Staging Soft-Launch Hardening

## 1. Scope

Front-end UX fix for the dashboard connection-error state plus operator-facing documentation for VPS log inspection and Namecheap DNS configuration. No backend, no schema, no API contract changes.

## 2. Components Touched

| File | Change |
| --- | --- |
| `web/src/components/EventsDashboard.tsx` | Replace static error block (lines ~241-246) with a retry-capable component |
| `web/src/components/EventsDashboard.test.tsx` | Add tests for retry button + diagnostic detail rendering |
| `web/src/components/EventsDashboard.css` | Styles for new retry button and diagnostic-detail rows |
| `web/src/api/events.ts` | Export `getApiBaseUrl`; add `ApiError` class so HTTP status surfaces |
| `.env.example` | Document `VITE_SHOW_ERROR_DETAIL` flag |
| `docs/deployment/staging-production-topology.md` | Append "Operator Runbook" section (or split into new file if length warrants) |

## 3. Frontend Behaviour

### 3.1 Error-state component

When `eventsError` is truthy, the dashboard sidebar renders:

- The existing alert icon and headline ("Failed to connect to VigilAfrica Command Center").
- A secondary line showing **the API base URL that was attempted** (read from `import.meta.env.VITE_API_BASE_URL` with `window.location.origin` fallback — same logic as `getApiBaseUrl()` in [`web/src/api/events.ts:7`](web/src/api/events.ts:7)). **Gated behind `VITE_SHOW_ERROR_DETAIL === 'true'`** so production end-users see only the generic copy (per developers-react §10.4).
- A muted line showing the error message (`error instanceof Error ? error.message : String(error)`). If the underlying response had a status code, it is included. **Same gate as above.**
- A primary "Retry" button (`<button type="button">`) that calls the React Query `refetch()` for the events query. Disabled while `isFetching` is true and shows the existing spinner pattern next to its label.
- The container retains `role` semantics; the button has an accessible name "Retry connection".

### 3.2 React Query wiring

`useQuery` for events must expose `refetch` to the component. If the hook is currently destructured as `{ data, isLoading, isError, error }`, extend to `{ data, isLoading, isError, error, refetch, isFetching }`. No changes to query keys or staleness config.

### 3.3 No environment leakage

The displayed base URL is already public (it ships in the bundle). Do **not** display headers, tokens, or any value not already in `import.meta.env.VITE_*`.

## 4. Tests (Vitest + RTL)

`web/src/components/EventsDashboard.test.tsx` adds:

1. **"shows retry button on connection error"** — mock `fetchEvents` to reject; assert the button with name `/retry/i` is in the document.
2. **"clicking retry triggers refetch"** — same setup; click the button; assert `fetchEvents` is called a second time.
3. **"error state shows attempted URL and message"** — set `VITE_API_BASE_URL` via `vi.stubEnv`; assert both strings render.

Existing tests must continue to pass.

## 5. Documentation Deliverables

### 5.1 VPS log runbook section

New "Operator Runbook" section under `docs/deployment/staging-production-topology.md` (kept inline unless it exceeds ~150 lines, in which case extracted to `docs/deployment/operator-runbook.md` with a backlink). Must cover:

- **SSH entry**: `ssh $VPS_USER@$VPS_HOST` with a note that the user must be in the `production`/`staging` GitHub Environment reviewer list.
- **Tail logs (live)**: `docker compose -f /opt/vigilafrica/staging/docker-compose.yml logs -f --tail=200`.
- **Per-service logs**: `... logs api`, `... logs caddy`, `... logs db`.
- **Container status**: `docker compose -f ... ps`.
- **Health probe from VPS**: `curl -sS http://localhost:8080/health | jq`.
- **Health probe from outside**: `curl -sS https://api.staging.vigilafrica.org/health | jq`.
- **Caddy reload**: pointer to existing reload procedure (do not duplicate).
- **Rollback**: link to `docs/deployment/release-process.md` rollback section — do not re-document.

### 5.2 Namecheap DNS checklist

A table with the exact records:

| Host | Type | Value | TTL | Purpose |
| --- | --- | --- | --- | --- |
| `staging` | CNAME | `cname.vercel-dns.com` | Automatic | Frontend (Vercel staging project) |
| `api.staging` | A | `<VPS_IP>` | Automatic | Backend (VPS — staging compose stack) |
| `@` (apex) | ALIAS/A | `76.76.21.21` (Vercel) | Automatic | Frontend (Vercel production project) |
| `api` | A | `<VPS_IP>` | Automatic | Backend (VPS — production compose stack) |

Plus verification commands:

```bash
dig +short staging.vigilafrica.org
dig +short api.staging.vigilafrica.org
curl -sS https://api.staging.vigilafrica.org/health
```

The checklist explicitly notes that record creation is **not** part of this change — it is operator action tracked under `chore-vps-v1-launch`.

## 6. Acceptance Criteria

- [ ] Dashboard error state renders a working "Retry" button that re-issues the events fetch.
- [ ] Error state displays the attempted API base URL and the underlying error message.
- [ ] Three new Vitest cases pass; existing dashboard tests continue to pass.
- [ ] `npm run build` and `npm run lint` (within `web/`) pass with no new warnings.
- [ ] `docs/deployment/staging-production-topology.md` (or the new runbook) contains both the log-inspection commands and the DNS table verbatim above.
- [ ] No code changes outside the files listed in §2.
- [ ] All required Namecheap DNS records are created and verified (§8).
- [ ] `chore-vps-v1-launch` spec updated to delegate DNS record creation to this change.
- [ ] `openspec/changes/` and `openspec-verify` CI remain green.

## 7. Governance Notes

- This change takes over DNS-record creation from `chore-vps-v1-launch` — see §8. The chore-spec is updated in the same PR to remove that responsibility and link here.
- Branch: `fix/staging-soft-launch`. PR target: `development`.
- Follows the same merge cadence as recent fixes (small PR, squash merge, no archive entry until merged).

## 8. DNS Operator Checklist (Namecheap)

This is operator action — values to be entered in the Namecheap DNS dashboard. Mark each box once the record is live (`dig +short` returns the expected value) and propagation is confirmed.

### Already created (Phase 2 of chore-vps-v1-launch)

- [x] `api` — `A` — `178.104.104.122` — VPS production API
- [x] `api.staging` — `A` — `178.104.104.122` — VPS staging API

### To create under this change

- [x] `staging` — `CNAME` — `cname.vercel-dns.com` — Vercel staging frontend (unblocks `https://staging.vigilafrica.org`) — verified 2026-04-26: Vercel reports "Valid Configuration", DNS resolves to Vercel edge, HTTPS returns 200.
- [ ] `@` (apex) — `ALIAS` or `A` — `76.76.21.21` (or value Vercel surfaces in the production project's domain settings) — Vercel production frontend

### Verification (run after each record propagates)

```bash
dig +short staging.vigilafrica.org
# expected: cname.vercel-dns.com. + Vercel's resolved A record(s)

dig +short vigilafrica.org
# expected: 76.76.21.21 (or the value Vercel provided)

curl -sSI https://staging.vigilafrica.org | head -n 1
# expected: HTTP/2 200 (or 308/301 depending on Vercel redirect config)

curl -sSI https://vigilafrica.org | head -n 1
# expected: HTTP/2 200
```

### Sign-off

- [x] **Staging** — operator (DidiPepple) confirmed `https://staging.vigilafrica.org` loads and renders events from `https://api.staging.vigilafrica.org` end-to-end. Date: 2026-04-26.
- [ ] **Production** — apex record + sign-off pending Vercel production project setup (gated by `chore-vps-v1-launch` Phase 5).

