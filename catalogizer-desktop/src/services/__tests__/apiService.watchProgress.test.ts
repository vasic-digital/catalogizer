import { describe, it, expect, vi, beforeEach } from 'vitest'
import { invoke } from '@tauri-apps/api/core'
import { apiService, msToSeconds } from '../apiService'

// WPROG3-FU: the server answers PUT /media/:id/progress with persisted:true|false and 409 concurrent_update
// (the Tauri command returns the body text for every status). The client must not claim "saved" without
// persisted:true, must retry a 409, and must send position/duration in seconds (VLC reports milliseconds).
const mockInvoke = vi.mocked(invoke)
const cfg = { server_url: 'http://localhost:8080', auth_token: 't', theme: 'dark', auto_start: false }

function bodies(seq: string[]) {
  let i = 0
  let puts = 0
  mockInvoke.mockImplementation(async (cmd) => {
    if (cmd === 'get_config') return cfg as any
    if (cmd === 'make_http_request') {
      puts++
      return seq[Math.min(i++, seq.length - 1)] as any
    }
    return undefined as any
  })
  return () => puts
}

const body = { media_id: 1, position: 120, duration: 7200, timestamp: 1 }

describe('apiService.updateWatchProgress result (WPROG3-FU-3)', () => {
  beforeEach(() => mockInvoke.mockReset())

  it('reports persisted:true only when the server says so', async () => {
    bodies(['{"success":true,"persisted":true}'])
    const r = await apiService.updateWatchProgress(1, body)
    expect(r.persisted).toBe(true)
  })

  it('does not claim saved for 200 persisted:false and surfaces the reason', async () => {
    const n = bodies(['{"success":true,"persisted":false,"reason":"unknown_duration_no_row"}'])
    const r = await apiService.updateWatchProgress(1, body)
    expect(r.persisted).toBe(false)
    expect(r.reason).toBe('unknown_duration_no_row')
    expect(n()).toBe(1)
  })

  it('does not claim saved when the body carries no persisted flag', async () => {
    bodies(['{}'])
    expect((await apiService.updateWatchProgress(1, body)).persisted).toBe(false)
  })

  it('retries a 409 concurrent_update and reports the eventual success', async () => {
    const n = bodies([
      '{"success":false,"persisted":false,"reason":"concurrent_update"}',
      '{"success":true,"persisted":true}',
    ])
    const r = await apiService.updateWatchProgress(1, body)
    expect(r.persisted).toBe(true)
    expect(n()).toBe(2)
  })

  it('gives up after bounded retries on a persistent 409 and reports not persisted', async () => {
    const n = bodies(['{"success":false,"persisted":false,"reason":"concurrent_update"}'])
    const r = await apiService.updateWatchProgress(1, body)
    expect(r.persisted).toBe(false)
    expect(r.reason).toBe('concurrent_update')
    expect(n()).toBe(3)
  })
})

describe('apiService.updateWatchProgress non-2xx answers (CLPROG-R2-n2)', () => {
  beforeEach(() => mockInvoke.mockReset())

  it('reports the server error text as the reason, not "unknown", for a non-409 error body', async () => {
    bodies(['{"success":false,"error":"invalid duration"}'])
    const r = await apiService.updateWatchProgress(1, body)
    expect(r.persisted).toBe(false)
    expect(r.reason).toBe('invalid duration')
  })

  it('uses a message field when there is no error field', async () => {
    bodies(['{"success":false,"message":"forbidden"}'])
    expect((await apiService.updateWatchProgress(1, body)).reason).toBe('forbidden')
  })

  it('says why the reason is unknown when the body has no flag and no text (status is not exposed by make_http_request)', async () => {
    bodies(['{}'])
    const r = await apiService.updateWatchProgress(1, body)
    expect(r.persisted).toBe(false)
    expect(r.reason).toBe('no_persisted_flag_http_status_not_exposed')
  })

  it('does not retry a non-409 error', async () => {
    const n = bodies(['{"success":false,"error":"nope"}'])
    await apiService.updateWatchProgress(1, body)
    expect(n()).toBe(1)
  })
})

// CLFU-R-1 / CLFU-R-6: the REAL server bodies (catalog-api/handlers/media_handler.go, UpdateWatchProgress) always carry
// "message" next to "reason" / "persisted"; the earlier fixtures omitted it, so a client reading message before reason
// (mutant D1) or reporting a reason on success (mutant D3) was not caught.
const REAL_STORED = '{"success":true,"message":"Watch progress updated","persisted":true}'
const REAL_NOT_STORED_200 = '{"success":true,"message":"Watch progress not stored","persisted":false,"reason":"unknown_duration_no_row"}'
const REAL_409 = '{"success":false,"message":"Watch progress not stored","persisted":false,"reason":"concurrent_update"}'

describe('apiService.updateWatchProgress with the real server body shape (CLFU-R-1, CLFU-R-6)', () => {
  beforeEach(() => mockInvoke.mockReset())

  it('reports the reason field, not the generic message, for a 200 persisted:false', async () => {
    const n = bodies([REAL_NOT_STORED_200])
    const r = await apiService.updateWatchProgress(1, body)
    expect(r.persisted).toBe(false)
    expect(r.reason).toBe('unknown_duration_no_row')
    expect(n()).toBe(1)
  })

  it('treats the real 409 body as concurrent_update: retries 3 times and keeps that reason', async () => {
    const n = bodies([REAL_409])
    const r = await apiService.updateWatchProgress(1, body)
    expect(r.persisted).toBe(false)
    expect(r.reason).toBe('concurrent_update')
    expect(r.attempts).toBe(3)
    expect(n()).toBe(3)
  })

  it('retries the real 409 body once and reports the real stored body as saved, with no reason', async () => {
    const n = bodies([REAL_409, REAL_STORED])
    const r = await apiService.updateWatchProgress(1, body)
    expect(r.persisted).toBe(true)
    expect(r.reason).toBeUndefined()
    expect(r.attempts).toBe(2)
    expect(n()).toBe(2)
  })

  it('sets no reason at all on success, even when the body carries a message (mutant D3)', async () => {
    bodies([REAL_STORED])
    const r = await apiService.updateWatchProgress(1, body)
    expect(r.persisted).toBe(true)
    expect('reason' in r ? r.reason : undefined).toBeUndefined()
  })

  it('prefers reason over error over message when all are present', async () => {
    bodies(['{"success":false,"reason":"r","error":"e","message":"m"}'])
    expect((await apiService.updateWatchProgress(1, body)).reason).toBe('r')
    bodies(['{"success":false,"error":"e","message":"m"}'])
    expect((await apiService.updateWatchProgress(1, body)).reason).toBe('e')
  })
})

describe('msToSeconds (VLC ms to server seconds)', () => {
  it('converts milliseconds to seconds', () => {
    expect(msToSeconds(125000)).toBe(125)
    expect(msToSeconds(1500)).toBe(1.5)
  })
})
