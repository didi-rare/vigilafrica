// Does `prefers-reduced-motion: reduce` actually stop the animations that moved
// onto pseudo-elements?
//
// #191 moved the `.signal-dot` pulse onto `::after` and nearly shipped with the
// reduced-motion rule still pointing at the element — reduced-motion users would
// have kept the pulse. chore-web-audit-leftovers does the same move for the
// staging banner's glow, so this reads the COMPUTED animation on each pseudo-
// element under both media states rather than trusting the selector.
//
// Usage (run against a STAGING build, which is the only one with the banner):
//   TARGET_URL=http://localhost:4175/ node scripts/bench-dashboard-cls/check-reduced-motion.mjs
import { chromium } from 'playwright'

const TARGET_URL = process.env.TARGET_URL ?? 'http://localhost:4175/'

const CHECKS = [
  { selector: '.staging-banner', pseudo: '::before', label: 'staging stripe' },
  { selector: '.staging-banner', pseudo: '::after',  label: 'staging glow' },
  { selector: '.signal-dot',     pseudo: '::after',  label: 'signal ring' },
  { selector: '.load-progress',  pseudo: '::after',  label: 'load-progress segment' },
  { selector: '.spinner',        pseudo: null,       label: 'spinner' },
]

const browser = await chromium.launch()
try {
  for (const reducedMotion of ['no-preference', 'reduce']) {
    const page = await browser.newPage({ viewport: { width: 1350, height: 940 }, reducedMotion })
    await page.route('**/v1/**', r => r.fulfill({ json: { data: [], meta: { total: 0, limit: 50, offset: 0 }, states: [], location: null, nearby_events: [] } }))
    await page.route('**/health', r => r.fulfill({ json: { status: 'ok', version: 'bench', last_ingestion: null } }))
    // Hold the chunk so the fallback (progress bar + spinner) is present.
    await page.route('**/assets/EventsDashboard-*.js', () => new Promise(() => {}))
    await page.goto(TARGET_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForSelector('.dashboard-fallback', { timeout: 15000 })

    const rows = await page.evaluate((checks) => checks.map(({ selector, pseudo, label }) => {
      const el = document.querySelector(selector)
      if (!el) return { label, present: false }
      const cs = getComputedStyle(el, pseudo)
      return {
        label, present: true,
        animationName: cs.animationName,
        iterationCount: cs.animationIterationCount,
        duration: cs.animationDuration,
        opacity: cs.opacity,
        transform: cs.transform,
        width: cs.width,
      }
    }), CHECKS)

    console.log(`\nprefers-reduced-motion: ${reducedMotion}`)
    for (const r of rows) {
      console.log(r.present
        ? `  ${r.label.padEnd(24)} animation=${r.animationName} x${r.iterationCount} ${r.duration}  opacity=${r.opacity}  transform=${r.transform}  width=${r.width}`
        : `  ${r.label.padEnd(24)} (not on this page)`)
    }
    await page.close()
  }
} finally {
  await browser.close()
}
