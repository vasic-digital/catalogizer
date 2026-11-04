import { test } from '@playwright/test';
import { mockAllEndpoints } from '../../../../catalog-web/e2e/fixtures/api-mocks';
test.describe('mocked group', () => {
  test.beforeEach(async ({ page }) => { await mockAllEndpoints(page); });
  test('inside the hook scope', async ({ page }) => { await page.goto('/a'); });
});
test.describe('real group', () => {
  test('outside the hook scope', async ({ page }) => { await page.goto('/b'); });
});
