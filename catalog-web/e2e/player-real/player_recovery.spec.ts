import { test, expect, type Page } from '@playwright/test'
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { startTicketServer } from './ticket_server.mjs'

// Real <video> in chromium and firefox, real MediaPlayer, local ticket server with a short TTL.
// After an expiry-triggered recovery the element must be PLAYING (not paused) and advancing, at or after the seek target.
const API_PORT = Number(process.env.PLAYER_API_PORT || 18932)
const TTL_MS = 8_000
const DURATION_S = 60
let dir: string, file: string
let server: Awaited<ReturnType<typeof startTicketServer>>

test.beforeAll(async () => {
  dir = fs.mkdtempSync(path.join(os.tmpdir(), 'player-real-'))
  file = path.join(dir, 'noisy.mp4')
  execFileSync('ffmpeg', ['-v', 'error', '-f', 'lavfi', '-i', 'testsrc2=size=640x360:rate=25', '-t', String(DURATION_S), '-vf', 'noise=alls=35:allf=t',
    '-c:v', 'libx264', '-preset', 'ultrafast', '-pix_fmt', 'yuv420p', '-b:v', '3M', '-movflags', '+faststart', file])
  server = await startTicketServer({ port: API_PORT, file, ttlMs: TTL_MS, durationS: DURATION_S })
})
test.afterAll(async () => {
  await server?.close()
  if (dir && dir.includes('player-real-')) fs.rmSync(dir, { recursive: true, force: true })
})

const state = (page: Page) => page.evaluate(() => {
  const v = document.querySelector('video')!
  return { t: v.currentTime, paused: v.paused, err: v.error?.code ?? null, dur: v.duration,
    buf: Array.from({ length: v.buffered.length }, (_, i) => [v.buffered.start(i), v.buffered.end(i)]) }
})

async function startPlaying(page: Page) {
  await page.goto('/e2e/player-real/harness.html?id=1')
  const video = page.locator('video')
  await video.waitFor({ state: 'attached' })
  await expect.poll(() => video.getAttribute('src')).toContain('ticket=')
  await video.evaluate((v: HTMLVideoElement) => { v.muted = true; return v.play().catch(() => undefined) })
  await expect.poll(async () => (await state(page)).t, { timeout: 20_000 }).toBeGreaterThan(0.5)
  return video
}

test('a seek past the buffer after ticket expiry recovers PLAYING (not paused) and advances from the seek target', async ({ page }) => {
  server.log.length = 0
  await startPlaying(page)
  await page.waitForTimeout(TTL_MS + 1500) // the ticket in the element URL is now dead
  const s = await state(page)
  const bufEnd = Math.max(0, ...s.buf.map((b) => b[1]))
  const target = Math.max(Math.ceil(bufEnd) + 6, Math.floor(s.dur * 0.75)) // far enough that the browser opens a NEW range request
  expect(target, `target must leave room (buffered ${JSON.stringify(s.buf)})`).toBeLessThan(s.dur - 8)
  const mintsBefore = server.log.filter((e) => e.kind === 'mint').length
  await page.evaluate((t) => { document.querySelector('video')!.currentTime = t }, target)
  await expect.poll(async () => (await state(page)).t, { timeout: 30_000, message: 'never reached the seek target' }).toBeGreaterThanOrEqual(target)
  const at = await state(page)
  await expect.poll(async () => { const x = await state(page); return !x.paused && x.t > at.t + 1 }, { timeout: 20_000, message: `after the recovery the element must be playing and advancing: ${JSON.stringify(await state(page))}` }).toBe(true)
  expect((await state(page)).err).toBeNull()
  expect(server.log.filter((e) => e.kind === 'mint').length, 'a fresh ticket was minted for the late seek').toBeGreaterThan(mintsBefore)
  expect(await page.evaluate(() => (window as any).playerErrors)).toBe(0)
})

test('a cut connection after expiry recovers PLAYING and keeps advancing', async ({ page, request }) => {
  server.log.length = 0
  await startPlaying(page)
  await request.post(`http://127.0.0.1:${API_PORT}/ctl?cut=1`)
  try {
    await page.waitForTimeout(TTL_MS + 1000)
    const before = await state(page)
    // the buffer drains (throttled server, connection cut once the ticket is dead); the element must come back and keep playing
    await expect.poll(async () => server.log.filter((e) => e.kind === 'mint').length, { timeout: 40_000, message: 'no recovery mint after the connection was cut' }).toBeGreaterThan(1)
    await expect.poll(async () => { const x = await state(page); return !x.paused && x.t > before.t + 3 }, { timeout: 40_000, message: `not playing after recovery: ${JSON.stringify(await state(page))}` }).toBe(true)
    expect(await page.evaluate(() => (window as any).playerErrors)).toBe(0)
  } finally {
    await request.post(`http://127.0.0.1:${API_PORT}/ctl?cut=0`)
  }
})
