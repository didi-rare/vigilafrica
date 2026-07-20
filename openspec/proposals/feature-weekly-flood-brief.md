---
id: feature-weekly-flood-brief
status: draft — decision note, not ready to implement
branch: tbd
---

# Note: Reframe the Flood Digest From Daily Events to a Weekly Risk Brief (feature-weekly-flood-brief)

> **This is a decision note, not an implementation-ready proposal.** It records a product question surfaced on 2026-07-20 so it is not lost, and states the recommendation and the evidence behind it. It needs the maintainer's call before it becomes a spec. It is a prerequisite for setting `DIGEST_TO`.

## The question

[feature-daily-flood-digest](openspec/proposals/feature-daily-flood-digest.md) specifies a **daily** email of the day's flood events, by admin name, as the NRCS pilot deliverable. Should it stay daily?

## Why it is now in doubt

The daily cadence was specified while we believed the flood feed was denser than it is. Measured on 2026-07-20:

- EONET carries **149 African flood events per 730 days** ≈ 75/year across the whole continent.
- Within the **Nigeria bbox specifically: 4 flood events in the last 365 days.**

A daily email against a feed that produces a Nigeria flood every ~90 days will read **"No flood events recorded today"** on roughly 99% of mornings.

That is a product failure, not a data failure. An operational contact who opens 90 consecutive empty emails learns the message carries no information and stops reading — so the digest is least likely to be read on the one morning it finally has something in it. For an emergency-response partner this trains exactly the wrong reflex, and it burns the NRCS relationship on a deliverable that technically works.

Note this is a *separate* problem from [fix-eonet-closed-events-ingest](openspec/proposals/fix-eonet-closed-events-ingest.md). That fix is necessary — without it the digest can never report a flood at all — but it is not sufficient. It changes "always empty" to "empty ~99% of days."

## What the sources actually support

Investigation on 2026-07-20 established that the three candidate sources are not competing feeds for the same job. They occupy **different time horizons**, which is why comparing them head-to-head kept producing false negatives:

| Source | Horizon | What it actually carries |
| --- | --- | --- |
| **ReliefWeb** (appname approved, tested, working) | weeks before, weeks after | OCHA *West and Central Africa Flooding Outlook*, published on a rolling weekly cadence and naming Nigeria; WFP sitreps; post-flood assessments. 21 Nigeria flood reports since 2026-06-01. Never the event moment. |
| **GDACS / EONET** | the ~48h event moment | Point coordinates, alert level, severity, affected-population estimate. Sparse but precise. |
| **EONET wildfires** | continuous | The one dense live dataset we already hold — 142/year in the Nigeria bbox, 27 open on 2026-07-20. |

A **weekly** brief can draw on all three horizons at once — forward-looking outlook, confirmed events from the week, and the admin-name translation layer that is VigilAfrica's actual differentiator ([product.md:15](openspec/specs/vigilafrica/product.md)). A daily email can only report the middle column, which is usually empty.

## Recommendation

Move the NRCS pilot deliverable to a **weekly Nigeria flood-risk brief**. It matches what the sources genuinely support instead of fighting them, and it is far likelier to be opened.

**Open questions for the maintainer — these are the blockers:**

1. **Does a weekly cadence still satisfy what NRCS was promised?** The sprint commitment was a daily digest. This may need renegotiating rather than silently substituting — and that conversation is itself a useful partnership touchpoint.
2. **Does ReliefWeb ingestion belong in the pilot scope, or should the first brief ship from EONET/GDACS data only?** ReliefWeb ingestion is a genuine architectural change (reports are not geolocated events — needs geocoding, provenance, and a confidence model).
3. **Is "flood" still the right frame?** Wildfires are our densest live dataset. A general weekly *hazard* brief may be more honest about what we can actually observe than a flood-specific one.

## Known limitation to state plainly in any pitch

Neither cadence fixes urban Lagos. GDACS caught the 2026-06-30 Lagos flood but **not** the 2026-07-13 event that brought the city to a standstill. If a partner's implicit test is "did it show me the flood that stopped the city," we still fail it.

Positioning that the data genuinely supports: **regional flood-season awareness across Nigeria and Ghana.** Positioning it does not support: **Lagos incident detection.** Closing that gap needs a Nigerian-domestic near-real-time source (NiMet / NEMA / NIHSA — machine-readable availability still unknown, and a good question to put to NRCS) or Nigerian news aggregation.

## Do not implement before

- [ ] Maintainer decides cadence (daily vs weekly vs hybrid alert-on-event)
- [ ] Q1–Q3 above answered
- [ ] [fix-eonet-closed-events-ingest](openspec/proposals/fix-eonet-closed-events-ingest.md) merged and confirmed storing floods in staging
- [ ] `DIGEST_TO` remains **unset** until the above are settled

## Origin

Surfaced 2026-07-20 during the ReliefWeb appname investigation, alongside the root-cause discovery in `fix-eonet-closed-events-ingest`.
