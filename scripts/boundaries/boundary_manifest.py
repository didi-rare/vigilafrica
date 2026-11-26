#!/usr/bin/env python3
"""
Build and verify entries of the boundary manifest
(api/internal/boundaries/manifest.json, feature-continental-coverage spec §1).

Two subcommands, both stdlib-only (Python 3.9+):

  build   Download one country's HDX COD-AB GeoJSON zip, extract its ADM1
          member (never an `_em` variant), and write that country's manifest
          entry: archive_url, member, member_sha256, vintage (end of the HDX
          reference period), name_field, pcode_field and bbox (the member's
          extent plus 0.1 degrees, rounded outward to 2 decimals). An existing
          entry's `enabled` and `name_overrides` are kept, so a refresh is a
          one-line diff.

              python -I scripts/boundaries/boundary_manifest.py build \\
                  --iso3 NGA --code NG --name Nigeria

  verify  Re-download every entry in the manifest and check the extracted
          member's SHA-256 against `member_sha256`. Exits 1 on any mismatch or
          download failure. Run it on every PR that changes the manifest.

              python -I scripts/boundaries/boundary_manifest.py verify

Downloads are untrusted data. They go to a fresh temporary directory (or
--work-dir, which must be empty) and are never written inside the repository.
Run with `python -I` so nothing in the working directory is imported.
"""

from __future__ import annotations

import argparse
import hashlib
import io
import json
import math
import re
import sys
import tempfile
import urllib.request
import zipfile
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_MANIFEST = REPO_ROOT / "api" / "internal" / "boundaries" / "manifest.json"

HDX_PACKAGE_API = "https://data.humdata.org/api/3/action/package_show?id=cod-ab-{iso3}"
USER_AGENT = "vigilafrica-boundary-manifest/1.0"

# The loader's default BOUNDARY_MAX_ARCHIVE_BYTES; the script refuses anything
# the loader would refuse, so a manifest entry cannot pass here and fail there.
MAX_ARCHIVE_BYTES = 128 * 1024 * 1024

BBOX_PAD_DEG = 0.1

# ADM1 name property, first match wins. The list is the one in
# scripts/generate_boundary_migration.py (ADM1_NAME_CANDIDATES), which produced
# migration 000010; the 2025 HDX release for Nigeria and Ghana uses adm1_name.
ADM1_NAME_CANDIDATES = [
    "adm1_name",
    "ADM1_EN",
    "admin1Name_en",
    "admin1Name",
    "NAME_1",
    "name_en",
    "name",
]

# ADM1 P-code property. generate_boundary_migration.py has no such list; this
# one starts with the 2025 HDX spelling and adds the older COD-AB spellings.
ADM1_PCODE_CANDIDATES = [
    "adm1_pcode",
    "ADM1_PCODE",
    "admin1Pcode",
    "ADM1_PCODE_",
]

ENTRY_KEY_ORDER = [
    "code",
    "name",
    "levels",
    "source",
    "enabled",
    "archive_url",
    "member",
    "member_sha256",
    "vintage",
    "name_field",
    "pcode_field",
    "name_overrides",
    "bbox",
]


class ManifestError(Exception):
    """A condition the operator must fix; printed without a traceback."""


# ── Network ────────────────────────────────────────────────────────────────────


def fetch(url: str, max_bytes: int = MAX_ARCHIVE_BYTES, timeout: int = 300) -> bytes:
    """GET url, refusing bodies larger than max_bytes."""
    req = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
    with urllib.request.urlopen(req, timeout=timeout) as resp:  # noqa: S310 (https only, checked by caller)
        length = resp.headers.get("Content-Length")
        if length is not None and int(length) > max_bytes:
            raise ManifestError(f"{url}: Content-Length {length} exceeds {max_bytes} bytes")
        body = resp.read(max_bytes + 1)
    if len(body) > max_bytes:
        raise ManifestError(f"{url}: body exceeds {max_bytes} bytes")
    return body


def hdx_package(iso3: str) -> dict:
    body = fetch(HDX_PACKAGE_API.format(iso3=iso3.lower()), max_bytes=16 * 1024 * 1024)
    doc = json.loads(body)
    if not doc.get("success"):
        raise ManifestError(f"HDX package_show for cod-ab-{iso3.lower()} did not succeed")
    return doc["result"]


# ── Pure helpers (unit-tested offline in test_boundary_manifest.py) ────────────


def pick_geojson_zip(resources: list[dict], iso3: str) -> str:
    """Return the URL of the `<iso3>_admin_boundaries.geojson.zip` resource."""
    want = f"{iso3.lower()}_admin_boundaries.geojson.zip"
    matches = [r["url"] for r in resources if str(r.get("url", "")).lower().endswith("/" + want)]
    if not matches:
        names = ", ".join(sorted(str(r.get("name")) for r in resources))
        raise ManifestError(f"no {want} resource (shapefile-only country?); resources: {names}")
    if len(matches) > 1:
        raise ManifestError(f"{len(matches)} resources end in {want}: {matches}")
    url = matches[0]
    if not url.startswith("https://"):
        raise ManifestError(f"archive URL is not https: {url}")
    return url


def vintage_from_dataset_date(dataset_date: str) -> str:
    """End of HDX's reference period: '[2019-04-17T00:00:00 TO 2025-10-30T23:59:59]' -> '2025-10-30'."""
    m = re.fullmatch(r"\[\s*(\S+)\s+TO\s+(\S+)\s*\]", dataset_date.strip())
    end = m.group(2) if m else dataset_date.strip()
    d = re.match(r"(\d{4}-\d{2}-\d{2})", end)
    if not d:
        raise ManifestError(f"cannot read a date from dataset_date {dataset_date!r}")
    return d.group(1)


def is_em_variant(member: str) -> bool:
    return member.lower().endswith("_em.geojson")


def pick_adm1_member(names: list[str], iso3: str) -> str:
    """The ADM1 GeoJSON member, never an `_em` variant."""
    want = f"{iso3.lower()}_admin1.geojson"
    exact = [n for n in names if n.lower() == want]
    if len(exact) == 1:
        return exact[0]
    candidates = [
        n for n in names
        if re.search(r"admin1\.geojson$|adm1\.geojson$", n.lower()) and not is_em_variant(n)
    ]
    if len(candidates) != 1:
        raise ManifestError(f"expected one ADM1 member like {want}, found {candidates or 'none'} in {names}")
    return candidates[0]


def extract_member(archive: bytes, member: str, max_bytes: int = MAX_ARCHIVE_BYTES) -> bytes:
    if is_em_variant(member):
        raise ManifestError(f"refusing _em member {member}")
    with zipfile.ZipFile(io.BytesIO(archive)) as zf:
        try:
            info = zf.getinfo(member)
        except KeyError:
            raise ManifestError(f"member {member} not in archive; members: {zf.namelist()}") from None
        if info.file_size > max_bytes:
            raise ManifestError(f"member {member} is {info.file_size} bytes uncompressed, over {max_bytes}")
        with zf.open(info) as fh:
            data = fh.read(max_bytes + 1)
    if len(data) > max_bytes:
        raise ManifestError(f"member {member} exceeds {max_bytes} bytes")
    return data


def _walk_coords(coords, out_x: list, out_y: list) -> None:
    if coords and isinstance(coords[0], (int, float)):
        out_x.append(float(coords[0]))
        out_y.append(float(coords[1]))
        return
    for c in coords:
        _walk_coords(c, out_x, out_y)


def extent(features: list[dict]) -> tuple[float, float, float, float]:
    xs: list[float] = []
    ys: list[float] = []
    for f in features:
        geom = f.get("geometry")
        if not geom:
            raise ManifestError("feature with no geometry")
        _walk_coords(geom["coordinates"], xs, ys)
    if not xs:
        raise ManifestError("no coordinates")
    return min(xs), min(ys), max(xs), max(ys)


def padded_bbox(ext: tuple[float, float, float, float], pad: float = BBOX_PAD_DEG) -> list[float]:
    """Extent plus pad, rounded OUTWARD to 2 decimals so the box never shrinks below the rule."""
    min_x, min_y, max_x, max_y = ext

    def down(v: float) -> float:
        return math.floor(round(v * 100, 6)) / 100

    def up(v: float) -> float:
        return math.ceil(round(v * 100, 6)) / 100

    return [down(min_x - pad), down(min_y - pad), up(max_x + pad), up(max_y + pad)]


def detect_field(features: list[dict], candidates: list[str]) -> str | None:
    """First candidate that is a non-empty string on EVERY feature."""
    for key in candidates:
        if all(isinstance(f.get("properties", {}).get(key), str) and f["properties"][key].strip() for f in features):
            return key
    return None


def check_crs(fc: dict) -> None:
    crs = (fc.get("crs") or {}).get("properties", {}).get("name")
    if crs and not re.search(r"CRS84$|EPSG::?4326$", crs):
        raise ManifestError(f"member CRS is {crs}, not WGS84 lon/lat")


def ordered_entry(entry: dict) -> dict:
    out = {k: entry[k] for k in ENTRY_KEY_ORDER if k in entry}
    out.update({k: v for k, v in entry.items() if k not in out})
    return out


def format_manifest(doc: dict) -> str:
    """One country per line, sorted by code, so a country change is a one-line diff."""
    countries = sorted(doc["countries"], key=lambda e: e["code"])
    lines = [json.dumps(ordered_entry(e), ensure_ascii=False, separators=(", ", ": ")) for e in countries]
    body = ",\n    ".join(lines)
    return (
        "{\n"
        f'  "schema_version": {doc.get("schema_version", 1)},\n'
        '  "countries": [\n'
        f"    {body}\n"
        "  ]\n"
        "}\n"
    )


def build_entry(*, code: str, name: str, archive_url: str, member: str, member_bytes: bytes,
                vintage: str, existing: dict | None) -> tuple[dict, int]:
    fc = json.loads(member_bytes)
    check_crs(fc)
    features = fc.get("features") or []
    if not features:
        raise ManifestError(f"{member} has no features")
    name_field = detect_field(features, ADM1_NAME_CANDIDATES)
    if name_field is None:
        raise ManifestError(f"no ADM1 name field among {ADM1_NAME_CANDIDATES} is set on every feature")
    pcode_field = detect_field(features, ADM1_PCODE_CANDIDATES)
    entry = {
        "code": code,
        "name": name,
        "levels": "adm1",
        "source": "hdx-cod-ab",
        "enabled": True,
        "archive_url": archive_url,
        "member": member,
        "member_sha256": hashlib.sha256(member_bytes).hexdigest(),
        "vintage": vintage,
        "name_field": name_field,
        "name_overrides": {},
        "bbox": padded_bbox(extent(features)),
    }
    if pcode_field:
        entry["pcode_field"] = pcode_field
    if existing:
        entry["enabled"] = existing.get("enabled", True)
        entry["name_overrides"] = existing.get("name_overrides", {})
    return ordered_entry(entry), len(features)


# ── Commands ───────────────────────────────────────────────────────────────────


def read_manifest(path: Path) -> dict:
    if not path.exists():
        return {"schema_version": 1, "countries": []}
    return json.loads(path.read_text(encoding="utf-8"))


def work_dir(arg: str | None) -> Path:
    if arg is None:
        return Path(tempfile.mkdtemp(prefix="vigil-boundaries-"))
    p = Path(arg)
    p.mkdir(parents=True, exist_ok=True)
    if any(p.iterdir()):
        raise ManifestError(f"--work-dir {p} must be empty (downloads are untrusted)")
    if REPO_ROOT in p.resolve().parents or p.resolve() == REPO_ROOT:
        raise ManifestError("--work-dir must be outside the repository")
    return p


def cmd_build(args: argparse.Namespace) -> int:
    code = args.code.upper()
    if not re.fullmatch(r"[A-Z]{2}", code):
        raise ManifestError(f"--code must be ISO 3166-1 alpha-2, got {args.code!r}")
    iso3 = args.iso3.lower()
    pkg = hdx_package(iso3)
    url = pick_geojson_zip(pkg["resources"], iso3)
    vintage = vintage_from_dataset_date(pkg.get("dataset_date", ""))
    archive = fetch(url)
    wd = work_dir(args.work_dir)
    (wd / f"{iso3}.zip").write_bytes(archive)  # kept for inspection; outside the repo
    with zipfile.ZipFile(io.BytesIO(archive)) as zf:
        member = pick_adm1_member(zf.namelist(), iso3)
    data = extract_member(archive, member)

    manifest_path = Path(args.manifest)
    doc = read_manifest(manifest_path)
    existing = next((e for e in doc["countries"] if e["code"] == code), None)
    entry, n = build_entry(code=code, name=args.name, archive_url=url, member=member,
                           member_bytes=data, vintage=vintage, existing=existing)
    doc["countries"] = [e for e in doc["countries"] if e["code"] != code] + [entry]
    print(json.dumps(entry, ensure_ascii=False))
    print(f"{code}: {n} features, archive {len(archive)} bytes, member {len(data)} bytes, "
          f"dataset_date {pkg.get('dataset_date')}, metadata_modified {pkg.get('metadata_modified')}",
          file=sys.stderr)
    if args.dry_run:
        return 0
    manifest_path.write_text(format_manifest(doc), encoding="utf-8", newline="\n")
    print(f"wrote {manifest_path}", file=sys.stderr)
    return 0


def cmd_verify(args: argparse.Namespace) -> int:
    doc = read_manifest(Path(args.manifest))
    failures = 0
    for entry in doc["countries"]:
        code = entry["code"]
        try:
            if not entry["archive_url"].startswith("https://"):
                raise ManifestError("archive_url is not https")
            body = fetch(entry["archive_url"])
            member = entry.get("member")
            data = extract_member(body, member) if member else body
            got = hashlib.sha256(data).hexdigest()
            if got != entry["member_sha256"]:
                raise ManifestError(f"checksum mismatch: manifest {entry['member_sha256']}, downloaded {got}")
            print(f"OK       {code} {member or entry['archive_url']} {got}")
        except Exception as exc:  # report every country, then fail
            failures += 1
            print(f"FAILED   {code}: {exc}")
    print(f"{len(doc['countries']) - failures}/{len(doc['countries'])} entries verified")
    return 1 if failures else 0


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--manifest", default=str(DEFAULT_MANIFEST), help="manifest path (default: %(default)s)")
    sub = parser.add_subparsers(dest="command", required=True)

    b = sub.add_parser("build", help="build or refresh one country's entry")
    b.add_argument("--iso3", required=True, help="HDX country code, e.g. NGA (dataset cod-ab-nga)")
    b.add_argument("--code", required=True, help="ISO 3166-1 alpha-2 code, e.g. NG")
    b.add_argument("--name", required=True, help="canonical English name, e.g. Nigeria")
    b.add_argument("--work-dir", help="empty directory outside the repo for the download (default: a fresh temp dir)")
    b.add_argument("--dry-run", action="store_true", help="print the entry without writing the manifest")
    b.set_defaults(func=cmd_build)

    v = sub.add_parser("verify", help="re-download every entry and check member_sha256")
    v.set_defaults(func=cmd_verify)

    args = parser.parse_args(argv)
    try:
        return args.func(args)
    except ManifestError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
