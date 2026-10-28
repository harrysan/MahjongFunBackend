import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// In dev the Go server runs on :8080; /ws is proxied to it.
export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/ws': { target: 'ws://localhost:8080', ws: true },
    },
  },
})
