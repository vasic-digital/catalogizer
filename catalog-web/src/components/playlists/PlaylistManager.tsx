import React, { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { 
  Plus, 
  Search, 
  MoreVertical,
  Play,
  Shuffle,
  Edit3,
  Trash2,
  Lock,
  Unlock,
  Clock,
  Music,
  Grid,
  List
} from 'lucide-react';
import { Button } from '../ui/Button';
import { Input } from '../ui/Input';
import { Select } from '../ui/Select';
import { playlistApi } from '../../lib/playlistsApi';
import { usePlaylists } from '../../hooks/usePlaylists';
import { Playlist, PlaylistViewMode, PlaylistSortBy, getMediaIconWithMap } from '../../types/playlists';
import { toast } from 'react-hot-toast';

interface PlaylistManagerProps {
  onCreatePlaylist: () => void;
  onEditPlaylist: (playlist: Playlist) => void;
  onPlaylistSelect: (playlist: Playlist) => void;
  className?: string;
}

/**
 * PlaylistManager provides playlist browsing, searching, sorting, and
 * CRUD operations including create, edit, delete, and shuffle.
 */
export const PlaylistManager: React.FC<PlaylistManagerProps> = ({
  onCreatePlaylist,
  onEditPlaylist,
  onPlaylistSelect,
  className = ''
}) => {
  const [searchQuery, setSearchQuery] = useState('');
  const [viewMode, setViewMode] = useState<PlaylistViewMode>('grid');
  const [sortBy, setSortBy] = useState<PlaylistSortBy>('name');
  const [showDropdown, setShowDropdown] = useState<string | null>(null);

  const {
    playlists = [],
    isLoading,
    error,
    refetchPlaylists
  } = usePlaylists();

  const filteredAndSortedPlaylists = React.useMemo(() => {
    let filtered = playlists;

    // Apply search filter
    if (searchQuery) {
      filtered = filtered.filter(playlist =>
        playlist.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        playlist.description?.toLowerCase().includes(searchQuery.toLowerCase())
      );
    }

    // Apply sorting
    return filtered.sort((a, b) => {
      switch (sortBy) {
        case 'name':
          return a.name.localeCompare(b.name);
        case 'created_at':
          return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
        case 'updated_at':
          return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime();
        case 'duration':
          return (b.total_duration || 0) - (a.total_duration || 0);
        case 'item_count':
          return b.item_count - a.item_count;
        default:
          return 0;
      }
    });
  }, [playlists, searchQuery, sortBy]);

  const handlePlaylistAction = async (playlist: Playlist, action: string) => {
    try {
      switch (action) {
        case 'play':
          await playlistApi.playPlaylist(playlist.id);
          onPlaylistSelect(playlist);
          toast.success(`Playing ${playlist.name}`);
          break;
        
        case 'shuffle':
          await playlistApi.shufflePlaylist(playlist.id);
          onPlaylistSelect(playlist);
          toast.success(`Shuffling ${playlist.name}`);
          break;
        
        case 'duplicate': {
          const duplicateName = `${playlist.name} (Copy)`;
          const duplicate = await playlistApi.createPlaylist({
            name: duplicateName,
            description: playlist.description,
            is_public: playlist.is_public,
            items: playlist.items || []
          });
          toast.success(`Playlist duplicated as ${duplicate.name}`);
          refetchPlaylists();
          break;
        }

        case 'toggle_public': {
          const updated = await playlistApi.updatePlaylist(playlist.id, {
            is_public: !playlist.is_public
          });
          toast.success(`Playlist is now ${updated.is_public ? 'public' : 'private'}`);
          refetchPlaylists();
          break;
        }
        
        case 'delete':
          if (window.confirm(`Are you sure you want to delete "${playlist.name}"?`)) {
            await playlistApi.deletePlaylist(playlist.id);
            toast.success(`Deleted ${playlist.name}`);
            refetchPlaylists();
          }
          break;
      }
    } catch (error) {
      console.error("Error:", error);
      toast.error(`Failed to ${action} playlist`);
    } finally {
      setShowDropdown(null);
    }
  };

  const formatDuration = (seconds: number) => {
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = Math.floor(seconds % 60);
    
    if (hours > 0) {
      return `${hours}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
    }
    return `${minutes}:${secs.toString().padStart(2, '0')}`;
  };

  const getMediaTypes = (playlist: Playlist) => {
    const types = new Set(playlist.items?.map(item => item.media_item.media_type));
    return Array.from(types);
  };

  const renderPlaylistCard = (playlist: Playlist) => {
    const Icon = getMediaIconWithMap(playlist.primary_media_type || '');
    const mediaTypes = getMediaTypes(playlist);
    const isDropdownOpen = showDropdown === playlist.id;

    return (
      <motion.div
        key={playlist.id}
        layout
        initial={{ opacity: 0, scale: 0.9 }}
        animate={{ opacity: 1, scale: 1 }}
        exit={{ opacity: 0, scale: 0.9 }}
        whileHover={{ y: -4 }}
        className="group relative bg-white dark:bg-gray-800 rounded-lg shadow-sm hover:shadow-md transition-all duration-200 border border-gray-200 dark:border-gray-700"
      >
        {/* Playlist Cover */}
        <div className="aspect-video bg-gradient-to-br from-blue-500 to-purple-600 rounded-t-lg relative overflow-hidden">
          <div className="absolute inset-0 bg-black/20 flex items-center justify-center">
            <Icon className="w-16 h-16 text-white/80" />
          </div>
          
          {/* Media Type Badges */}
          <div className="absolute top-2 left-2 flex gap-1">
            {mediaTypes.slice(0, 2).map((type) => {
              const TypeIcon = getMediaIconWithMap(type);
              return (
                <div
                  key={type}
                  className="bg-white/90 backdrop-blur-sm p-1 rounded"
                  title={type}
                >
                  <TypeIcon className="w-3 h-3 text-gray-700" />
                </div>
              );
            })}
            {mediaTypes.length > 2 && (
              <div className="bg-white/90 backdrop-blur-sm p-1 rounded text-xs text-gray-700">
                +{mediaTypes.length - 2}
              </div>
            )}
          </div>

          {/* Public/Private Badge */}
          <div className="absolute top-2 right-2">
            {playlist.is_public ? (
              <Unlock className="w-4 h-4 text-white/90" />
            ) : (
              <Lock className="w-4 h-4 text-white/90" />
            )}
          </div>

          {/* Action Buttons */}
          <div className="absolute inset-0 bg-black/60 opacity-0 group-hover:opacity-100 transition-opacity duration-200 flex items-center justify-center gap-2">
            <Button
              size="sm"
              variant="secondary"
              onClick={(e) => {
                e.stopPropagation();
                handlePlaylistAction(playlist, 'play');
              }}
              className="bg-white/90 hover:bg-white text-gray-900"
            >
              <Play className="w-4 h-4" />
            </Button>
            <Button
              size="sm"
              variant="secondary"
              onClick={(e) => {
                e.stopPropagation();
                handlePlaylistAction(playlist, 'shuffle');
              }}
              className="bg-white/90 hover:bg-white text-gray-900"
            >
              <Shuffle className="w-4 h-4" />
            </Button>
            <Button
              size="sm"
              variant="secondary"
              onClick={(e) => {
                e.stopPropagation();
                onEditPlaylist(playlist);
              }}
              className="bg-white/90 hover:bg-white text-gray-900"
            >
              <Edit3 className="w-4 h-4" />
            </Button>
          </div>
        </div>

        {/* Playlist Info */}
        <div className="p-4">
          <div className="flex justify-between items-start mb-2">
            <div className="flex-1 min-w-0">
              <h3 className="font-semibold text-gray-900 dark:text-white truncate">
                {playlist.name}
              </h3>
              {playlist.description && (
                <p className="text-sm text-gray-600 dark:text-gray-400 mt-1 line-clamp-2">
                  {playlist.description}
                </p>
              )}
            </div>
            <div className="relative ml-2">
              <Button
                variant="ghost"
                size="sm"
                onClick={(e) => {
                  e.stopPropagation();
                  setShowDropdown(isDropdownOpen ? null : playlist.id);
                }}
                className="text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
              >
                <MoreVertical className="w-4 h-4" />
              </Button>
              
              {/* Dropdown Menu */}
              <AnimatePresence>
                {isDropdownOpen && (
                  <motion.div
                    initial={{ opacity: 0, scale: 0.95, y: -10 }}
                    animate={{ opacity: 1, scale: 1, y: 0 }}
                    exit={{ opacity: 0, scale: 0.95, y: -10 }}
                    transition={{ duration: 0.15 }}
                    className="absolute right-0 mt-1 w-48 bg-white dark:bg-gray-800 rounded-lg shadow-lg border border-gray-200 dark:border-gray-700 z-10"
                  >
                    <div className="py-1">
                      <button
                        onClick={() => handlePlaylistAction(playlist, 'duplicate')}
                        className="w-full px-4 py-2 text-left text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
                      >
                        <Plus className="w-4 h-4" />
                        Duplicate
                      </button>
                      <button
                        onClick={() => handlePlaylistAction(playlist, 'toggle_public')}
                        className="w-full px-4 py-2 text-left text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
                      >
                        {playlist.is_public ? (
                          <>
                            <Lock className="w-4 h-4" />
                            Make Private
                          </>
                        ) : (
                          <>
                            <Unlock className="w-4 h-4" />
                            Make Public
                          </>
                        )}
                      </button>
                      <button
                        onClick={() => handlePlaylistAction(playlist, 'delete')}
                        className="w-full px-4 py-2 text-left text-sm text-red-600 dark:text-red-400 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
                      >
                        <Trash2 className="w-4 h-4" />
                        Delete
                      </button>
                    </div>
                  </motion.div>
                )}
              </AnimatePresence>
            </div>
          </div>

          {/* Playlist Stats */}
          <div className="flex items-center gap-3 text-xs text-gray-500 dark:text-gray-400">
            <span className="flex items-center gap-1">
              <Grid className="w-3 h-3" />
              {playlist.item_count} items
            </span>
            {playlist.total_duration && playlist.total_duration > 0 && (
              <span className="flex items-center gap-1">
                <Clock className="w-3 h-3" />
                {formatDuration(playlist.total_duration)}
              </span>
            )}
            <span className="flex items-center gap-1">
              updated {new Date(playlist.updated_at).toLocaleDateString()}
            </span>
          </div>
        </div>
      </motion.div>
    );
  };

  const renderPlaylistListItem = (playlist: Playlist) => {
    const Icon = getMediaIconWithMap(playlist.primary_media_type || '');

    return (
      <motion.div
        key={playlist.id}
        layout
        initial={{ opacity: 0, x: -20 }}
        animate={{ opacity: 1, x: 0 }}
        exit={{ opacity: 0, x: 20 }}
        className="group bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4 hover:shadow-md transition-all duration-200"
      >
        <div className="flex items-center gap-4">
          {/* Thumbnail */}
          <div className="w-16 h-16 bg-gradient-to-br from-blue-500 to-purple-600 rounded-lg flex items-center justify-center flex-shrink-0">
            <Icon className="w-8 h-8 text-white/80" />
          </div>

          {/* Info */}
          <div className="flex-1 min-w-0">
            <div className="flex items-start justify-between">
              <div className="flex-1 min-w-0">
                <h3 className="font-semibold text-gray-900 dark:text-white truncate">
                  {playlist.name}
                </h3>
                {playlist.description && (
                  <p className="text-sm text-gray-600 dark:text-gray-400 mt-1 line-clamp-1">
                    {playlist.description}
                  </p>
                )}
                <div className="flex items-center gap-3 text-xs text-gray-500 dark:text-gray-400 mt-2">
                  <span>{playlist.item_count} items</span>
                  {playlist.total_duration && playlist.total_duration > 0 && (
                    <span>{formatDuration(playlist.total_duration)}</span>
                  )}
                  <span>updated {new Date(playlist.updated_at).toLocaleDateString()}</span>
                </div>
              </div>

              {/* Actions */}
              <div className="flex items-center gap-2 ml-4">
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => handlePlaylistAction(playlist, 'play')}
                  className="opacity-0 group-hover:opacity-100 transition-opacity"
                >
                  <Play className="w-4 h-4" />
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => onEditPlaylist(playlist)}
                  className="opacity-0 group-hover:opacity-100 transition-opacity"
                >
                  <Edit3 className="w-4 h-4" />
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={(e) => {
                    e.stopPropagation();
                    setShowDropdown(showDropdown === playlist.id ? null : playlist.id);
                  }}
                >
                  <MoreVertical className="w-4 h-4" />
                </Button>
              </div>
            </div>
          </div>
        </div>
      </motion.div>
    );
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-gray-500 dark:text-gray-400">Loading playlists...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-64 text-center">
        <p className="text-red-600 dark:text-red-400 mb-4">Failed to load playlists</p>
        <Button onClick={() => refetchPlaylists()} variant="outline">
          Try Again
        </Button>
      </div>
    );
  }

  return (
    <div className={`space-y-6 ${className}`}>
      {/* Header */}
      <div className="flex flex-col sm:flex-row gap-4 items-start sm:items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gray-900 dark:text-white">
            Playlists
          </h2>
          <p className="text-gray-600 dark:text-gray-400 mt-1">
            {playlists.length} playlist{playlists.length !== 1 ? 's' : ''}
          </p>
        </div>
        <Button onClick={onCreatePlaylist} className="flex items-center gap-2">
          <Plus className="w-4 h-4" />
          Create Playlist
        </Button>
      </div>

      {/* Filters and Controls */}
      <div className="flex flex-col sm:flex-row gap-4">
        {/* Search */}
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-400" />
          <Input
            placeholder="Search playlists..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-10"
          />
        </div>

        {/* View Mode Toggle */}
        <div className="flex items-center gap-2">
          <Button
            variant={viewMode === 'grid' ? 'secondary' : 'ghost'}
            size="sm"
            onClick={() => setViewMode('grid')}
          >
            <Grid className="w-4 h-4" />
          </Button>
          <Button
            variant={viewMode === 'list' ? 'secondary' : 'ghost'}
            size="sm"
            onClick={() => setViewMode('list')}
          >
            <List className="w-4 h-4" />
          </Button>
        </div>

        {/* Sort */}
        <Select
          aria-label="Sort playlists"
          value={sortBy}
          onChange={(value) => setSortBy(value as PlaylistSortBy)}
          options={[
            { value: 'name', label: 'Name' },
            { value: 'created_at', label: 'Created' },
            { value: 'updated_at', label: 'Updated' },
            { value: 'duration', label: 'Duration' },
            { value: 'item_count', label: 'Items' }
          ]}
          className="w-40"
        />
      </div>

      {/* Playlists Grid/List */}
      {filteredAndSortedPlaylists.length === 0 ? (
        <div className="text-center py-12">
          <Music className="w-16 h-16 text-gray-300 dark:text-gray-600 mx-auto mb-4" />
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">
            {searchQuery ? 'No playlists found' : 'No playlists yet'}
          </h3>
          <p className="text-gray-600 dark:text-gray-400 mb-6">
            {searchQuery 
              ? 'Try adjusting your search terms'
              : 'Create your first playlist to start organizing your media'
            }
          </p>
          {!searchQuery && (
            <Button onClick={onCreatePlaylist}>
              <Plus className="w-4 h-4 mr-2" />
              Create Playlist
            </Button>
          )}
        </div>
      ) : (
        <div className={viewMode === 'grid' 
          ? 'grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4'
          : 'space-y-2'
        }>
          <AnimatePresence mode="popLayout">
            {filteredAndSortedPlaylists.map((playlist) => 
              viewMode === 'grid' 
                ? renderPlaylistCard(playlist)
                : renderPlaylistListItem(playlist)
            )}
          </AnimatePresence>
        </div>
      )}
    </div>
  );
};