# Scripts

Maintenance and data-preparation utilities for VigilAfrica.

## generate_boundary_migration.py

Converts an HDX COD Admin Boundaries GeoJSON file into a VigilAfrica PostgreSQL migration.

**When to use**: Adding a new country or replacing simplified boundary rectangles with production-quality HDX polygons.

**Requirements**: Python 3.9+, no external dependencies.

**Usage**:

```bash
python scripts/generate_boundary_migration.py \
  --input path/to/gha_admbnda_adm1.geojson \
  --country-code GH \
  --country-name Ghana \
  --migration-number 000008
```

**Output**: `api/db/migrations/000008_boundary_GH.up.sql`

**Where to get the GeoJSON**:

1. Go to [data.humdata.org](https://data.humdata.org/)
2. Search for "COD Admin Boundaries [Country Name]"
3. Download the GeoJSON for ADM1 level
4. Verify it is a FeatureCollection with Polygon or MultiPolygon geometries

**After generating**:

1. Review the output SQL — check row count matches expected ADM1 count
2. Apply: `migrate -database $DATABASE_URL -path api/db/migrations up 1`
3. Verify: `SELECT COUNT(*) FROM admin_boundaries WHERE country_code = 'GH';`
4. Check enrichment rate via `GET /v1/enrichment-stats`

See also: `openspec/specs/vigilafrica/country-onboarding-template.md` §1.4

### Regenerate the existing NG + GH boundaries (review shortcut for `000010_replace_boundary_data.up.sql`)

Reviewers of a boundary migration shouldn't read 2.3MB of WKT. Verify by
regenerating from HDX instead:

```bash
mkdir -p .tmp-hdx && cd .tmp-hdx

# 1. Download both HDX COD zips via the CKAN API
for slug in nga gha; do
  url=$(curl -sS "https://data.humdata.org/api/3/action/package_show?id=cod-ab-${slug}" \
    | python3 -c "import json,sys;print([r['url'] for r in json.load(sys.stdin)['result']['resources'] if r['format'].lower()=='geojson'][0])")
  curl -sSL --max-time 120 -o "${slug}.zip" "$url"
done

# 2. Extract the ADM1 GeoJSONs
python3 -c "import zipfile; zipfile.ZipFile('nga.zip').extract('nga_admin1.geojson'); zipfile.ZipFile('gha.zip').extract('gha_admin1.geojson')"

cd ..

# 3. Generate per-country SQL
python3 scripts/generate_boundary_migration.py \
  --input .tmp-hdx/nga_admin1.geojson --country-code NG --country-name Nigeria \
  --migration-number 000010 --output-dir .tmp-hdx

python3 scripts/generate_boundary_migration.py \
  --input .tmp-hdx/gha_admin1.geojson --country-code GH --country-name Ghana \
  --migration-number 000010 --output-dir .tmp-hdx

# 4. The INSERT VALUES tuples inside .tmp-hdx/000010_boundary_{NG,GH}.up.sql
#    should match the corresponding INSERT sections in the committed
#    api/db/migrations/000010_replace_boundary_data.up.sql byte-for-byte.
#    Use `diff` to compare the two INSERT sections if you need certainty.

# 5. Cleanup
rm -rf .tmp-hdx
```

### Tests

```bash
python3 scripts/generate_boundary_migration_test.py
```

The test exercises property auto-detection, WKT emission for Polygon and
MultiPolygon, and SQL escaping using a small synthetic GeoJSON fixture.
