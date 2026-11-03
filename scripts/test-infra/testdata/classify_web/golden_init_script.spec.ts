import { test } from '@playwright/test';
test('init script replaces fetch', async ({ page }) => {
  await page.addInitScript(() => { window.fetch = async () => new Response('{}'); });
  await page.goto('/');
});
test('plain navigation', async ({ page }) => { await page.goto('/real'); });
