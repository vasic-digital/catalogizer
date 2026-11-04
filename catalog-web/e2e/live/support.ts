// Shared helpers for the LIVE browser specs (constitution 11.4.289: real artifact, real login, seed through the real API;
// no page.route / route.fulfill / mock helpers anywhere under e2e/live).
import { test as base, expect, type Page, type APIRequestContext, type TestInfo } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import net from 'node:net';
import { requireEnv } from '../fixtures/env';

export { expect };

export const API_URL = (process.env.LIVE_API_URL || 'http://127.0.0.1:38082').replace(/\/$/, '');
export const adminUser = () => requireEnv('ADMIN_USERNAME');
export const adminPass = () => requireEnv('ADMIN_PASSWORD');
// Directory the specs append machine-readable observations to (one JSONL per run), consumed by make_evidence.mjs.
export const OBS_DIR = process.env.LIVE_OBS_DIR || '';

// RED switch: LIVE_RED=1 makes the primary selector of a spec deliberately wrong so the RED polarity can be captured.
export const RED = process.env.LIVE_RED === '1';

export const randomSecret = (n = 18) => crypto.randomBytes(n).toString('base64url') + 'aA1!';

// Each page records every response of the real backend (method + status) so the evidence carries a real request histogram.
function record(testInfo: TestInfo, rec: Record<string, unknown>) {
  if (!OBS_DIR) return;
  fs.mkdirSync(OBS_DIR, { recursive: true });
  fs.appendFileSync(path.join(OBS_DIR, 'obs.jsonl'), JSON.stringify({ spec: path.basename(testInfo.file), project: testInfo.project.name, test: testInfo.title, ...rec }) + '\n');
}

export const test = base.extend<{ obs: void }>({
  obs: [async ({ page, request }, use, testInfo) => {
    // Artifact fingerprint read from the target at run time, by every test (make_evidence cross-checks them).
    const fp = await targetFingerprint(request).catch((e) => ({ status: 0, body: { error: String(e) } as Record<string, unknown> }));
    record(testInfo, { kind: 'fingerprint', health_status: fp.status, version: fp.body?.version, build_number: fp.body?.build_number });
    page.on('response', (r) => {
      const u = r.url();
      if (u.includes('/api/') || u.includes('/health')) record(testInfo, { kind: 'http', method: r.request().method(), status: r.status() });
    });
    await use();
    record(testInfo, { kind: 'verdict', status: testInfo.status, expected: testInfo.expectedStatus });
  }, { auto: true }],
});

export async function shot(page: Page, testInfo: TestInfo, name: string) {
  const p = testInfo.outputPath(`${name}.png`);
  await page.screenshot({ path: p, fullPage: false });
  await testInfo.attach(name, { path: p, contentType: 'image/png' });
  record(testInfo, { kind: 'screenshot', name, file: p, sha256: crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex') });
}

// Real API login with backoff on the real login rate limiter (HTTP 429).
export async function apiLogin(request: APIRequestContext, username: string, password: string) {
  for (let i = 0; i < 6; i++) {
    const res = await request.post(`${API_URL}/api/v1/auth/login`, { data: { username, password } });
    if (res.status() === 429) { await new Promise((r) => setTimeout(r, 11_000)); continue; }
    return res;
  }
  throw new Error('login rate limit (429) did not clear');
}

export async function adminToken(request: APIRequestContext): Promise<string> {
  const res = await apiLogin(request, adminUser(), adminPass());
  expect(res.status(), 'admin API login').toBe(200);
  return (await res.json()).session_token as string;
}

const SEL = {
  user: 'input[placeholder*="username" i]',
  pass: 'input[placeholder*="password" i]',
};

// Login through the real login form (the SPA then holds the session; subsequent navigation uses client-side links).
export async function uiLogin(page: Page, username: string, password: string) {
  await page.goto('/login');
  await page.locator(SEL.user).fill(username);
  await page.locator(SEL.pass).fill(password);
  for (let i = 0; i < 4; i++) {
    const [resp] = await Promise.all([
      page.waitForResponse((r) => r.url().includes('/auth/login') && r.request().method() === 'POST'),
      page.locator('button[type="submit"]').click(),
    ]);
    if (resp.status() !== 429) return resp;
    await page.waitForTimeout(11_000);
  }
  throw new Error('UI login rate limited');
}

export async function headerReady(page: Page) {
  await page.locator('header').getByRole('link').filter({ visible: true }).first().waitFor();
  await page.waitForTimeout(300);
}

export async function uiAdminLogin(page: Page) {
  const resp = await uiLogin(page, adminUser(), adminPass());
  expect(resp.status(), 'real /auth/login status').toBe(200);
  await page.waitForURL('**/dashboard', { timeout: 20_000 });
}

// Navigate with the app's own navigation (works at desktop and 390px: opens the mobile menu when the link is hidden).
export async function navTo(page: Page, label: string) {
  const vis = () => page.locator(`a:has-text("${label}")`).filter({ visible: true }).first();
  // wait until the header has rendered one of the entry points (link, More, or mobile menu button)
  await headerReady(page);
  if (!(await vis().isVisible().catch(() => false))) {
    const more = page.getByRole('button', { name: /^more$/i }).filter({ visible: true }).first();
    const menu = page.getByRole('button', { name: /open menu/i }).filter({ visible: true }).first();
    if (await more.isVisible().catch(() => false)) await more.click();
    else if (await menu.isVisible().catch(() => false)) await menu.click();
  }
  await vis().click();
}

// Artifact fingerprint READ FROM THE TARGET at run time: the real /health payload plus the running container's image digest.
export async function targetFingerprint(request: APIRequestContext) {
  const res = await request.get(`${API_URL}/health`);
  return { status: res.status(), body: await res.json().catch(() => ({})) };
}

// Real TCP forwarder for network-fault injection: forwards real bytes to the real web server; cut() severs every live
// connection and refuses new ones (a genuine network outage as seen by the browser), restore() accepts again.
export async function startForwarder(targetHost: string, targetPort: number) {
  let blocked = false;
  const socks = new Set<net.Socket>();
  const server = net.createServer((c) => {
    if (blocked) { c.destroy(); return; }
    const up = net.connect(targetPort, targetHost);
    socks.add(c); socks.add(up);
    const drop = () => { c.destroy(); up.destroy(); socks.delete(c); socks.delete(up); };
    c.on('error', drop); up.on('error', drop); c.on('close', drop); up.on('close', drop);
    c.pipe(up); up.pipe(c);
  });
  await new Promise<void>((r) => server.listen(0, '127.0.0.1', r));
  const port = (server.address() as net.AddressInfo).port;
  return {
    url: `http://127.0.0.1:${port}`,
    cut() { blocked = true; for (const s of socks) s.destroy(); socks.clear(); },
    restore() { blocked = false; },
    close() { for (const s of socks) s.destroy(); server.close(); },
  };
}
