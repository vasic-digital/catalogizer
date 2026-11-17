import { useState, useEffect, useCallback, useRef } from 'react';
import { invoke } from '@tauri-apps/api/core';
import { apiService, msToWholeSeconds } from '../services/apiService';

export type PlaybackState = 
  | 'Idle' 
  | 'Opening' 
  | 'Buffering' 
  | 'Playing' 
  | 'Paused' 
  | 'Stopped' 
  | 'Ended' 
  | 'Error';

export interface TrackInfo {
  id: number;
  name: string;
  language?: string;
  isSelected: boolean;
}

export interface TrackListResponse {
  audioTracks: TrackInfo[];
  subtitleTracks: TrackInfo[];
  videoTracks: TrackInfo[];
}

export interface PlaybackStatus {
  state: PlaybackState;
  position: number;
  time: number;
  duration: number;
  volume: number;
  isPlaying: boolean;
  isMuted: boolean;
  rate: number;
}

// Progress tracking constants
const PROGRESS_SAVE_INTERVAL_MS = 5000; // Save every 5 seconds
const MIN_PROGRESS_TO_SAVE = 0.05; // Only save if > 5% watched
const MAX_PROGRESS_TO_SAVE = 0.95; // Don't save if > 95% (completed)

interface UseVLCPlayerReturn {
  // State
  status: PlaybackStatus | null;
  tracks: TrackListResponse | null;
  error: string | null;
  isLoading: boolean;
  
  // Controls
  play: (url: string, mediaId?: number, title?: string) => Promise<void>;
  pause: () => Promise<void>;
  resume: () => Promise<void>;
  stop: () => Promise<void>;
  togglePlayPause: () => Promise<boolean>;
  seek: (position: number) => Promise<void>;
  seekForward: (milliseconds: number) => Promise<number>;
  seekBackward: (milliseconds: number) => Promise<number>;
  setVolume: (volume: number) => Promise<number>;
  toggleMute: () => Promise<boolean>;
  setRate: (rate: number) => Promise<number>;
  setAudioTrack: (trackId: number) => Promise<void>;
  setSubtitleTrack: (trackId: number) => Promise<void>;
  setSubtitleDelay: (delayMs: number) => Promise<void>;
  setAudioDelay: (delayMs: number) => Promise<void>;
  setAspectRatio: (ratio: string) => Promise<void>;
  takeSnapshot: (width: number, height: number, filepath: string) => Promise<void>;
  refreshTracks: () => Promise<void>;
  refreshStatus: () => Promise<void>;
  saveProgress: (mediaId: number) => Promise<void>;
}

export function useVLCPlayer(): UseVLCPlayerReturn {
  const [status, setStatus] = useState<PlaybackStatus | null>(null);
  const [tracks, setTracks] = useState<TrackListResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const statusIntervalRef = useRef<NodeJS.Timeout | null>(null);

  // Initialize VLC
  useEffect(() => {
    const initVLC = async () => {
      try {
        await invoke('vlc_initialize');
      } catch (err) {
        console.warn('VLC initialization warning:', err);
        // VLC might already be initialized
      }
    };
    initVLC();

    return () => {
      // Stop playback on unmount
      invoke('vlc_stop').catch((err) => {
        console.warn('[useVLCPlayer] vlc_stop on unmount failed', err);
      });
      if (statusIntervalRef.current) {
        clearInterval(statusIntervalRef.current);
      }
    };
  }, []);

  // Start status polling when playing
  useEffect(() => {
    if (status?.isPlaying) {
      statusIntervalRef.current = setInterval(() => {
        refreshStatus();
      }, 500);
    } else {
      if (statusIntervalRef.current) {
        clearInterval(statusIntervalRef.current);
        statusIntervalRef.current = null;
      }
    }

    return () => {
      if (statusIntervalRef.current) {
        clearInterval(statusIntervalRef.current);
      }
    };
  }, [status?.isPlaying]);

  const refreshStatus = useCallback(async () => {
    try {
      const newStatus = await invoke<PlaybackStatus>('vlc_get_status');
      setStatus(newStatus);
    } catch (err) {
      console.error('Failed to get status:', err);
    }
  }, []);

  const refreshTracks = useCallback(async () => {
    try {
      const newTracks = await invoke<TrackListResponse>('vlc_get_tracks');
      setTracks(newTracks);
    } catch (err) {
      console.error('Failed to get tracks:', err);
    }
  }, []);

  const play = useCallback(async (url: string, mediaId?: number, title?: string) => {
    setIsLoading(true);
    setError(null);
    try {
      await invoke('vlc_play', {
        request: { url, mediaId, title }
      });
      await refreshStatus();
      await refreshTracks();
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to play media';
      setError(errorMessage);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, [refreshStatus, refreshTracks]);

  const pause = useCallback(async () => {
    try {
      await invoke('vlc_pause');
      await refreshStatus();
    } catch (err) {
      console.error('Failed to pause:', err);
    }
  }, [refreshStatus]);

  const resume = useCallback(async () => {
    try {
      await invoke('vlc_resume');
      await refreshStatus();
    } catch (err) {
      console.error('Failed to resume:', err);
    }
  }, [refreshStatus]);

  const stop = useCallback(async () => {
    try {
      await invoke('vlc_stop');
      await refreshStatus();
    } catch (err) {
      console.error('Failed to stop:', err);
    }
  }, [refreshStatus]);

  const togglePlayPause = useCallback(async () => {
    try {
      const isNowPlaying = await invoke<boolean>('vlc_toggle_playback');
      await refreshStatus();
      return isNowPlaying;
    } catch (err) {
      console.error('Failed to toggle playback:', err);
      return false;
    }
  }, [refreshStatus]);

  const seek = useCallback(async (position: number) => {
    try {
      await invoke('vlc_seek', { position });
      await refreshStatus();
    } catch (err) {
      console.error('Failed to seek:', err);
    }
  }, [refreshStatus]);

  const seekForward = useCallback(async (milliseconds: number) => {
    try {
      const newTime = await invoke<number>('vlc_seek_forward', { milliseconds });
      await refreshStatus();
      return newTime;
    } catch (err) {
      console.error('Failed to seek forward:', err);
      return 0;
    }
  }, [refreshStatus]);

  const seekBackward = useCallback(async (milliseconds: number) => {
    try {
      const newTime = await invoke<number>('vlc_seek_backward', { milliseconds });
      await refreshStatus();
      return newTime;
    } catch (err) {
      console.error('Failed to seek backward:', err);
      return 0;
    }
  }, [refreshStatus]);

  const setVolume = useCallback(async (volume: number) => {
    try {
      const newVolume = await invoke<number>('vlc_set_volume', { volume });
      await refreshStatus();
      return newVolume;
    } catch (err) {
      console.error('Failed to set volume:', err);
      return volume;
    }
  }, [refreshStatus]);

  const toggleMute = useCallback(async () => {
    try {
      const isMuted = await invoke<boolean>('vlc_toggle_mute');
      await refreshStatus();
      return isMuted;
    } catch (err) {
      console.error('Failed to toggle mute:', err);
      return false;
    }
  }, [refreshStatus]);

  const setRate = useCallback(async (rate: number) => {
    try {
      const newRate = await invoke<number>('vlc_set_rate', { rate });
      await refreshStatus();
      return newRate;
    } catch (err) {
      console.error('Failed to set rate:', err);
      return rate;
    }
  }, [refreshStatus]);

  const setAudioTrack = useCallback(async (trackId: number) => {
    try {
      await invoke('vlc_set_audio_track', { trackId });
      await refreshTracks();
    } catch (err) {
      console.error('Failed to set audio track:', err);
    }
  }, [refreshTracks]);

  const setSubtitleTrack = useCallback(async (trackId: number) => {
    try {
      await invoke('vlc_set_subtitle_track', { trackId });
      await refreshTracks();
    } catch (err) {
      console.error('Failed to set subtitle track:', err);
    }
  }, [refreshTracks]);

  const setSubtitleDelay = useCallback(async (delayMs: number) => {
    try {
      await invoke('vlc_set_subtitle_delay', { delayMs });
    } catch (err) {
      console.error('Failed to set subtitle delay:', err);
    }
  }, []);

  const setAudioDelay = useCallback(async (delayMs: number) => {
    try {
      await invoke('vlc_set_audio_delay', { delayMs });
    } catch (err) {
      console.error('Failed to set audio delay:', err);
    }
  }, []);

  const setAspectRatio = useCallback(async (ratio: string) => {
    try {
      await invoke('vlc_set_aspect_ratio', { ratio });
    } catch (err) {
      console.error('Failed to set aspect ratio:', err);
    }
  }, []);

  const takeSnapshot = useCallback(async (width: number, height: number, filepath: string) => {
    try {
      await invoke('vlc_take_snapshot', { width, height, filepath });
    } catch (err) {
      console.error('Failed to take snapshot:', err);
    }
  }, []);

  // Save watch progress to server
  const saveProgress = useCallback(async (mediaId: number) => {
    if (mediaId <= 0 || !status) return;
    
    const currentTime = status.time;
    const duration = status.duration;
    
    if (duration <= 0) return;
    
    const progress = currentTime / duration;
    
    // Only save if progress is in valid range
    if (progress >= MIN_PROGRESS_TO_SAVE && progress <= MAX_PROGRESS_TO_SAVE) {
      try {
        const result = await apiService.updateWatchProgress(mediaId, {
          media_id: mediaId,
          position: msToWholeSeconds(currentTime),
          duration: msToWholeSeconds(duration),
          timestamp: Date.now()
        });
        if (result.persisted) {
          console.log('Watch progress saved:', `${(progress * 100).toFixed(1)}%`);
        } else {
          console.warn('Watch progress NOT saved:', result.reason ?? 'unknown');
        }
      } catch (err) {
        console.warn('Failed to save watch progress:', err);
      }
    }
  }, [status]);

  // Auto-save progress periodically when playing
  useEffect(() => {
    let progressInterval: NodeJS.Timeout | null = null;
    
    if (status?.isPlaying) {
      progressInterval = setInterval(() => {
        // Get current mediaId from the hook state if available
        // This is handled by the component calling saveProgress on unmount
      }, PROGRESS_SAVE_INTERVAL_MS);
    }
    
    return () => {
      if (progressInterval) {
        clearInterval(progressInterval);
      }
    };
  }, [status?.isPlaying]);

  return {
    status,
    tracks,
    error,
    isLoading,
    play,
    pause,
    resume,
    stop,
    togglePlayPause,
    seek,
    seekForward,
    seekBackward,
    setVolume,
    toggleMute,
    setRate,
    saveProgress,
    setAudioTrack,
    setSubtitleTrack,
    setSubtitleDelay,
    setAudioDelay,
    setAspectRatio,
    takeSnapshot,
    refreshTracks,
    refreshStatus,
  };
}
