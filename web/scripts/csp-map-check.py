"""Serve the built SPA with the PRODUCTION CSP header and check the map renders.

    python web/scripts/csp-map-check.py web/dist web/vercel.json

Requires: pip install playwright && python -m playwright install chromium
NOT run in CI (needs a browser download); run it whenever maplibre, the worker
wiring, or the CSP header changes.

Why this exists: maplibre-gl v6 dropped the CSP build this app deliberately used,
and the v6 migration broke the map TWICE in ways every other gate passed.
The standard build starts its worker from a blob URL. Our policy grants
`worker-src 'self' blob:`, so it *should* be allowed — but a CSP violation kills
the map at runtime and shows up in NO other gate: type-check, lint, unit tests
and `npm run build` all pass regardless. This loads the real bundle under the
real header in a real browser and fails if the map does not initialise.
"""
import http.server
import json
import re
import socketserver
import sys
import threading
from pathlib import Path

from playwright.sync_api import sync_playwright

DIST = Path(sys.argv[1])
VERCEL = Path(sys.argv[2])

csp = None
cfg = json.loads(VERCEL.read_text(encoding="utf-8"))
for entry in cfg.get("headers", []):
    for h in entry.get("headers", []):
        if h.get("key", "").lower() == "content-security-policy":
            csp = h["value"]
if not csp:
    print("FAIL: no CSP found in vercel.json")
    sys.exit(2)
print("CSP under test:\n  " + csp[:150] + "...\n")


class Handler(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *a, **kw):
        super().__init__(*a, directory=str(DIST), **kw)

    def end_headers(self):
        self.send_header("Content-Security-Policy", csp)
        super().end_headers()

    def do_GET(self):  # SPA fallback
        p = (DIST / self.path.lstrip("/")).resolve()
        if not p.exists() and "." not in Path(self.path).name:
            self.path = "/index.html"
        return super().do_GET()

    def log_message(self, *a):
        pass


with socketserver.TCPServer(("127.0.0.1", 0), Handler) as httpd:
    port = httpd.server_address[1]
    threading.Thread(target=httpd.serve_forever, daemon=True).start()
    url = f"http://127.0.0.1:{port}/"
    print(f"serving {DIST} at {url}")

    violations, errors, worker_urls = [], [], []
    with sync_playwright() as pw:
        browser = pw.chromium.launch(args=["--use-gl=swiftshader", "--enable-unsafe-swiftshader"])
        page = browser.new_page()

        page.on("console", lambda m: (
            violations.append(m.text) if "Content Security Policy" in m.text or "Refused to" in m.text
            else errors.append(m.text) if m.type == "error" else None))
        page.on("pageerror", lambda e: errors.append(str(e)))
        page.on("worker", lambda w: worker_urls.append(w.url))
        failed = []
        page.on("response", lambda r: failed.append((r.status, r.url)) if r.status >= 400 else None)

        page.goto(url, wait_until="networkidle", timeout=60000)
        page.wait_for_timeout(6000)

        canvas = page.locator("canvas.maplibregl-canvas")
        canvas_count = canvas.count()
        container = page.locator(".maplibregl-map").count()
        page.screenshot(path=str(DIST.parent / "csp-map-check.png"), full_page=False)

        browser.close()

print("\n=== RESULT ===")
print(f"workers started            : {len(worker_urls)}  {[u[:40] for u in worker_urls[:3]]}")
print(f"maplibre container present : {container}")
print(f"maplibre canvas present    : {canvas_count}")
print(f"failed requests            : {len(failed)}")
for st, u in failed[:8]:
    print(f"   ! {st}  {u}")
print(f"CSP violations             : {len(violations)}")
for v in violations[:6]:
    print("   ! " + v[:180])
real_errors = [e for e in errors if "WebGL" not in e and "swiftshader" not in e.lower()]
print(f"page errors (non-WebGL)    : {len(real_errors)}")
for e in real_errors[:6]:
    print("   ! " + e[:180])

# ⚠️ A dead worker leaves the container and canvas present, so those alone are
# NOT sufficient. Any failed maplibre asset request must fail this check.
maplibre_404 = [u for _, u in failed if "maplibre" in u.lower()]
print(f"failed maplibre assets     : {len(maplibre_404)}")
for u in maplibre_404:
    print("   ! " + u)
ok = (not violations) and container > 0 and canvas_count > 0 and not maplibre_404
print("\n" + ("PASS: map initialised under the production CSP with no violations"
              if ok else "FAIL: see above"))
sys.exit(0 if ok else 1)
