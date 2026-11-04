// LIVE (11.4.289): media playback over the real stream endpoint, with bytes checked against the corpus manifest sha256.
import fs from 'node:fs';
import crypto from 'node:crypto';
import { test, expect, uiAdminLogin, navTo, adminToken, API_URL, shot } from './support';
import { requireEnv } from '../fixtures/env';

type Entry = { size: number; sha256: string; duration_s?: number };
let files: Record<string, Entry> = {};
let auth: Record<string, string> = {};
const ids: Record<string, number> = {};

async function idOf(request: import('@playwright/test').APIRequestContext, rel: string): Promise<number> {
  if (ids[rel]) return ids[rel];
  const title = rel.split('/').pop()!;
  const r = await request.get(`${API_URL}/api/v1/media/search?query=${encodeURIComponent(title)}&limit=50`, { headers: auth });
  const it = ((await r.json()).items as Array<{ id: number; title: string; directory_path: string }>).find((i) => i.directory_path === rel || i.title === title);
  expect(it, `corpus file ${rel} is catalogued`).toBeTruthy();
  return (ids[rel] = it!.id);
}

test.beforeAll(async ({ request }) => {
  files = JSON.parse(fs.readFileSync(requireEnv('LIVE_CORPUS_MANIFEST'), 'utf8')).files;
  auth = { Authorization: `Bearer ${await adminToken(request)}` };
});

const STREAMED = ['movies/Movie.Name.2020.1080p.mp4', 'movies/Another.Movie.2019.vp9.webm', 'music/Misc/Plain Tone.mp3', 'music/Misc/Plain Tone.flac'];

test.describe('stream endpoint (real API)', () => {
  for (const rel of STREAMED) {
    test(`${rel}: full read through /stream/:id is byte-identical to the corpus (sha256) and ranges answer 206`, async ({ request }) => {
      const id = await idOf(request, rel);
      const e = files[rel];
      const full = await request.get(`${API_URL}/api/v1/stream/${id}`, { headers: auth });
      expect(full.status()).toBe(200);
      expect(crypto.createHash('sha256').update(await full.body()).digest('hex'), 'stream bytes == corpus file').toBe(e.sha256);
      const part = await request.get(`${API_URL}/api/v1/stream/${id}`, { headers: { ...auth, Range: 'bytes=0-99' } });
      expect(part.status()).toBe(206);
      expect(part.headers()['content-range']).toBe(`bytes 0-99/${e.size}`);
      expect((await part.body()).length).toBe(100);
      const tail = await request.get(`${API_URL}/api/v1/stream/${id}`, { headers: { ...auth, Range: `bytes=${e.size - 50}-` } });
      expect(tail.status()).toBe(206);
      expect((await tail.body()).length).toBe(50);
      const bad = await request.get(`${API_URL}/api/v1/stream/${id}`, { headers: { ...auth, Range: `bytes=${e.size + 10}-${e.size + 20}` } });
      expect(bad.status(), 'range beyond EOF').toBe(416);
    });
  }

  test('ticket path: POST /stream/:id/ticket then ?ticket= streams byte-identical bytes; the ticket is refused for another media id', async ({ request }) => {
    const rel = STREAMED[0];
    const id = await idOf(request, rel);
    const other = await idOf(request, STREAMED[2]);
    const minted = await request.post(`${API_URL}/api/v1/stream/${id}/ticket`, { headers: auth, data: {} });
    expect(minted.status()).toBe(200);
    const { ticket, expires_in } = await minted.json();
    expect(expires_in).toBeLessThanOrEqual(60);
    const bare = await request.get(`${API_URL}/api/v1/stream/${id}?ticket=${encodeURIComponent(ticket)}`);
    expect(bare.status(), 'ticket needs no Authorization header').toBe(200);
    expect(crypto.createHash('sha256').update(await bare.body()).digest('hex')).toBe(files[rel].sha256);
    const ranged = await request.get(`${API_URL}/api/v1/stream/${id}?ticket=${encodeURIComponent(ticket)}`, { headers: { Range: 'bytes=0-99' } });
    expect(ranged.status()).toBe(206);
    expect((await request.get(`${API_URL}/api/v1/stream/${other}?ticket=${encodeURIComponent(ticket)}`)).status(), 'ticket for another media id').toBe(401);
    expect((await request.get(`${API_URL}/api/v1/media/stats?ticket=${encodeURIComponent(ticket)}`)).status(), 'ticket on another route').toBe(401);
    expect((await request.post(`${API_URL}/api/v1/stream/${id}/ticket`, { data: {} })).status(), 'minting needs auth').toBe(401);
  });

  test('stream requires authentication', async ({ request }) => {
    const id = await idOf(request, STREAMED[0]);
    expect((await request.get(`${API_URL}/api/v1/stream/${id}`)).status()).toBe(401);
  });
});

test.describe('player in the web UI (real browser, real stream)', () => {
  async function playFromUi(page: import('@playwright/test').Page, testInfo: import('@playwright/test').TestInfo, request: import('@playwright/test').APIRequestContext, rel: string) {
    const id = await idOf(request, rel);
    const streamed: string[] = [];
    page.on('response', (r) => { if (r.url().includes('/stream/') || r.url().includes(files[rel] ? rel.split('/').pop()! : '')) streamed.push(`${r.status()} ${r.request().method()} ${r.url().replace(/ticket=[^&]+/, 'ticket=<t>')}`); });
    await uiAdminLogin(page);
    await navTo(page, 'Media');
    await page.locator('input[placeholder*="Search your media collection" i]').fill(rel.split('/').pop()!);
    const card = page.getByTestId(`media-item-${id}`);
    await expect(card).toBeVisible({ timeout: 20_000 });
    await card.hover();
    await page.getByTestId(`play-button-${id}`).click();
    const video = page.locator('video');
    await expect(video).toBeVisible();
    await shot(page, testInfo, 'player-open');
    return { video, streamed, id };
  }

  test('video: play advances currentTime and the element is fed from /api/v1/stream/:id', async ({ page, request }, testInfo) => {
    const { video, streamed, id } = await playFromUi(page, testInfo, request, 'movies/Movie.Name.2020.1080p.mp4');
    await video.evaluate((v: HTMLVideoElement) => { v.muted = true; return v.play().catch(() => undefined); });
    const t1 = await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime), { timeout: 15_000, message: `video never advanced; sources seen: ${JSON.stringify(streamed)}` }).toBeGreaterThan(0.3).then(() => video.evaluate((v: HTMLVideoElement) => v.currentTime));
    await page.waitForTimeout(1000);
    const t2 = await video.evaluate((v: HTMLVideoElement) => v.currentTime);
    expect(t2, 'currentTime keeps advancing').toBeGreaterThan(t1 as number);
    expect(await video.evaluate((v: HTMLVideoElement) => v.error), 'no media error').toBeNull();
    expect(streamed.some((s) => /^20[06] GET /.test(s) && s.includes(`/stream/${id}?ticket=<t>`)), `player streamed /stream/${id}?ticket=; saw ${JSON.stringify(streamed)}`).toBeTruthy();
    expect(streamed.some((s) => /^200 POST /.test(s) && s.includes(`/stream/${id}/ticket`)), 'player minted a ticket with the authenticated client').toBeTruthy();
    expect(streamed.some((s) => s.includes('access_token')), 'session token never in a media URL').toBeFalsy();
    await shot(page, testInfo, 'video-playing');
  });

  test('audio file: the same player plays an mp3 from the real stream', async ({ page, request }, testInfo) => {
    const { video, streamed, id } = await playFromUi(page, testInfo, request, 'music/Misc/Plain Tone.mp3');
    await video.evaluate((v: HTMLVideoElement) => { v.muted = true; return v.play().catch(() => undefined); });
    await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime), { timeout: 15_000, message: `audio never advanced; sources seen: ${JSON.stringify(streamed)}` }).toBeGreaterThan(0.3);
    expect(streamed.some((s) => s.includes(`/stream/${id}`)), `saw ${JSON.stringify(streamed)}`).toBeTruthy();
  });

  test('long playback: plays past the 60 s ticket lifetime on a file the browser cannot buffer fully; a seek far ahead after expiry is network-served and continues from the seek target', async ({ page, request }, testInfo) => {
    test.setTimeout(190_000);
    const rel = 'movies/Big.Seek.Movie.2026.140s.mp4'; // ~200 MB, 140 s (e2e/live/gen_big_fixture.sh); registered in the corpus manifest
    expect(files[rel], `${rel} is in the corpus manifest: run e2e/live/gen_big_fixture.sh <corpus-dir> and rescan`).toBeTruthy();
    const { video, streamed, id } = await playFromUi(page, testInfo, request, rel);
    const t0 = Date.now();
    await video.evaluate((v: HTMLVideoElement) => { v.muted = true; return v.play().catch(() => undefined); });
    await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime), { timeout: 20_000, message: `never advanced; saw ${JSON.stringify(streamed)}` }).toBeGreaterThan(0.3);
    // Let the original ticket (60 s) die while the element keeps playing.
    await page.waitForTimeout(Math.max(0, 66_000 - (Date.now() - t0)));
    expect(await video.evaluate((v: HTMLVideoElement) => v.error), 'no media error while playing past ticket expiry').toBeNull();
    const before = await video.evaluate((v: HTMLVideoElement) => v.currentTime);
    expect(before, 'it kept playing past the ticket lifetime').toBeGreaterThan(55);
    // Browsers buffer a different amount ahead (chromium ~25 s, firefox ~55 s), so the seek target is chosen beyond the
    // buffered end: the target is then NOT buffered by construction and needs a new range request (and a live ticket).
    const buffered = await video.evaluate((v: HTMLVideoElement) => Array.from({ length: v.buffered.length }, (_, i) => [v.buffered.start(i), v.buffered.end(i)]));
    const duration = await video.evaluate((v: HTMLVideoElement) => v.duration);
    const bufEnd = Math.max(...buffered.map(([, b]) => b));
    const TARGET = Math.max(120, Math.ceil(bufEnd) + 5);
    expect(TARGET, `fixture must leave room after the seek (buffered=${JSON.stringify(buffered)}, duration=${duration})`).toBeLessThan(duration - 6);
    expect(buffered.some(([a, b]) => TARGET >= a && TARGET <= b), `target ${TARGET} must not be buffered (buffered=${JSON.stringify(buffered)})`).toBe(false);
    const mintsBefore = streamed.filter((x) => /^200 POST /.test(x) && x.includes(`/stream/${id}/ticket`)).length;
    await video.evaluate((v: HTMLVideoElement, t: number) => { v.currentTime = t; }, TARGET);
    // No loop: a reset to 0 after the refresh would fail here (T1).
    await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime), { timeout: 30_000, message: `after the late seek the position fell back; saw ${JSON.stringify(streamed.slice(-8))}` }).toBeGreaterThanOrEqual(TARGET);
    const seekedAt = await video.evaluate((v: HTMLVideoElement) => v.currentTime);
    expect(seekedAt, 'position after the refresh is at least the pre-refresh position').toBeGreaterThanOrEqual(before);
    await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime), { timeout: 20_000, message: 'currentTime stopped advancing after the late seek' }).toBeGreaterThan(seekedAt + 1);
    expect(await video.evaluate((v: HTMLVideoElement) => v.error), 'no media error after the late seek').toBeNull();
    const mints = streamed.filter((x) => /^200 POST /.test(x) && x.includes(`/stream/${id}/ticket`)).length;
    expect(mints, `the late seek got a fresh ticket (before ${mintsBefore}, after ${mints}); saw ${JSON.stringify(streamed.slice(-12))}`).toBeGreaterThan(mintsBefore);
    expect(streamed.some((x) => x.includes('access_token')), 'session token never in a media URL').toBeFalsy();
    testInfo.annotations.push({ type: 'stream-401s', description: String(streamed.filter((x) => /^401 /.test(x)).length) });
    await shot(page, testInfo, 'long-playback-after-seek');
  });
});
