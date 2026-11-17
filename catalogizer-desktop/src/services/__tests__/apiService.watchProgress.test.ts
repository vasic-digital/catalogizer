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

describe('msToSeconds (VLC ms to server seconds)', () => {
  it('converts milliseconds to seconds', () => {
    expect(msToSeconds(125000)).toBe(125)
    expect(msToSeconds(1500)).toBe(1.5)
  })
})
