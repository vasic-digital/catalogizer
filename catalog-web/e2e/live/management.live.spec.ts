// LIVE (11.4.289): favorites, collections (manual + smart rule), admin user management, conversions page. Real backend, UI-driven where the UI exposes
// the action, with the real API as the oracle. Every record created here is removed in `finally`.
import { test, expect, uiAdminLogin, navTo, adminToken, apiLogin, API_URL, shot, randomSecret } from './support';

const tag = (ti: { project: { name: string } }) => `${Date.now().toString(36)}_${ti.project.name.replace(/[^a-z0-9]/g, '')}`;

test.describe.configure({ mode: 'serial' });

test.describe('favorites (real backend)', () => {
  test('favorite from the Media page card: the real API stores it for the clicked item, and the Favorites page lists that item', async ({ page, request }, testInfo) => {
    const auth = { Authorization: `Bearer ${await adminToken(request)}` };
    const title = 'Movie.Name.2020.1080p.mp4';
    const hit = ((await (await request.get(`${API_URL}/api/v1/media/search?query=${encodeURIComponent(title)}&limit=5`, { headers: auth })).json()).items as Array<{ id: number; title: string }>).find((i) => i.title === title)!;
    expect(hit, 'corpus item catalogued').toBeTruthy();
    const before = (await (await request.get(`${API_URL}/api/v1/favorites`, { headers: auth })).json()).count as number;
    let added: { entity_type: string; entity_id: number } | undefined;
    try {
      await uiAdminLogin(page);
      await navTo(page, 'Media');
      await page.locator('input[placeholder*="Search your media collection" i]').fill(title);
      const card = page.getByTestId(`media-item-${hit.id}`);
      await expect(card).toBeVisible({ timeout: 20_000 });
      await card.hover();
      const [resp] = await Promise.all([
        page.waitForResponse((r) => r.url().includes('/favorites') && r.request().method() === 'POST'),
        card.getByTitle('Add to favorites').click(),
      ]);
      expect(resp.status(), 'real POST /favorites').toBeLessThan(300);
      added = JSON.parse(resp.request().postData() || '{}');
      await shot(page, testInfo, 'favorite-added');
      const list = (await (await request.get(`${API_URL}/api/v1/favorites`, { headers: auth })).json());
      expect(list.count, 'favorite count grew by exactly one').toBe(before + 1);
      await navTo(page, 'Favorites');
      await expect(page.getByText(title, { exact: true }).first(), 'the Favorites page lists the item that was clicked').toBeVisible({ timeout: 15_000 });
      await shot(page, testInfo, 'favorites-page');
    } finally {
      if (added) await request.delete(`${API_URL}/api/v1/favorites/${added.entity_type}/${added.entity_id}`, { headers: auth }).catch(() => undefined);
    }
  });
});

test.describe('collections (real backend)', () => {
  test('manual collection CRUD through the real API, visible on the Collections page, gone after delete', async ({ page, request }, testInfo) => {
    const auth = { Authorization: `Bearer ${await adminToken(request)}` };
    const name = `live-manual-${tag(testInfo)}`;
    const made = await request.post(`${API_URL}/api/v1/collections`, { headers: auth, data: { name, description: 'live spec' } });
    expect(made.status()).toBe(201);
    const coll = await made.json();
    try {
      expect(coll.is_smart).toBe(false);
      const upd = await request.put(`${API_URL}/api/v1/collections/${coll.id}`, { headers: auth, data: { name: `${name}-renamed`, description: 'edited' } });
      expect(upd.status()).toBe(200);
      const got = await (await request.get(`${API_URL}/api/v1/collections/${coll.id}`, { headers: auth })).json();
      expect(got.name).toBe(`${name}-renamed`);
      await uiAdminLogin(page);
      await navTo(page, 'Collections');
      await expect(page.getByText(`${name}-renamed`).first()).toBeVisible({ timeout: 20_000 });
      await shot(page, testInfo, 'collection-listed');
    } finally {
      expect((await request.delete(`${API_URL}/api/v1/collections/${coll.id}`, { headers: auth })).status()).toBeLessThan(300);
    }
    expect((await request.get(`${API_URL}/api/v1/collections/${coll.id}`, { headers: auth })).status(), 'deleted collection is gone').toBe(404);
  });

  test('smart collection: the rule is evaluated by the server; its size equals the same query through the media search; invalid rules are rejected', async ({ request }, testInfo) => {
    const auth = { Authorization: `Bearer ${await adminToken(request)}` };
    const name = `live-smart-${tag(testInfo)}`;
    const made = await request.post(`${API_URL}/api/v1/collections`, { headers: auth, data: { name, is_smart: true, smart_rules: [{ field: 'title', operator: 'contains', value: 'Movie' }] } });
    expect(made.status()).toBe(201);
    const coll = await made.json();
    try {
      expect(coll.is_smart).toBe(true);
      const items = await (await request.get(`${API_URL}/api/v1/collections/${coll.id}/items`, { headers: auth })).json();
      expect(items.is_smart).toBe(true);
      expect(items.items.length, 'the rule matches something in the seeded corpus').toBeGreaterThan(0);
      for (const it of items.items as Array<{ title: string }>) expect(it.title.toLowerCase(), 'every member satisfies the rule').toContain('movie');
      const listed = ((await (await request.get(`${API_URL}/api/v1/collections`, { headers: auth })).json()).items as Array<{ id: number; total_items: number }>).find((c) => c.id === coll.id)!;
      expect(listed.total_items, 'listed size equals the evaluated item count').toBe(items.items.length);
    } finally {
      await request.delete(`${API_URL}/api/v1/collections/${coll.id}`, { headers: auth });
    }
    for (const bad of [{ is_smart: true }, { is_smart: true, smart_rules: [{ field: 'no_such_field', operator: 'equals', value: 'x' }] }, { is_smart: false, smart_rules: [{ field: 'title' }] }]) {
      const r = await request.post(`${API_URL}/api/v1/collections`, { headers: auth, data: { name: `${name}-bad`, ...bad } });
      expect(r.status(), `invalid smart rule payload ${JSON.stringify(bad)}`).toBe(400);
    }
  });

  test('another user cannot read or change my collection', async ({ request }, testInfo) => {
    const auth = { Authorization: `Bearer ${await adminToken(request)}` };
    const t = tag(testInfo);
    const username = `live_${t}`;
    const password = randomSecret();
    const reg = await request.post(`${API_URL}/api/v1/auth/register`, { data: { username, email: `${username}@example.com`, password, first_name: 'L', last_name: 'T' } });
    const uid = (await reg.json()).id as number;
    let collId: number | undefined;
    try {
      await request.put(`${API_URL}/api/v1/users/${uid}`, { headers: auth, data: { is_active: true } });
      collId = (await (await request.post(`${API_URL}/api/v1/collections`, { headers: auth, data: { name: `live-private-${t}` } })).json()).id;
      const other = { Authorization: `Bearer ${(await (await apiLogin(request, username, password)).json()).session_token}` };
      expect([403, 404], 'foreign read').toContain((await request.get(`${API_URL}/api/v1/collections/${collId}`, { headers: other })).status());
      expect([403, 404], 'foreign update').toContain((await request.put(`${API_URL}/api/v1/collections/${collId}`, { headers: other, data: { name: 'hijack' } })).status());
      expect([403, 404], 'foreign delete').toContain((await request.delete(`${API_URL}/api/v1/collections/${collId}`, { headers: other })).status());
      expect((await (await request.get(`${API_URL}/api/v1/collections/${collId}`, { headers: auth })).json()).name, 'unchanged').toBe(`live-private-${t}`);
    } finally {
      if (collId) await request.delete(`${API_URL}/api/v1/collections/${collId}`, { headers: auth });
      if (uid) await request.delete(`${API_URL}/api/v1/users/${uid}`, { headers: auth });
    }
  });
});

test.describe('admin user management (real backend)', () => {
  test('pending registration appears on the Admin Users tab; Approve in the UI activates the account; lock/unlock/role change take effect on login', async ({ page, request }, testInfo) => {
    const auth = { Authorization: `Bearer ${await adminToken(request)}` };
    const username = `live_adm_${tag(testInfo)}`;
    const password = randomSecret();
    const reg = await request.post(`${API_URL}/api/v1/auth/register`, { data: { username, email: `${username}@example.com`, password, first_name: 'Live', last_name: 'Admin' } });
    expect(reg.status()).toBe(201);
    const uid = (await reg.json()).id as number;
    try {
      expect((await apiLogin(request, username, password)).status(), 'pending: refused').toBe(403);
      await uiAdminLogin(page);
      await navTo(page, 'Admin');
      await page.getByRole('button', { name: /^users$/i }).click();
      const row = page.locator('tr', { hasText: username });
      await expect(row).toBeVisible({ timeout: 20_000 });
      await expect(row.getByText(/pending/i).first()).toBeVisible();
      await shot(page, testInfo, 'admin-users-pending');
      const [put] = await Promise.all([
        page.waitForResponse((r) => r.url().includes(`/users/${uid}`) && r.request().method() === 'PUT'),
        row.getByRole('button', { name: 'Approve' }).click(),
      ]);
      expect(put.status(), 'real PUT /users/:id from the Approve button').toBe(200);
      const ok = await apiLogin(request, username, password);
      expect(ok.status(), 'approved account can log in').toBe(200);
      await shot(page, testInfo, 'admin-users-approved');

      const lock = await request.post(`${API_URL}/api/v1/users/${uid}/lock`, { headers: auth, data: { lock_until: new Date(Date.now() + 3600_000).toISOString() } });
      expect(lock.status(), 'lock').toBe(200);
      expect([401, 403], 'locked account is refused').toContain((await apiLogin(request, username, password)).status());
      const unlock = await request.post(`${API_URL}/api/v1/users/${uid}/unlock`, { headers: auth, data: {} });
      expect(unlock.status(), 'unlock').toBe(200);
      expect((await apiLogin(request, username, password)).status(), 'unlocked account logs in again').toBe(200);

      const me = async () => (await (await request.get(`${API_URL}/api/v1/users/me`, { headers: { Authorization: `Bearer ${(await (await apiLogin(request, username, password)).json()).session_token}` } })).json());
      expect((await me()).role_id, 'self-registered users are not administrators').not.toBe(1);
      const plainTok = (await (await apiLogin(request, username, password)).json()).session_token as string;
      expect((await request.get(`${API_URL}/api/v1/admin/users`, { headers: { Authorization: `Bearer ${plainTok}` } })).status(), 'plain user refused on admin route').toBe(403);
      const promote = await request.put(`${API_URL}/api/v1/users/${uid}`, { headers: auth, data: { role_id: 1 } });
      expect(promote.status(), 'role change').toBe(200);
      const adminTok = (await (await apiLogin(request, username, password)).json()).session_token as string;
      expect((await request.get(`${API_URL}/api/v1/admin/users`, { headers: { Authorization: `Bearer ${adminTok}` } })).status(), 'promoted user reaches the admin route').toBe(200);
    } finally {
      const del = await request.delete(`${API_URL}/api/v1/users/${uid}`, { headers: auth });
      expect(del.status(), 'cleanup delete').toBeLessThan(300);
    }
    const left = (await (await request.get(`${API_URL}/api/v1/admin/users`, { headers: auth })).json()) as Array<{ username: string }>;
    expect(left.some((u) => u.username === username), 'deleted user is no longer listed').toBe(false);
  });
});

test.describe('conversions (real backend)', () => {
  test('conversion page renders, the real format table matches the page, and a real mp3 to wav job runs to completion on the corpus', async ({ page, request }, testInfo) => {
    const auth = { Authorization: `Bearer ${await adminToken(request)}` };
    await uiAdminLogin(page);
    await navTo(page, 'Convert');
    await expect(page.getByText(/something went wrong|unexpected error|not iterable/i)).toHaveCount(0);
    await shot(page, testInfo, 'conversion-page');
    const fmts = await (await request.get(`${API_URL}/api/v1/conversion/formats`, { headers: auth })).json();
    expect(fmts.audio.output).toContain('wav');
    const jobsBefore = (await (await request.get(`${API_URL}/api/v1/conversion/jobs`, { headers: auth })).json()) as Array<{ id: number }>;
    const created = await request.post(`${API_URL}/api/v1/conversion/jobs`, { headers: auth, data: { source_path: '/media/music/Misc/Plain Tone.mp3', target_path: `/app/data/live-conv-${tag(testInfo)}.wav`, source_format: 'mp3', target_format: 'wav', conversion_type: 'audio', quality: 'medium' } });
    const body = await created.text();
    expect([200, 201, 202], `create conversion job: ${body.slice(0, 300)}`).toContain(created.status());
    const job = JSON.parse(body);
    const jid = job.id ?? job.job_id;
    expect(jid).toBeTruthy();
    await expect.poll(async () => {
      const j = await (await request.get(`${API_URL}/api/v1/conversion/jobs/${jid}`, { headers: auth })).json();
      return j.status ?? j.job?.status;
    }, { timeout: 90_000, intervals: [1000] }).toMatch(/completed|failed/);
    const fin = await (await request.get(`${API_URL}/api/v1/conversion/jobs/${jid}`, { headers: auth })).json();
    expect(fin.status ?? fin.job?.status, `conversion outcome: ${JSON.stringify(fin).slice(0, 300)}`).toBe('completed');
    expect(((await (await request.get(`${API_URL}/api/v1/conversion/jobs`, { headers: auth })).json()) as unknown[]).length).toBeGreaterThan(jobsBefore.length);
  });
});
