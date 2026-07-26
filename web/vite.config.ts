import { defineConfig, type Plugin } from 'vitest/config'
import react from '@vitejs/plugin-react'

// assertDeploymentEnvVars fails the build if required env vars are unset for
// a non-local deploy. The motivating bug: a Vercel project that forgot
// VITE_API_BASE_URL silently fell back to window.location.origin and shipped
// a broken bundle. chore-post-v11-quality-sweep F3.
//
// chore-analytics-and-feedback extends the same guard to the Umami tracker:
// VITE_ANALYTICS_URL + VITE_ANALYTICS_WEBSITE_ID are substituted into
// index.html at build time, so a staging/production build that forgets them
// would ship a tracker pointing at the literal `%VITE_ANALYTICS_URL%` string.
// Analytics stays optional in local dev (VITE_ENV=local), where the tracker
// simply fails to load.
const REQUIRED_DEPLOY_ENV_VARS: ReadonlyArray<{ name: string; reason: string }> = [
  {
    name: 'VITE_API_BASE_URL',
    reason: 'the frontend cannot infer the API host for this deploy',
  },
  {
    name: 'VITE_ANALYTICS_URL',
    reason: 'the Umami tracker script src is built from it (analytics.vigilafrica.org)',
  },
  {
    name: 'VITE_ANALYTICS_WEBSITE_ID',
    reason: 'the Umami tracker needs the website-id to attribute pageviews',
  },
]

function assertDeploymentEnvVars(): void {
  const env = process.env.VITE_ENV
  if (env !== 'staging' && env !== 'production') return

  for (const { name, reason } of REQUIRED_DEPLOY_ENV_VARS) {
    const value = process.env[name]
    if (!value || value.trim() === '') {
      throw new Error(
        `vite: ${name} must be set when VITE_ENV=${env} — ${reason}. ` +
          `Set it in the Vercel project's environment variables.`,
      )
    }
  }
}
assertDeploymentEnvVars()

// HTML transform: flip the `robots` meta value at build time based on VITE_ENV.
// Production keeps `index, follow`; staging emits `noindex, nofollow` so search
// engines don't index pre-release builds alongside production. Without this,
// the static `<meta name="robots">` in index.html would be identical for both
// environments and staging would compete with production in search results.
function robotsMetaPlugin(): Plugin {
  return {
    name: 'vigilafrica-robots-meta',
    transformIndexHtml(html) {
      const isStaging = process.env.VITE_ENV === 'staging'
      const robotsValue = isStaging ? 'noindex, nofollow' : 'index, follow'
      return html.replace(
        /<meta\s+name="robots"\s+content="[^"]*"\s*\/?>/,
        `<meta name="robots" content="${robotsValue}" />`,
      )
    },
  }
}

// Emit robots.txt + sitemap.xml into the build output. These cannot live in
// web/public/ as static files: that copies the same bytes into every deploy,
// so the staging build would ship an indexable robots.txt that contradicts the
// `noindex, nofollow` meta tag robotsMetaPlugin writes one function above.
//
// Both files 404'd in production until now, which is why Search Console had no
// crawl map for the site at all.
//
// The staging/production split mirrors robotsMetaPlugin deliberately: a build
// with VITE_ENV unset is treated as production and gets the permissive
// robots.txt. Defaulting the other way would mean a misconfigured production
// deploy silently ships `Disallow: /` and deindexes the whole site — a far
// more expensive failure than a stray crawl of a preview build, which the
// meta tag still covers.
const SITE_ORIGIN = 'https://vigilafrica.org'

// Only stable, canonical routes belong here. /events/:id is deliberately
// excluded — those IDs come from upstream NASA EONET feeds and expire, so
// listing them would fill the sitemap with URLs that 404 within days.
const SITEMAP_ROUTES: ReadonlyArray<string> = ['/']

function seoFilesPlugin(): Plugin {
  return {
    name: 'vigilafrica-seo-files',
    apply: 'build',
    generateBundle() {
      const isStaging = process.env.VITE_ENV === 'staging'

      const robotsTxt = isStaging
        ? ['User-agent: *', 'Disallow: /', ''].join('\n')
        : [
            'User-agent: *',
            'Allow: /',
            '',
            `Sitemap: ${SITE_ORIGIN}/sitemap.xml`,
            '',
          ].join('\n')

      this.emitFile({ type: 'asset', fileName: 'robots.txt', source: robotsTxt })

      // A staging sitemap would either advertise production URLs from a
      // noindex host or list staging URLs nothing should crawl. Skip it.
      if (isStaging) return

      const lastmod = new Date().toISOString().slice(0, 10)
      const urls = SITEMAP_ROUTES.map(
        (route) =>
          `  <url>\n    <loc>${SITE_ORIGIN}${route}</loc>\n    <lastmod>${lastmod}</lastmod>\n  </url>`,
      ).join('\n')

      this.emitFile({
        type: 'asset',
        fileName: 'sitemap.xml',
        source: `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${urls}\n</urlset>\n`,
      })

      // llms.txt — guidance for LLM agents and AI crawlers, the same way
      // robots.txt addresses search crawlers. Lighthouse 13.3.0's "Agentic
      // Browsing" category checks for it; we scored 2/2 on the scored audits but
      // the llms.txt check reported NOT APPLICABLE because no file existed.
      //
      // Emitted after the `isStaging` return above, so it is production-only for
      // the same reason the sitemap is: a staging copy would either advertise
      // production URLs from a noindex host or point agents at a preview build.
      //
      // Deliberately links to the canonical API contract rather than restating
      // endpoints here — duplicating it would drift from
      // openspec/specs/vigilafrica/openapi.yaml, which is the source of truth.
      // Must contain at least one H1 to satisfy the audit.
      const llmsTxt = [
        '# VigilAfrica',
        '',
        'Open-source natural-event awareness for Africa. VigilAfrica translates raw',
        'NASA satellite event data into local context — floods and wildfires located by',
        'country and state rather than by bare coordinates. Nigeria and Ghana are live.',
        '',
        '## Important limitation',
        '',
        'VigilAfrica is an awareness and situational-context tool, not an emergency',
        'service or an authoritative alerting system. Event data is upstream-sourced and',
        'may be incomplete, delayed, or superseded. Do not present it as a basis for',
        'life-safety decisions; direct people to their local authorities for',
        'confirmation and for guidance on any active hazard.',
        '',
        '## Data source',
        '',
        'NASA EONET (Earth Observatory Natural Event Tracker). Locations are enriched',
        'server-side to country and admin-1 (state) names.',
        '',
        '## Key pages',
        '',
        `- ${SITE_ORIGIN}/ — event map and current feed`,
        `- ${SITE_ORIGIN}/for-partners — for NGOs, newsrooms and response organisations`,
        '',
        '## For developers and agents',
        '',
        '- Source and issues: https://github.com/didi-rare/vigilafrica',
        '- Public API host: https://api.vigilafrica.org',
        '- API contract: openspec/specs/vigilafrica/openapi.yaml in the repository',
        '',
        'Event identifiers originate from upstream feeds and expire, so individual',
        'event URLs are intentionally excluded from the sitemap and should not be',
        'treated as stable or archival links.',
        '',
      ].join('\n')

      this.emitFile({ type: 'asset', fileName: 'llms.txt', source: llmsTxt })
    },
  }
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), robotsMetaPlugin(), seoFilesPlugin()],
  build: {
    // MapLibre is intentionally isolated behind lazy boundaries and a separate worker.
    chunkSizeWarningLimit: 1000,
    // Emit source maps. Lighthouse reports "Large JavaScript file is missing a
    // source map" for map-vendor, and without them a production stack trace is
    // unreadable. `true` rather than 'hidden': hidden maps omit the
    // `sourceMappingURL` comment, so neither devtools nor Lighthouse can find
    // them, which defeats the point. This project is open-source under a public
    // repo, so published maps disclose nothing that is not already readable.
    sourcemap: true,
    rollupOptions: {
      output: {
        manualChunks(id) {
          const normalizedId = id.replace(/\\/g, '/')

          if (normalizedId.includes('/node_modules/maplibre-gl/')) {
            return 'map-vendor'
          }

          if (
            normalizedId.includes('/node_modules/react/') ||
            normalizedId.includes('/node_modules/react-dom/') ||
            normalizedId.includes('/node_modules/react-router-dom/') ||
            normalizedId.includes('/node_modules/@tanstack/react-query/')
          ) {
            return 'react-vendor'
          }
        },
      },
    },
  },
  server: {
    proxy: {
      '/v1': 'http://127.0.0.1:8080',
      '/health': 'http://127.0.0.1:8080',
    },
  },
  test: {
    environment: 'jsdom',
    environmentOptions: {
      jsdom: {
        url: 'https://vigil.test/',
      },
    },
    setupFiles: ['./src/setupTests.ts'],
    css: true,
  },
})
