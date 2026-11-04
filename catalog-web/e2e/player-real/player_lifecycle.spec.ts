import { test, expect, type Page } from '@playwright/test'
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { startTicketServer } from './ticket_server.mjs'

// WL13 (docs/FOLLOW_UPS_NEXT_PHASE.md): ticket-recovery lifecycle races of the real MediaPlayer in a real <video>
// (chromium + firefox) against the local ticket server. The "ticket is near expiry" condition (45 s client-side age) is produced
// with page.clock.setSystemTime (it moves Date.now() only; the media pipeline and the server keep real time), so no test waits 45 s.
// The server TTL is large so the seek-mint path (handleSeeking), not the 401 error path, is what is exercised.
const API_PORT = Number(process.env.PLAYER_API_PORT || 18932)
const DURATION_S = 60
let dir: string, file: string
let server: Awaited<ReturnType<typeof startTicketServer>>

test.beforeAll(async () => {
  dir = fs.mkdtempSync(path.join(os.tmpdir(), 'player-real-'))
  file = path.join(dir, 'noisy.mp4')
  execFileSync('ffmpeg', ['-v', 'error', '-f', 'lavfi', '-i', 'testsrc2=size=640x360:rate=25', '-t', String(DURATION_S), '-vf', 'noise=alls=35:allf=t',
    '-c:v', 'libx264', '-preset', 'ultrafast', '-pix_fmt', 'yuv420p', '-b:v', '3M', '-movflags', '+faststart', file])
  server = await startTicketServer({ port: API_PORT, file, ttlMs: 300_000, durationS: DURATION_S })
})
test.afterAll(async () => {
  await server?.close()
  if (dir && dir.includes('player-real-')) fs.rmSync(dir, { recursive: true, force: true })
})

const ctl = (page: Page, q: string) => page.request.post(`http://127.0.0.1:${API_PORT}/ctl?${q}`)
const mints = () => server.log.filter((e) => e.kind === 'mint').length
const state = (page: Page) => page.evaluate(() => {
  const v = document.querySelector('video')!
  return { t: v.currentTime, paused: v.paused, ended: v.ended, rs: v.readyState, err: v.error?.code ?? null, dur: v.duration,
    buf: Array.from({ length: v.buffered.length }, (_, i) => [v.buffered.start(i), v.buffered.end(i)]) }
})
const inBuf = (s: { buf: number[][] }, t: number) => s.buf.some((b) => t >= b[0] && t <= b[1])

test.beforeEach(async ({ page }) => { server.log.length = 0; await page.clock.install() })
test.afterEach(async ({ page }) => { await ctl(page, 'getDelay=0&mintDelay=0') })

// The big Play/Pause toggle is the only control rendered with a w-6 h-6 icon (the controls carry no accessible name).
/** Open the player and start playback with the real Play button (a user gesture, like a click in the app). */
async function startPlayingByButton(page: Page) {
  await page.goto('/e2e/player-real/harness.html?id=1')
  const video = page.locator('video')
  await video.waitFor({ state: 'attached' })
  await expect.poll(() => video.getAttribute('src')).toContain('ticket=')
  await video.evaluate((v: HTMLVideoElement) => { v.muted = true })
  await page.locator('button:has(svg.w-6.h-6)').click()
  await expect.poll(async () => (await state(page)).t, { timeout: 20_000 }).toBeGreaterThan(1)
}
/** The ticket in the element URL is now 50 s old for the component (Date.now() only). */
const ageTicket = (page: Page) => page.clock.setSystemTime(new Date(Date.now() + 50_000))
const waitMetadataAfterMint = async (page: Page, mintsBefore: number) => {
  await expect.poll(mints, { timeout: 20_000, message: 'no ticket was minted' }).toBeGreaterThan(mintsBefore)
  await expect.poll(async () => (await state(page)).rs, { timeout: 30_000, message: 'reloaded element never got metadata' }).toBeGreaterThanOrEqual(1)
}

test('WL13-F1: a Pause click during the reload after a seek-mint is honoured (the element stays paused)', async ({ page }) => {
  await startPlayingByButton(page)
  const s0 = await state(page)
  const target = Math.floor(s0.dur * 0.75)
  expect(inBuf(s0, target), `target must be unbuffered ${JSON.stringify(s0.buf)}`).toBe(false)
  await ageTicket(page)
  await ctl(page, 'getDelay=4000') // the reload's response is held: this is the window in which the user clicks Pause
  const m0 = mints()
  await page.evaluate((t) => { document.querySelector('video')!.currentTime = t }, target)
  await expect.poll(mints, { timeout: 20_000, message: 'the near-expiry unbuffered seek did not mint' }).toBeGreaterThan(m0)
  await page.waitForTimeout(500) // setTicketSrc -> v.load(): the element is reloading, no metadata yet
  expect((await state(page)).rs, 'must still be reloading when Pause is clicked').toBe(0)
  // the controls auto-hide 3 s after the last mouse move (the Play click started that timer): bring them back like a user would
  await page.mouse.move(300, 200)
  await page.mouse.move(310, 205)
  await page.locator('button:has(svg.w-6.h-6)').click()
  await waitMetadataAfterMint(page, m0)
  await page.waitForTimeout(2500)
  const end = await state(page)
  expect(end.paused, `the user paused during the reload; the player resumed playing: ${JSON.stringify(end)}`).toBe(true)
  expect(end.t).toBeGreaterThanOrEqual(target - 1)
})

test('WL13-F1 (decision probe): an OS media-key pause during a reload is invisible to the page (already paused, no pause event)', async ({ page }) => {
  await startPlayingByButton(page)
  const s0 = await state(page)
  const target = Math.floor(s0.dur * 0.75)
  await ageTicket(page)
  await ctl(page, 'getDelay=4000')
  const m0 = mints()
  await page.evaluate((t) => { document.querySelector('video')!.currentTime = t }, target)
  await expect.poll(mints, { timeout: 20_000 }).toBeGreaterThan(m0)
  await page.waitForTimeout(500)
  const r = await page.evaluate(() => new Promise<{ pausedBefore: boolean; pauseEvents: number }>((resolve) => {
    const v = document.querySelector('video')!
    let n = 0
    v.addEventListener('pause', () => n++)
    const pausedBefore = v.paused
    v.pause() // what a hardware media key / MediaSession pause does
    setTimeout(() => resolve({ pausedBefore, pauseEvents: n }), 500)
  }))
  // load() already paused the element, so pause() is a no-op: there is NO event the player could honour. The player therefore
  // cannot tell a media-key pause from "still reloading"; this is documented in docs/FOLLOW_UPS_NEXT_PHASE.md (WL13-F1).
  expect(r).toEqual({ pausedBefore: true, pauseEvents: 0 })
})

test('WL13-F2: the LAST seek wins when seek 2 lands in a buffered range while a seek-mint for seek 1 is in flight', async ({ page }) => {
  await startPlayingByButton(page)
  const s0 = await state(page)
  const target = Math.floor(s0.dur * 0.75)
  expect(inBuf(s0, target), `seek 1 target must be unbuffered ${JSON.stringify(s0.buf)}`).toBe(false)
  await ageTicket(page)
  await ctl(page, 'mintDelay=4000') // seek 1's mint stays in flight while seek 2 happens
  const m0 = mints()
  await page.evaluate((t) => { document.querySelector('video')!.currentTime = t }, target) // seek 1 -> refreshTicket(true, target)
  await page.waitForTimeout(400) // seek 1's `seeking` event has been handled (mint requested, held)
  const mid = await state(page)
  const t2 = 1
  expect(inBuf(mid, t2), `seek 2 target must be inside a buffered range ${JSON.stringify(mid.buf)}`).toBe(true)
  await page.evaluate((t) => { document.querySelector('video')!.currentTime = t }, t2) // seek 2: buffered -> handleSeeking returns early
  await waitMetadataAfterMint(page, m0)
  await page.waitForTimeout(1500)
  const end = await state(page)
  expect(end.t, `seek 2 (t=${t2}) was the last seek, the player ended at ${JSON.stringify(end)}`).toBeLessThan(t2 + 10)
})

test('WL13-F3: after the media ended, seeking back to a near-expiry unbuffered target does not auto-play', async ({ page }) => {
  await startPlayingByButton(page)
  const dur = (await state(page)).dur
  await page.evaluate((t) => { document.querySelector('video')!.currentTime = t }, dur - 3) // young ticket: plain seek, no mint
  await expect.poll(async () => (await state(page)).ended, { timeout: 40_000, message: 'the media never ended' }).toBe(true)
  const s1 = await state(page)
  const back = 20
  expect(inBuf(s1, back), `the seek-back target must be unbuffered ${JSON.stringify(s1.buf)}`).toBe(false)
  await ageTicket(page)
  const m0 = mints()
  await page.evaluate((t) => { document.querySelector('video')!.currentTime = t }, back)
  await waitMetadataAfterMint(page, m0)
  await page.waitForTimeout(2500)
  const end = await state(page)
  expect(end.t).toBeGreaterThanOrEqual(back - 1)
  expect(end.paused, `the user had not pressed play after the end; the player auto-played: ${JSON.stringify(end)}`).toBe(true)
  expect(end.t, 'must not have advanced').toBeLessThan(back + 1.5)
})
