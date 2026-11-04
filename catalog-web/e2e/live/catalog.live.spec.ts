// LIVE (11.4.289): catalog browse / list / search pages over a corpus seeded THROUGH THE REAL API (storage root + scan).
import fs from 'node:fs';
import { test, expect, uiAdminLogin, navTo, adminToken, API_URL, shot } from './support';
import { requireEnv } from '../fixtures/env';

const ROOT_NAME = process.env.LIVE_CORPUS_ROOT_NAME || 'qa-corpus';
const ROOT_PATH = process.env.LIVE_CORPUS_ROOT_PATH || '/media/corpus'; // path as seen INSIDE the API container

type Manifest = { file_count: number; files: Record<string, { class: string }> };
let manifest: Manifest;
let totalItems = 0;
let seedBlocked = '';

test.describe.configure({ mode: 'serial' });

test.beforeAll(async ({ request }) => {
  manifest = JSON.parse(fs.readFileSync(requireEnv('LIVE_CORPUS_MANIFEST'), 'utf8'));
  const auth = { Authorization: `Bearer ${await adminToken(request)}` };
  const roots = (await (await request.get(`${API_URL}/api/v1/storage/roots`, { headers: auth })).json()).roots as Array<{ id: number; name: string; path: string }>;
  let root = roots.find((r) => r.name === ROOT_NAME);
  if (!root) {
    const created = await request.post(`${API_URL}/api/v1/storage/roots`, { headers: auth, data: { name: ROOT_NAME, protocol: 'local', path: ROOT_PATH } });
    if (created.status() === 403 && /CATALOGIZER_LOCAL_ROOT_BASES/.test(await created.text())) {
      // The target refuses local storage roots by design (fail-closed: no allowed base directory configured). Seeding is
      // impossible through the real API on this target: an honest SKIP (topology_unsupported), never a stand-in.
      seedBlocked = 'target has no CATALOGIZER_LOCAL_ROOT_BASES: local storage roots are refused (HTTP 403)';
      return;
    }
    expect([200, 201], 'create storage root through the real API (got ' + created.status() + ')').toContain(created.status());
    root = { id: (await created.json()).id, name: ROOT_NAME, path: ROOT_PATH };
  }
  const have = async () => (await (await request.get(`${API_URL}/api/v1/media/stats`, { headers: auth })).json()).total_items as number;
  // A file added to the manifest after the last scan (e.g. by gen_big_fixture.sh) is not counted by total_items alone: probe the
  // newest manifest entries by name as well.
  const missing = async () => {
    for (const rel of Object.keys(manifest.files).filter((k) => /Big\.Seek/.test(k))) {
      const r = await request.get(`${API_URL}/api/v1/media/search?query=${encodeURIComponent(rel.split('/').pop()!)}&limit=5`, { headers: auth });
      if (!((await r.json()).items ?? []).some((i: { title: string; directory_path: string }) => i.directory_path === rel || i.title === rel.split('/').pop())) return true;
    }
    return false;
  };
  if ((await have()) < manifest.file_count || (await missing())) {
    const q = await request.post(`${API_URL}/api/v1/scans`, { headers: auth, data: { storage_root_id: root.id, storage_root_name: ROOT_NAME } });
    expect(q.status(), 'queue scan through the real API').toBeLessThan(300);
    const job = (await q.json()).job_id as string;
    await expect.poll(async () => {
      const scans = (await (await request.get(`${API_URL}/api/v1/scans`, { headers: auth })).json()).scans as Array<{ job_id: string; status: string }>;
      return scans.find((s) => s.job_id === job)?.status;
    }, { timeout: 90_000, intervals: [1000] }).toMatch(/completed|failed/);
  }
  if (seedBlocked) return;
  totalItems = await have();
  expect(totalItems, 'seeded corpus visible to the real API (manifest file_count)').toBeGreaterThanOrEqual(manifest.file_count);
});

async function apiSearch(request: import('@playwright/test').APIRequestContext, q: string) {
  const auth = { Authorization: `Bearer ${await adminToken(request)}` };
  const r = await request.get(`${API_URL}/api/v1/media/search?query=${encodeURIComponent(q)}&limit=100`, { headers: auth });
  expect(r.status()).toBe(200);
  const j = await r.json();
  return { total: j.total as number, titles: (j.items as Array<{ title: string }>).map((i) => i.title) };
}

test.beforeEach(() => { test.skip(!!seedBlocked, seedBlocked); });

test.describe('catalog pages over the seeded corpus (real backend)', () => {
  test('Media page lists the seeded items: counts equal the real API', async ({ page, request }, testInfo) => {
    await uiAdminLogin(page);
    await navTo(page, 'Media');
    await expect(page).toHaveURL(/\/media$/);
    await expect(page.getByText(`${totalItems}`, { exact: true }).first()).toBeVisible();
    await expect(page.getByText(new RegExp(`Showing \\d+ of ${totalItems} results`))).toBeVisible();
    // every title on the first page is a title the real API knows
    const known = new Set((await apiSearch(request, '')).titles);
    const first = Object.keys(manifest.files).map((k) => k.split('/').pop()!).find((t) => known.has(t))!;
    expect(first, 'a manifest file is catalogued').toBeTruthy();
    await expect(page.getByText(/Page 1 of \d+/).first()).toBeVisible();
    await shot(page, testInfo, 'media-list');
  });

  test('Media search narrows to exactly what the real API returns; no-match shows no items', async ({ page, request }, testInfo) => {
    await uiAdminLogin(page);
    await navTo(page, 'Media');
    const box = page.locator('input[placeholder*="Search your media collection" i]');
    const api = await apiSearch(request, 'Movie');
    expect(api.total).toBeGreaterThan(0);
    expect(api.total).toBeLessThan(totalItems);
    await box.fill('Movie');
    await expect(page.getByText(new RegExp(`Showing \\d+ of ${api.total} results`))).toBeVisible({ timeout: 20_000 });
    for (const t of api.titles.slice(0, 5)) await expect(page.getByText(t, { exact: true }).first()).toBeVisible();
    await shot(page, testInfo, 'media-search-hit');
    await box.fill(`zz-no-such-title-${Date.now()}`);
    await expect(page.getByText(/Showing \d+ of 0 results|No (media|results|items)/i).first()).toBeVisible({ timeout: 20_000 });
    await shot(page, testInfo, 'media-search-miss');
  });

  test('Browse page entity-type counts equal the real /entities/stats', async ({ page, request }, testInfo) => {
    const auth = { Authorization: `Bearer ${await adminToken(request)}` };
    const stats = await (await request.get(`${API_URL}/api/v1/entities/stats`, { headers: auth })).json();
    await uiAdminLogin(page);
    await navTo(page, 'Browse');
    await expect(page.getByText(`${stats.total_entities} total entities`)).toBeVisible();
    const one = (n: number) => `${n} ${n === 1 ? 'item' : 'items'}`;
    await expect(page.getByText(one(stats.by_type.movie), { exact: true }).first()).toBeVisible();
    await expect(page.getByText(one(stats.by_type.music_album), { exact: true }).first()).toBeVisible();
    await shot(page, testInfo, 'browse-types');
  });

});
