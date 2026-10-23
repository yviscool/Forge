import { defineConfig, devices } from '@playwright/test'
import path from 'node:path'

const port = 18080
const baseURL = `http://127.0.0.1:${port}`

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  // 教师台 DOM 很大，firefox/webkit 上逐 tab 的 axe 全页扫描可能超过 45s。
  timeout: 90_000,
  expect: {
    timeout: 8_000,
  },
  reporter: [
    ['list'],
    ['html', { open: 'never', outputFolder: 'playwright-report' }],
  ],
  use: {
    baseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  webServer: {
    command: 'go run ./cmd/forge',
    cwd: path.resolve(import.meta.dirname, '..'),
    url: `${baseURL}/healthz`,
    timeout: 120_000,
    reuseExistingServer: !process.env.CI,
    env: {
      FORGE_ADDR: `127.0.0.1:${port}`,
      FORGE_DATA_DIR: './data/e2e',
      FORGE_STORE: 'memory',
      FORGE_ADMIN_PASSWORD: 'E2E-Admin-Password-2026!',
      FORGE_AUTOJUDGE: 'false',
    },
  },
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
      },
    },
    {
      name: 'firefox',
      use: {
        ...devices['Desktop Firefox'],
      },
    },
    {
      name: 'webkit',
      use: {
        ...devices['Desktop Safari'],
      },
    },
  ],
})
