import { test, expect } from '@playwright/test';
import { setupServer } from 'msw/node';
const server = setupServer();
test('msw-backed test', async ({ page }) => { await page.goto('/'); });
test('also msw-backed (file-level server)', async ({ page }) => { await page.goto('/y'); });
