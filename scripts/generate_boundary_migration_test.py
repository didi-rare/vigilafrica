#!/usr/bin/env python3
"""
Tests for generate_boundary_migration.py.

Locks the behaviours that the chore-hdx-boundaries migration depends on:
  - HDX 2025 COD property names (`adm1_name`) are auto-detected
  - detect_property returns the property KEY, not its value (regression
    guard for the pre-fix bug found in chore-hdx-boundaries)
  - Polygon and MultiPolygon geometries both round-trip to PostGIS WKT
  - Single quotes in state names are SQL-escaped

Run: python3 scripts/generate_boundary_migration_test.py
Requires: Python 3.9+; no external dependencies.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
SCRIPT = HERE / "generate_boundary_migration.py"

# Import the script's functions so we can unit-test them directly.
sys.path.insert(0, str(HERE))
import generate_boundary_migration as gen  # noqa: E402


def _polygon_feature(name: str, name_key: str = "adm1_name") -> dict:
    return {
        "type": "Feature",
        "properties": {name_key: name, "adm1_pcode": "TST001"},
        "geometry": {
            "type": "Polygon",
            "coordinates": [[[0.0, 0.0], [1.0, 0.0], [1.0, 1.0], [0.0, 1.0], [0.0, 0.0]]],
        },
    }


def _multipolygon_feature(name: str) -> dict:
    return {
        "type": "Feature",
        "properties": {"adm1_name": name},
        "geometry": {
            "type": "MultiPolygon",
            "coordinates": [
                [[[0.0, 0.0], [1.0, 0.0], [1.0, 1.0], [0.0, 1.0], [0.0, 0.0]]],
                [[[2.0, 2.0], [3.0, 2.0], [3.0, 3.0], [2.0, 3.0], [2.0, 2.0]]],
            ],
        },
    }


class TestDetectProperty(unittest.TestCase):
    def test_returns_key_not_value(self):
        """Regression guard for chore-hdx-boundaries D7."""
        feature = _polygon_feature("Abia")
        result = gen.detect_property(feature, gen.ADM1_NAME_CANDIDATES)
        # Must be the KEY ('adm1_name'), NOT the value ('Abia').
        self.assertEqual(result, "adm1_name")

    def test_hdx_2025_format_detected(self):
        """HDX COD 2025 uses lowercase `adm1_name`; must be in the list."""
        self.assertIn("adm1_name", gen.ADM1_NAME_CANDIDATES)
        self.assertIn("adm0_name", gen.ADM0_NAME_CANDIDATES)

    def test_legacy_uppercase_format_still_detected(self):
        """Older HDX exports use `ADM1_EN`; must still work."""
        feature = {"type": "Feature", "properties": {"ADM1_EN": "Legacy State"}, "geometry": None}
        self.assertEqual(gen.detect_property(feature, gen.ADM1_NAME_CANDIDATES), "ADM1_EN")

    def test_missing_property_returns_none(self):
        feature = {"type": "Feature", "properties": {"unrelated": "x"}, "geometry": None}
        self.assertIsNone(gen.detect_property(feature, gen.ADM1_NAME_CANDIDATES))

    def test_empty_value_is_treated_as_missing(self):
        """An adm1_name with empty string MUST NOT match."""
        feature = {"type": "Feature", "properties": {"adm1_name": ""}, "geometry": None}
        self.assertIsNone(gen.detect_property(feature, gen.ADM1_NAME_CANDIDATES))


class TestGeoJSONToWKT(unittest.TestCase):
    def test_polygon_roundtrip(self):
        geom = {"type": "Polygon", "coordinates": [[[0.0, 0.0], [1.0, 0.0], [1.0, 1.0], [0.0, 0.0]]]}
        wkt = gen.geojson_geometry_to_wkt(geom)
        self.assertTrue(wkt.startswith("POLYGON("))
        self.assertIn("0.0 0.0", wkt)
        self.assertIn("1.0 1.0", wkt)

    def test_multipolygon_roundtrip(self):
        geom = {
            "type": "MultiPolygon",
            "coordinates": [
                [[[0.0, 0.0], [1.0, 0.0], [1.0, 1.0], [0.0, 0.0]]],
                [[[2.0, 2.0], [3.0, 2.0], [3.0, 3.0], [2.0, 2.0]]],
            ],
        }
        wkt = gen.geojson_geometry_to_wkt(geom)
        self.assertTrue(wkt.startswith("MULTIPOLYGON("))
        # Both polygon vertex sets present.
        self.assertIn("0.0 0.0", wkt)
        self.assertIn("3.0 3.0", wkt)

    def test_unsupported_geometry_type_raises(self):
        geom = {"type": "Point", "coordinates": [0.0, 0.0]}
        with self.assertRaises(ValueError):
            gen.geojson_geometry_to_wkt(geom)


class TestSQLEscape(unittest.TestCase):
    def test_no_quotes_unchanged(self):
        self.assertEqual(gen.escape_sql_string("Lagos"), "Lagos")

    def test_single_quote_doubled(self):
        self.assertEqual(gen.escape_sql_string("Cote d'Ivoire"), "Cote d''Ivoire")

    def test_multiple_quotes(self):
        self.assertEqual(gen.escape_sql_string("a'b'c"), "a''b''c")


class TestEndToEndGeneration(unittest.TestCase):
    """Run the script as a subprocess with a synthetic fixture."""

    def test_polygon_features_produce_valid_migration(self):
        fixture = {
            "type": "FeatureCollection",
            "features": [
                _polygon_feature("Region A"),
                _polygon_feature("Region B"),
                _multipolygon_feature("Region C"),
                # A name with an apostrophe to exercise the SQL escape path.
                _polygon_feature("Cote d'Ivoire State"),
            ],
        }

        with tempfile.TemporaryDirectory() as td:
            td_path = Path(td)
            fixture_path = td_path / "fixture.geojson"
            fixture_path.write_text(json.dumps(fixture), encoding="utf-8")

            result = subprocess.run(
                [
                    sys.executable,
                    str(SCRIPT),
                    "--input", str(fixture_path),
                    "--country-code", "XY",
                    "--country-name", "Xyland",
                    "--migration-number", "099999",
                    "--output-dir", str(td_path),
                ],
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(result.returncode, 0, msg=f"stderr: {result.stderr}")

            sql_path = td_path / "099999_boundary_XY.up.sql"
            self.assertTrue(sql_path.exists())
            sql = sql_path.read_text(encoding="utf-8")

            # Structural expectations.
            self.assertEqual(sql.count("('XY', 'Xyland', 1,"), 4)
            self.assertEqual(sql.count("ST_GeomFromText("), 4)
            self.assertEqual(sql.count("MULTIPOLYGON("), 1)
            # 3 standalone POLYGON values + 1 POLYGON substring inside the
            # one MULTIPOLYGON literal = 4 occurrences of the substring.
            self.assertEqual(sql.count("POLYGON("), 4)
            # Every geometry MUST carry SRID 4326.
            self.assertEqual(sql.count(", 4326)"), 4)
            # All single-quote escapes are doubled, none bare.
            self.assertIn("Cote d''Ivoire State", sql)


class TestRegressionGuards(unittest.TestCase):
    """Targeted guards for bugs that have shipped in the past."""

    def test_detect_property_does_not_return_value(self):
        """
        Pre-fix behaviour returned the property value (e.g. 'Abia'), which
        the downstream code then used as a column name, causing every
        feature to be skipped with 'empty ADM1 name'. Lock this in.
        """
        feature = _polygon_feature("Abia")
        result = gen.detect_property(feature, gen.ADM1_NAME_CANDIDATES)
        self.assertNotIn(result, ("Abia",))  # would mean the value is being returned
        self.assertEqual(result, "adm1_name")  # the key, as intended


if __name__ == "__main__":
    # Always run from the script's own directory so subprocess invocations
    # of generate_boundary_migration.py resolve correctly regardless of cwd.
    os.chdir(HERE.parent)
    unittest.main(verbosity=2)
