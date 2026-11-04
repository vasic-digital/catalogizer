import { test } from '@playwright/test';
async function setup(page) { await page.route('**/api/**', (r) => r.fulfill({ status: 200, body: '{}' })); }
test('indirect mock via helper', async ({ page }) => { await setup(page); await page.goto('/'); });
test('also indirect', async ({ page }) => { await setup(page); await page.goto('/y'); });
