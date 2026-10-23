import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'
import { mkdirSync, writeFileSync } from 'node:fs'

const embedDist = path.resolve(import.meta.dirname, '../internal/web/dist')

const keepEmbedPlaceholder = {
  name: 'forge-keep-embed-placeholder',
  closeBundle() {
    mkdirSync(embedDist, { recursive: true })
    writeFileSync(
      path.join(embedDist, 'placeholder.txt'),
      'This directory is populated by `npm run build` in frontend/.\n'
    )
  },
}

// Forge 开发态：产物直出到 Go embed 目录，教师机零 Node 可跑。
export default defineConfig({
  plugins: [react(), keepEmbedPlaceholder],
  base: '/app/',
  resolve: {
    alias: {
      '@': path.resolve(import.meta.dirname, './src'),
    },
  },
  build: {
    outDir: embedDist,
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
      '/healthz': 'http://localhost:8080',
      '/app': 'http://localhost:8080',
    },
  },
})
