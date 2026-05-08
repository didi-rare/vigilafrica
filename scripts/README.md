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
