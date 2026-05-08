import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
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
