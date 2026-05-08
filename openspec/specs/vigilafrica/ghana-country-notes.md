# Ghana Country Notes

**Added in**: v0.6  
**Stable in**: v0.7  
**Maintained by**: @didi-rare  
**Template followed**: `openspec/specs/vigilafrica/country-onboarding-template.md`

---

## Tier Classification

Ghana is **Tier 2** per the onboarding template criteria:

| Criterion | Assessment |
|---|---|
| EONET event frequency | Moderate — floods and wildfires occur but at lower frequency than Nigeria |
| HDX COD ADM1 coverage | Good — 16 regions post-2019 administrative reorganisation; COD data available at [data.humdata.org/dataset/cod-ab-gha](https://data.humdata.org/dataset/cod-ab-gha) |
| NGO demand signal | Indirect — no confirmed partner yet; proximity to Nigeria makes co-deployment logical |

Expected enrichment success rate: ≥ 70% (Tier 2 baseline). Target for v0.7: ≥ 85%.

---

## EONET Bounding Box

Ghana bounding box committed in `api/internal/ingestor/eonet.go`:

```
BBox: [4]float64{-3.5, 4.5, 1.2, 11.2}
// [min_lon, min_lat, max_lon, max_lat]
```

### Overlap Validation

Nigeria bounding box: `[2.0, 4.0, 15.0, 14.0]`

**Result: Zero overlap confirmed.**

- Ghana max_lon = **1.2**
- Nigeria min_lon = **2.0**
- Longitudinal gap = 0.8°

No EONET event can simultaneously fall within both bounding boxes. Events near the Ghana–Togo border (lon ~0.0 to 1.2) are correctly captured by Ghana's box and do not overlap with Nigeria's minimum longitude.

---

## Administrative Boundary Data

### Current state (v0.6 / v0.7)

16 ADM1 regions using simplified bounding-box rectangles committed in migration `000005_admin_boundary_data.up.sql` — post-2019 reorganisation regions:

| Region | Coverage note |
|---|---|
| Greater Accra | Coastal, densely populated — high confidence for flood events |
| Ashanti | Central highland — main flood exposure |
| Western / Western North | Split region post-2019; rectangles approximate correctly |
| Volta / Oti | Split region post-2019; Oti carved from Volta 2019 |
| Northern / Savannah / North East | Three-way split post-2019 from old Northern Region |
| Upper East / Upper West | Northern border regions |
| Eastern / Central / Ahafo / Bono / Bono East | Central / southern regions |

### Known limitations

- Rectangle approximations are accurate for events well within a region (> ~50km from borders)
- Events near shared boundaries (Ghana–Togo lon ~1.2, Ghana–Burkina Faso lat ~11.0, Ghana–Côte d'Ivoire lon ~-3.0) may enrich to an adjacent region
- The Volta/Oti split (post-2019) has the highest ambiguity risk: EONET events near Lat 8.7, Lon -0.1 fall on the boundary between the two rectangles — the enrichment trigger uses `ORDER BY ST_Area ASC` to prefer the smaller polygon, which is Oti (correct for events clearly in the north)

### Upgrade path

Replace simplified rectangles with official HDX COD GeoJSON using:

```bash
python scripts/generate_boundary_migration.py \
  --input path/to/gha_admbnda_adm1.geojson \
  --country-code GH \
  --country-name Ghana \
  --migration-number 000008
```

Run the enrichment rate query after migration to confirm improvement:

```sql
SELECT country_name, COUNT(*) AS total,
       COUNT(*) FILTER (WHERE state_name IS NOT NULL) AS enriched,
       ROUND(COUNT(*) FILTER (WHERE state_name IS NOT NULL)::numeric / COUNT(*) * 100, 1) AS pct
FROM events
WHERE country_name = 'Ghana'
GROUP BY country_name;
```

---

## Deviations from Onboarding Template

**None.** Ghana followed all phases of the onboarding template in v0.6:

- [x] Phase 0: Feasibility — EONET bbox query confirmed events exist
- [x] Phase 1: Boundary data — 16 ADM1 rectangles committed in 000005
- [x] Phase 2: EONET coverage — bbox confirmed, no Nigeria overlap
- [x] Phase 3: Database integration — enrichment trigger updated in 000006
- [x] Phase 4: API + enrichment — `?country=Ghana` filter active
- [x] Phase 5: Acceptance criteria — to be verified via `/v1/enrichment-stats` in v0.7

---

## Edge Cases

| Scenario | Behaviour |
|---|---|
| Event in Atlantic Ocean (near Accra coast) | `state_name = null`, `country_name = null` — outside all ADM1 rectangles |
| Event near Ghana–Togo border (lon ~1.0) | May enrich as Volta or Oti depending on lat; enrichment trigger picks smallest polygon |
| Event in northern Ghana near Burkina Faso (lat ~11.0) | Correctly enriches to Upper East or Upper West for most EONET point events |
| Event at Ghana–Nigeria maritime border (Gulf of Guinea) | Both bboxes have 0.8° longitudinal gap; event falls in neither bbox and is not ingested |
| EONET event with polygon geometry | Centroid used for enrichment; large flood polygons spanning two regions will enrich to whichever region contains the centroid |
