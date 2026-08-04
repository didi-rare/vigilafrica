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

## Ground truth — PRE-REGISTERED 2026-08-04, frozen before any retrieval

Assembled by an independent research pass constrained to sources **outside the ingestion channel under test**. Punch, Premium Times and Vanguard were forbidden as evidence (they are what we are testing); TheCable and Daily Trust were excluded entirely on robots.txt AI-crawler grounds.

**This table is frozen. Do not add events after retrieval begins.**

| # | date | areas named | corroboration | tier |
|---|---|---|---|---|
| 1 | **~17–18 May 2025** | Lekki Phase 1 | Vanguard, Daily Trust only | ⚠️ **excluded from scoring** |
| 2 | **3–4 Aug 2025** | Ikorodu: Ijede, Oko Ope, Anjorin, Abule Eko, Odetedo; Agric | Sahara Reporters, Channels TV, The Eagle | domestic non-test-channel |
| 3 | **20 Sept 2025** | Lagos aggregate (no neighbourhood breakdown) | **ACT Alliance citing NEMA** — 57,951 affected, 3,680 displaced | official/humanitarian |
| 4 | **25–27 Sept 2025** | Lekki/Ikota — Kusenla Rd, drainage systems 156/157 | Nairametrics, Channels TV, Arise, The Nation | domestic non-test-channel |
| 5 | **28 Jun 2026** ⚓ | Murtala Muhammed Int'l Airport terminal | **Xinhua** (FAAN spokesman quoted) | international wire |
| 6 | **30 Jun 2026** ⚓ | Lekki, Victoria Island, Okota, Anthony Village, Oshodi-Apapa, Okokomaiko, Somolu, Ipaja, Ayobo | **AFP + Xinhua + Channels TV** | international wire ×2 |
| 7 | **14 Jul 2026** | Ikoyi, Lekki, Victoria Island, Oworonshoki | BusinessDay NG | domestic non-test-channel |

**Scoring set = rows 2–7 (six events).** Row 1 is retained for the record but **excluded from scoring**: it is documented only inside the test channel, so scoring against it would be circular. The maintainer confirms it corresponds to their recollection of "flooding around June 2025" — the month was misremembered; the event is real.

⚠️ **Coordinates in the source research are approximate and were not looked up with a mapping tool.** Adequate for coarse retrieval scoring; **not** survey-precision. Geocoding accuracy must be judged against named localities, not against those coordinates.

### What the sourcing itself revealed

- **Both June 2026 anchors are confirmed by international wire**, independently of any Nigerian daily — the strongest corroboration available.
- **Independent non-domestic sourcing exists ONLY for the June 2026 events.** Every 2025 event rests on domestic Nigerian outlets outside the exclusion list. Those are independent *of the ingestion channel* — which is what the test requires — but they share a media ecosystem, so they are a weaker tier and labelled as such.
- 🎯 **Evidence FOR the depth thesis:** street-level detail for 28 June (Oshodi, Mushin, Surulere/Fashoro St, Akowonjo Rd) appears **only** in Punch/Premium Times. The international wires covered the same date but reported only *airport, Lekki, VI*. **The ingestion channel carries finer location granularity than international sources** — precisely the capability the product depends on.

## Pre-registered pass criteria

**These are fixed before the test runs and must not be renegotiated afterwards.** This project has twice produced a result that looked positive only because the bar was set after seeing the data.

### ⚠️ The n=6 problem, and how this design handles it

Six scoring events makes a 75% bar brittle: 5/6 = 83% (pass), 4/6 = 67% (fail). **A single ambiguous case would flip the verdict** — unacceptable for a decision this size. Three changes fix it without inventing events.

**1. The primary metric is per-LOCALITY, not per-event.** Retrieval is not binary. Each event names multiple neighbourhoods, and each `(event, locality)` pair is an independent retrieval-and-geocoding test. The frozen set yields **22 locality mentions**:

| event | localities named | n |
|---|---|---|
| 3–4 Aug 2025 | Ijede, Oko Ope, Anjorin, Abule Eko, Odetedo, Agric | 6 |
| 20 Sept 2025 | *(aggregate — no localities)* | 0 |
| 25–27 Sept 2025 | Lekki/Ikota, Kusenla Rd | 2 |
| 28 Jun 2026 | Murtala Muhammed Int'l Airport | 1 |
| 30 Jun 2026 | Lekki, Victoria Island, Okota, Anthony Village, Oshodi-Apapa, Okokomaiko, Somolu, Ipaja, Ayobo | 9 |
| 14 Jul 2026 | Ikoyi, Lekki, Victoria Island, Oworonshoki | 4 |
| | **total** | **22** |

**n = 22 is a workable sample. n = 6 is not.** This costs nothing extra — the same retrieval run produces both — and it measures what the product actually needs: not "did we notice a flood happened somewhere in Lagos," but "did we correctly place it in *your* neighbourhood."

**2. The event-level decision is three-way, not binary.** Its brittleness only bites in the middle band, so the middle band is explicitly declared inconclusive rather than force-resolved:

| event-level retrieval | verdict |
|---|---|
| **≤ 3 / 6** | **Depth NOT viable. Stop.** No statistics needed — the channel doesn't carry the events. |
| **4–5 / 6** | **INCONCLUSIVE.** Expand the ground-truth set before deciding. Do **not** resolve by argument. |
| **6 / 6** | **Viable.** Proceed on the locality-level metrics. |

**3. Report an interval, not a point estimate.** Clopper-Pearson exact 95% CIs, computed 2026-08-04:

| result | rate | 95% CI | width |
|---|---|---|---|
| 4/6 events | 66.7% | 22.3 – 95.7% | 73 pp |
| 5/6 events | 83.3% | **35.9 – 99.6%** | 64 pp |
| 6/6 events | 100% | 54.1 – 100% | 46 pp |
| 17/22 localities | 77.3% | 54.6 – 92.2% | 38 pp |
| 20/22 localities | 90.9% | 70.8 – 98.9% | 28 pp |
| 22/22 localities | 100% | 84.6 – 100% | 15 pp |

Quoting "83%" from 5/6 as though it were a measurement would be false precision — the interval spans nearly the whole range.

### ⚠️ Read this before interpreting any result: the test is asymmetric

Even at n=22 the intervals are wide, and this must not be glossed. **To confirm a true rate above 75% at 95% confidence, the lower CI bound must clear 75% — which needs ≈22/22.** Even 20/22 (90.9%) has a lower bound of **70.8%**, below the bar.

**Therefore:**

- **A poor result is decisive.** 8/22 (36.4%) has an upper bound well below 75% → depth confidently rejected.
- **A good result is *permissive*, not confirmatory.** It means "not disproven — proceed, carefully." It does **not** establish that retrieval genuinely exceeds 75%.

**This spike is far better at killing the depth thesis than at proving it.** That is an acceptable and even desirable property for a one-week, decision-gating test — cheap tests should be good at saying no. But the write-up must not report a pass as though depth were validated. It is a green light to invest further, not evidence the pipeline works.

Anyone tempted to resolve a borderline result by argument should re-read this section.

### The bars

| metric | n | bar |
|---|---|---|
| **Locality retrieval** *(primary)* — named localities findable in ≥1 permitted source | 22 | **> 75%** |
| **Geocoding** — exact-or-close, of retrieved localities | ≤22 | **> 80%** |
| **Wrong** — confidently placed in the wrong area | ≤22 | **< 5%** |
| **Lag** — median publication delay after event | 6 | **< 24 h** |
| *Event retrieval (secondary, coarse)* | 6 | *see three-way rule above* |

Miss any badly → **depth is not viable; recommend breadth.**

**Stopping rule, pre-registered:** if the first three events processed yield **zero** retrieved localities, stop immediately and report not-viable. Do not process the remainder hoping for recovery.

⚠️ **`wrong` and `failed` are tracked separately and are not equivalent.** Failing to place an event is recoverable — you show it without a location. Confidently placing a flood in the wrong neighbourhood is a *harm* in a safety-adjacent product. This mirrors the enrichment finding where `unassigned`, not raw accuracy, was the deciding column.

## Method

### Day 1 — ✅ DONE 2026-08-04, ground truth frozen

The reference set is in **Ground truth — PRE-REGISTERED** above: 6 scoring events, 22 locality mentions, assembled from sources outside the ingestion channel. **It is frozen. Do not add events after retrieval begins** — grading against whatever the pipeline happens to find is an unfalsifiable result.

Day 1 also produced a finding worth carrying: independent corroboration for Lagos urban flooding is **thin before June 2026**. Only the two June 2026 events have international-wire sourcing; everything in 2025 rests on domestic outlets. That is adequate for this test (they are independent of the *ingestion channel*) but it caps how strong any conclusion can be.

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
