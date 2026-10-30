/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// In dev the Go server runs on :8080; /ws is proxied to it.
// host: true also serves on the LAN so friends on the same network can join
// (Vite prints the "Network:" URL to open).
export default defineConfig({
  plugins: [vue()],
  server: {
    host: true,
    proxy: {
      '/ws': { target: 'ws://localhost:8080', ws: true },
    },
  },
  // Unit tests (npm test). Browser tests live in e2e/ and run with Playwright.
  test: {
    environment: 'happy-dom',
    include: ['src/**/*.test.ts'],
  },
})
