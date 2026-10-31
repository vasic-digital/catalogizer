import { Page } from '@playwright/test';
import { requireEnv } from './env';

// Required: no literal fallback. Fails the run, naming the variable, when unset.
const ADMIN_PASSWORD = requireEnv('ADMIN_PASSWORD');

export const testUser = {
  username: 'admin',
  password: ADMIN_PASSWORD,
  role: 'admin',
};

export const adminUser = {
  username: 'admin',
  password: ADMIN_PASSWORD,
  role: 'admin',
};

/**
 * Mock all authentication-related API endpoints.
 */
export async function mockAuthEndpoints(page: Page) {
  // Init status
  await page.route('**/api/v1/auth/init-status', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ initialized: true, admin_exists: true }),
    });
  });

  // Login
  await page.route('**/api/v1/auth/login', async (route) => {
    let postData: { username?: string; password?: string } = {};
    try {
      postData = JSON.parse(route.request().postData() || '{}');
    } catch {
      postData = {};
    }

    if (
      (postData.username === testUser.username && postData.password === testUser.password) ||
      (postData.username === adminUser.username && postData.password === adminUser.password)
    ) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          token: 'mock-jwt-token-' + Date.now(),
          refresh_token: 'mock-refresh-token-' + Date.now(),
          user: {
            id: postData.username === adminUser.username ? 1 : 2,
            username: postData.username,
            role: postData.username === adminUser.username ? 'admin' : 'user',
          },
        }),
      });
    } else {
      await route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'Invalid username or password' }),
      });
    }
  });

  // Auth status check
  await page.route('**/api/v1/auth/status', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        authenticated: true,
        user: { id: 1, username: testUser.username, role: 'admin' },
      }),
    });
  });

  // Token refresh
  await page.route('**/api/v1/auth/refresh', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        token: 'mock-refreshed-jwt-token-' + Date.now(),
        refresh_token: 'mock-refreshed-refresh-token-' + Date.now(),
      }),
    });
  });

  // Register
  await page.route('**/api/v1/auth/register', async (route) => {
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({
        message: 'User created successfully',
        user: { id: 3, username: 'newuser', role: 'user' },
      }),
    });
  });

  // Logout
  await page.route('**/api/v1/auth/logout', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ message: 'Logged out successfully' }),
    });
  });

  // Users list (admin)
  await page.route('**/api/v1/auth/users**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        users: [
          { id: 1, username: 'admin', role: 'admin', created_at: new Date().toISOString() },
          { id: 2, username: 'user', role: 'user', created_at: new Date().toISOString() },
        ],
        total: 2,
      }),
    });
  });
}

/**
 * Login as a specific user. Navigates to /login, fills credentials, submits,
 * and waits for redirect to dashboard.
 */
export async function loginAs(page: Page, user: { username: string; password: string }) {
  await page.goto('/login');
  await page.waitForLoadState('networkidle');
  await page.locator('input[placeholder*="username" i]').fill(user.username);
  await page.locator('input[placeholder*="password" i]').fill(user.password);
  await page.click('button[type="submit"]');
  await page.waitForURL(/.*dashboard/, { timeout: 10000 });
}

/**
 * Log out the current user by clicking the logout button.
 */
export async function logout(page: Page) {
  const logoutButton = page.locator(
    'button:has-text("Logout"), button:has-text("Sign out")'
  );
  if (await logoutButton.first().isVisible({ timeout: 3000 })) {
    await logoutButton.first().click();
    await page.waitForTimeout(1000);
  }
}
