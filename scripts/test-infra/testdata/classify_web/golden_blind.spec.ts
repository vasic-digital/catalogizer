import { test } from '@playwright/test';
const name = 'dynamic title';
test(name, async ({ page }) => { await page.goto('/'); });
