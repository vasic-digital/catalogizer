import React from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act, render, waitFor } from '@testing-library/react'
import { invoke } from '@tauri-apps/api/core'

vi.mock('../../services/apiService', async () => {
  const actual = await vi.importActual<any>('../../services/apiService')
  return {
    ...actual,
    apiService: {
      ...actual.apiService,
      updateWatchProgress: vi.fn().mockResolvedValue({ persisted: true, attempts: 1 }),
    },
  }
})

import { apiService } from '../../services/apiService'
import { useVLCPlayer } from '../useVLCPlayer'
import { VLCPlayer } from '../../components/VLCPlayer'

// WPROG3-FU call-site tests: VLC reports milliseconds, the server route takes SECONDS.
const mockInvoke = vi.mocked(invoke)
const put = vi.mocked(apiService.updateWatchProgress)

function vlcStatus(state: string, time: number, duration: number) {
  return { state, position: time / duration, time, duration, volume: 100, isPlaying: state === 'Playing', isMuted: false, rate: 1 }
}

describe('desktop watch progress call sites (seconds, not ms)', () => {
  beforeEach(() => {
    put.mockClear()
    mockInvoke.mockReset()
  })

  it('useVLCPlayer.saveProgress sends position and duration in seconds', async () => {
    mockInvoke.mockImplementation(async (cmd: string) =>
      (cmd === 'vlc_get_status' ? vlcStatus('Paused', 600_000, 7_200_000) : undefined) as any)
    const { result } = renderHook(() => useVLCPlayer())
    await act(async () => { await result.current.refreshStatus() })
    await act(async () => { await result.current.saveProgress(42) })

    expect(put).toHaveBeenCalledTimes(1)
    const [id, body] = put.mock.calls[0] as [number, any]
    expect(id).toBe(42)
    expect(body.position).toBe(600)
    expect(body.duration).toBe(7200)
  })

  it('VLCPlayer Ended path sends the duration in seconds', async () => {
    mockInvoke.mockImplementation(async (cmd: string) =>
      (cmd === 'vlc_get_status' ? vlcStatus('Ended', 7_200_000, 7_200_000) : undefined) as any)
    render(<VLCPlayer mediaUrl="http://x/v.mp4" mediaTitle="t" mediaId={9} onClose={() => {}} />)
    // the hook only fills status from refreshStatus/play; play() refreshes it
    await waitFor(() => expect(put).toHaveBeenCalled(), { timeout: 3000 })
    const body = put.mock.calls[0][1] as any
    expect(body.position).toBe(7200)
    expect(body.duration).toBe(7200)
  })

  it('VLCPlayer Ended path with an unknown duration sends the fraction 1.0', async () => {
    mockInvoke.mockImplementation(async (cmd: string) =>
      (cmd === 'vlc_get_status' ? vlcStatus('Ended', 0, 0) : undefined) as any)
    render(<VLCPlayer mediaUrl="http://x/v.mp4" mediaTitle="t" mediaId={9} onClose={() => {}} />)
    await waitFor(() => expect(put).toHaveBeenCalled(), { timeout: 3000 })
    expect(put.mock.calls[0][1]).toEqual({ progress: 1 })
  })
})
