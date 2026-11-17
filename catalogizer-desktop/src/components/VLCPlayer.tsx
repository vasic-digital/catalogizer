import React, { useState, useEffect, useCallback, useRef } from 'react';
import { 
  Play, 
  Pause, 
  Square, 
  Volume2, 
  VolumeX, 
  Maximize, 
  Minimize,
  SkipBack,
  SkipForward,
  Settings,
  Subtitles,
  Languages,
  X
} from 'lucide-react';
import { useVLCPlayer } from '../hooks/useVLCPlayer';
import { apiService, msToWholeSeconds } from '../services/apiService';

interface VLCPlayerProps {
  mediaUrl: string;
  mediaTitle: string;
  mediaId?: number;
  onClose: () => void;
  onEnded?: () => void;
}

const SPEED_OPTIONS = [0.25, 0.5, 0.75, 1.0, 1.25, 1.5, 2.0, 3.0];
const ASPECT_RATIOS = ['16:9', '4:3', '1:1', '2.21:1', '2.35:1', 'Default'];

export const VLCPlayer: React.FC<VLCPlayerProps> = ({
  mediaUrl,
  mediaTitle,
  mediaId,
  onClose,
  onEnded,
}) => {
  const {
    status,
    tracks,
    error,
    isLoading,
    play,
    togglePlayPause,
    stop,
    seek,
    seekForward,
    seekBackward,
    setVolume,
    toggleMute,
    setRate,
    saveProgress,
    setAudioTrack,
    setSubtitleTrack,
    setAspectRatio,
  } = useVLCPlayer();

  const [showControls, setShowControls] = useState(true);
  const [showSettings, setShowSettings] = useState(false);
  const [showAudioMenu, setShowAudioMenu] = useState(false);
  const [showSubtitleMenu, setShowSubtitleMenu] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const controlsTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  // Start playback when component mounts
  useEffect(() => {
    play(mediaUrl, mediaId, mediaTitle);
  }, [mediaUrl, mediaId, mediaTitle]);

  // Handle playback ended
  useEffect(() => {
    if (status?.state === 'Ended') {
      // Mark as completed (100% progress)
      if (mediaId) {
        const durationSec = msToWholeSeconds(status?.duration || 0);
        apiService.updateWatchProgress(mediaId, {
          media_id: mediaId,
          position: durationSec,
          duration: durationSec,
          timestamp: Date.now()
        }).then((r) => {
          if (!r.persisted) {
            console.warn('[VLCPlayer] watch progress NOT saved', { mediaId, reason: r.reason });
          }
        }).catch((err) => {
          console.warn('[VLCPlayer] updateWatchProgress failed', { mediaId, err });
        });
      }
      onEnded?.();
    }
  }, [status?.state, onEnded, mediaId, status?.duration]);

  // Save progress when component unmounts
  useEffect(() => {
    return () => {
      if (mediaId) {
        saveProgress(mediaId);
      }
    };
  }, [mediaId, saveProgress]);

  // Auto-hide controls
  const resetControlsTimeout = useCallback(() => {
    if (controlsTimeoutRef.current) {
      clearTimeout(controlsTimeoutRef.current);
    }
    if (showControls && status?.isPlaying) {
      controlsTimeoutRef.current = setTimeout(() => {
        setShowControls(false);
        setShowSettings(false);
        setShowAudioMenu(false);
        setShowSubtitleMenu(false);
      }, 5000);
    }
  }, [showControls, status?.isPlaying]);

  useEffect(() => {
    resetControlsTimeout();
    return () => {
      if (controlsTimeoutRef.current) {
        clearTimeout(controlsTimeoutRef.current);
      }
    };
  }, [resetControlsTimeout]);

  const handleMouseMove = useCallback(() => {
    setShowControls(true);
    resetControlsTimeout();
  }, [resetControlsTimeout]);

  const handleSeek = (e: React.ChangeEvent<HTMLInputElement>) => {
    const position = parseFloat(e.target.value);
    seek(position);
  };

  const toggleFullscreen = () => {
    if (!document.fullscreenElement) {
      containerRef.current?.requestFullscreen();
      setIsFullscreen(true);
    } else {
      document.exitFullscreen();
      setIsFullscreen(false);
    }
  };

  const formatTime = (ms: number): string => {
    if (!ms || ms < 0) return '00:00';
    const seconds = Math.floor(ms / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    const remainingMinutes = minutes % 60;
    const remainingSeconds = seconds % 60;

    if (hours > 0) {
      return `${hours}:${remainingMinutes.toString().padStart(2, '0')}:${remainingSeconds.toString().padStart(2, '0')}`;
    }
    return `${remainingMinutes.toString().padStart(2, '0')}:${remainingSeconds.toString().padStart(2, '0')}`;
  };

  // Keyboard controls
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      switch (e.key) {
        case ' ':
        case 'k':
          e.preventDefault();
          togglePlayPause();
          break;
        case 'ArrowLeft':
          e.preventDefault();
          seekBackward(10000);
          break;
        case 'ArrowRight':
          e.preventDefault();
          seekForward(10000);
          break;
        case 'ArrowUp':
          e.preventDefault();
          setVolume(Math.min(100, (status?.volume || 50) + 5));
          break;
        case 'ArrowDown':
          e.preventDefault();
          setVolume(Math.max(0, (status?.volume || 50) - 5));
          break;
        case 'f':
          e.preventDefault();
          toggleFullscreen();
          break;
        case 'Escape':
          if (showSettings || showAudioMenu || showSubtitleMenu) {
            setShowSettings(false);
            setShowAudioMenu(false);
            setShowSubtitleMenu(false);
          } else if (document.fullscreenElement) {
            document.exitFullscreen();
            setIsFullscreen(false);
          }
          break;
        case 'm':
          e.preventDefault();
          toggleMute();
          break;
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [togglePlayPause, seekBackward, seekForward, setVolume, toggleMute, status?.volume, showSettings, showAudioMenu, showSubtitleMenu]);

  if (error) {
    return (
      <div className="fixed inset-0 bg-black flex items-center justify-center z-50">
        <div className="text-center text-white p-8">
          <X className="w-16 h-16 mx-auto mb-4 text-red-500" />
          <h2 className="text-2xl font-bold mb-2">Playback Error</h2>
          <p className="text-gray-400 mb-6">{error}</p>
          <button
            onClick={onClose}
            className="px-6 py-2 bg-primary text-white rounded-lg hover:bg-primary/80 transition-colors"
          >
            Close
          </button>
        </div>
      </div>
    );
  }

  return (
    <div
      ref={containerRef}
      className="fixed inset-0 bg-black z-50"
      onMouseMove={handleMouseMove}
      onClick={() => setShowControls(true)}
    >
      {/* Video Container - VLC renders in native window behind webview */}
      <div className="absolute inset-0 flex items-center justify-center">
        {isLoading && (
          <div className="text-center text-white">
            <div className="animate-spin w-12 h-12 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4" />
            <p className="text-lg">Loading...</p>
          </div>
        )}
        {!status?.isPlaying && !isLoading && (
          <div className="text-center text-white">
            <Play className="w-24 h-24 mx-auto mb-4 opacity-50" />
            <p className="text-xl">{mediaTitle}</p>
          </div>
        )}
      </div>

      {/* Controls Overlay */}
      {showControls && (
        <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-transparent to-black/40 pointer-events-auto">
          {/* Top Bar */}
          <div className="absolute top-0 left-0 right-0 p-4 flex items-center justify-between">
            <div className="flex items-center gap-4">
              <button
                onClick={() => {
                  stop();
                  onClose();
                }}
                className="p-2 text-white hover:bg-white/10 rounded-lg transition-colors"
              >
                <X className="w-6 h-6" />
              </button>
              <h1 className="text-white text-lg font-semibold truncate max-w-md">
                {mediaTitle}
              </h1>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={toggleFullscreen}
                className="p-2 text-white hover:bg-white/10 rounded-lg transition-colors"
              >
                {isFullscreen ? <Minimize className="w-6 h-6" /> : <Maximize className="w-6 h-6" />}
              </button>
            </div>
          </div>

          {/* Center Play/Pause Button */}
          <div className="absolute inset-0 flex items-center justify-center">
            <button
              onClick={togglePlayPause}
              className="p-6 bg-primary/90 hover:bg-primary text-white rounded-full transition-all transform hover:scale-110"
            >
              {status?.isPlaying ? (
                <Pause className="w-12 h-12" />
              ) : (
                <Play className="w-12 h-12 ml-1" />
              )}
            </button>
          </div>

          {/* Bottom Controls */}
          <div className="absolute bottom-0 left-0 right-0 p-4 space-y-4">
            {/* Progress Bar */}
            <div className="flex items-center gap-4">
              <span className="text-white text-sm w-16 text-right">
                {formatTime(status?.time || 0)}
              </span>
              <div className="flex-1 relative">
                <input
                  type="range"
                  min={0}
                  max={1}
                  step={0.001}
                  value={status?.position || 0}
                  onChange={handleSeek}
                  className="w-full h-2 bg-white/20 rounded-lg appearance-none cursor-pointer accent-primary hover:accent-primary/80"
                />
              </div>
              <span className="text-white text-sm w-16">
                {formatTime(status?.duration || 0)}
              </span>
            </div>

            {/* Control Buttons */}
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                {/* Play/Pause */}
                <button
                  onClick={togglePlayPause}
                  className="p-2 text-white hover:bg-white/10 rounded-lg transition-colors"
                >
                  {status?.isPlaying ? <Pause className="w-6 h-6" /> : <Play className="w-6 h-6" />}
                </button>

                {/* Stop */}
                <button
                  onClick={() => {
                    stop();
                    onClose();
                  }}
                  className="p-2 text-white hover:bg-white/10 rounded-lg transition-colors"
                >
                  <Square className="w-6 h-6" />
                </button>

                {/* Skip Backward */}
                <button
                  onClick={() => seekBackward(10000)}
                  className="p-2 text-white hover:bg-white/10 rounded-lg transition-colors"
                >
                  <SkipBack className="w-6 h-6" />
                </button>

                {/* Skip Forward */}
                <button
                  onClick={() => seekForward(10000)}
                  className="p-2 text-white hover:bg-white/10 rounded-lg transition-colors"
                >
                  <SkipForward className="w-6 h-6" />
                </button>

                {/* Volume */}
                <div className="flex items-center gap-2 ml-4">
                  <button
                    onClick={toggleMute}
                    className="p-2 text-white hover:bg-white/10 rounded-lg transition-colors"
                  >
                    {status?.isMuted ? <VolumeX className="w-6 h-6" /> : <Volume2 className="w-6 h-6" />}
                  </button>
                  <input
                    type="range"
                    min={0}
                    max={100}
                    value={status?.isMuted ? 0 : (status?.volume || 50)}
                    onChange={(e) => setVolume(parseInt(e.target.value))}
                    className="w-24 h-1 bg-white/20 rounded-lg appearance-none cursor-pointer accent-primary"
                  />
                </div>

                {/* Playback Speed */}
                <select
                  value={status?.rate || 1.0}
                  onChange={(e) => setRate(parseFloat(e.target.value))}
                  className="ml-4 px-3 py-1 bg-white/10 text-white rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                >
                  {SPEED_OPTIONS.map((speed) => (
                    <option key={speed} value={speed} className="bg-gray-900">
                      {speed}x
                    </option>
                  ))}
                </select>
              </div>

              <div className="flex items-center gap-2">
                {/* Audio Track */}
                <button
                  onClick={() => {
                    setShowAudioMenu(!showAudioMenu);
                    setShowSubtitleMenu(false);
                    setShowSettings(false);
                  }}
                  className="p-2 text-white hover:bg-white/10 rounded-lg transition-colors"
                >
                  <Languages className="w-6 h-6" />
                </button>

                {/* Subtitle Track */}
                <button
                  onClick={() => {
                    setShowSubtitleMenu(!showSubtitleMenu);
                    setShowAudioMenu(false);
                    setShowSettings(false);
                  }}
                  className="p-2 text-white hover:bg-white/10 rounded-lg transition-colors"
                >
                  <Subtitles className="w-6 h-6" />
                </button>

                {/* Settings */}
                <button
                  onClick={() => {
                    setShowSettings(!showSettings);
                    setShowAudioMenu(false);
                    setShowSubtitleMenu(false);
                  }}
                  className="p-2 text-white hover:bg-white/10 rounded-lg transition-colors"
                >
                  <Settings className="w-6 h-6" />
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Audio Track Menu */}
      {showAudioMenu && tracks?.audioTracks && tracks.audioTracks.length > 0 && (
        <div className="absolute right-4 bottom-24 bg-gray-900/95 rounded-lg p-4 min-w-[200px] border border-white/10">
          <h3 className="text-white font-semibold mb-3">Audio Track</h3>
          <div className="space-y-1">
            {tracks.audioTracks.map((track) => (
              <button
                key={track.id}
                onClick={() => {
                  setAudioTrack(track.id);
                  setShowAudioMenu(false);
                }}
                className={`w-full text-left px-3 py-2 rounded-lg text-sm transition-colors ${
                  track.isSelected
                    ? 'bg-primary text-white'
                    : 'text-gray-300 hover:bg-white/10'
                }`}
              >
                {track.name}
                {track.language && ` (${track.language})`}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Subtitle Menu */}
      {showSubtitleMenu && tracks?.subtitleTracks && (
        <div className="absolute right-4 bottom-24 bg-gray-900/95 rounded-lg p-4 min-w-[200px] border border-white/10">
          <h3 className="text-white font-semibold mb-3">Subtitles</h3>
          <div className="space-y-1">
            <button
              onClick={() => {
                setSubtitleTrack(-1);
                setShowSubtitleMenu(false);
              }}
              className={`w-full text-left px-3 py-2 rounded-lg text-sm transition-colors ${
                !tracks.subtitleTracks.some((t) => t.isSelected)
                  ? 'bg-primary text-white'
                  : 'text-gray-300 hover:bg-white/10'
              }`}
            >
              Off
            </button>
            {tracks.subtitleTracks.map((track) => (
              <button
                key={track.id}
                onClick={() => {
                  setSubtitleTrack(track.id);
                  setShowSubtitleMenu(false);
                }}
                className={`w-full text-left px-3 py-2 rounded-lg text-sm transition-colors ${
                  track.isSelected
                    ? 'bg-primary text-white'
                    : 'text-gray-300 hover:bg-white/10'
                }`}
              >
                {track.name}
                {track.language && ` (${track.language})`}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Settings Menu */}
      {showSettings && (
        <div className="absolute right-4 bottom-24 bg-gray-900/95 rounded-lg p-4 min-w-[200px] border border-white/10">
          <h3 className="text-white font-semibold mb-3">Settings</h3>
          <div className="space-y-4">
            <div>
              <label className="text-gray-400 text-sm block mb-2">Aspect Ratio</label>
              <select
                onChange={(e) => setAspectRatio(e.target.value)}
                className="w-full px-3 py-2 bg-white/10 text-white rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                {ASPECT_RATIOS.map((ratio) => (
                  <option key={ratio} value={ratio} className="bg-gray-900">
                    {ratio}
                  </option>
                ))}
              </select>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default VLCPlayer;
