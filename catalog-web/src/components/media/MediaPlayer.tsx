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

const streamUrlFor = (id: number): string => {
  let token = ''
  try { token = localStorage.getItem('auth_token') || '' } catch { /* storage blocked: the request will 401 visibly */ }
  const base = (import.meta.env.VITE_API_BASE_URL || '') + `/api/v1/stream/${id}`
  return token ? `${base}?access_token=${encodeURIComponent(token)}` : base
}

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
      videoRef.current.pause()
    } else {
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
    }

    const handleEnded = () => {
      setIsPlaying(false)
      onEnded?.()
    }

    const handleError = () => {
      onError?.(new Error('Video playback failed'))
    }

    video.addEventListener('timeupdate', handleTimeUpdate)
    video.addEventListener('loadedmetadata', handleLoadedMetadata)
    video.addEventListener('ended', handleEnded)
    video.addEventListener('error', handleError)

    return () => {
      video.removeEventListener('timeupdate', handleTimeUpdate)
      video.removeEventListener('loadedmetadata', handleLoadedMetadata)
      video.removeEventListener('ended', handleEnded)
      video.removeEventListener('error', handleError)
    }
  }, [onProgress, onEnded, onError])

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

  // Determine the media source: an entity stream URL takes priority; otherwise the real range-capable stream endpoint
  // of the catalogued item. A <video>/<audio> request cannot carry an Authorization header, so the session token travels as
  // the documented query fallback (middleware/auth.go). directory_path is a storage-relative path, never a playable URL.
  const videoSrc = entityStreamUrl || streamUrlFor(media.id)

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
            className="absolute inset-0 w-full h-full"
            onPlay={() => setIsPlaying(true)}
            onPause={() => setIsPlaying(false)}
          >
            <source src={videoSrc} />
            Your browser does not support the video tag.
          </video>

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