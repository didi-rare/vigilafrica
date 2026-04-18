# Country Onboarding Template

**Version**: 1.0  
**Status**: ACTIVE — use for every new country added to VigilAfrica  
**Maintained by**: @didi-rare

This template defines the repeatable process for adding a new African country to VigilAfrica. Follow each phase in order. A country is considered "onboarded" when all acceptance criteria in Phase 5 pass.

---

## Table of Contents

- [Tier Classification](#tier-classification)
- [Phase 0 — Feasibility Assessment](#phase-0--feasibility-assessment)
- [Phase 1 — Boundary Data](#phase-1--boundary-data)
- [Phase 2 — EONET Coverage](#phase-2--eonet-coverage)
- [Phase 3 — Database Integration](#phase-3--database-integration)
- [Phase 4 — API + Enrichment](#phase-4--api--enrichment)
- [Phase 5 — Acceptance Criteria](#phase-5--acceptance-criteria)
- [Fallback Logic](#fallback-logic)
- [Enrichment Validation Rules](#enrichment-validation-rules)

---

## Tier Classification

Before beginning onboarding, classify the country against the following tiers to set expectations for timeline and accuracy.

| Tier | Criteria | Examples | Expected enrichment success rate |
|---|---|---|---|
| **Tier 1** | High EONET event frequency; COD Admin Level 1 data available on HDX; documented NGO demand signal | Nigeria, Kenya, Ethiopia, DRC | ≥ 85% |
| **Tier 2** | Moderate EONET events; partial COD data or lower ADM1 coverage; indirect demand signal | Ghana, Mozambique, Mali | ≥ 70% |
| **Tier 3** | Low EONET event frequency; poor or absent HDX boundary data; no confirmed demand signal | Comoros, Eswatini, Djibouti | Backlog — do not onboard until data improves |

Tier is re-evaluated once real ingestion data is available. A country initially classified as Tier 2 may be promoted to Tier 1 after 90 days of ingestion.

---

## Phase 0 — Feasibility Assessment

Before writing any code, confirm all three items:

### 0.1 EONET Event Volume Check

Query the NASA EONET v3 API directly with the candidate country's bounding box and confirm at least 3 events exist in the past 12 months:

```bash
# Replace bbox values with the candidate country's bounding box
# Format: min_lon,min_lat,max_lon,max_lat
curl -s "https://eonet.gsfc.nasa.gov/api/v3/events?bbox=MIN_LON,MIN_LAT,MAX_LON,MAX_LAT&category=floods,wildfires&status=open,closed" \
  | jq '.events | length'
```

If the result is 0 or under 3, the country is likely Tier 3 — defer.

### 0.2 HDX Boundary Data Check

Verify that COD (Common Operational Dataset) Admin Boundaries exist for the country at ADM1 level:

1. Go to [data.humdata.org](https://data.humdata.org/)
2. Search: `COD Admin Boundaries [COUNTRY_NAME]`
3. Filter by organisation: OCHA
4. Confirm a GeoJSON or Shapefile download exists with ADM1 polygons

If only ADM0 (national boundary) is available, the country enrichment will only produce `country_name`, not `state_name`. This is Tier 3 behaviour — defer unless a specific use case justifies it.

### 0.3 Bounding Box Overlap Check

Confirm the candidate bounding box does not significantly overlap with an already-onboarded country. Use [bboxfinder.com](http://bboxfinder.com/) to visualise both bounding boxes.

Overlap > 5° of longitude or latitude in any direction requires an explicit decision: either tighten the bounding box or accept that events near the shared border may require manual review.

---

## Phase 1 — Boundary Data

### 1.1 Download HDX COD Boundaries

```bash
# Example for Ghana (GHA)
# 1. Visit: https://data.humdata.org/dataset/cod-ab-gha
# 2. Download: gha_admbnda_adm1_gss_20210308.geojson (ADM1)
# 3. Also download: gha_admbnda_adm0_gss_20210308.geojson (ADM0)
```

### 1.2 Validate the GeoJSON

```bash
# Install: npm install -g geojsonhint  (or use ogr2ogr)
geojsonhint gha_admbnda_adm1_gss_20210308.geojson

# Check CRS — must be EPSG:4326 (WGS84)
cat gha_admbnda_adm1_gss_20210308.geojson | jq '.crs'
# Expected: null (defaults to WGS84) or { "type": "name", "properties": { "name": "urn:ogc:def:crs:OGC:1.3:CRS84" } }
```

If CRS is not WGS84, reproject with ogr2ogr before proceeding:

```bash
ogr2ogr -f GeoJSON -t_srs EPSG:4326 output_wgs84.geojson input.geojson
```

### 1.3 Identify the ADM1 Name Field

HDX files use inconsistent field names. Find the correct property for state/province names:

```bash
cat gha_admbnda_adm1_gss_20210308.geojson | jq '[.features[0].properties | keys]'
# Look for: ADM1_EN, admin1Name_en, shapeName, NAME_1, etc.
```

Record the field name — you will use it in the migration generation script.

### 1.4 Generate the Migration SQL

Use the following script to convert the GeoJSON to SQL INSERT statements:

```python
#!/usr/bin/env python3
"""
generate_boundary_migration.py
Usage: python3 generate_boundary_migration.py INPUT.geojson COUNTRY_CODE COUNTRY_NAME ADM1_FIELD_NAME
Example: python3 generate_boundary_migration.py gha_admbnda_adm1_gss_20210308.geojson GH Ghana ADM1_EN
"""
import json
import sys

if len(sys.argv) != 5:
    print("Usage: generate_boundary_migration.py INPUT.geojson COUNTRY_CODE COUNTRY_NAME ADM1_FIELD")
    sys.exit(1)

geojson_path, country_code, country_name, adm1_field = sys.argv[1:]

with open(geojson_path) as f:
    data = json.load(f)

print("-- Auto-generated from HDX COD boundaries")
print(f"-- Source: {geojson_path}")
print(f"-- Country: {country_name} ({country_code})")
print()
print("DO $$ BEGIN")
print(f"  IF NOT EXISTS (SELECT 1 FROM admin_boundaries WHERE country_code = '{country_code}' AND adm_level = 1) THEN")

for feature in data['features']:
    adm_name = feature['properties'].get(adm1_field, 'Unknown')
    # Escape single quotes
    adm_name = adm_name.replace("'", "''")
    geom_json = json.dumps(feature['geometry'])
    geom_json_escaped = geom_json.replace("'", "''")
    print(f"    INSERT INTO admin_boundaries (country_code, country_name, adm_level, adm_name, geom)")
    print(f"    VALUES ('{country_code}', '{country_name}', 1, '{adm_name}',")
    print(f"      ST_Multi(ST_GeomFromGeoJSON('{geom_json_escaped}')));")

print("  END IF;")
print("END $$;")
```

Run and redirect to the migration file:

```bash
python3 generate_boundary_migration.py \
  gha_admbnda_adm1_gss_20210308.geojson \
  GH Ghana ADM1_EN \
  > api/db/migrations/000XXX_COUNTRY_CODE_admin_boundaries.up.sql
```

### 1.5 Add ADM0 (National Boundary)

The ADM0 row is used as a fallback — if a point is in the country but doesn't match any ADM1 boundary (data gap), the trigger still sets `country_name` correctly.

Add to the same migration file:

```sql
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM admin_boundaries WHERE country_code = 'GH' AND adm_level = 0) THEN
    INSERT INTO admin_boundaries (country_code, country_name, adm_level, adm_name, geom)
    SELECT 'GH', 'Ghana', 0, 'Ghana',
           ST_Multi(ST_Union(geom))
    FROM admin_boundaries
    WHERE country_code = 'GH' AND adm_level = 1;
  END IF;
END $$;
```

This generates the ADM0 boundary by unioning all ADM1 polygons — avoids managing a separate ADM0 file.

---

## Phase 2 — EONET Coverage

### 2.1 Determine Bounding Box

Find the country's tight bounding box from the ADM0 geometry:

```sql
SELECT ST_XMin(ST_Extent(geom)), ST_YMin(ST_Extent(geom)),
       ST_XMax(ST_Extent(geom)), ST_YMax(ST_Extent(geom))
FROM admin_boundaries
WHERE country_code = 'GH' AND adm_level = 0;
-- Returns: min_lon, min_lat, max_lon, max_lat
```

Add a 0.1° buffer to the bbox to capture events right on the border:

```
min_lon = result - 0.1
min_lat = result - 0.1
max_lon = result + 0.1
max_lat = result + 0.1
```

### 2.2 Register the Country in `DefaultCountries`

Add the country to `api/internal/ingestor/eonet.go`:

```go
var DefaultCountries = []CountryConfig{
    {Code: "NG", Name: "Nigeria", BBox: [4]float64{2.0, 4.0, 15.0, 14.0}},
    {Code: "GH", Name: "Ghana",   BBox: [4]float64{-3.5, 4.5, 1.2, 11.2}},
    // Add new country here:
    // {Code: "KE", Name: "Kenya", BBox: [4]float64{33.9, -4.7, 41.9, 5.0}},
}
```

The scheduler automatically picks up the new entry — no scheduler changes needed.

### 2.3 Verify No Bounding Box Overlap

After adding the new bbox, confirm no event would be double-counted between countries. Run a PostGIS check:

```sql
SELECT a.country_code, b.country_code,
       ST_Area(ST_Intersection(a.geom, b.geom)) AS overlap_area_sq_deg
FROM admin_boundaries a
JOIN admin_boundaries b ON a.country_code < b.country_code
WHERE a.adm_level = 0 AND b.adm_level = 0
  AND ST_Intersects(a.geom, b.geom);
```

If `overlap_area_sq_deg` > 0.5, review the bounding boxes and tighten if necessary.

---

## Phase 3 — Database Integration

### 3.1 Number the Migration

Follow the existing naming convention. Check the highest existing migration number:

```bash
ls api/db/migrations/*.up.sql | sort -V | tail -1
```

Name the new migration `000XXX_GH_admin_boundaries.up.sql` where XXX is the next number.

### 3.2 Make the Migration Idempotent

Wrap all INSERTs in existence checks to allow safe re-runs:

```sql
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM admin_boundaries WHERE country_code = 'GH') THEN
    -- INSERT statements here
  END IF;
END $$;
```

### 3.3 Test the Migration

```bash
# Fresh database test
docker compose up -d
cd api && go run ./cmd/server/  # migrations run on startup

# Verify boundary rows loaded
psql $DATABASE_URL -c "SELECT country_code, adm_level, COUNT(*) FROM admin_boundaries GROUP BY 1,2 ORDER BY 1,2;"
# Expected output:
#  country_code | adm_level | count
# --------------+-----------+-------
#  GH           |         0 |     1
#  GH           |         1 |    16
#  NG           |         0 |     1
#  NG           |         1 |    37
```

---

## Phase 4 — API + Enrichment

### 4.1 Test Enrichment Trigger

Insert a test event at a known coordinate and verify enrichment:

```sql
-- Insert test event at Accra, Ghana
INSERT INTO events (source_id, source, title, category, status, geom, geom_type, latitude, longitude, event_date)
VALUES ('test-gh-accra', 'test', 'Test Ghana Event', 'floods', 'open',
        ST_SetSRID(ST_MakePoint(-0.1870, 5.6037), 4326), 'Point', 5.6037, -0.1870, NOW())
ON CONFLICT (source_id) DO NOTHING;

-- Verify enrichment
SELECT source_id, country_name, state_name FROM events WHERE source_id = 'test-gh-accra';
-- Expected: country_name = 'Ghana', state_name = 'Greater Accra'

-- Clean up
DELETE FROM events WHERE source_id = 'test-gh-accra';
```

If `state_name` is null, check:
1. Is the geometry within the ADM1 boundary polygon? (`ST_Intersects`)
2. Are the ADM1 boundaries loaded? (`SELECT COUNT(*) FROM admin_boundaries WHERE country_code = 'GH'`)
3. Is the trigger active? (`SELECT tgname FROM pg_trigger WHERE tgname = 'enrich_event_location_trigger'`)

### 4.2 Test the API Country Filter

```bash
# Verify Ghana events are returned
curl "http://localhost:8080/v1/events?country=Ghana"

# Verify Nigeria events are still correct
curl "http://localhost:8080/v1/events?country=Nigeria"

# Verify combined (no filter)
curl "http://localhost:8080/v1/events" | jq '.meta.total'
```

### 4.3 Measure Enrichment Success Rate

After at least one ingestion run for the new country:

```sql
SELECT
    country_name,
    COUNT(*) AS total_events,
    COUNT(state_name) AS enriched_events,
    ROUND(100.0 * COUNT(state_name) / COUNT(*), 1) AS enrichment_pct
FROM events
GROUP BY country_name
ORDER BY country_name;
```

Target: ≥ 85% for Tier 1, ≥ 70% for Tier 2.

---

## Phase 5 — Acceptance Criteria

A country is considered successfully onboarded when all of the following pass:

- [ ] ADM0 and ADM1 boundary rows exist in `admin_boundaries` for the new country
- [ ] Migration is idempotent — running it twice yields the same row count
- [ ] At least one test event at a known coordinate enriches with the correct `state_name`
- [ ] Enrichment success rate meets the tier target (Tier 1: ≥ 85%, Tier 2: ≥ 70%)
- [ ] EONET ingestion produces events for the country in `ingestion_runs`
- [ ] `GET /v1/events?country=COUNTRY_NAME` returns the correct subset
- [ ] `GET /v1/events` (no filter) returns events for all countries combined
- [ ] No events are double-counted between countries (bounding box overlap check passes)
- [ ] `CONTRIBUTING.md` updated if local setup steps change

---

## Fallback Logic

The enrichment trigger (`trg_enrich_event_location`) handles these edge cases:

| Scenario | Outcome | Reasoning |
|---|---|---|
| Event geometry is `NULL` | `country_name = null`, `state_name = null` | No geometry, no enrichment |
| Event is within an ADM1 boundary | `country_name` and `state_name` populated correctly | Normal case |
| Event is within ADM0 but outside all ADM1 polygons | `country_name = null`, `state_name = null` | ADM1 data gap — investigate HDX source |
| Event is outside all known boundaries | `country_name = null`, `state_name = null` | Expected for events near sea or at borders |
| Event is on the border of two ADM1 regions | Smallest-area matching region wins (`ORDER BY ST_Area ASC`) | Area heuristic prefers the most specific boundary |

Events with `country_name = null` are valid and must not cause API errors. The frontend renders these as coordinate-only events with no state label.

---

## Enrichment Validation Rules

An event is considered **successfully enriched** if all three conditions hold:

1. `country_name IS NOT NULL`
2. `state_name IS NOT NULL`
3. `enriched_at IS NOT NULL`

An event is considered **partially enriched** if `country_name IS NOT NULL` but `state_name IS NULL`. This indicates an ADM1 data gap and should be investigated.

An event is considered **unenriched** if both are null. This is expected for:
- Events with no geometry
- Events geographically outside all loaded boundaries

**Do not** treat unenriched events as errors — they flow through the API normally and display with raw coordinates in the frontend.

---

## Reference: Countries Supported

| Country | Code | Tier | ADM1 Regions | EONET BBox | HDX Source |
|---|---|---|---|---|---|
| Nigeria | NG | 1 | 36 states + FCT | `2.0,4.0,15.0,14.0` | [HDX COD NGA](https://data.humdata.org/dataset/cod-ab-nga) |
| Ghana | GH | 2 | 16 regions | `-3.5,4.5,1.2,11.2` | [HDX COD GHA](https://data.humdata.org/dataset/cod-ab-gha) |
