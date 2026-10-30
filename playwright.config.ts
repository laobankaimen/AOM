import { defineConfig } from '@playwright/test';
export default defineConfig({
  globalSetup: './tests/auth-setup.ts',
  testDir: './tests', testMatch: '**/*.spec.ts', fullyParallel: false, workers: 1,
  timeout: 45000, expect: { timeout: 8000 },
  use: { baseURL: 'http://127.0.0.1:18080', actionTimeout: 10000, headless: true, viewport: { width: 1440, height: 1000 }, screenshot: 'only-on-failure', trace: 'retain-on-failure' },
  reporter: [['list']],
  webServer: { command: 'node scripts/test-server.mjs', url: 'http://127.0.0.1:18080/api/health', timeout: 120000, reuseExistingServer: false },
});
