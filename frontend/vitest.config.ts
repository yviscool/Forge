import { defineConfig } from 'vitest/config'
import path from 'path'

export default defineConfig({
  resolve: {
    alias: [
      { find: '@', replacement: path.resolve(import.meta.dirname, './src') },
      // Monaco's package entry is an AMD/ESM hybrid that Vitest's Node
      // resolver cannot load. Unit tests never mount a real editor, so point
      // them at a tiny stand-in while the production build keeps the real one.
      {
        find: /^monaco-editor(?:\/.*)?$/,
        replacement: path.resolve(import.meta.dirname, './src/test/monacoMock.ts'),
      },
    ],
  },
  test: {
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
    environment: 'jsdom',
    globals: true,
    setupFiles: './src/test/setup.ts',
    css: false,
    // jsdom 环境创建 + React 渲染在并行 worker 争抢 CPU 的机器上远超默认 5s，
    // 超时抬到 20s 保证 CI 与开发机上的确定性（单独跑单文件不受影响）。
    testTimeout: 20000,
    hookTimeout: 20000,
    // 固定 2 个 worker：覆盖率插桩下 CPU 争抢会造成偶发超时，
    // 有界并发换来可复现的 CI 结果（22 个文件约 1-2 分钟）。
    maxWorkers: 2,
    coverage: {
      provider: 'v8',
      include: ['src/**/*.{ts,tsx}'],
      exclude: [
        'src/**/*.{test,spec}.{ts,tsx}',
        'src/test/**',
        'src/**/*.d.ts',
        // 类型-only 模块：没有可执行语句。
        'src/features/filesubmit/types.ts',
        'src/features/filesubmit/henan/types.ts',
        // 挂载壳与桌面专用外壳：由 Playwright E2E 与桌面构建验证，
        // jsdom 单测无法承载（Wails 桥/路由挂载），不计入单元覆盖面。
        'src/main.tsx',
        'src/App.tsx',
        'src/components/UbuntuDesktop.tsx',
        'src/components/ubuntu/DesktopPreview.tsx',
      ],
      reporter: ['text', 'lcov'],
      thresholds: {
        statements: 55,
        branches: 51,
        functions: 50,
        lines: 58,
      },
    },
  },
})
