import { defineConfig } from '@playwright/test';

// Real-browser (chromium + firefox) tests of the real MediaPlayer against a local ticket server. Not live evidence (no real API):
// they exist because jsdom models the media element wrongly (e.g. Chromium sets paused=true before it fires the error event).
const PORT = Number(process.env.PLAYER_WEB_PORT || 18933);
const API_PORT = Number(process.env.PLAYER_API_PORT || 18932);
const chromiumPath = process.env.PW_CHROMIUM_PATH;
const firefoxPath = process.env.PW_FIREFOX_PATH;

export default defineConfig({
  testDir: './e2e/player-real',
  testMatch: /.*\.spec\.ts/,
  timeout: 120_000,
  expect: { timeout: 20_000 },
  retries: 0,
  workers: 1,
  fullyParallel: false,
  reporter: [['list']],
  outputDir: process.env.PLAYER_OUTPUT_DIR || 'test-results/player-real',
  use: { baseURL: `http://127.0.0.1:${PORT}` },
  webServer: {
    command: `VITE_API_BASE_URL=http://127.0.0.1:${API_PORT} npx vite --port ${PORT} --strictPort --host 127.0.0.1`,
    url: `http://127.0.0.1:${PORT}/e2e/player-real/harness.html`,
    reuseExistingServer: false,
    timeout: 120_000,
  },
  projects: [
    { name: 'chromium', use: { browserName: 'chromium', ...(chromiumPath ? { launchOptions: { executablePath: chromiumPath } } : {}) } },
    { name: 'firefox', use: { browserName: 'firefox', ...(firefoxPath ? { launchOptions: { executablePath: firefoxPath } } : {}) } },
  ],
});
