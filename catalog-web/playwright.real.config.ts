import { defineConfig, devices } from '@playwright/test';

// Live-backend config: no webServer, no mocks; point PLAYWRIGHT_BASE_URL at a running stack.
export default defineConfig({
  testDir: './e2e',
  testMatch: /real-backend-pages\.spec\.ts/,
  timeout: 60_000,
  retries: 0,
  reporter: [['list']],
  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:3000',
    ...devices['Desktop Chrome'],
    // Optional: use an already-installed Chromium when the bundled revision is not downloaded.
    ...(process.env.PW_CHROMIUM_PATH ? { launchOptions: { executablePath: process.env.PW_CHROMIUM_PATH } } : {}),
  },
});
