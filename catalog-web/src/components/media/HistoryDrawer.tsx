import React, { useEffect, useState } from 'react'
import { X } from 'lucide-react'
import { PlaybackFormatter } from '@/lib/playbackFormatter'
import { getEntityProgress, listEntityHistory } from '@/lib/playbackApi'
import type { UiPlaybackProgress, UiPlaybackSession } from '@/types/playback'

interface HistoryDrawerProps {
  mediaItemId: number | null
  mediaTitle: string
  open: boolean
  onClose: () => void
}

/**
 * Reproduction history drawer. Opens from ProgressBadge click, loads
 * fresh data on mount so a play that finished seconds ago is visible.
 * Displays the 5-row aggregate summary followed by per-session rows.
 */
export const HistoryDrawer: React.FC<HistoryDrawerProps> = ({
  mediaItemId,
  mediaTitle,
  open,
  onClose,
}) => {
  const [progress, setProgress] = useState<UiPlaybackProgress | null>(null)
  const [sessions, setSessions] = useState<UiPlaybackSession[]>([])
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    if (!open || mediaItemId == null) return
    let cancelled = false
    setLoaded(false)
    setProgress(null)
    setSessions([])
    Promise.all([
      getEntityProgress(mediaItemId),
      listEntityHistory(mediaItemId, 100),
    ])
      .then(([prog, hist]) => {
        if (cancelled) return
        setProgress(prog)
        setSessions(hist)
        setLoaded(true)
      })
      .catch(() => {
        if (cancelled) return
        setLoaded(true)
      })
    return () => {
      cancelled = true
    }
  }, [open, mediaItemId])

  useEffect(() => {
    if (!open) return
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [open, onClose])

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4">
      <button
        type="button"
        className="absolute inset-0 h-full w-full cursor-default bg-transparent"
        onClick={onClose}
        tabIndex={-1}
        aria-label="Close history"
        data-testid="history-drawer-backdrop"
      />
      <div
        className="relative max-h-[85vh] w-full max-w-2xl overflow-y-auto rounded-xl bg-neutral-900 p-6 text-white shadow-2xl"
        role="dialog"
        aria-modal="true"
        aria-labelledby="history-drawer-title"
      >
        <div className="mb-4 flex items-start justify-between gap-4">
          <div>
            <h2
              id="history-drawer-title"
              className="text-xl font-bold"
            >
              Reproduction History
            </h2>
            <p className="text-sm text-neutral-400">{mediaTitle}</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-md p-1 hover:bg-white/10 focus:outline-none focus:ring-2 focus:ring-white/60"
            aria-label="Close history drawer"
            data-testid="history-drawer-close"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <HistorySummary progress={progress} />

        <div className="mt-4">
          <HistorySessionList sessions={sessions} loaded={loaded} />
        </div>
      </div>
    </div>
  )
}

const HistorySummary: React.FC<{ progress: UiPlaybackProgress | null }> = ({
  progress,
}) => {
  const rows: Array<[string, string]> = progress
    ? [
        [
          'Total duration',
          PlaybackFormatter.formatAmount(
            progress.durationTotal ?? 0,
            progress.positionUnit,
          ),
        ],
        [
          'Current position',
          PlaybackFormatter.formatProgress(
            progress.lastPosition,
            progress.durationTotal,
            progress.positionUnit,
          ),
        ],
        [
          'Last session',
          PlaybackFormatter.formatAmount(
            progress.lastSessionAmount,
            progress.positionUnit,
          ),
        ],
        [
          'Times played',
          PlaybackFormatter.formatReproductionCount(progress.totalReproductions),
        ],
        [
          'Total reproduced',
          PlaybackFormatter.formatAmount(
            progress.aggregateAmount,
            progress.positionUnit,
          ),
        ],
      ]
    : [['Status', 'Never reproduced']]

  return (
    <div className="rounded-lg bg-white/5 p-4">
      {rows.map(([label, value]) => (
        <div
          key={label}
          className="flex items-center justify-between py-1 text-sm"
        >
          <span className="text-neutral-400">{label}</span>
          <span className="font-semibold">{value}</span>
        </div>
      ))}
    </div>
  )
}

const HistorySessionList: React.FC<{
  sessions: UiPlaybackSession[]
  loaded: boolean
}> = ({ sessions, loaded }) => {
  if (!loaded) {
    return <p className="text-sm text-neutral-400">Loading…</p>
  }
  if (sessions.length === 0) {
    return (
      <p className="text-sm text-neutral-400">
        No sessions yet. Press Play to record your first.
      </p>
    )
  }
  return (
    <div>
      <h3 className="mb-2 text-sm font-bold">Sessions</h3>
      <ul className="space-y-2">
        {sessions.map((s) => (
          <li
            key={s.id}
            className="flex items-center gap-3 rounded-md bg-white/5 px-3 py-2 text-sm"
            data-testid={`history-session-${s.id}`}
          >
            <span className="w-40 shrink-0">{formatInstant(s.startedAt)}</span>
            <span className="w-24 shrink-0">
              {PlaybackFormatter.formatAmount(s.totalAmount, s.positionUnit)}
            </span>
            <span
              className={
                s.completed ? 'text-emerald-400' : 'text-amber-400'
              }
            >
              {s.completed ? 'Completed' : 'In progress'}
            </span>
          </li>
        ))}
      </ul>
    </div>
  )
}

function formatInstant(iso: string | null | undefined): string {
  if (!iso) return '-'
  try {
    const d = new Date(iso)
    if (isNaN(d.getTime())) return '-'
    const y = d.getFullYear()
    const m = String(d.getMonth() + 1).padStart(2, '0')
    const day = String(d.getDate()).padStart(2, '0')
    const h = String(d.getHours()).padStart(2, '0')
    const min = String(d.getMinutes()).padStart(2, '0')
    return `${y}-${m}-${day} ${h}:${min}`
  } catch {
    return '-'
  }
}
