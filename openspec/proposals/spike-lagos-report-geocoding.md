---
id: spike-lagos-report-geocoding
status: proposed
branch: tbd
---

# Spike: Can Lagos Flood Reports Be Reliably Located? (spike-lagos-report-geocoding)

> **Time-boxed spike — one week, no production code.** It answers one question and then stops. A spike that turns into an implementation has failed at its job.

## The question

**Can we reliably turn a Nigerian news report of Lagos flooding into a located, dated, confidence-scored event?**

Everything downstream of "go deep rather than broad" rests on this. If the answer is no, the depth strategy is dead and continental breadth becomes the honest default — which is a *good* week's outcome, not a wasted one.

## Why this is the right question

Four independent sources agree that Nigeria + Ghana generate almost no discrete, satellite-detectable hazard events (see [[project-flood-data-source-gap]]): EONET 43 events / 1 flood, GDACS **0**, ReliefWeb 3 declared disasters in 22 months. But Lagos flooding is **chronic**, not rare.

Concretely — **three distinct flood events inside four days**, all widely reported, of which **EONET carries exactly one**:

| date | areas reported | in EONET? |
|---|---|---|
| 2026-06-28 | Oshodi, Mushin, Surulere (Fashoro St), Egbeda (Akowonjo Rd), Gbagada Expressway, Idi-Oro | ❌ |
| 2026-06-30 | Lekki, Victoria Island, Ikoyi, Lagos Island, Ikota, Ajah, Surulere, Ikeja, Alimosho, Agege, Ikorodu | ✅ `EONET_20881` |
| 2026-07-01 | FESTAC, Gbagada, Evans, Olushi, Anikantamo, Adeniji Adele | ❌ |

That is the gap, evidenced rather than argued. Note also that Lagos recorded **586.4 mm to 3 July 2026 — 12.8% *below* the 2010–2025 average**: this is drainage failure, not exceptional rainfall, so it recurs predictably rather than being a freak event.

## Pre-registered pass criteria

**These are fixed before the test runs and must not be renegotiated afterwards.** This project has twice produced a result that looked positive only because the bar was set after seeing the data.

| metric | bar |
|---|---|
| **Retrieval** — events findable in ≥1 permitted source | **> 75%** |
| **Geocoding** — exact-or-close of retrieved events | **> 80%** |
| **Wrong** — confidently placed in the wrong area | **< 5%** |
| **Lag** — median publication delay after event | **< 24 h** |

Miss any badly → **depth is not viable; recommend breadth.**

⚠️ **`wrong` and `failed` are tracked separately and are not equivalent.** Failing to place an event is recoverable — you show it without a location. Confidently placing a flood in the wrong neighbourhood is a *harm* in a safety-adjacent product. This mirrors the enrichment finding where `unassigned`, not raw accuracy, was the deciding column.

## Method

### Day 1 — ground truth FIRST

Build the reference set of **12–15 Lagos flood events** (2025 + 2026 rainy seasons) with date, neighbourhood(s), and approximate coordinates. Seed from the table above and the 2025 events (Ijede/Ikorodu submersion; the Aug 2025 relocation warnings for Lekki/Ikorodu/Ajegunle).

**Ground truth is fixed and written down before any retrieval is attempted.** Build the pipeline first and you will grade it against whatever it happens to find — an unfalsifiable result.

### Days 2–3 — retrieval (the real go/no-go)

For each reference event, search the permitted sources. Record: **found y/n**, publication timestamp, and whether the text names a location precisely enough to place.

**This gate comes before any geocoding work.** If reports don't exist with usable place detail, geocoding accuracy is irrelevant — there is no input. Expect this to be the step that kills it, if anything does.

### Days 4–5 — geocoding

Extract place names from retrieved text and resolve to coordinates (LLM extraction + a Nigerian gazetteer). Score each against Day-1 truth:

- **Exact** — correct LGA/ward
- **Close** — right general area, wrong sub-unit
- **Wrong** — different part of the city
- **Failed** — no location extracted

### Day 6 — write-up

Retrieval rate, the four-way geocoding split, median lag. Then answer plainly: **would this have told a Lekki resident something useful, in time?**

## ⚠️ Source constraints — measured 2026-08-03

| source | homepage | RSS | AI-crawler policy | use? |
|---|---|---|---|---|
| Punch | 200 | ✅ 30 items | none stated | ✅ |
| Premium Times | 200 | ✅ 15 items | none stated | ✅ |
| Vanguard | 403 | ✅ 20 items | none stated | ✅ (RSS only) |
| Guardian NG | 200 | ❌ 403 | none stated | ⚠️ manual only |
| Channels TV | 200 | ❌ not RSS | none stated | ⚠️ manual only |
| **TheCable** | 200 | ✅ 10 items | **disallows `ClaudeBot`, `GPTBot`, `CCBot`, `Google-Extended`…** | ❌ **excluded** |
| **Daily Trust** | 200 | ✅ 14 items | **disallows `ClaudeBot`, `GPTBot`, `CCBot`, `Google-Extended`…** | ❌ **excluded** |

**TheCable and Daily Trust publicly opt out of AI crawling.** Their RSS is technically open, but running their articles through an LLM contradicts a clearly stated preference. **Excluded from any LLM-processing step** — a design constraint, not a footnote. Punch, Premium Times and Vanguard state no such restriction and carry the coverage.

⚠️ **RSS is live-only (10–30 recent items), not an archive.** So RSS is the right *product* mechanism going forward, but this *spike* needs historical retrieval via search. Do not conclude from a working RSS feed that back-catalogue access is solved.

## Out of Scope

- **Any ingestion code, scraper, schema change or UI.** Spreadsheet and manual search are the expected tools. Writing a scraper commits you to the answer you are meant to be testing.
- Sources outside Lagos, and hazards other than flooding.
- Social-media ingestion — different provenance and verification problem entirely.
- Deciding breadth-vs-depth. This spike *informs* that decision; it does not make it.

## Deliverable

A one-page result in this proposal's `## Findings` section: the four metrics against their bars, the four-way geocoding split, and a **viable / not viable** verdict with the evidence attached.

## Related, and still cheaper

**Five user conversations remain the cheapest unrun test** — *"where do you currently find out if your area is flooding?"* The 2026-05-27 business review flagged these in May and they are still outstanding ([[project-traction-data-gap]]). They cost less than this spike and could invalidate its premise. If retrieval looks plausible on Day 2, start them in parallel.

## Origin

Arising from the 2026-08-02/03 data-density investigation, which established that (a) event sparsity for Nigeria + Ghana is real across four independent sources, and (b) the events that matter to Lagos residents are chronic, recurring, and invisible to every global feed. Pass bars agreed with the maintainer before the spike was written; retrieval raised from 70% to **75%** at their request.
