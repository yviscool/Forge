import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  base: '/app/',
  resolve: { alias: { '@': path.resolve(import.meta.dirname, './src') } },
  server: { port: 5199, strictPort: true, host: '127.0.0.1' },
})
