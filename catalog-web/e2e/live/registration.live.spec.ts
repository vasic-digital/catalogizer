// LIVE (11.4.289): self-registration with administrator approval (REGISTRATION_MODE=approval, the default) against the real API.
import { test, expect, uiLogin, apiLogin, adminToken, API_URL, randomSecret, shot } from './support';

const field = (name: string) => `input[name="${name}"]`;

test.describe('registration with admin approval (real backend)', () => {
  test('client-side validation: mismatched passwords make no registration request', async ({ page }, testInfo) => {
    const posts: string[] = [];
    page.on('request', (r) => { if (r.url().includes('/auth/register') && r.method() === 'POST') posts.push(r.url()); });
    await page.goto('/register');
    await page.locator(field('firstName')).fill('Live');
    await page.locator(field('lastName')).fill('Tester');
    await page.locator(field('username')).fill(`val_${Date.now()}`);
    await page.locator(field('email')).fill('val@example.com');
    await page.locator(field('password')).fill(randomSecret());
    await page.locator(field('confirmPassword')).fill(randomSecret());
    await page.locator('button[type="submit"]').click();
    await expect(page.getByText('Passwords do not match')).toBeVisible();
    expect(posts).toHaveLength(0);
    await shot(page, testInfo, 'register-validation');
  });

  test('new user cannot log in until the admin approves, then can', async ({ page, request }, testInfo) => {
    const username = `live_${Date.now().toString(36)}_${testInfo.project.name.replace(/[^a-z0-9]/g, '')}`;
    const password = randomSecret();
    let userId: number | undefined;
    const admin = await adminToken(request);
    const auth = { Authorization: `Bearer ${admin}` };
    try {
      // 1. register through the real form
      await page.goto('/register');
      await page.locator(field('firstName')).fill('Live');
      await page.locator(field('lastName')).fill('Tester');
      await page.locator(field('username')).fill(username);
      await page.locator(field('email')).fill(`${username}@example.com`);
      await page.locator(field('password')).fill(password);
      await page.locator(field('confirmPassword')).fill(password);
      const [reg] = await Promise.all([
        page.waitForResponse((r) => r.url().includes('/auth/register') && r.request().method() === 'POST'),
        page.locator('button[type="submit"]').click(),
      ]);
      expect([200, 201], 'real /auth/register status').toContain(reg.status());
      const regBody = await reg.json();
      userId = regBody.user?.id ?? regBody.id;
      expect(userId, 'registered user id returned by the real API').toBeTruthy();
      await page.waitForURL('**/login', { timeout: 15_000 });

      // 2. approval mode: the account exists but is pending -> no session may be issued
      expect(regBody.registration_status, 'registration must report pending in approval mode').toBe('pending');
      const pendingLogin = await apiLogin(request, username, password);
      expect(pendingLogin.status(), 'pending account must not obtain a session').toBe(403);
      expect((await pendingLogin.json()).code).toBe('account_pending_approval');

      // 3. the same through the UI: stays on /login with a visible error
      const uiResp = await uiLogin(page, username, password);
      expect(uiResp.status()).toBe(403);
      await expect(page.locator('[data-testid="login-error"]')).toBeVisible();
      await expect(page).toHaveURL(/\/login$/);
      await shot(page, testInfo, 'pending-login-refused');

      // 4. admin approves through the real API
      const approve = await request.put(`${API_URL}/api/v1/users/${userId}`, { headers: auth, data: { is_active: true } });
      expect(approve.status(), 'real admin approval').toBe(200);

      // 5. now the user can log in through the UI
      await page.goto('/login');
      const ok = await uiLogin(page, username, password);
      expect(ok.status(), 'login after approval').toBe(200);
      await page.waitForURL('**/dashboard', { timeout: 20_000 });
      await shot(page, testInfo, 'approved-login-dashboard');
    } finally {
      if (userId) await request.delete(`${API_URL}/api/v1/users/${userId}`, { headers: auth }).catch(() => undefined);
    }
  });
});
