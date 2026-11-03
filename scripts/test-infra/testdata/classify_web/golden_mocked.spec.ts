import { test, expect } from '@playwright/test';
import { mockAllEndpoints } from '../../../../catalog-web/e2e/fixtures/api-mocks';
test.describe('golden mocked', () => {
  test.beforeEach(async ({ page }) => { await mockAllEndpoints(page); });
  test('a', async ({ page }) => { await page.goto('/'); await expect(page).toHaveURL(/./); });
  test('b', async ({ page }) => { await page.goto('/x'); });
});
