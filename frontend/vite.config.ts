import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Forge 开发态：产物直出到 Go embed 目录，教师机零 Node 可跑。
export default defineConfig({
  plugins: [vue()],
  base: '/app/',
  build: {
    outDir: '../internal/web/dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
      '/healthz': 'http://localhost:8080',
    },
  },
})
