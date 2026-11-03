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

  test('stream requires authentication', async ({ request }) => {
    const id = await idOf(request, STREAMED[0]);
    expect((await request.get(`${API_URL}/api/v1/stream/${id}`)).status()).toBe(401);
  });
});

test.describe('player in the web UI (real browser, real stream)', () => {
  async function playFromUi(page: import('@playwright/test').Page, testInfo: import('@playwright/test').TestInfo, request: import('@playwright/test').APIRequestContext, rel: string) {
    const id = await idOf(request, rel);
    const streamed: string[] = [];
    page.on('response', (r) => { if (r.url().includes('/stream/') || r.url().includes(files[rel] ? rel.split('/').pop()! : '')) streamed.push(`${r.status()} ${r.url()}`); });
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
    expect(streamed.some((s) => s.includes(`/stream/${id}`) && /^(200|206) /.test(s)), `player requested /stream/${id}; saw ${JSON.stringify(streamed)}`).toBeTruthy();
    await shot(page, testInfo, 'video-playing');
  });

  test('audio file: the same player plays an mp3 from the real stream', async ({ page, request }, testInfo) => {
    const { video, streamed, id } = await playFromUi(page, testInfo, request, 'music/Misc/Plain Tone.mp3');
    await video.evaluate((v: HTMLVideoElement) => { v.muted = true; return v.play().catch(() => undefined); });
    await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime), { timeout: 15_000, message: `audio never advanced; sources seen: ${JSON.stringify(streamed)}` }).toBeGreaterThan(0.3);
    expect(streamed.some((s) => s.includes(`/stream/${id}`)), `saw ${JSON.stringify(streamed)}`).toBeTruthy();
  });
});
