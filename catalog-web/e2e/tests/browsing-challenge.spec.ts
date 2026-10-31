import { test, expect, Page } from '@playwright/test';
import { requireEnv } from '../fixtures/env';

const username = process.env.ADMIN_USERNAME || 'admin';
const password = requireEnv('ADMIN_PASSWORD');

/**
 * Browsing Challenge E2E Tests
 *
 * These tests run against a REAL backend (no mocks).
 * They validate that cataloged content is properly browsable
 * through the web application after the First Catalog suite.
 */

async function loginAsAdmin(page: Page) {
  await page.goto('/login');
  await page.waitForLoadState('networkidle');

  await page.locator('input[placeholder*="username" i]').fill(username);
  await page.locator('input[placeholder*="password" i]').fill(password);
  await page.click('button[type="submit"]');

  await page.waitForURL('**/dashboard', { timeout: 15000 });
}

test.describe('Browsing Challenge', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page);
  });

  test('dashboard loads with real data', async ({ page }) => {
    // Dashboard should be loaded (we're already here from login)
    await expect(page).toHaveURL(/.*dashboard/);

    // Page should have meaningful content, not just loading spinners
    await page.waitForLoadState('networkidle');

    // Check that the page has rendered content (not stuck on loading)
    const body = await page.textContent('body');
    expect(body).toBeTruthy();
    expect(body!.length).toBeGreaterThan(50);

    // Should not have perpetual loading indicators after network idle
    const spinners = page.locator('[class*="spinner" i], [class*="loading" i], [role="progressbar"]');
    const spinnerCount = await spinners.count();
    // Allow some spinners for lazy-loaded sections, but main content should be loaded
    expect(spinnerCount).toBeLessThan(5);
  });

  test('media browser shows actual content', async ({ page }) => {
    // Navigate to media/catalog section
    const mediaLink = page.locator('a[href*="media"], a[href*="catalog"], a[href*="browse"]').first();
    if (await mediaLink.isVisible({ timeout: 5000 }).catch(() => false)) {
      await mediaLink.click();
      await page.waitForLoadState('networkidle');

      // Should show some content items
      const content = await page.textContent('body');
      expect(content).toBeTruthy();
      expect(content!.length).toBeGreaterThan(100);
    } else {
      // Try direct navigation
      await page.goto('/catalog');
      await page.waitForLoadState('networkidle');

      const content = await page.textContent('body');
      expect(content).toBeTruthy();
    }
  });

  test('navigation links are present and functional', async ({ page }) => {
    // Check that navigation elements exist
    const nav = page.locator('nav, [role="navigation"], aside');
    await expect(nav.first()).toBeVisible({ timeout: 5000 });

    // Should have multiple navigation links
    const links = nav.first().locator('a');
    const linkCount = await links.count();
    expect(linkCount).toBeGreaterThan(0);

    // Click the first available nav link and verify page doesn't break
    if (linkCount > 0) {
      const firstLink = links.first();
      const href = await firstLink.getAttribute('href');
      if (href && !href.startsWith('http')) {
        await firstLink.click();
        await page.waitForLoadState('networkidle');
        // Page should still have content
        const body = await page.textContent('body');
        expect(body).toBeTruthy();
      }
    }
  });

  test('no console errors on pages', async ({ page }) => {
    const errors: string[] = [];

    page.on('console', (msg) => {
      if (msg.type() === 'error') {
        errors.push(msg.text());
      }
    });

    // Visit dashboard
    await page.goto('/dashboard');
    await page.waitForLoadState('networkidle');

    // Filter out known acceptable errors (e.g., favicon, third-party)
    const criticalErrors = errors.filter(
      (e) =>
        !e.includes('favicon') &&
        !e.includes('404') &&
        !e.includes('Failed to load resource') &&
        !e.includes('net::ERR')
    );

    expect(criticalErrors).toHaveLength(0);
  });

  test('no placeholder or unknown titles in media browser', async ({ page }) => {
    const invalidPatterns = ['unknown', 'untitled', 'placeholder', 'n/a', 'tbd'];

    // Navigate to catalog/media section
    await page.goto('/catalog');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    const content = await page.textContent('body');
    if (content) {
      const lowerContent = content.toLowerCase();
      for (const pattern of invalidPatterns) {
        // Check for standalone occurrences (not part of other words)
        const regex = new RegExp(`\\b${pattern}\\b`, 'i'); // eslint-disable-line security/detect-non-literal-regexp
        // This is a soft check - the content might legitimately contain these words
        // in different contexts, so we just log if found
        if (regex.test(lowerContent)) {
          // Check if it's in a title/heading context
          const headings = await page.locator('h1, h2, h3, h4, h5, h6, [class*="title" i]').allTextContents();
          const invalidHeadings = headings.filter((h) =>
            new RegExp(`^\\s*${pattern}\\s*$`, 'i').test(h) // eslint-disable-line security/detect-non-literal-regexp
          );
          expect(invalidHeadings).toHaveLength(0);
        }
      }
    }
  });
});
