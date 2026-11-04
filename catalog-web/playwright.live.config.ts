import { defineConfig } from '@playwright/test';

// LIVE config (constitution 11.4.289): no webServer, no mocks, no route interception.
// Point PLAYWRIGHT_BASE_URL at the real catalog-web (nginx SPA + /api,/ws proxy) and LIVE_API_URL at the real catalog-api.
// Credentials come from the environment only (ADMIN_USERNAME / ADMIN_PASSWORD, see e2e/fixtures/env.ts requireEnv).
// Four projects = {chromium, firefox} x {desktop, 390px}. Run one project at a time: ./e2e/live/run_live.sh
const base = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:33020';
const chromiumPath = process.env.PW_CHROMIUM_PATH;
const firefoxPath = process.env.PW_FIREFOX_PATH;
const desktop = { width: 1280, height: 800 };
const phone = { width: 390, height: 844 };

export default defineConfig({
  testDir: './e2e/live',
  testMatch: /.*\.live\.spec\.ts/,
  timeout: 120_000,
  expect: { timeout: 15_000 },
  retries: 0,
  workers: 1, // shared server state + login rate limit: strictly serial
  fullyParallel: false,
  reporter: [['list'], ['json', { outputFile: process.env.LIVE_JSON_REPORT || 'test-results/live-report.json' }]],
  outputDir: process.env.LIVE_OUTPUT_DIR || 'test-results/live',
  use: { baseURL: base, screenshot: 'off', trace: 'off', ignoreHTTPSErrors: false },
  projects: [
    { name: 'chromium-desktop', use: { browserName: 'chromium', viewport: desktop, ...(chromiumPath ? { launchOptions: { executablePath: chromiumPath } } : {}) } },
    { name: 'chromium-390', use: { browserName: 'chromium', viewport: phone, ...(chromiumPath ? { launchOptions: { executablePath: chromiumPath } } : {}) } },
    { name: 'firefox-desktop', use: { browserName: 'firefox', viewport: desktop, ...(firefoxPath ? { launchOptions: { executablePath: firefoxPath } } : {}) } },
    { name: 'firefox-390', use: { browserName: 'firefox', viewport: phone, ...(firefoxPath ? { launchOptions: { executablePath: firefoxPath } } : {}) } },
  ],
});
