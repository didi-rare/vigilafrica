import { defineConfig, type Plugin } from 'vitest/config'
import react from '@vitejs/plugin-react'

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
