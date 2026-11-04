import { useState, useRef, useEffect, useCallback } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import {
  Play,
  Pause,
  Volume2,
  VolumeX,
  Maximize2,
  Subtitles,
  Settings,
  SkipForward,
  SkipBack
} from 'lucide-react'
import { api } from '@/lib/api'
import type { MediaItem, MediaEntity } from '@/types/media'
import type { SubtitleTrack } from '@/types/subtitles'

interface MediaPlayerProps {
  media: MediaItem
  entity?: MediaEntity
  entityStreamUrl?: string
  siblings?: MediaEntity[]
  subtitles?: SubtitleTrack[]
  onProgress?: (currentTime: number, duration: number) => void
  onEnded?: () => void
  onError?: (error: Error) => void
  onNavigate?: (entityId: number) => void
}

const API_BASE = (import.meta.env.VITE_API_BASE_URL || '') + '/api/v1'
const TICKET_NEAR_EXPIRY_MS = 45_000 // tickets live 60 s server-side; a seek needing the network after this age gets a fresh one first
const MAX_CONSECUTIVE_RECOVERIES = 3
// Real progress means advancing this far past the resume point: the resume itself fires timeupdate at the floor, and a source that
// fails every fraction of a second must not keep earning a fresh budget.
const PROGRESS_STEP_S = 2
const MAX_TOTAL_RECOVERIES = 100 // hard bound per source (generous: a long film recovers a few times per hour)

/**
 * A <video>/<audio> element cannot send an Authorization header, and the session JWT must not go into the URL (logs, history,
 * Referer). The player therefore mints a short-lived stream ticket with the authenticated API client and uses ?ticket= for this
 * media id only. Range/seek requests re-use the ticket within its lifetime; a timer refreshes it and a media error mints a new one.
 */
export const mintStreamTicket = async (id: number): Promise<string> => {
  const res = await api.post(`/stream/${id}/ticket`, {})
  return res.data.ticket as string
}
const streamUrl = (id: number, ticket: string) => `${API_BASE}/stream/${id}?ticket=${encodeURIComponent(ticket)}`

/**
 * MediaPlayer renders an HTML5 video/audio player with transport controls,
 * volume, subtitles, fullscreen, and sibling navigation support.
 */
export const MediaPlayer: React.FC<MediaPlayerProps> = ({
  media,
  entity,
  entityStreamUrl,
  siblings = [],
  subtitles = [],
  onProgress,
  onEnded,
  onError,
  onNavigate,
}) => {
  const [isPlaying, setIsPlaying] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [volume, setVolume] = useState(1)
  const [isMuted, setIsMuted] = useState(false)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [selectedSubtitle, setSelectedSubtitle] = useState<string>('')
  const [showSubtitles, setShowSubtitles] = useState(false)
  const [showControls, setShowControls] = useState(true)
  
  const videoRef = useRef<HTMLVideoElement>(null)
  const playerRef = useRef<HTMLDivElement>(null)
  const controlsTimeoutRef = useRef<NodeJS.Timeout>()

  // Source: an explicit entity stream URL wins; otherwise the real range-capable stream endpoint of the catalogued item, authorised
  // by a short-lived ticket (see mintStreamTicket). directory_path is a storage-relative path, never a playable URL.
  const [ticketSrc, setTicketSrc] = useState('')
  const [authError, setAuthError] = useState(false)
  // Ticket lifecycle (all in refs: none of it drives rendering).
  //  - resumeRef: where to continue after the source is re-pointed. It is written only while the element is NOT mid-reload, so an
  //    error that lands before metadata cannot overwrite the real position with 0/paused (the last write used to win).
  //  - errorTriesRef: consecutive recoveries WITHOUT real progress. Reset only when currentTime advances past the resume point
  //    (never on loadedmetadata: a corrupt file fires loadedmetadata and then errors again on every reload).
  //  - mintedAtRef: age of the ticket currently in the URL; no timer ever re-points a playing element (that dropped its buffer).
  const resumeRef = useRef<{ time: number; play: boolean } | null>(null)
  const reloadingRef = useRef(false)
  const errorTriesRef = useRef(0)
  const totalRecoveriesRef = useRef(0)
  const progressFloorRef = useRef(0)
  const mintedAtRef = useRef(0)
  // Generation of the media id currently shown, and the generation that has a mint in flight (-1 = none). Every mint is tagged
  // with its generation and a stale result is dropped, so switching items while a mint is pending can never play the old item.
  const genRef = useRef(0)
  const mediaIdRef = useRef(media.id)
  const inflightRef = useRef(-1)
  // Latest seek target requested while a mint is in flight: the user's last seek wins.
  const seekDuringMintRef = useRef<number | null>(null)
  // Play intent, driven by user actions and play/pause events NOT caused by an error or a reload. Chromium sets paused=true
  // before it fires the error event, so reading !paused at recovery time would resume a playing video as paused.
  const wantPlayRef = useRef(false)
  const videoSrc = entityStreamUrl || ticketSrc

  /** Mint a new ticket and re-point the element at it. `at` overrides the resume position (a seek target). */
  const refreshTicket = useCallback(async (resume: boolean, at?: number) => {
    const gen = genRef.current
    const id = media.id
    if (inflightRef.current === gen) {
      if (resume && at !== undefined) seekDuringMintRef.current = at
      return
    }
    inflightRef.current = gen
    seekDuringMintRef.current = resume && at !== undefined ? at : null
    try {
      const url = streamUrl(id, await mintStreamTicket(id))
      if (gen !== genRef.current || id !== mediaIdRef.current) return // the item changed while minting: drop the stale ticket
      const v = videoRef.current
      if (resume && v && !reloadingRef.current) {
        const time = seekDuringMintRef.current ?? v.currentTime
        resumeRef.current = { time, play: wantPlayRef.current }
        progressFloorRef.current = time
      }
      mintedAtRef.current = Date.now()
      setTicketSrc(url)
      setAuthError(false)
    } catch {
      if (gen === genRef.current) setAuthError(true)
    } finally {
      if (inflightRef.current === gen) inflightRef.current = -1
      if (gen === genRef.current) seekDuringMintRef.current = null
    }
  }, [media.id])

  useEffect(() => {
    if (entityStreamUrl) return
    genRef.current += 1
    mediaIdRef.current = media.id
    inflightRef.current = -1
    seekDuringMintRef.current = null
    resumeRef.current = null
    reloadingRef.current = false
    errorTriesRef.current = 0
    totalRecoveriesRef.current = 0
    progressFloorRef.current = 0
    mintedAtRef.current = 0
    setTicketSrc('') // never keep playing the previous item while the new ticket is minted
    void refreshTicket(false)
  }, [media.id, entityStreamUrl, refreshTicket])

  // A changed source does not reload an element by itself. The single loadedmetadata listener (events effect below) restores
  // position and play state once the new source is ready.
  useEffect(() => {
    const v = videoRef.current
    if (!v || !ticketSrc) return
    reloadingRef.current = true
    v.load()
  }, [ticketSrc])

  // Format time display
  const formatTime = (time: number) => {
    const minutes = Math.floor(time / 60)
    const seconds = Math.floor(time % 60)
    return `${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`
  }

  // Handle play/pause
  const togglePlay = useCallback(() => {
    if (!videoRef.current) return

    if (isPlaying) {
      wantPlayRef.current = false
      videoRef.current.pause()
    } else {
      wantPlayRef.current = true
      videoRef.current.play()
    }
    setIsPlaying(!isPlaying)
  }, [isPlaying])

  // Handle volume change
  const handleVolumeChange = useCallback((newVolume: number) => {
    if (!videoRef.current) return

    videoRef.current.volume = newVolume
    setVolume(newVolume)
    setIsMuted(newVolume === 0)
  }, [])

  // Handle mute toggle
  const toggleMute = useCallback(() => {
    if (!videoRef.current) return

    videoRef.current.muted = !isMuted
    setIsMuted(!isMuted)
  }, [isMuted])

  // Handle fullscreen toggle
  const toggleFullscreen = useCallback(() => {
    if (!playerRef.current) return

    if (!isFullscreen) {
      if (playerRef.current.requestFullscreen) {
        playerRef.current.requestFullscreen()
      }
    } else {
      if (document.exitFullscreen) {
        document.exitFullscreen()
      }
    }
    setIsFullscreen(!isFullscreen)
  }, [isFullscreen])

  // Handle seek
  const handleSeek = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
    if (!videoRef.current) return

    const newTime = parseFloat(event.target.value)
    videoRef.current.currentTime = newTime
    setCurrentTime(newTime)
  }, [])

  // Skip forward/backward
  const skip = useCallback((seconds: number) => {
    if (!videoRef.current) return

    videoRef.current.currentTime += seconds
  }, [])

  // Show/hide controls
  const handleMouseMove = useCallback(() => {
    setShowControls(true)

    if (controlsTimeoutRef.current) {
      clearTimeout(controlsTimeoutRef.current)
    }

    controlsTimeoutRef.current = setTimeout(() => {
      setShowControls(false)
    }, 3000)
  }, [])

  // Video event handlers
  useEffect(() => {
    const video = videoRef.current
    if (!video) return

    const handleTimeUpdate = () => {
      setCurrentTime(video.currentTime)
      onProgress?.(video.currentTime, video.duration)
    }

    const handleLoadedMetadata = () => {
      setDuration(video.duration)
      reloadingRef.current = false
      const r = resumeRef.current
      resumeRef.current = null
      if (r) {
        // A position the user set while the source was reloading (the element holds it as its start position) beats the stored one.
        if (!(video.currentTime > 0.5)) video.currentTime = r.time
        if (r.play) void video.play().catch(() => undefined)
      }
    }

    const handlePlay = () => { wantPlayRef.current = true }
    const handlePause = () => {
      // Chromium pauses the element when it errors; a pause caused by an error, a reload or the end is not the user's intent.
      if (video.error || reloadingRef.current || video.ended) return
      wantPlayRef.current = false
    }

    // Real progress is the only thing that restores the recovery budget.
    const handleProgress = () => {
      if (!video.paused && video.currentTime > progressFloorRef.current + PROGRESS_STEP_S) errorTriesRef.current = 0
    }

    // A seek that needs the network after the ticket is (nearly) dead gets a fresh ticket first, resuming at the seek target.
    // Seeks inside already-buffered ranges need no request and are left alone.
    const handleSeeking = () => {
      if (entityStreamUrl) return
      if (reloadingRef.current) return // a position set during a reload is kept by loadedmetadata (see there)
      if (!mintedAtRef.current) return
      if (Date.now() - mintedAtRef.current < TICKET_NEAR_EXPIRY_MS) return
      const t = video.currentTime
      for (let i = 0; i < video.buffered.length; i++) {
        if (t >= video.buffered.start(i) && t <= video.buffered.end(i)) return
      }
      void refreshTicket(true, t)
    }

    const handleEnded = () => {
      setIsPlaying(false)
      onEnded?.()
    }

    const handleError = () => {
      // An expired ticket makes the next range request 401, which the element reports as an error: mint a new one and resume
      // (bounded, so a genuinely broken file does not loop).
      // A decode error (code 3) is the file, not the ticket: retrying only loops. Everything else (401 on a range request surfaces
      // as a network / not-supported error) is retried, bounded by consecutive recoveries without progress.
      const code = video.error?.code
      if (!entityStreamUrl && code !== 3 && errorTriesRef.current < MAX_CONSECUTIVE_RECOVERIES && totalRecoveriesRef.current < MAX_TOTAL_RECOVERIES) {
        errorTriesRef.current += 1
        totalRecoveriesRef.current += 1
        void refreshTicket(true)
        return
      }
      onError?.(new Error('Video playback failed'))
    }

    video.addEventListener('timeupdate', handleTimeUpdate)
    video.addEventListener('loadedmetadata', handleLoadedMetadata)
    video.addEventListener('ended', handleEnded)
    video.addEventListener('timeupdate', handleProgress)
    video.addEventListener('seeking', handleSeeking)
    video.addEventListener('error', handleError)
    video.addEventListener('play', handlePlay)
    video.addEventListener('pause', handlePause)

    return () => {
      video.removeEventListener('timeupdate', handleTimeUpdate)
      video.removeEventListener('loadedmetadata', handleLoadedMetadata)
      video.removeEventListener('ended', handleEnded)
      video.removeEventListener('timeupdate', handleProgress)
      video.removeEventListener('seeking', handleSeeking)
      video.removeEventListener('error', handleError)
      video.removeEventListener('play', handlePlay)
      video.removeEventListener('pause', handlePause)
    }
  }, [onProgress, onEnded, onError, entityStreamUrl, refreshTicket])

  // Entity navigation (next/previous for episodes/tracks)
  const currentIndex = entity ? siblings.findIndex((s) => s.id === entity.id) : -1
  const prevSibling = currentIndex > 0 ? siblings[currentIndex - 1] : null
  const nextSibling = currentIndex >= 0 && currentIndex < siblings.length - 1 ? siblings[currentIndex + 1] : null

  const handlePrev = useCallback(() => {
    if (prevSibling && onNavigate) onNavigate(prevSibling.id)
  }, [prevSibling, onNavigate])

  const handleNext = useCallback(() => {
    if (nextSibling && onNavigate) onNavigate(nextSibling.id)
  }, [nextSibling, onNavigate])

  // Auto-hide controls
  useEffect(() => {
    if (isPlaying) {
      controlsTimeoutRef.current = setTimeout(() => {
        setShowControls(false)
      }, 3000)
    } else {
      setShowControls(true)
    }

    return () => {
      if (controlsTimeoutRef.current) {
        clearTimeout(controlsTimeoutRef.current)
      }
    }
  }, [isPlaying])

  return (
    <Card className="w-full max-w-4xl mx-auto">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Play className="h-5 w-5" />
          {entity?.title || media.title || 'Unknown Title'}
        </CardTitle>
      </CardHeader>
      
      <CardContent className="p-0">
        {/* Video Container */}
        <div 
          ref={playerRef}
          className="relative bg-black aspect-video"
          onMouseMove={handleMouseMove}
        >
          {/* Video Element */}
          <video
            ref={videoRef}
            src={videoSrc || undefined}
            className="absolute inset-0 w-full h-full"
            onPlay={() => setIsPlaying(true)}
            onPause={() => setIsPlaying(false)}
          >
            Your browser does not support the video tag.
          </video>
          {authError && <div role="alert" className="absolute inset-0 flex items-center justify-center text-white">Could not authorise playback</div>}

          {/* Controls Overlay */}
          {showControls && (
            <div className="absolute inset-0 bg-gradient-to-t from-black/70 via-transparent to-transparent pointer-events-none">
              <div className="absolute bottom-0 left-0 right-0 p-4 pointer-events-auto">
                {/* Progress Bar */}
                <div className="mb-4">
                  <input
                    type="range"
                    min="0"
                    max={duration || 0}
                    value={currentTime}
                    onChange={handleSeek}
                    className="w-full h-1 bg-white/30 rounded-lg appearance-none cursor-pointer slider"
                  />
                  <div className="flex justify-between text-xs text-white/80 mt-1">
                    <span>{formatTime(currentTime)}</span>
                    <span>{formatTime(duration)}</span>
                  </div>
                </div>

                {/* Control Buttons */}
                <div className="flex items-center justify-between">
                  <div className="flex items-center space-x-2">
                    {/* Previous Episode/Track */}
                    {prevSibling && (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={handlePrev}
                        className="text-white hover:text-white/80"
                        title={`Previous: ${prevSibling.title}`}
                      >
                        <SkipBack className="w-4 h-4" />
                        <SkipBack className="w-4 h-4 -ml-2" />
                      </Button>
                    )}

                    {/* Skip Back 10s */}
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => skip(-10)}
                      className="text-white hover:text-white/80"
                    >
                      <SkipBack className="w-4 h-4" />
                    </Button>

                    {/* Play/Pause */}
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={togglePlay}
                      className="text-white hover:text-white/80"
                    >
                      {isPlaying ? (
                        <Pause className="w-6 h-6" />
                      ) : (
                        <Play className="w-6 h-6" />
                      )}
                    </Button>

                    {/* Skip Forward 10s */}
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => skip(10)}
                      className="text-white hover:text-white/80"
                    >
                      <SkipForward className="w-4 h-4" />
                    </Button>

                    {/* Next Episode/Track */}
                    {nextSibling && (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={handleNext}
                        className="text-white hover:text-white/80"
                        title={`Next: ${nextSibling.title}`}
                      >
                        <SkipForward className="w-4 h-4" />
                        <SkipForward className="w-4 h-4 -ml-2" />
                      </Button>
                    )}
                  </div>

                  <div className="flex items-center space-x-2">
                    {/* Volume Control */}
                    <div className="flex items-center space-x-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={toggleMute}
                        className="text-white hover:text-white/80"
                      >
                        {isMuted ? (
                          <VolumeX className="w-4 h-4" />
                        ) : (
                          <Volume2 className="w-4 h-4" />
                        )}
                      </Button>
                      <input
                        type="range"
                        min="0"
                        max="1"
                        step="0.1"
                        value={isMuted ? 0 : volume}
                        onChange={(e) => handleVolumeChange(parseFloat(e.target.value))}
                        className="w-20 h-1 bg-white/30 rounded-lg appearance-none cursor-pointer"
                      />
                    </div>

                    {/* Subtitles */}
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setShowSubtitles(!showSubtitles)}
                      className="text-white hover:text-white/80"
                    >
                      <Subtitles className="w-4 h-4" />
                    </Button>

                    {/* Settings */}
                    <Button
                      variant="ghost"
                      size="sm"
                      className="text-white hover:text-white/80"
                    >
                      <Settings className="w-4 h-4" />
                    </Button>

                    {/* Fullscreen */}
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={toggleFullscreen}
                      className="text-white hover:text-white/80"
                    >
                      <Maximize2 className="w-4 h-4" />
                    </Button>
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* Subtitle Selection Modal */}
          {showSubtitles && subtitles.length > 0 && (
            <div className="absolute top-4 right-4 bg-black/80 rounded-lg p-4 text-white max-w-xs">
              <h3 className="text-sm font-medium mb-2">Subtitles</h3>
              <div className="space-y-2 max-h-48 overflow-y-auto">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setSelectedSubtitle('')}
                  className={`w-full justify-start text-left ${
                    selectedSubtitle === '' ? 'bg-white/20' : ''
                  }`}
                >
                  Off
                </Button>
                {subtitles.map(subtitle => (
                  <Button
                    key={subtitle.id}
                    variant="ghost"
                    size="sm"
                    onClick={() => setSelectedSubtitle(subtitle.id)}
                    className={`w-full justify-start text-left ${
                      selectedSubtitle === subtitle.id ? 'bg-white/20' : ''
                    }`}
                  >
                    <div className="flex items-center justify-between w-full">
                      <span>{subtitle.language}</span>
                      <span className="text-xs opacity-70">{subtitle.language_name || subtitle.language}</span>
                    </div>
                  </Button>
                ))}
              </div>
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  )
}