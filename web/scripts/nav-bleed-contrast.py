#!/usr/bin/env python3
"""Guard the sticky nav against showing the page content that scrolls under it.

`.nav` is `position: sticky` with a translucent `--bg-nav`, so whatever scrolls
beneath it composites through. Found on staging 2026-09-09: at alpha 0.85 the
"Showing 1-45 of 45 matching events" line was legible across the VigilAfrica
wordmark. No existing gate could see it -- it is not a layout error, an a11y
violation or a failing assertion, just an alpha that lets too much through.

This computes the WCAG contrast between two pixels of the *bar itself*: one
with a bright heading behind it, one with the plain page background behind it.
That ratio is how visible the bleed-through is. Below ~1.10 the eye cannot
resolve text; 1.46 is what the staging screenshot showed.

Pessimistic by construction: it ignores `backdrop-filter: blur()`, which only
ever reduces the peak. If this passes, the blurred reality is quieter still.

Usage:  python web/scripts/nav-bleed-contrast.py
Exits non-zero when the bar leaks.
"""
import pathlib
import re
import sys

MAX_BLEED_CONTRAST = 1.10

TOKENS = pathlib.Path(__file__).resolve().parent.parent / "src" / "styles" / "tokens.css"


def _read_token(name):
    text = TOKENS.read_text(encoding="utf-8")
    m = re.search(rf"^\s*{re.escape(name)}:\s*([^;]+);", text, re.MULTILINE)
    if not m:
        sys.exit(f"FAIL: {name} not found in {TOKENS}")
    return m.group(1).strip()


def _rgba(value):
    m = re.fullmatch(r"rgba?\(\s*([\d.]+)[,\s]+([\d.]+)[,\s]+([\d.]+)(?:[,/\s]+([\d.]+))?\s*\)", value)
    if not m:
        sys.exit(f"FAIL: cannot parse {value!r} as rgba()")
    r, g, b = (float(m.group(i)) for i in (1, 2, 3))
    a = float(m.group(4)) if m.group(4) else 1.0
    return (r, g, b), a


def _hex(value):
    v = value.lstrip("#")
    return tuple(int(v[i:i + 2], 16) for i in (0, 2, 4))


def _resolve(name):
    """Follow one level of var() indirection, which is how tokens.css is built."""
    value = _read_token(name)
    m = re.fullmatch(r"var\(\s*(--[\w-]+)\s*\)", value)
    return _hex(_read_token(m.group(1)) if m else value)


def _linear(c):
    c /= 255.0
    return c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4


def _luminance(rgb):
    r, g, b = (_linear(c) for c in rgb)
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


def _contrast(a, b):
    la, lb = _luminance(a), _luminance(b)
    hi, lo = max(la, lb), min(la, lb)
    return (hi + 0.05) / (lo + 0.05)


def _over(top, alpha, behind):
    return tuple(alpha * top[i] + (1 - alpha) * behind[i] for i in range(3))


def main():
    nav_rgb, alpha = _rgba(_read_token("--bg-nav"))
    text = _resolve("--text-primary")
    page = _resolve("--bg-primary")

    bleed = _contrast(_over(nav_rgb, alpha, text), _over(nav_rgb, alpha, page))

    print(f"--bg-nav alpha           : {alpha}")
    print(f"text behind the bar      : rgb{text}")
    print(f"page behind the bar      : rgb{page}")
    print(f"bleed-through contrast   : {bleed:.2f} (max {MAX_BLEED_CONTRAST})")

    if bleed > MAX_BLEED_CONTRAST:
        print("FAIL: content scrolling under the nav is legible through it.")
        return 1
    print("PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
