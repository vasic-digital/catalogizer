import React, { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Search,
  Plus,
  Play,
  Shuffle,
  Heart,
  Clock,
  MoreHorizontal,
  Grid,
  List,
  Download,
  Share2,
  Edit3,
  Trash2,
  Music,
  Copy,
  ExternalLink
} from 'lucide-react';

import { Button } from '../ui/Button';
import { Input } from '../ui/Input';
import { Select } from '../ui/Select';
import { PlaylistPlayer } from './PlaylistPlayer';
import { playlistApi } from '../../lib/playlistsApi';
import { usePlaylists } from '../../hooks/usePlaylists';
import { useFavorites } from '../../hooks/useFavorites';
import { Playlist, PlaylistViewMode, PlaylistSortBy, getMediaIconWithMap } from '../../types/playlists';
import { toast } from 'react-hot-toast';

interface PlaylistGridProps {
  onCreatePlaylist?: () => void;
  onEditPlaylist?: (playlist: Playlist) => void;
  className?: string;
}

/**
 * PlaylistGrid renders all playlists in a filterable grid or list layout
 * with inline player, batch selection, and sharing controls.
 */
export const PlaylistGrid: React.FC<PlaylistGridProps> = ({
  onCreatePlaylist,
  onEditPlaylist,
  className = ''
}) => {
  const [searchQuery, setSearchQuery] = useState('');
  const [viewMode, setViewMode] = useState<PlaylistViewMode>('grid');
  const [sortBy, setSortBy] = useState<PlaylistSortBy>('name');
  const [filterMediaType, setFilterMediaType] = useState<string>('all');
  const [filterVisibility, setFilterVisibility] = useState<string>('all');
  const [selectedPlaylist, setSelectedPlaylist] = useState<Playlist | null>(null);
  const [selectedItems, setSelectedItems] = useState<Set<string>>(new Set());
  const [showPlayer, setShowPlayer] = useState(false);
  const [showDropdown, setShowDropdown] = useState<string | null>(null);

  const {
    playlists = [],
    isLoading,
    error,
    refetchPlaylists
  } = usePlaylists();

  const { checkFavoriteStatus } = useFavorites();

  const filteredAndSortedPlaylists = React.useMemo(() => {
    let filtered = playlists;

    // Apply search filter
    if (searchQuery) {
      filtered = filtered.filter(playlist =>
        playlist.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        playlist.description?.toLowerCase().includes(searchQuery.toLowerCase())
      );
    }

    // Apply media type filter
    if (filterMediaType !== 'all') {
      filtered = filtered.filter(playlist => 
        playlist.primary_media_type === filterMediaType
      );
    }

    // Apply visibility filter
    if (filterVisibility !== 'all') {
      filtered = filtered.filter(playlist => 
        filterVisibility === 'public' ? playlist.is_public : !playlist.is_public
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
        case 'name_desc':
          return b.name.localeCompare(a.name);
        default:
          return 0;
      }
    });
  }, [playlists, searchQuery, sortBy, filterMediaType, filterVisibility]);

  const handlePlaylistAction = async (playlist: Playlist, action: string) => {
    try {
      switch (action) {
        case 'play': {
          const _items = await playlistApi.getPlaylistItems(playlist.id);
          setSelectedPlaylist(playlist);
          setShowPlayer(true);
          toast.success(`Playing ${playlist.name}`);
          break;
        }

        case 'shuffle': {
          const _shuffleItems = await playlistApi.getPlaylistItems(playlist.id);
          await playlistApi.shufflePlaylist(playlist.id);
          setSelectedPlaylist(playlist);
          setShowPlayer(true);
          toast.success(`Shuffling ${playlist.name}`);
          break;
        }

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

        case 'export': {
          const exportData = await playlistApi.exportPlaylist(playlist.id);
          const blob = new Blob([JSON.stringify(exportData, null, 2)], {
            type: 'application/json'
          });
          const url = URL.createObjectURL(blob);
          const a = document.createElement('a');
          a.href = url;
          a.download = `${playlist.name.replace(/[^a-z0-9]/gi, '_')}.json`;
          document.body.appendChild(a);
          a.click();
          document.body.removeChild(a);
          URL.revokeObjectURL(url);
          toast.success(`Exported ${playlist.name}`);
          break;
        }
        
        case 'share':
          if (navigator.share) {
            await navigator.share({
              title: playlist.name,
              text: playlist.description || `Check out this playlist: ${playlist.name}`,
              url: window.location.origin + `/playlists/${playlist.id}`
            });
          } else {
            await navigator.clipboard.writeText(
              window.location.origin + `/playlists/${playlist.id}`
            );
            toast.success('Playlist link copied to clipboard');
          }
          break;
        
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

  const handleSelectPlaylist = (playlist: Playlist, isSelected: boolean) => {
    const newSelection = new Set(selectedItems);
    if (isSelected) {
      newSelection.add(playlist.id);
    } else {
      newSelection.delete(playlist.id);
    }
    setSelectedItems(newSelection);
  };

  const handleSelectAll = () => {
    if (selectedItems.size === filteredAndSortedPlaylists.length) {
      setSelectedItems(new Set());
    } else {
      setSelectedItems(new Set(filteredAndSortedPlaylists.map(p => p.id)));
    }
  };

  const handleBulkAction = async (action: string) => {
    try {
      switch (action) {
        case 'delete':
          if (window.confirm(`Are you sure you want to delete ${selectedItems.size} playlists?`)) {
            await Promise.all(
              Array.from(selectedItems).map(id => playlistApi.deletePlaylist(id))
            );
            toast.success(`Deleted ${selectedItems.size} playlists`);
            setSelectedItems(new Set());
            refetchPlaylists();
          }
          break;
        
        case 'export': {
          const selectedPlaylists = playlists.filter(p => selectedItems.has(p.id));
          const exportData = {
            playlists: selectedPlaylists,
            exported_at: new Date().toISOString(),
            version: '1.0'
          };
          const blob = new Blob([JSON.stringify(exportData, null, 2)], {
            type: 'application/json'
          });
          const url = URL.createObjectURL(blob);
          const a = document.createElement('a');
          a.href = url;
          a.download = `playlists_export_${new Date().toISOString().split('T')[0]}.json`;
          document.body.appendChild(a);
          a.click();
          document.body.removeChild(a);
          URL.revokeObjectURL(url);
          toast.success(`Exported ${selectedItems.size} playlists`);
          break;
        }
      }
    } catch (error) {
      console.error("Error:", error);
      toast.error(`Failed to ${action} playlists`);
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
    const isSelected = selectedItems.has(playlist.id);

    return (
      <motion.div
        key={playlist.id}
        layout
        initial={{ opacity: 0, scale: 0.9 }}
        animate={{ opacity: 1, scale: 1 }}
        exit={{ opacity: 0, scale: 0.9 }}
        whileHover={{ y: -4 }}
        className={`group relative bg-white dark:bg-gray-800 rounded-lg shadow-sm hover:shadow-md transition-all duration-200 border ${
          isSelected 
            ? 'border-blue-500 dark:border-blue-400' 
            : 'border-gray-200 dark:border-gray-700'
        }`}
      >
        {/* Selection Checkbox */}
        <label className="touch-target justify-center absolute top-0 left-0 z-10 p-2">
          <input
            type="checkbox"
            aria-label={`Select playlist ${playlist.name}`}
            checked={isSelected}
            onChange={(e) => {
              e.stopPropagation();
              handleSelectPlaylist(playlist, e.target.checked);
            }}
            className="w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500 opacity-0 group-hover:opacity-100 focus-visible:opacity-100 [@media(pointer:coarse)]:opacity-100 transition-opacity"
          />
        </label>

        {/* Playlist Cover */}
        <div className="aspect-video bg-gradient-to-br from-blue-500 to-purple-600 rounded-t-lg relative overflow-hidden">
          <div className="absolute inset-0 bg-black/20 flex items-center justify-center">
            <Icon className="w-16 h-16 text-white/80" />
          </div>
          
          {/* Media Type Badges */}
          <div className="absolute top-2 left-8 flex gap-1">
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

          {/* Favorite Badge */}
          {checkFavoriteStatus(parseInt(playlist.id)) && (
            <div className="absolute top-2 right-2">
              <Heart className="w-4 h-4 text-red-500 fill-current" />
            </div>
          )}

          {/* Quick Actions */}
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
                onEditPlaylist?.(playlist);
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
                <MoreHorizontal className="w-4 h-4" />
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
                        <Copy className="w-4 h-4" />
                        Duplicate
                      </button>
                      <button
                        onClick={() => handlePlaylistAction(playlist, 'export')}
                        className="w-full px-4 py-2 text-left text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
                      >
                        <Download className="w-4 h-4" />
                        Export
                      </button>
                      <button
                        onClick={() => handlePlaylistAction(playlist, 'share')}
                        className="w-full px-4 py-2 text-left text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
                      >
                        <Share2 className="w-4 h-4" />
                        Share
                      </button>
                      <button
                        onClick={() => {
                          window.open(`/playlists/${playlist.id}`, '_blank');
                        }}
                        className="w-full px-4 py-2 text-left text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
                      >
                        <ExternalLink className="w-4 h-4" />
                        Open in New Tab
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
            <span>
              {playlist.is_public ? 'Public' : 'Private'}
            </span>
            <span>
              {new Date(playlist.updated_at).toLocaleDateString()}
            </span>
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
      {/* Playlist Player Modal */}
      <AnimatePresence>
        {showPlayer && selectedPlaylist && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4"
            onClick={() => setShowPlayer(false)}
          >
            <motion.div
              initial={{ opacity: 0, scale: 0.95 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.95 }}
              transition={{ duration: 0.2 }}
              className="bg-white dark:bg-gray-900 rounded-lg max-w-4xl w-full max-h-[90vh] overflow-y-auto"
              onClick={(e) => e.stopPropagation()}
            >
              <PlaylistPlayer
                playlist={selectedPlaylist}
                items={selectedPlaylist.items || []}
                onClose={() => setShowPlayer(false)}
              />
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>

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

      {/* Bulk Actions */}
      {selectedItems.size > 0 && (
        <motion.div
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          className="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4"
        >
          <div className="flex items-center justify-between">
            <p className="text-blue-800 dark:text-blue-200">
              {selectedItems.size} playlist{selectedItems.size !== 1 ? 's' : ''} selected
            </p>
            <div className="flex items-center gap-2">
              <Button
                size="sm"
                variant="outline"
                onClick={() => handleBulkAction('export')}
              >
                <Download className="w-4 h-4 mr-2" />
                Export Selected
              </Button>
              <Button
                size="sm"
                variant="destructive"
                onClick={() => handleBulkAction('delete')}
              >
                <Trash2 className="w-4 h-4 mr-2" />
                Delete Selected
              </Button>
            </div>
          </div>
        </motion.div>
      )}

      {/* Filters and Controls */}
      <div className="flex flex-col lg:flex-row gap-4">
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

        {/* Filters */}
        <div className="flex items-center gap-2">
          <Select
            aria-label="Filter by media type"
            value={filterMediaType}
            onChange={(value) => setFilterMediaType(value)}
            options={[
              { value: 'all', label: 'All Types' },
              { value: 'music', label: 'Music' },
              { value: 'video', label: 'Video' },
              { value: 'image', label: 'Images' },
              { value: 'document', label: 'Documents' }
            ]}
            className="w-32"
          />
          
          <Select
            aria-label="Filter by visibility"
            value={filterVisibility}
            onChange={(value) => setFilterVisibility(value)}
            options={[
              { value: 'all', label: 'All' },
              { value: 'public', label: 'Public' },
              { value: 'private', label: 'Private' }
            ]}
            className="w-32"
          />
        </div>

        {/* View Mode Toggle */}
        <div className="flex items-center gap-2">
          <Button
            variant={viewMode === 'grid' ? 'secondary' : 'ghost'}
            size="sm"
            onClick={() => setViewMode('grid')}
            aria-label="Grid view"
            aria-pressed={viewMode === 'grid'}
          >
            <Grid className="w-4 h-4" />
          </Button>
          <Button
            variant={viewMode === 'list' ? 'secondary' : 'ghost'}
            size="sm"
            onClick={() => setViewMode('list')}
            aria-label="List view"
            aria-pressed={viewMode === 'list'}
          >
            <List className="w-4 h-4" />
          </Button>
        </div>

        {/* Sort */}
        <div className="flex items-center gap-2">
          <Select
            aria-label="Sort playlists"
            value={sortBy}
            onChange={(value) => setSortBy(value as PlaylistSortBy)}
            options={[
              { value: 'name', label: 'Name (A-Z)' },
              { value: 'name_desc', label: 'Name (Z-A)' },
              { value: 'created_at', label: 'Created' },
              { value: 'updated_at', label: 'Updated' },
              { value: 'duration', label: 'Duration' },
              { value: 'item_count', label: 'Items' }
            ]}
            className="w-40"
          />
        </div>

        {/* Select All */}
        <label className="touch-target gap-2 cursor-pointer">
          <input
            type="checkbox"
            checked={selectedItems.size === filteredAndSortedPlaylists.length && filteredAndSortedPlaylists.length > 0}
            onChange={handleSelectAll}
            className="w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500"
          />
          <span className="text-sm text-gray-600 dark:text-gray-400">
            Select All
          </span>
        </label>
      </div>

      {/* Playlists Grid */}
      {filteredAndSortedPlaylists.length === 0 ? (
        <div className="text-center py-12">
          <Music className="w-16 h-16 text-gray-300 dark:text-gray-600 mx-auto mb-4" />
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">
            {searchQuery || filterMediaType !== 'all' || filterVisibility !== 'all' 
              ? 'No playlists found' 
              : 'No playlists yet'
            }
          </h3>
          <p className="text-gray-600 dark:text-gray-400 mb-6">
            {searchQuery || filterMediaType !== 'all' || filterVisibility !== 'all'
              ? 'Try adjusting your filters or search terms'
              : 'Create your first playlist to start organizing your media'
            }
          </p>
          {!searchQuery && filterMediaType === 'all' && filterVisibility === 'all' && (
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
                : (
                    <div key={playlist.id} className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4 hover:shadow-md transition-all duration-200">
                      <div className="flex items-center justify-between">
                        <div className="flex-1">
                          <h3 className="font-semibold text-gray-900 dark:text-white">
                            {playlist.name}
                          </h3>
                          <p className="text-sm text-gray-600 dark:text-gray-400">
                            {playlist.item_count} items • {formatDuration(playlist.total_duration || 0)}
                          </p>
                        </div>
                        <div className="flex items-center gap-2">
                          <Button
                            size="sm"
                            variant="ghost"
                            onClick={() => handlePlaylistAction(playlist, 'play')}
                          >
                            <Play className="w-4 h-4" />
                          </Button>
                          <Button
                            size="sm"
                            variant="ghost"
                            onClick={() => onEditPlaylist?.(playlist)}
                          >
                            <Edit3 className="w-4 h-4" />
                          </Button>
                        </div>
                      </div>
                    </div>
                  )
            )}
          </AnimatePresence>
        </div>
      )}
    </div>
  );
};