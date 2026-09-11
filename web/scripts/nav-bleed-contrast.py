#!/usr/bin/env python3
"""Guard the sticky nav against showing the page content that scrolls under it.

`.nav` is `position: sticky` with a translucent `--bg-nav`, so whatever scrolls
beneath it composites through. Found on staging 2026-09-09: at alpha 0.85 the
"Showing 1-45 of 45 matching events" line was legible across the VigilAfrica
wordmark. No existing gate could see it -- it is not a layout error, an a11y
violation or a failing assertion, just an alpha that lets too much through.

This computes the WCAG contrast between two pixels of the *bar itself*: one
with a bright heading behind it, one with the plain page background behind it.
That ratio is how visible the bleed-through is: 1.46 is what the staging
screenshot showed; MAX_BLEED_CONTRAST below is a heuristic project cutoff, not
a physiological legibility guarantee -- WCAG defines the ratio calculation,
not a threshold below which no human can read text. Treat it as "materially
quieter than the observed defect," not as proof of invisibility.

Pessimistic by construction: it ignores `backdrop-filter: blur()`, which only
ever reduces the peak (it cannot raise a channel above the input extrema), and
there is no `@supports` fallback -- an unsupported browser just keeps the flat
`rgba()` background this script already models, it does not go opaque or
transparent. If this passes, the blurred, supported-browser reality is
quieter still.

Run in CI (see .github/workflows/ci-cd.yml) so a reverted alpha fails the
build rather than silently passing every other check, per this repo's
standing lesson that a check nobody runs is not a check.

Usage:  python web/scripts/nav-bleed-contrast.py
Exits non-zero when the bar leaks.
"""
import pathlib
import re
import sys

MAX_BLEED_CONTRAST = 1.10
MAX_ALIAS_HOPS = 8

TOKENS = pathlib.Path(__file__).resolve().parent.parent / "src" / "styles" / "tokens.css"

# Matches a `--name: value;` declaration whose value is on one line. Comments
# are stripped from the source before this is applied (see _tokens_text), so
# a commented-out declaration can no longer be matched at all -- earlier
# versions of this script anchored `^\s*` to line-start, which does not
# exclude the interior of a `/* ... */` block when the declaration sits on
# its own line inside one, and a dead value would silently outrank the live
# one because re.search returns the first match in document order.
_TOKEN_RE_TEMPLATE = r"^\s*{name}:\s*([^;]+);"


def _tokens_text():
    text = TOKENS.read_text(encoding="utf-8")
    # Strip /* ... */ comments (CSS has no nested or single-line comment
    # syntax) before any token is searched for, so a commented-out
    # declaration can never be selected in place of the live one.
    return re.sub(r"/\*.*?\*/", "", text, flags=re.DOTALL)


def _read_token(name, text=None):
    text = _tokens_text() if text is None else text
    pattern = _TOKEN_RE_TEMPLATE.format(name=re.escape(name))
    matches = list(re.finditer(pattern, text, re.MULTILINE))
    if not matches:
        sys.exit(f"FAIL: {name} not found (or only found inside a comment) in {TOKENS}")
    if len(matches) > 1:
        sys.exit(
            f"FAIL: {name} declared {len(matches)} times in {TOKENS} -- "
            "ambiguous cascade, refusing to guess which wins"
        )
    return matches[0].group(1).strip()


def _rgba(value):
    # CSS `rgb()`/`rgba()` accepts exactly one of two channel grammars: all
    # commas (legacy: `rgba(r, g, b, a)`) or all spaces with a `/` before
    # alpha (modern: `rgb(r g b / a)`). Accepting a mix of the two, as a bare
    # `[,\s]+` separator class does, parses strings the browser rejects --
    # the browser then falls back to `background: transparent` for an
    # invalid `--bg-nav`, which this script would otherwise report as safe.
    legacy = re.fullmatch(
        r"rgba?\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)\s*(?:,\s*([\d.]+)\s*)?\)", value
    )
    modern = re.fullmatch(
        r"rgba?\(\s*([\d.]+)\s+([\d.]+)\s+([\d.]+)\s*(?:/\s*([\d.]+)\s*)?\)", value
    )
    m = legacy or modern
    if not m:
        sys.exit(f"FAIL: {value!r} is not valid rgb()/rgba() (mixed comma/space separators are not valid CSS)")
    r, g, b = (float(m.group(i)) for i in (1, 2, 3))
    a = float(m.group(4)) if m.group(4) else 1.0
    for channel, v in (("r", r), ("g", g), ("b", b)):
        if not 0 <= v <= 255:
            sys.exit(f"FAIL: {value!r} channel {channel}={v} out of range 0-255")
    if not 0 <= a <= 1:
        sys.exit(f"FAIL: {value!r} alpha={a} out of range 0-1")
    return (r, g, b), a


def _hex(value):
    v = value.lstrip("#")
    if len(v) == 3:
        v = "".join(c * 2 for c in v)
    if len(v) not in (6, 8) or not re.fullmatch(r"[0-9a-fA-F]+", v):
        sys.exit(f"FAIL: {value!r} is not a recognised hex colour (3, 6 or 8 digits)")
    return tuple(int(v[i:i + 2], 16) for i in (0, 2, 4))


def _resolve(name):
    """Resolve a token to concrete RGB, following var() aliases to their end.

    tokens.css chains through multiple layers (e.g. --text-primary ->
    --slate-50 -> #f8fafc), so a single hop is not safe -- it works only
    until someone inserts one more layer of indirection, at which point this
    would raise on a plain var() string instead of resolving it. Cycle- and
    depth-guarded rather than assuming the file's current shape holds.
    """
    text = _tokens_text()
    seen = []
    value = _read_token(name, text=text)
    for _ in range(MAX_ALIAS_HOPS):
        m = re.fullmatch(r"var\(\s*(--[\w-]+)\s*\)", value)
        if not m:
            return _hex(value)
        alias = m.group(1)
        if alias in seen:
            sys.exit(f"FAIL: token alias cycle detected: {' -> '.join(seen + [alias])}")
        seen.append(alias)
        value = _read_token(alias, text=text)
    sys.exit(f"FAIL: {name} did not resolve to a concrete colour within {MAX_ALIAS_HOPS} hops")


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
    print(f"bleed-through contrast   : {bleed:.2f} (max {MAX_BLEED_CONTRAST}, a heuristic cutoff -- not a legibility proof)")

    if bleed > MAX_BLEED_CONTRAST:
        print("FAIL: content scrolling under the nav is legible through it.")
        return 1
    print("PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
