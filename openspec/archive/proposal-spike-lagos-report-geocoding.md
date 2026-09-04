---
id: spike-lagos-report-geocoding
status: complete
branch: docs/spike-days-2-3-findings
---

# Spike: Can Lagos Flood Reports Be Reliably Located? (spike-lagos-report-geocoding)

> **Time-boxed spike — one week, no production code.** It answers one question and then stops. A spike that turns into an implementation has failed at its job.
>
> ## 🔴 COMPLETE 2026-08-04 — ANSWER: **NO. Depth is not viable as specified; recommend breadth.**
>
> Reports are **retrievable** (6/6 events, 77.3% of localities) and **fast** (median ≈ 12 h). They are **not reliably locatable**: **58.8%** exact-or-close against a **> 80%** bar, and **23.5%** placed in the **wrong part of the city** against a **< 5%** bar — a ~5× miss on the metric the spec named in advance as the deciding one.
>
> **The bottleneck is the gazetteer, not the journalism.** Both "Lekki" mentions — the product's flagship user — resolve ~39 km east into the wrong LGA, and *both independent gazetteers make the same error*, so cross-source agreement does not catch it. No cheap post-processing rule rescues it; every filter that removes wrong placements removes correct ones too.
>
> Full evidence in **[Findings — Day 6: verdict](#findings--day-6-verdict)**. Harness: [`scripts/spike-lagos-geocoding/`](../../scripts/spike-lagos-geocoding/).

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

## Findings — Days 2–3 (retrieval), run 2026-08-04

Searched the three permitted outlets (Punch, Premium Times, Vanguard) for each frozen ground-truth event. TheCable and Daily Trust excluded throughout on robots.txt AI-crawler grounds.

### Event-level retrieval: **6 / 6**

| event | found in | corroborating detail recovered |
|---|---|---|
| 3–4 Aug 2025 | Vanguard, Punch | Ijede/Ikorodu; "28 houses with fences destroyed"; 13 h rainfall |
| 20 Sept 2025 | Punch | **57,951 affected, 3,680 displaced, 3,244 houses** — matches the NEMA/ACT ground-truth figures exactly |
| 25–27 Sept 2025 | Vanguard, Punch | Kusenla Rd; the "1.2 m" drainage-invert misalignment; systems 156/157 |
| 28 Jun 2026 | Premium Times, Punch | terminal powerhouse shutdown; airlines moved to Terminal 2 |
| 30 Jun 2026 | Vanguard, Punch, Premium Times | Okota canoe evacuation; Ago Palace Way |
| 14 Jul 2026 | Punch | "worst-hit areas included Ikoyi, Lekki, Victoria Island and Oworonshoki" |

Per the pre-registered three-way rule: **6/6 → viable, proceed to locality metrics.**

### Locality retrieval (primary metric): **17 / 22 = 77.3%**

| event | n | retrieved | missed |
|---|---|---|---|
| 3–4 Aug 2025 | 6 | 5 | Agric |
| 20 Sept 2025 | 0 | — | *(aggregate event, no localities)* |
| 25–27 Sept 2025 | 2 | 2 | — |
| 28 Jun 2026 | 1 | 1 | — |
| 30 Jun 2026 | 9 | 5 | Victoria Island, Anthony Village, Somolu, Ayobo |
| 14 Jul 2026 | 4 | 4 | — |
| **total** | **22** | **17** | **5** |

**Bar was > 75%. Result 77.3% — passes by a single locality.**

⚠️ **95% CI: 54.6 – 92.2%. The lower bound is below the bar.** Per the asymmetry section written *before* any data was seen: **this result is permissive, not confirmatory.** It means "not disproven — proceed carefully." It does **not** establish that true retrieval exceeds 75%. One locality either way flips the point estimate.

*"Somolu" appeared only in a NiMet **forecast**, not an event report — scored as not retrieved, per the warning-vs-event rule.*

### 🎯 The observation that matters more than the score

**The misses cluster almost entirely in one event, for an instructive reason.**

For 30 June the Nigerian dailies covered the flood *extensively* but named a **different and larger** set of localities than AFP/Xinhua: Ago Palace Way, Gbagada, Isashi, Iba, Ojo, Ikorodu Road, Ikeja, Maryland, Mushin, Ogudu, Agege, Alimosho, Obalende, Ajah, Mafoluku.

The ground truth used the **wire's** list. The channel used its own — and it is *richer*, not poorer. So 5/9 is **not** the channel failing to cover Lagos; it is two sources naming different neighbourhoods for the same flood. **The metric understates channel capability.**

**This observation is recorded but the score is NOT adjusted.** Rescoring against localities discovered during retrieval is precisely the goalpost-move this spec forbids. If a future run wants credit for those localities, they must be added to a frozen ground truth *before* that run.

This also reinforces the Day-1 finding: street-level detail (Ago Palace Way, Fashoro Street, Kusenla Road) exists in the ingestion channel and in **no** international or humanitarian source. That is the capability the product depends on.

### ⚠️ Not measured

**Publication lag was not measured.** Search results give article existence and approximate month, not timestamps. The `< 24 h` bar requires fetching each article and reading its publication time — deferred to Days 4–5. Stated rather than estimated.

Status of the four bars: locality retrieval **measured** · event retrieval **measured** · geocoding **pending** · lag **pending**.

*(Resolved below — both were measured on 2026-08-04. Lag passed; **geocoding failed both its bars**, which decided the spike.)*

## Findings — Days 4–5 (geocoding + lag), run 2026-08-04

### Method actually used

Each of the **17 retrieved** locality mentions was resolved through **two independent gazetteers** so accuracy could not be self-certified by a single source:

| | gazetteer | query |
|---|---|---|
| **A** | Nominatim → **OpenStreetMap** | `"<name>, Lagos, Nigeria"` — the realistic pipeline; the article states the city |
| **B** | Open-Meteo → **GeoNames** | bare `"<name>"`, Nigerian hit preferred — the ambiguity diagnostic |

Scoring uses **A** (the realistic pipeline). Nominatim was rate-limited to 1 req/s per its usage policy.

**The harness is committed** at [`scripts/spike-lagos-geocoding/`](../../scripts/spike-lagos-geocoding/) — `geocode.mjs`, `score.mjs`, `mitigate.mjs`, and the `geocode-results.json` snapshot these numbers were computed from — so this verdict is reproducible rather than self-certified.

### The four-way split: **FAILS BOTH BARS**

| outcome | n | rate | 95% CI | bar | verdict |
|---|---|---|---|---|---|
| **Exact** (correct LGA) | 9 | 52.9% | 27.8 – 77.0% | — | — |
| **Close** (right area, wrong sub-unit) | 1 | 5.9% | 0.1 – 28.7% | — | — |
| **Wrong** (different part of the city) | 4 | **23.5%** | 6.8 – 49.9% | **< 5%** | ❌ **FAIL — ≈5× over** |
| **Failed** (nothing extracted) | 3 | 17.6% | 3.8 – 43.4% | — | — |
| **Exact-or-close** | 10 | **58.8%** | 32.9 – 81.6% | **> 80%** | ❌ **FAIL** |

**The `wrong` column is the one the spec singled out in advance** as the deciding metric — "confidently placing a flood in the wrong neighbourhood is a *harm*." It missed by roughly five times.

### The four wrong placements, and why they are not flukes

| locality | what the gazetteer returned | error | failure class |
|---|---|---|---|
| **Lekki** (×2 — 30 Jun, 14 Jul) | `Ibeju Lekki` LGA boundary, 3.815°E | **~39 km east, wrong LGA** | **name spans two LGAs** |
| **Oworonshoki** | `Apapa-Oworonshoki Expressway`, Amuwo Odofin | **~11 km, wrong LGA** | **road named after a place** |
| **Anjorin** | `Anjorin Street, Idimu, Alimosho` | **~32 km from Ikorodu** | **road named after a place** |

Verified by re-querying with the LGA supplied: `"Oworonsoki, Kosofe, Lagos"` returns the real place at `6.5502,3.4022` (Kosofe) — but the pipeline does not know the LGA, that being the thing it is trying to determine.

**The Lekki case is the most serious result in this spike.** Lekki is the single most-mentioned locality in the corpus and the product's flagship user ("would this have told a **Lekki** resident something useful?"). Constraining to Eti-Osa returns the urbanised, flooded Lekki at 3.45–3.53°E; unconstrained, both gazetteers return **Ibeju-Lekki**, a different LGA ~39 km east. **Both independent sources make the same error**, so cross-source agreement — the usual defence — does not catch it.

### The three failures are a different, milder problem

- **Oko Ope**, **Odetedo** — absent from *both* gazetteers even with `Ikorodu` supplied. Small riverine settlements simply are not in OSM or GeoNames.
- **Murtala Muhammed International Airport** — `"…, Lagos, Nigeria"` returns **nothing**, while the looser `"Murtala Muhammed Airport Lagos"` resolves correctly to Terminal 2 in Ikeja. A brittle exact-string match, not missing data.

These are recoverable: you show the event without a pin. They are **not** equivalent to the wrong placements.

### ⚠️ No cheap post-processing rule rescues this — tested, not assumed

Three of four wrong placements were road matches, so the obvious mitigation is to reject roads. **It was evaluated rather than asserted** ([`mitigate.mjs`](../../scripts/spike-lagos-geocoding/mitigate.mjs)):

| rule | exact-or-close (bar > 80%) | wrong (bar < 5%) |
|---|---|---|
| 0. as measured | 58.8% ❌ | 23.5% ❌ |
| 1. reject every `highway/*` match | **41.2%** ❌ | 11.8% ❌ |
| 2. reject `highway/*` unless the query names a road | **47.1%** ❌ | 11.8% ❌ |
| 3. rule 2 + reject LGA-boundary centroids | **47.1%** ❌ | **0.0%** ✅ |

Every rule that removes wrong placements also removes **correct** ones — `Okota` and `Kusenla Road` are genuine road matches that were right. The tension is fundamental: **the failure is name ambiguity, not match type**, so filtering by match type cannot separate them.

**Rule 3 is the honest summary of the whole exercise: you can make this pipeline safe, or useful, but not both, with off-the-shelf gazetteers.** It reaches zero harm only by refusing to place nearly half the localities.

### Sensitivity — the conclusion survives the contestable call

"Lekki" is genuinely ambiguous in ordinary usage (it can mean the whole peninsula, which includes Ibeju-Lekki). Scoring it generously:

| scoring | exact-or-close | wrong |
|---|---|---|
| strict (as reported) | 58.8% ❌ | 23.5% ❌ |
| both Lekki mentions → *Close* | 70.6% ❌ | 11.8% ❌ |
| **maximally generous** — all 4 wrong → *Close* | 82.4% ✅ (CI 56.6–96.2%) | 0.0% ✅ |

**Both bars still fail under the generous-but-defensible reading.** Only the indefensible one passes — and treating a 39 km, wrong-LGA placement as "close" is not available in a safety-adjacent product. **The verdict does not rest on a judgement call.**

### Publication lag: **PASSES** — median ≈ 12 h

All six events measured from article metadata (`datePublished` / `article:published_time`), not estimated. Lagos is **UTC+1**; times below are converted.

| event | earliest permitted-source article | published (WAT) | lag |
|---|---|---|---|
| 3–4 Aug 2025 Ikorodu | Punch | 4 Aug, 17:13 | ~22 h |
| 20 Sept 2025 NEMA aggregate | Punch | **23 Sept, 05:12** | **~73 h** ❌ |
| 25–27 Sept 2025 Lekki/Ikota | Punch | 24 Sept, 21:17 | ≤ 0 — *precedes* the truth window |
| 28 Jun 2026 airport | Punch | 29 Jun, 00:00 | ~14–18 h |
| 30 Jun 2026 | Vanguard | 30 Jun, 17:34 | same day |
| 14 Jul 2026 | Punch | 14 Jul, 00:01 | ~0 h |

**5 of 6 within 24 h; median ≈ 12 h. Bar `< 24 h` passes.**

Precision is deliberately limited to hours — **event onset times are not known precisely**, so lag cannot be stated more finely than the inputs justify. Two events demonstrate this directly: the Sept-2025 coverage *predates* the ground-truth window, and a Vanguard Lekki piece ran 12 July for a 14 July event. **Urban flooding is multi-day; "the event date" is fuzzy by ±1–2 days.**

The single miss is the **NEMA aggregate**, which is a different kind of artefact — an official casualty tally, inherently retrospective. Incident reporting is fast; *official* reporting is not.

### ⚠️ Premium Times could not be fetched

`premiumtimesng.com` returns a **1,228-byte anti-bot JavaScript challenge** (`aes.js`), not article HTML, to a plain client. It contributed to Days 2–3 retrieval via search snippets but **no Premium Times timestamp is in the lag table.** Recorded because it is a real constraint on any ingestion build, not a one-off.

## Findings — Day 6: verdict

| metric | n | bar | result | verdict |
|---|---|---|---|---|
| Event retrieval *(secondary)* | 6 | three-way rule | **6/6** | ✅ viable |
| **Locality retrieval** *(primary)* | 22 | > 75% | **77.3%** (CI 54.6–92.2) | ⚠️ **passes, permissive only** |
| **Geocoding exact-or-close** | 17 | > 80% | **58.8%** (CI 32.9–81.6) | ❌ **FAIL** |
| **Wrong placements** | 17 | < 5% | **23.5%** (CI 6.8–49.9) | ❌ **FAIL — ≈5×** |
| Publication lag (median) | 6 | < 24 h | **≈12 h** | ✅ pass |

### 🔴 Verdict: **depth is NOT viable as specified. Recommend breadth.**

This is the pre-registered consequence — *"miss any badly → depth is not viable; recommend breadth"* — and it is the **decisive direction** of an asymmetric test. Per the spec's own warning, a pass would have been merely permissive; **a failure of this size is real evidence.**

### But the failure is precisely located, and it is not where it was expected

Day 2's prediction — *"expect retrieval to be the step that kills it, if anything does"* — was **wrong**, and that is the useful part:

- ✅ The channel **carries** the events — 6/6.
- ✅ It **names** the neighbourhoods, at street level (Ago Palace Way, Kusenla Road, Fashoro Street) — detail that exists in **no** international or humanitarian feed.
- ✅ It publishes **fast** — median ~12 h.
- ❌ **Nothing can reliably turn those names into coordinates.**

**The bottleneck is the gazetteer, not the journalism.** The source material is good; Lagos micro-geography is not represented well enough in OSM or GeoNames to place it safely.

### What would actually be required

Not a tuned prompt or a better matcher — a **Lagos-specific gazetteer** with LGA-disambiguated aliases (`Lekki` → Eti-Osa *and* Ibeju-Lekki, resolved by context), place-vs-road precedence, informal settlement names absent from OSM, and an explicit **"refuse when ambiguous"** rule so failures land in the recoverable column rather than the harmful one. That is a substantially larger build than this spike assumed — **which is exactly what a one-week spike is for discovering.**

### Answering the Day-6 question plainly

> **Would this have told a Lekki resident something useful, in time?**

**In time — yes.** Useful — **no.** For both June-2026 and July-2026 events the pipeline would have placed the Lekki flood **~39 km east, in the wrong LGA**, with full confidence. A resident of Lekki Phase 1 would have seen a pin near the Free Zone and reasonably concluded they were not affected.

**That is worse than showing nothing**, which is the finding that decides it.

### Consequences

1. **`feature-continental-coverage` (breadth) is now the recommended next investment.** It relies on EONET's own coordinates and carries none of this geocoding risk.
2. **Do not build report-ingestion depth on an off-the-shelf gazetteer.** Retrieval and lag are proven adequate, so the thesis is not dead — but it is gated on a gazetteer build that must be scoped and costed separately.
3. **The five user conversations remain unrun and are still the cheapest test** — and this result raises their value. They would establish whether residents want *localised pins* (blocked here) or *"is my area flooding right now"* (which retrieval alone already supports).

## Deliverable

A one-page result in this proposal's `## Findings` section: the four metrics against their bars, the four-way geocoding split, and a **viable / not viable** verdict with the evidence attached.

## Related, and still cheaper

**Five user conversations remain the cheapest unrun test** — *"where do you currently find out if your area is flooding?"* The 2026-05-27 business review flagged these in May and they are still outstanding ([[project-traction-data-gap]]). They cost less than this spike and could invalidate its premise. If retrieval looks plausible on Day 2, start them in parallel.

## Origin

Arising from the 2026-08-02/03 data-density investigation, which established that (a) event sparsity for Nigeria + Ghana is real across four independent sources, and (b) the events that matter to Lagos residents are chronic, recurring, and invisible to every global feed. Pass bars agreed with the maintainer before the spike was written; retrieval raised from 70% to **75%** at their request.
