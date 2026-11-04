// LIVE (11.4.289): logout, token revocation, tampered/invalid token, and reload persistence against the real backend.
import { test, expect, uiAdminLogin, headerReady, API_URL, shot } from './support';

async function openUserMenuIfNeeded(page: import('@playwright/test').Page) {
  await headerReady(page);
  const direct = page.getByRole('button', { name: /log ?out/i }).filter({ visible: true }).first();
  if (await direct.isVisible().catch(() => false)) return direct;
  await page.getByRole('button', { name: /open menu/i }).filter({ visible: true }).first().click(); // mobile menu
  return page.getByRole('button', { name: /log ?out/i }).filter({ visible: true }).first();
}

test.describe('session lifecycle (real backend)', () => {
  test('logout: real POST /auth/logout, local session cleared, back on /login, old token revoked server-side', async ({ page, request }, testInfo) => {
    await uiAdminLogin(page);
    const token = (await page.evaluate(() => localStorage.getItem('auth_token')))!;
    const logout = await openUserMenuIfNeeded(page);
    const [resp] = await Promise.all([
      page.waitForResponse((r) => r.url().includes('/auth/logout') && r.request().method() === 'POST'),
      logout.click(),
    ]);
    expect(resp.status(), 'real /auth/logout').toBeLessThan(300);
    await page.waitForURL('**/login', { timeout: 15_000 });
    expect(await page.evaluate(() => localStorage.getItem('auth_token'))).toBeNull();
    const reuse = await request.get(`${API_URL}/api/v1/auth/me`, { headers: { Authorization: `Bearer ${token}` } });
    expect(reuse.status(), 'a logged-out token must be rejected by the real API').toBe(401);
    await shot(page, testInfo, 'after-logout');
  });

  test('tampered token: real API says 401 and the app drops to /login', async ({ page, request }, testInfo) => {
    await uiAdminLogin(page);
    const token = (await page.evaluate(() => localStorage.getItem('auth_token')))!;
    const bad = token.slice(0, -4) + (token.endsWith('AAAA') ? 'BBBB' : 'AAAA');
    const direct = await request.get(`${API_URL}/api/v1/auth/me`, { headers: { Authorization: `Bearer ${bad}` } });
    expect(direct.status(), 'real API must reject a tampered JWT').toBe(401);
    await page.evaluate((t) => localStorage.setItem('auth_token', t), bad);
    await page.reload();
    await page.waitForURL('**/login', { timeout: 20_000 });
    await expect(page.locator('input[placeholder*="username" i]')).toBeVisible();
    await shot(page, testInfo, 'tampered-token-login');
  });

  test('a valid session survives a page reload (stays on /dashboard)', async ({ page }, testInfo) => {
    await uiAdminLogin(page);
    await page.reload();
    await page.waitForTimeout(4000);
    await expect(page).toHaveURL(/\/dashboard$/);
    await expect(page.locator('input[placeholder*="password" i]')).toHaveCount(0);
    await shot(page, testInfo, 'after-reload');
  });

  test('a valid session survives a deep link (hard navigation to /media)', async ({ page }, testInfo) => {
    await uiAdminLogin(page);
    await page.goto('/media');
    await page.waitForTimeout(4000);
    await expect(page).toHaveURL(/\/media$/);
    await shot(page, testInfo, 'deep-link-media');
  });
});
