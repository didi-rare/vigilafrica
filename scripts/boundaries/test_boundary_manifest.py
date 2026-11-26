#!/usr/bin/env python3
"""Offline tests for boundary_manifest.py's pure helpers (no network).

    python -I scripts/boundaries/test_boundary_manifest.py
"""

import io
import json
import sys
import unittest
import zipfile
from pathlib import Path

# -I drops the script directory from sys.path; add it back explicitly (it is
# the repository, not a download directory).
sys.path.insert(0, str(Path(__file__).resolve().parent))

import boundary_manifest as bm  # noqa: E402


def _zip(members: dict) -> bytes:
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as zf:
        for name, data in members.items():
            zf.writestr(name, data)
    return buf.getvalue()


class PaddedBBoxTest(unittest.TestCase):
    def test_nigeria_extent_rounds_outward(self):
        # Extent of nga_admin1.geojson (HDX, vintage 2025-10-30), measured 2026-11-26.
        ext = (2.66853356300004, 4.273007390000146, 14.67881623400018, 13.894419133000099)
        self.assertEqual(bm.padded_bbox(ext), [2.56, 4.17, 14.78, 14.0])

    def test_ghana_extent_rounds_outward(self):
        ext = (-3.262084960999971, 4.740722656000059, 1.200317383000083, 11.174682616999974)
        self.assertEqual(bm.padded_bbox(ext), [-3.37, 4.64, 1.31, 11.28])

    def test_box_always_contains_extent_plus_pad(self):
        ext = (10.0, -5.0, 11.0, -4.0)
        box = bm.padded_bbox(ext)
        self.assertLessEqual(box[0], 9.9)
        self.assertLessEqual(box[1], -5.1)
        self.assertGreaterEqual(box[2], 11.1)
        self.assertGreaterEqual(box[3], -3.9)


class HDXMetadataTest(unittest.TestCase):
    def test_vintage_is_end_of_reference_period(self):
        self.assertEqual(bm.vintage_from_dataset_date("[2019-04-17T00:00:00 TO 2025-10-30T23:59:59]"), "2025-10-30")

    def test_vintage_single_date(self):
        self.assertEqual(bm.vintage_from_dataset_date("2024-01-02T00:00:00"), "2024-01-02")

    def test_pick_geojson_zip(self):
        resources = [
            {"name": "nga_admin_boundaries.shp.zip", "url": "https://x/download/nga_admin_boundaries.shp.zip"},
            {"name": "nga_admin_boundaries.geojson.zip", "url": "https://x/download/nga_admin_boundaries.geojson.zip"},
        ]
        self.assertEqual(bm.pick_geojson_zip(resources, "NGA"), "https://x/download/nga_admin_boundaries.geojson.zip")

    def test_pick_geojson_zip_refuses_shapefile_only(self):
        with self.assertRaises(bm.ManifestError):
            bm.pick_geojson_zip([{"name": "a.shp.zip", "url": "https://x/a.shp.zip"}], "TZA")

    def test_pick_geojson_zip_refuses_http(self):
        with self.assertRaises(bm.ManifestError):
            bm.pick_geojson_zip([{"name": "n", "url": "http://x/nga_admin_boundaries.geojson.zip"}], "NGA")


class MemberTest(unittest.TestCase):
    def test_pick_adm1_member_skips_em(self):
        names = ["nga_admin0.geojson", "nga_admin1_em.geojson", "nga_admin1.geojson", "nga_admin2.geojson"]
        self.assertEqual(bm.pick_adm1_member(names, "nga"), "nga_admin1.geojson")

    def test_pick_adm1_member_refuses_only_em(self):
        with self.assertRaises(bm.ManifestError):
            bm.pick_adm1_member(["xyz_admin1_em.geojson"], "xyz")

    def test_extract_member_refuses_em(self):
        with self.assertRaises(bm.ManifestError):
            bm.extract_member(_zip({"a_admin1_em.geojson": "{}"}), "a_admin1_em.geojson")

    def test_extract_member_size_cap(self):
        with self.assertRaises(bm.ManifestError):
            bm.extract_member(_zip({"a_admin1.geojson": "x" * 100}), "a_admin1.geojson", max_bytes=10)


class BuildEntryTest(unittest.TestCase):
    FC = {
        "type": "FeatureCollection",
        "crs": {"type": "name", "properties": {"name": "urn:ogc:def:crs:OGC:1.3:CRS84"}},
        "features": [
            {"type": "Feature", "properties": {"adm1_name": "A", "adm1_pcode": "XX01", "name": ""},
             "geometry": {"type": "Polygon", "coordinates": [[[1, 1], [2, 1], [2, 2], [1, 1]]]}},
            {"type": "Feature", "properties": {"adm1_name": "B", "adm1_pcode": "XX02", "name": "b"},
             "geometry": {"type": "Polygon", "coordinates": [[[2, 1], [3, 1], [3, 2], [2, 1]]]}},
        ],
    }

    def test_fields_and_kept_settings(self):
        data = json.dumps(self.FC).encode()
        existing = {"enabled": False, "name_overrides": {"A": "Aa"}}
        entry, n = bm.build_entry(code="XX", name="Exland", archive_url="https://x/a.zip", member="xx_admin1.geojson",
                                  member_bytes=data, vintage="2025-01-01", existing=existing)
        self.assertEqual(n, 2)
        self.assertEqual(entry["name_field"], "adm1_name")  # `name` is empty on one feature
        self.assertEqual(entry["pcode_field"], "adm1_pcode")
        self.assertEqual(entry["bbox"], [0.9, 0.9, 3.1, 2.1])
        self.assertIs(entry["enabled"], False)
        self.assertEqual(entry["name_overrides"], {"A": "Aa"})

    def test_rejects_projected_crs(self):
        fc = dict(self.FC, crs={"type": "name", "properties": {"name": "urn:ogc:def:crs:EPSG::32631"}})
        with self.assertRaises(bm.ManifestError):
            bm.build_entry(code="XX", name="X", archive_url="https://x", member="m", member_bytes=json.dumps(fc).encode(),
                           vintage="2025-01-01", existing=None)

    def test_format_is_one_line_per_country_sorted(self):
        doc = {"schema_version": 1, "countries": [{"code": "NG", "name": "N"}, {"code": "GH", "name": "G"}]}
        text = bm.format_manifest(doc)
        lines = [l for l in text.splitlines() if l.strip().startswith('{"code"')]
        self.assertEqual(len(lines), 2)
        self.assertIn('"GH"', lines[0])
        self.assertEqual(json.loads(text)["countries"][1]["code"], "NG")


if __name__ == "__main__":
    unittest.main()
