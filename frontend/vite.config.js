import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// The dev server proxies /api to the Go backend so the browser talks to a
// single origin. That keeps cookies, CORS and WebSocket upgrades behaving
// exactly as they do in production behind Nginx.
const backend = process.env.VITE_BACKEND_ORIGIN || 'http://localhost:8080'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    strictPort: false,
    proxy: {
      '/api': {
        target: backend,
        changeOrigin: true,
        ws: true,
      },
      '/healthz': { target: backend, changeOrigin: true },
      '/readyz': { target: backend, changeOrigin: true },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
    // Google Maps is loaded at runtime from the network, so the bundle stays
    // small; splitting vendor code keeps the app chunk cacheable across deploys.
    //
    // Written as a function rather than the object form: Vite 8 bundles with
    // rolldown, which only accepts the callback signature.
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return undefined
          if (id.includes('@googlemaps')) return 'maps'
          return 'vendor'
        },
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./tests/setup.js'],
    include: ['tests/**/*.spec.js'],
  },
})
