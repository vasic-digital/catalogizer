// LIVE (11.4.289): the real WebSocket (/ws) and the connection-status chip, against the real server through the real proxy.
import { test, expect, uiAdminLogin, startForwarder, shot } from './support';

const chip = (page: import('@playwright/test').Page) => page.locator('[data-testid="connection-status"]');

test.describe('websocket connection status (real backend)', () => {
  test('no "Disconnected" chip before login (nothing to connect yet)', async ({ page }, testInfo) => {
    await page.goto('/login');
    await page.waitForTimeout(2500);
    await expect(chip(page)).toHaveCount(0);
    await shot(page, testInfo, 'login-no-chip');
  });

  test('after login the real socket opens, stays open, receives no close, and the chip is hidden (= connected)', async ({ page }, testInfo) => {
    const sockets: Array<{ url: string; closed: boolean; frames: number }> = [];
    page.on('websocket', (ws) => {
      const s = { url: ws.url().replace(/token=[^&]+/, 'token=<redacted>'), closed: false, frames: 0 };
      sockets.push(s);
      ws.on('close', () => { s.closed = true; });
      ws.on('framereceived', () => { s.frames++; });
    });
    await uiAdminLogin(page);
    await expect.poll(() => sockets.some((s) => /\/ws\?/.test(s.url) && !s.closed), { timeout: 20_000 }).toBe(true);
    await page.waitForTimeout(4000);
    const live = sockets.filter((s) => !s.closed && /\/ws\?/.test(s.url));
    expect(live.length, 'exactly one live /ws connection after settling').toBe(1);
    await expect(chip(page)).toHaveCount(0);
    await shot(page, testInfo, 'connected-chip-hidden');
  });

  test('network outage shows the chip (Connecting/Disconnected); recovery reconnects a NEW socket and hides it', async ({ browser }, testInfo) => {
    // Real fault injection: the browser talks to the real web server through a real TCP forwarder that we can sever.
    const web = new URL(process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:33020');
    const fwd = await startForwarder(web.hostname, Number(web.port || 80));
    const context = await browser.newContext({ baseURL: fwd.url, viewport: testInfo.project.use.viewport ?? undefined });
    const page = await context.newPage();
    const opened: string[] = [];
    page.on('websocket', (ws) => opened.push(ws.url().replace(/token=[^&]+/, 'token=<redacted>')));
    try {
      await uiAdminLogin(page);
      await expect.poll(() => opened.length, { timeout: 20_000 }).toBeGreaterThan(0);
      await page.waitForTimeout(3000);
      await expect(chip(page)).toHaveCount(0);
      const before = opened.length;
      fwd.cut();
      await expect(chip(page)).toBeVisible({ timeout: 45_000 });
      await expect(chip(page)).toContainText(/connecting|disconnected/i);
      await shot(page, testInfo, 'outage-chip-visible');
      fwd.restore();
      await expect(chip(page)).toHaveCount(0, { timeout: 90_000 });
      expect(opened.length, 'a new real WebSocket was opened after recovery').toBeGreaterThan(before);
      await shot(page, testInfo, 'recovered-chip-hidden');
    } finally {
      await context.close();
      fwd.close();
    }
  });
});
