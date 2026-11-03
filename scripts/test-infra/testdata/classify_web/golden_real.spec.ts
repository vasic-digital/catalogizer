import { test, expect } from '@playwright/test';
test.describe('golden real', () => {
  test('login against the real stack', async ({ page }) => {
    await page.goto('/login');
    await page.locator('input[type=password]').fill(process.env.ADMIN_PASSWORD ?? '');
    await page.click('button[type="submit"]');
    await expect(page).toHaveURL(/dashboard/);
  });
  test('api direct', async ({ request }) => {
    const r = await request.get('/health');
    expect(r.status()).toBe(200);
  });
});
