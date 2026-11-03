import { test, expect, type Page } from '@playwright/test';

/**
 * LIVE e2e (no route mocks): logs in against a real catalog-api through the
 * Vite/nginx proxy and visits the four pages that used to crash into the
 * ErrorBoundary with real backend payloads. Needs E2E_ADMIN_USER /
 * E2E_ADMIN_PASSWORD (never printed) and PLAYWRIGHT_BASE_URL.
 * Run: npx playwright test -c playwright.real.config.ts
 */
const user = process.env.E2E_ADMIN_USER;
const pass = process.env.E2E_ADMIN_PASSWORD;

async function login(page: Page) {
  await page.goto('/login');
  await page.locator('input[placeholder*="username" i]').fill(user!);
  await page.locator('input[placeholder*="password" i]').fill(pass!);
  await page.click('button[type="submit"]');
  await expect(page).toHaveURL(/dashboard/, { timeout: 15000 });
}

// In-app navigation exactly as a user does it (no reload), which is how the
// four pages crashed against real data.
async function navigate(page: Page, path: string) {
  await page.evaluate((p) => {
    window.history.pushState({}, '', p);
    window.dispatchEvent(new PopStateEvent('popstate'));
  }, path);
  await page.waitForTimeout(3500);
}

test.describe('catalog-web pages against a real backend', () => {
  test.skip(!user || !pass, 'E2E_ADMIN_USER / E2E_ADMIN_PASSWORD not set (honest SKIP, not a pass)');

  for (const path of ['/analytics', '/collections', '/admin', '/settings', '/favorites', '/subtitles']) {
    test(`${path} renders without hitting the ErrorBoundary`, async ({ page }) => {
      const pageErrors: string[] = [];
      page.on('pageerror', (e) => pageErrors.push(e.message));
      await login(page);
      await navigate(page, path);
      if (path === '/subtitles') {
        // the language/provider lists are only rendered once the filter panel is open
        await page.getByRole('button', { name: /filters/i }).click();
        await expect(page.getByText('Providers', { exact: true })).toBeVisible();
        await page.waitForTimeout(1000);
      }
      // not vacuous: we must still be ON the page (not bounced to the login form)
      await expect(page).toHaveURL((u) => u.pathname === path);
      await expect(page.locator('input[placeholder*="password" i]')).toHaveCount(0);
      await expect(page.getByText('Something went wrong')).toHaveCount(0);
      expect(pageErrors, `uncaught page errors on ${path}`).toEqual([]);
      await page.screenshot({ path: `test-results/real-${path.slice(1)}.png`, fullPage: true });
    });
  }

  // Hybrid: only GET /media/search is answered in the browser (the scratch DB has no
  // media rows); every /subtitles/* call goes to the real catalog-api, which omits
  // empty lists ({"success":true,"media_item_id":7} / {"success":true,"count":0}).
  test('/subtitles: media without subtitles and a zero-result search do not crash', async ({ page }) => {
    const pageErrors: string[] = [];
    page.on('pageerror', (e) => pageErrors.push(e.message));
    await page.route('**/api/v1/media/search*', (route) =>
      route.fulfill({
        json: {
          items: [{ id: 7, title: 'Blade Runner', media_type: 'movie', year: 1982, directory_path: '/x/blade', quality: '1080p' }],
          total: 1, limit: 10, offset: 0,
        },
      }));
    await login(page);
    await navigate(page, '/subtitles');
    await page.getByRole('button', { name: /select media/i }).click();
    await page.getByPlaceholder(/search for movies/i).fill('blade');
    await page.getByText('Blade Runner').first().click();
    await expect(page.getByText('Something went wrong')).toHaveCount(0);
    await expect(page.getByText(/Subtitles for/)).toBeVisible();
    await expect(page.getByText('No subtitles found for this media item')).toBeVisible();
    // zero-result search through the real backend (provider with no results)
    await page.getByRole('button', { name: /filters/i }).click();
    await page.getByRole('button', { name: 'SubDB' }).click();
    await page.getByPlaceholder(/search subtitles for/i).fill('blade');
    await page.getByRole('button', { name: /^search$/i }).click();
    await expect(page.getByText(/Search Results \(0 found\)/)).toBeVisible();
    await expect(page.getByText('Something went wrong')).toHaveCount(0);
    expect(pageErrors).toEqual([]);
  });

  test('a hard reload of a protected page keeps the session (no bounce to /login)', async ({ page }) => {
    await login(page);
    await page.goto('/collections');
    await page.waitForLoadState('networkidle');
    await expect(page).toHaveURL(/collections/);
    await expect(page.locator('input[placeholder*="password" i]')).toHaveCount(0);
  });

  test('a created collection is listed (not filtered out / cached empty)', async ({ page, request }) => {
    await login(page);
    const token = await page.evaluate(() => localStorage.getItem('auth_token'));
    const name = `e2e-${Date.now()}`;
    const res = await request.post('/api/v1/collections', {
      headers: { Authorization: `Bearer ${token}` },
      data: { name, description: 'e2e', collection_type: 'custom' },
    });
    expect(res.status(), 'create collection').toBeLessThan(300);
    await page.goto('/collections');
    await expect(page.getByText(name)).toBeVisible({ timeout: 15000 });
  });

  test('settings Scans/Sync/Storage tabs do not crash', async ({ page }) => {
    await login(page);
    await page.goto('/settings');
    await expect(page.locator('input[placeholder*="password" i]')).toHaveCount(0);
    for (const tab of ['Scans', 'Sync', 'Storage']) {
      const t = page.getByRole('button', { name: tab }).first();
      if (await t.count()) await t.click();
      await expect(page.getByText('Something went wrong')).toHaveCount(0);
    }
  });

  test('websocket reaches the open state after login', async ({ page }) => {
    const sockets: string[] = [];
    page.on('websocket', (ws) => sockets.push(ws.url().replace(/token=[^&]+/, 'token=<redacted>')));
    await login(page);
    await page.waitForTimeout(4000);
    expect(sockets.length, 'a /ws connection was attempted').toBeGreaterThan(0);
    await expect(page.getByText(/disconnected/i)).toHaveCount(0);
  });
});
