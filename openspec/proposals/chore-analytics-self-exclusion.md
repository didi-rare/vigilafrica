---
id: chore-analytics-self-exclusion
status: proposed
branch: chore/analytics-self-exclusion-and-ci-noise
---

# Proposal: Stop Recording Our Own Activity in Umami and CI (chore-analytics-self-exclusion)

## Why

The first read of the shipped Umami data (2026-07-26) produced a usable answer — **there is essentially no external traffic yet** — but it was harder to read than it should have been, because **most of the recorded sessions were ours**. Of the five visible sessions in the last 7 days, three were Lagos sessions timestamped within the same hour as the v1.3.5 release cut and a batch of Lighthouse runs, and one of those three was an Android/Mobile session consistent with the **mobile Lighthouse run itself** (Lighthouse emulates a Moto G Power and sends an Android user agent).

That matters more at this traffic level than it would later: when the weekly total is ~5 sessions, a handful of self-inflicted ones do not skew the number, they *are* the number. Every future read-out will hit the same problem, and the next one is expected to inform whether the held mobile-performance work is worth doing.

The same class of problem — **our own activity generating signal we then have to discount** — is also present in CI, so it is fixed here too rather than left as a separate stub.

## What Changes

### 1. Honour the exclusion flag for custom events (`web/src/analytics.ts`)

Umami's documented opt-out is `localStorage.setItem('umami.disabled', 1)`, set per browser **and** per device. It suppresses the tracker's *automatic pageviews*.

**It does not stop explicit `umami.track()` calls** ([umami-software/umami#3031](https://github.com/umami-software/umami/issues/3031)) — and `track()` is how this app sends all six of its custom events ([`analytics.ts`](../../web/src/analytics.ts)). Umami's own docs do not state this either way, which is exactly how it goes unnoticed: setting the flag looks like it worked, because pageviews stop, while events keep recording. The observed session bears this out — 8 views **and** 7 events from one visitor.

Add an `isExcluded()` guard to `track()` covering both noise sources:

- **`umami.disabled` present in `localStorage`** — keyed on *presence*, not truthiness, so a stray `"0"` cannot read as "tracking enabled". Reusing Umami's own key means one console command now excludes both halves consistently instead of half-working.
- **A synthetic user agent** (`Chrome-Lighthouse`, `HeadlessChrome`, `PageSpeed`) — Lighthouse uses a fresh browser profile per run, so a localStorage flag can *never* persist for it. The user agent is the only usable signal.

Evaluated per call, not memoised, so setting the flag takes effect without a reload. `localStorage` access is wrapped: private modes and sandboxed iframes throw, and an unreadable store must **not** be treated as excluded — that would silently drop real traffic, inverting the intent.

### 2. Stop the `welcome` CI job running on our own PRs (`.github/workflows/community.yml`)

Add `if: github.actor != github.repository_owner`.

`actions/first-interaction` only comments for genuine first-time contributors, so on the maintainer's PRs it built a Docker image (it is a **Docker** action, `FROM node:20.10-buster-slim` — Debian buster, EOL), greeted nobody, and cost ~2 minutes. It also fails outright whenever Docker Hub is slow: **1 of the last 12 runs** failed on `dial tcp … registry-1.docker.io: i/o timeout`, turning an unrelated docs-only PR's checks red. Gating on the actor removes 100% of that cost today while keeping the greeting for the external contributor it exists to serve.

### 3. Fix the `label-issues` token permissions (same file)

`label-issues` declared no `permissions:` block, and neither did the workflow; the repository default is `default_workflow_permissions: "read"`. **The labeler could not write labels.** Add `permissions: issues: write`.

This is **latent, not observed** — the job has never executed, because every recorded run of this workflow has been a `pull_request` event and the job is gated `if: github.event_name == 'issues'`. It would have failed on the first issue anyone ever opened.

## Out of Scope

- **Umami's `data-before-send` hook** (v2.18.0+), which *would* also suppress automatic pageviews and is the theoretically complete fix. Excluded deliberately: it requires a global function resolvable by the **deferred** tracker script, and the `script-src` directive in [`web/vercel.json`](../../web/vercel.json) has already silently broken analytics once — shipped broken in v1.3.0, fixed in v1.3.1. The remaining gap is that **auto-pageviews from audit runs still land**; they are infrequent and identifiable, and the events — the part carrying product meaning — are now clean. Revisit only after confirming the running Umami version is ≥ 2.18.0 (the compose file pins a `postgresql-latest` digest, so the version is not inferable from the repo).
- Replacing `actions/first-interaction` with a non-Docker action. With the actor gate in place the Docker path is only reached for real external contributors, where a flaky greeting is harmless.
- Server-side or IP-based exclusion in Umami. The maintainer's IP is dynamic; this was already considered and rejected upstream.
- Any change to `script-src`, or to the tracker `<script>` tag in `index.html`.

## Verification

- [ ] `npm run test` — new `web/src/analytics.test.ts` covers: forwarding still works; suppressed when `umami.disabled` is set; suppressed for a `"0"` flag value; suppressed for each of the three synthetic agents; **not** suppressed for an ordinary mobile UA; **not** suppressed when `localStorage` throws
- [ ] Existing `FeedbackPrompt.test.tsx` / `Map.test.tsx` analytics assertions still pass (jsdom's UA does not match the synthetic pattern, so behaviour is unchanged under test)
- [ ] `npm run type-check` / `npm run lint` / `npm run build` clean
- [ ] `actionlint` or equivalent accepts `community.yml`; the `welcome` job reports **skipped** on this very PR
- [ ] Post-deploy: with the flag set in the maintainer's browser, exercise a filter + a marker click on prod and confirm **no** new events appear in Umami
- [ ] Post-deploy: run Lighthouse against prod and confirm **no** new events appear (a pageview still will — that is the documented, accepted gap)

## Origin

Surfaced 2026-07-26 while recording the first Umami read-out. Two of the three items here are byproducts of that session rather than the audit itself: the `welcome` job's cost was noticed when it failed on PR #189, and the `label-issues` permissions gap was found while reading the workflow to fix it. Recorded together because they are one problem — measuring ourselves and then having to subtract it.
