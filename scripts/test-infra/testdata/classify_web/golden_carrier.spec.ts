import { test, expect } from '@playwright/test';
// carrier: this comment mentions mockAllEndpoints(page) and page.route('**/api') but is not a mock
/* block comment: await mockAuthEndpoints(page); route.fulfill({}) */
test('real but mentions mocks in strings', async ({ page }) => {
  const note = "we never call mockAllEndpoints(page) or page.route(x) here";
  console.log(note);
  await page.goto('/');
});
