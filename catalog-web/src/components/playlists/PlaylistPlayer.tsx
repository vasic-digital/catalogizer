import React, { useState } from 'react';
import { motion } from 'framer-motion';
import {
  GripVertical,
  Play,
  Pause,
  SkipForward,
  SkipBack,
  Volume2,
  Maximize2,
  MoreHorizontal,
  Shuffle,
  Repeat,
  List,
  Share,
  BarChart3,
  X,
  ChevronLeft,
  ChevronRight,
  Music,
  FileText
} from 'lucide-react';

import { Button } from '../ui/Button';
import { Playlist, PlaylistItem, PlaylistAnalytics as PlaylistAnalyticsType, flattenPlaylistItem, getMediaIconWithMap } from '../../types/playlists';
import { MediaPlayer } from '../media/MediaPlayer';
import { FavoriteToggle } from '../favorites/FavoriteToggle';
import { playlistsApi } from '../../lib/playlistsApi';
import { toast } from 'react-hot-toast';
import { PlaylistAnalytics } from './PlaylistAnalytics';

interface PlaylistPlayerProps {
  playlist: Playlist;
  items: PlaylistItem[];
  initialIndex?: number;
  onClose?: () => void;
  onShuffle?: () => void;
  onRepeat?: () => void;
  className?: string;
}

/**
 * PlaylistPlayer renders a media player with a track queue sidebar,
 * supporting sequential playback, shuffle, repeat, and reordering.
 */
export const PlaylistPlayer: React.FC<PlaylistPlayerProps> = ({
  playlist,
  items,
  initialIndex = 0,
  onClose,
  onShuffle,
  onRepeat,
  className = ''
}) => {
  const [currentIndex, setCurrentIndex] = useState(initialIndex);
  const [isPlaying, setIsPlaying] = useState(false);
  const [isShuffled, setIsShuffled] = useState(false);
  const [isRepeating, setIsRepeating] = useState(false);
  const [volume, setVolume] = useState(0.8);
  const [isMuted, setIsMuted] = useState(false);
  const [showPlaylist, setShowPlaylist] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [showControls, setShowControls] = useState(true);
  const [showAnalytics, setShowAnalytics] = useState(false);
  const [analytics, setAnalytics] = useState<PlaylistAnalyticsType | null>(null);
  const [repeatMode, setRepeatMode] = useState<'off' | 'all' | 'one'>('off');

  const currentItem = items[currentIndex];
  const flattenedCurrentItem = currentItem ? flattenPlaylistItem(currentItem) : null;
  const progress = items.length > 0 ? ((currentIndex + 1) / items.length) * 100 : 0;

  // Auto-hide controls for video playback
  React.useEffect(() => {
    if (isPlaying && flattenedCurrentItem?.media_type === 'video' && !isFullscreen) {
      const timer = setTimeout(() => setShowControls(false), 3000);
      return () => clearTimeout(timer);
    }
  }, [isPlaying, flattenedCurrentItem?.media_type, isFullscreen]);

  const handlePrevious = () => {
    if (currentIndex > 0) {
      setCurrentIndex(currentIndex - 1);
    } else if (isRepeating) {
      setCurrentIndex(items.length - 1);
    }
  };

  const handleNext = () => {
    if (currentIndex < items.length - 1) {
      setCurrentIndex(currentIndex + 1);
    } else if (isRepeating) {
      setCurrentIndex(0);
    } else {
      setIsPlaying(false);
    }
  };

  const handleItemClick = (index: number) => {
    setCurrentIndex(index);
    setIsPlaying(true);
  };

  const handleShuffle = () => {
    setIsShuffled(!isShuffled);
    onShuffle?.();
  };

  const handleRepeat = () => {
    const nextMode = repeatMode === 'off' ? 'all' : repeatMode === 'all' ? 'one' : 'off';
    setRepeatMode(nextMode);
    setIsRepeating(nextMode !== 'off');
    onRepeat?.();
  };

  const handleVolumeChange = (newVolume: number) => {
    setVolume(newVolume);
    setIsMuted(newVolume === 0);
  };

  const handleMuteToggle = () => {
    if (isMuted) {
      setVolume(0.8);
      setIsMuted(false);
    } else {
      setVolume(0);
      setIsMuted(true);
    }
  };

  const handleFullscreen = () => {
    setIsFullscreen(!isFullscreen);
  };

  const formatDuration = (seconds: number) => {
    const minutes = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${minutes}:${secs.toString().padStart(2, '0')}`;
  };

  const formatTimeRemaining = () => {
    const totalDuration = items
      .slice(currentIndex + 1)
      .reduce((sum, item) => sum + (flattenPlaylistItem(item).duration || 0), 0);
    return formatDuration(totalDuration);
  };

  const renderNowPlaying = () => {
    if (!currentItem || !flattenedCurrentItem) return null;

    const Icon = getMediaIconWithMap(flattenedCurrentItem.media_type);

    return (
      <div className="flex items-center gap-4 p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
        {/* Media Thumbnail */}
        <div className="relative group">
          <div className="w-24 h-24 bg-gradient-to-br from-blue-500 to-purple-600 rounded-lg flex items-center justify-center">
            <Icon className="w-12 h-12 text-white/80" />
          </div>
          <Button
            size="sm"
            className="absolute inset-0 bg-black/60 opacity-0 group-hover:opacity-100 transition-opacity m-auto"
            onClick={() => setIsPlaying(!isPlaying)}
          >
            {isPlaying ? <Pause className="w-6 h-6" /> : <Play className="w-6 h-6" />}
          </Button>
        </div>

        {/* Media Info */}
        <div className="flex-1 min-w-0">
          <h3 className="font-semibold text-lg text-gray-900 dark:text-white truncate">
            {flattenedCurrentItem.title}
          </h3>
          <p className="text-gray-600 dark:text-gray-400 truncate">
            {flattenedCurrentItem.artist || flattenedCurrentItem.description || 'Unknown'}
          </p>
          <div className="flex items-center gap-3 text-sm text-gray-500 dark:text-gray-400 mt-1">
            <span>{currentIndex + 1} of {items.length}</span>
            {flattenedCurrentItem.duration && (
              <span>{formatDuration(flattenedCurrentItem.duration)}</span>
            )}
            <span>{formatTimeRemaining()} remaining</span>
          </div>
        </div>

        {/* Actions */}
        <div className="flex items-center gap-2">
          <FavoriteToggle
            mediaId={parseInt(flattenedCurrentItem.item_id)}
            size="sm"
          />
          <Button variant="ghost" size="sm">
            <MoreHorizontal className="w-4 h-4" />
          </Button>
        </div>
      </div>
    );
  };

  const renderControls = () => {
    return (
      <div className="flex items-center justify-center gap-4 p-4">
        <Button
          variant="ghost"
          size="sm"
          onClick={handleShuffle}
          className={isShuffled ? 'text-blue-600 dark:text-blue-400' : ''}
        >
          <Shuffle className="w-5 h-5" />
        </Button>

        <Button
          variant="ghost"
          size="sm"
          onClick={handlePrevious}
          disabled={currentIndex === 0 && !isRepeating}
        >
          <SkipBack className="w-5 h-5" />
        </Button>

        <Button
          size="lg"
          onClick={() => setIsPlaying(!isPlaying)}
          className="w-14 h-14 rounded-full"
        >
          {isPlaying ? <Pause className="w-6 h-6" /> : <Play className="w-6 h-6" />}
        </Button>

        <Button
          variant="ghost"
          size="sm"
          onClick={handleNext}
          disabled={currentIndex === items.length - 1 && !isRepeating}
        >
          <SkipForward className="w-5 h-5" />
        </Button>

        <Button
          variant="ghost"
          size="sm"
          onClick={handleRepeat}
          className={repeatMode !== 'off' ? 'text-blue-600 dark:text-blue-400' : ''}
        >
          <Repeat className={`w-5 h-5 ${repeatMode === 'one' ? 'scale-x-[-1]' : ''}`} />
        </Button>
      </div>
    );
  };

  const renderVolumeControls = () => {
    return (
      <div className="flex items-center gap-2 px-4">
        <Button
          variant="ghost"
          size="sm"
          onClick={handleMuteToggle}
        >
          <Volume2 className={`w-5 h-5 ${isMuted ? 'text-gray-400' : ''}`} />
        </Button>
        <input
          type="range"
          min="0"
          max="1"
          step="0.1"
          value={isMuted ? 0 : volume}
          onChange={(e) => handleVolumeChange(parseFloat(e.target.value))}
          className="w-24 h-1 bg-gray-200 dark:bg-gray-700 rounded-lg appearance-none cursor-pointer slider"
        />
      </div>
    );
  };

  const renderProgressBar = () => {
    return (
      <div className="px-4">
        <div className="relative">
          <div className="h-1 bg-gray-200 dark:bg-gray-700 rounded-full">
            <div 
              className="h-full bg-blue-600 dark:bg-blue-500 rounded-full transition-all duration-300"
              style={{ width: `${progress}%` }}
            />
          </div>
          <div className="flex justify-between mt-1">
            <span className="text-xs text-gray-500 dark:text-gray-400">
              {currentIndex + 1}
            </span>
            <span className="text-xs text-gray-500 dark:text-gray-400">
              {items.length}
            </span>
          </div>
        </div>
      </div>
    );
  };

  const renderPlaylist = () => {
    return (
      <div className={`border-t border-gray-200 dark:border-gray-700 ${showPlaylist ? 'h-64' : 'h-0'} transition-all duration-300 overflow-hidden`}>
        <div className="p-4">
          <div className="flex items-center justify-between mb-4">
            <h4 className="font-semibold text-gray-900 dark:text-white">
              {playlist.name}
            </h4>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setShowPlaylist(!showPlaylist)}
            >
              {showPlaylist ? <ChevronRight className="w-4 h-4" /> : <ChevronLeft className="w-4 h-4" />}
            </Button>
          </div>
          
          <div className="space-y-1 max-h-48 overflow-y-auto">
            {items.map((item, index) => {
              const flattenedItem = flattenPlaylistItem(item);
              const Icon = getMediaIconWithMap(flattenedItem.media_type);
              const isCurrentItem = index === currentIndex;

              return (
                <motion.div
                  key={flattenedItem.item_id}
                  layout
                  initial={{ opacity: 0, x: -20 }}
                  animate={{ opacity: 1, x: 0 }}
                  className={`flex items-center gap-3 p-2 rounded-lg cursor-pointer transition-colors ${
                    isCurrentItem
                      ? 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300'
                      : 'hover:bg-gray-100 dark:hover:bg-gray-800'
                  }`}
                  onClick={() => handleItemClick(index)}
                >
                  <GripVertical className="w-4 h-4 text-gray-400" />
                  <Icon className="w-4 h-4 text-gray-600 dark:text-gray-400" />
                  <div className="flex-1 min-w-0">
                    <p className="font-medium text-sm truncate text-gray-900 dark:text-white">
                      {flattenedItem.title}
                    </p>
                    {flattenedItem.artist && (
                      <p className="text-xs text-gray-600 dark:text-gray-400 truncate">
                        {flattenedItem.artist}
                      </p>
                    )}
                  </div>
                  <div className="text-xs text-gray-500 dark:text-gray-400">
                    {formatDuration(flattenedItem.duration || 0)}
                  </div>
                  {isCurrentItem && isPlaying && (
                    <div className="w-2 h-2 bg-blue-600 rounded-full animate-pulse" />
                  )}
                </motion.div>
              );
            })}
          </div>
        </div>
      </div>
    );
  };

  const renderMediaContent = () => {
    if (!currentItem || !flattenedCurrentItem) return null;

    if (flattenedCurrentItem.media_type === 'video') {
      return (
        <div className="relative">
          <MediaPlayer
            key={flattenedCurrentItem.item_id}
            media={{
              id: parseInt(flattenedCurrentItem.item_id),
              title: flattenedCurrentItem.title,
              media_type: 'video',
              directory_path: flattenedCurrentItem.file_path || '',
              created_at: currentItem.added_at,
              updated_at: currentItem.added_at,
              cover_image: flattenedCurrentItem.thumbnail_url,
              duration: flattenedCurrentItem.duration
            }}
            onEnded={handleNext}
          />
          <div className="absolute inset-0 flex items-center justify-center bg-black bg-opacity-40">
            <Button 
              size="lg"
              onClick={() => setIsPlaying(!isPlaying)}
              className="rounded-full w-16 h-16"
            >
              {isPlaying ? <Pause className="w-6 h-6" /> : <Play className="w-6 h-6" />}
            </Button>
          </div>
        </div>
      );
    }

    if (flattenedCurrentItem.media_type === 'image') {
      return (
        <div className="flex items-center justify-center bg-gray-100 dark:bg-gray-800 h-64">
          <img
            src={flattenedCurrentItem.thumbnail_url || flattenedCurrentItem.file_path || ''}
            alt={flattenedCurrentItem.title}
            className="max-w-full max-h-full object-contain"
            onError={(e) => {
              e.currentTarget.src = '/placeholder-image.png';
            }}
          />
        </div>
      );
    }

    if (flattenedCurrentItem.media_type === 'music') {
      return (
        <div className="flex flex-col items-center justify-center bg-gray-100 dark:bg-gray-800 h-64">
          <Music className="w-24 h-24 text-gray-400 mb-4" />
          <div className="text-center">
            <h3 className="text-xl font-bold text-gray-900 dark:text-white mb-2">
              {flattenedCurrentItem.title}
            </h3>
            <p className="text-gray-600 dark:text-gray-400 mb-4">
              {flattenedCurrentItem.artist || 'Unknown Artist'}
            </p>
            {flattenedCurrentItem.duration && (
              <p className="text-sm text-gray-500 dark:text-gray-400">
                {formatDuration(flattenedCurrentItem.duration)}
              </p>
            )}
          </div>
        </div>
      );
    }

    return (
      <div className="flex items-center justify-center bg-gray-100 dark:bg-gray-800 h-64">
        <FileText className="w-24 h-24 text-gray-400" />
      </div>
    );
  };

  if (!items.length) {
    return (
      <div className="flex items-center justify-center h-64 text-center">
        <div>
          <Music className="w-16 h-16 text-gray-300 dark:text-gray-600 mx-auto mb-4" />
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">
            No items in playlist
          </h3>
          <p className="text-gray-600 dark:text-gray-400">
            Add some media to start playing
          </p>
        </div>
      </div>
    );
  }

  const handleShare = async () => {
    try {
      const shareInfo = await playlistsApi.sharePlaylist(playlist.id, {
        can_view: true,
        can_comment: false,
        can_download: false
      });
      
      // Copy share link to clipboard
      await navigator.clipboard.writeText(shareInfo.share_url);
      toast.success('Share link copied to clipboard!');
    } catch (error) {
      console.error("Error:", error);
      toast.error('Failed to create share link');
    }
  };

  const handleShowAnalytics = async () => {
    try {
      const analyticsData = await playlistsApi.getPlaylistAnalytics(playlist.id);
      setAnalytics(analyticsData);
      setShowAnalytics(true);
    } catch (error) {
      console.error("Error:", error);
      toast.error('Failed to load playlist analytics');
    }
  };

  return (
    <>
      <div className={`bg-white dark:bg-gray-900 rounded-lg shadow-lg ${className}`}>
      {/* Header */}
      <div className="flex items-center justify-between p-4 border-b border-gray-200 dark:border-gray-700">
        <div>
          <h2 className="text-xl font-bold text-gray-900 dark:text-white">
            {playlist.name}
          </h2>
          <p className="text-gray-600 dark:text-gray-400">
            {items.length} items
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setShowPlaylist(!showPlaylist)}
          >
            <List className="w-4 h-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={handleShare}
            title="Share playlist"
          >
            <Share className="w-4 h-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={handleShowAnalytics}
            title="View analytics"
          >
            <BarChart3 className="w-4 h-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={handleFullscreen}
          >
            <Maximize2 className="w-4 h-4" />
          </Button>
          {onClose && (
            <Button
              variant="ghost"
              size="sm"
              onClick={onClose}
            >
              <X className="w-4 h-4" />
            </Button>
          )}
        </div>
      </div>

      {/* Media Content */}
      <div className="relative">
        {renderMediaContent()}
        
        {/* Overlay Controls for Video */}
        {flattenedCurrentItem?.media_type === 'video' && showControls && (
          <div 
            className="absolute inset-0 bg-gradient-to-t from-black/60 to-transparent flex flex-col justify-between"
            onMouseEnter={() => setShowControls(true)}
            onMouseLeave={() => setShowControls(false)}
          >
            <div className="p-4">
              {/* Spacer for top controls if needed */}
            </div>
            <div className="p-4">
              {renderControls()}
              {renderProgressBar()}
              {renderVolumeControls()}
            </div>
          </div>
        )}
      </div>

      {/* Now Playing Info (for non-video) */}
      {flattenedCurrentItem?.media_type !== 'video' && (
        <>
          {renderNowPlaying()}
          {renderProgressBar()}
          {renderControls()}
          {renderVolumeControls()}
        </>
      )}

      {/* Playlist */}
      {renderPlaylist()}
    </div>

    {/* Analytics Modal */}
    {showAnalytics && analytics && (
      <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
        <div className="bg-white dark:bg-gray-900 rounded-lg max-w-4xl w-full max-h-[90vh] overflow-y-auto">
          <div className="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-700">
            <h2 className="text-xl font-bold text-gray-900 dark:text-white">
              Playlist Analytics - {playlist.name}
            </h2>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setShowAnalytics(false)}
            >
              <X className="w-4 h-4" />
            </Button>
          </div>
          <div className="p-6">
            <PlaylistAnalytics analytics={analytics} />
          </div>
        </div>
      </div>
    )}
    </>
  );
};