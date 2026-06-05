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

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), robotsMetaPlugin()],
  build: {
    // MapLibre is intentionally isolated behind lazy boundaries and a separate worker.
    chunkSizeWarningLimit: 1000,
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
