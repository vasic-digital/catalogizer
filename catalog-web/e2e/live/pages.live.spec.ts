// LIVE (11.4.289): every main page of the SPA renders for a real admin session (no error boundary), real backend, no seeding needed.
import { test, expect, uiAdminLogin, navTo, shot } from './support';

test.describe('pages render for a real admin session', () => {
  for (const [label, url] of [['Collections', /\/collections$/], ['Favorites', /\/favorites$/], ['Admin', /\/admin$/]] as const) {
    test(`${label} page renders without the error boundary`, async ({ page }, testInfo) => {
      await uiAdminLogin(page);
      await navTo(page, label);
      await expect(page).toHaveURL(url);
      await page.waitForTimeout(2500);
      await expect(page.getByText(/Something went wrong in/i)).toHaveCount(0);
      await shot(page, testInfo, `${label.toLowerCase()}-page`);
    });
  }
});
