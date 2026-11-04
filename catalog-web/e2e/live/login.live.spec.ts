// LIVE (11.4.289): real login form -> real catalog-api. No mocks, no route interception.
import { test, expect, uiLogin, uiAdminLogin, adminUser, randomSecret, shot, RED } from './support';

// RED switch (LIVE_RED=1): the error locator is deliberately wrong so the spec fails for the right reason on a working page.
const loginError = (page: import('@playwright/test').Page) =>
  page.locator(RED ? '[data-testid="login-error-WRONG-SELECTOR"]' : '[data-testid="login-error"]');

test.describe('login (real backend)', () => {
  test('valid admin credentials: real 200, dashboard, token stored and accepted by /auth/me', async ({ page, request }, testInfo) => {
    await uiAdminLogin(page);
    await expect(page).toHaveURL(/\/dashboard$/);
    const token = await page.evaluate(() => localStorage.getItem('auth_token'));
    expect(token, 'session token persisted by the SPA').toBeTruthy();
    const me = await request.get(`${new URL(page.url()).origin}/api/v1/auth/me`, { headers: { Authorization: `Bearer ${token}` } });
    expect(me.status(), 'real /auth/me with the stored token').toBe(200);
    const body = await me.json();
    expect(JSON.stringify(body)).toContain(adminUser());
    await expect(page.locator('main, [role="main"]').first()).toContainText(/dashboard|media|total/i);
    await shot(page, testInfo, 'login-success-dashboard');
  });

  test('wrong password: real 401, stays on /login, visible error, no session stored', async ({ page }, testInfo) => {
    const resp = await uiLogin(page, adminUser(), randomSecret());
    expect(resp.status(), 'real /auth/login for a wrong password').toBe(401);
    await expect(loginError(page)).toBeVisible();
    await expect(loginError(page)).toContainText(/invalid|incorrect|credentials/i);
    await expect(page).toHaveURL(/\/login$/);
    expect(await page.evaluate(() => localStorage.getItem('auth_token'))).toBeNull();
    await shot(page, testInfo, 'login-failure-visible-error');
  });

  test('unknown user: rejected with the same 401 and no user enumeration in the message', async ({ page }) => {
    const resp = await uiLogin(page, `nouser_${Date.now()}`, randomSecret());
    expect(resp.status()).toBe(401);
    await expect(page.locator('[data-testid="login-error"]')).toBeVisible();
    const msg = (await page.locator('[data-testid="login-error"]').innerText()).toLowerCase();
    expect(msg).not.toMatch(/no such user|not found|does not exist|unknown user/);
  });
});
