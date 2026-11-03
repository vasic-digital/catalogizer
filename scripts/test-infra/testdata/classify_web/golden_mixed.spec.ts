import { test, expect } from '@playwright/test';
test.describe('golden mixed', () => {
  test('real one', async ({ page }) => { await page.goto('/real'); });
  test('intercepted one', async ({ page }) => {
    await page.route('**/api/v1/media/search*', (route) => route.fulfill({ json: { items: [] } }));
    await page.goto('/subtitles');
  });
});
