import React, { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { motion, AnimatePresence } from 'framer-motion'
import { Search, Download, Globe, Clock, CheckCircle, Trash2, Eye, RefreshCw, Languages, Filter, Upload, Film, X } from 'lucide-react'
import { subtitleApi } from '@/lib/subtitleApi'
import { mediaApi } from '@/lib/mediaApi'
import { SubtitleSyncModal } from '@/components/subtitles/SubtitleSyncModal'
import { SubtitleUploadModal } from '@/components/subtitles/SubtitleUploadModal'
import type {
  SubtitleSearchRequest,
  SubtitleSearchResult
} from '@/types/subtitles'
import type { MediaItem, MediaSearchRequest } from '@/types/media'
import { COMMON_LANGUAGES } from '@/types/subtitles'
import { onActivateKey } from '../lib/keyboard';

export const SubtitleManager: React.FC = () => {
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedLanguage, setSelectedLanguage] = useState('')
  const [selectedProviders, setSelectedProviders] = useState<string[]>([])
  const [showFilters, setShowFilters] = useState(false)
  const [selectedMedia, setSelectedMedia] = useState<MediaItem | null>(null)
  const [showSearchResults, setShowSearchResults] = useState(false)
  const [showMediaSelector, setShowMediaSelector] = useState(false)
  const [currentPage, setCurrentPage] = useState(1)
  const [searchLimit, _setSearchLimit] = useState(20)
  const [syncModalOpen, setSyncModalOpen] = useState(false)
  const [syncSubtitle, setSyncSubtitle] = useState<{ id: string; language: string } | null>(null)
  const [uploadModalOpen, setUploadModalOpen] = useState(false)

  const queryClient = useQueryClient()

  // Get supported languages and providers
  const { data: languages = COMMON_LANGUAGES } = useQuery({
    queryKey: ['subtitle-languages'],
    queryFn: () => subtitleApi.getSupportedLanguages(),
  })

  const { data: providers = [] } = useQuery({
    queryKey: ['subtitle-providers'],
    queryFn: () => subtitleApi.getSupportedProviders(),
  })

  // Search media for selection
  const { data: mediaSearchResults, isLoading: isSearchingMedia } = useQuery({
    queryKey: ['media-search', searchQuery, currentPage],
    queryFn: () => {
      const request: MediaSearchRequest = {
        query: searchQuery || undefined,
        limit: 10,
        offset: (currentPage - 1) * 10,
      }
      return mediaApi.searchMedia(request)
    },
    enabled: showMediaSelector && searchQuery.length > 0,
  })

  // Search subtitles
  const {
    data: searchResults,
    isLoading: isSearching,
    mutate: searchSubtitles,
  } = useMutation({
    mutationFn: (params: SubtitleSearchRequest) => subtitleApi.searchSubtitles(params),
    onSuccess: () => {
      setShowSearchResults(true)
    },
  })

  // Get media subtitles
  const { data: mediaSubtitles, isLoading: isLoadingMediaSubtitles } = useQuery({
    queryKey: ['media-subtitles', selectedMedia?.id],
    queryFn: () => selectedMedia ? subtitleApi.getMediaSubtitles(selectedMedia.id) : null,
    enabled: !!selectedMedia,
  })

  // Download subtitle
  const downloadMutation = useMutation({
    mutationFn: ({ id, mediaPath }: { id: string; mediaPath?: string }) =>
      subtitleApi.downloadSubtitle({ id, media_path: mediaPath || selectedMedia?.directory_path }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['media-subtitles', selectedMedia?.id] })
    },
  })

  // Delete subtitle
  const deleteMutation = useMutation({
    mutationFn: (subtitleId: string) => subtitleApi.deleteSubtitle(subtitleId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['media-subtitles', selectedMedia?.id] })
    },
  })

  // Handle media selection
  const handleMediaSelect = (media: MediaItem) => {
    setSelectedMedia(media)
    setShowMediaSelector(false)
    setSearchQuery('')
  }

  // Handle search
  const handleSearch = () => {
    if (!searchQuery.trim()) return

    if (showMediaSelector) {
      // Media search mode - handled by query
      return
    }

    // Subtitle search mode
    searchSubtitles({
      query: searchQuery,
      language: selectedLanguage,
      media_path: selectedMedia?.directory_path,
      title: selectedMedia?.title,
      year: selectedMedia?.year,
      providers: selectedProviders.length > 0 ? selectedProviders : undefined,
      limit: searchLimit,
      offset: (currentPage - 1) * searchLimit,
    })
  }

  // Handle download
  const handleDownload = (result: SubtitleSearchResult) => {
    downloadMutation.mutate({ 
      id: result.id, 
      mediaPath: selectedMedia?.directory_path 
    })
  }

  // Handle delete
  const handleDelete = (subtitleId: string) => {
    if (confirm('Are you sure you want to delete this subtitle?')) {
      deleteMutation.mutate(subtitleId)
    }
  }

  // Handle sync verification
  const handleSyncVerification = (subtitleId: string, language: string) => {
    setSyncSubtitle({ id: subtitleId, language })
    setSyncModalOpen(true)
  }

  // Handle upload success
  const handleUploadSuccess = () => {
    queryClient.invalidateQueries({ queryKey: ['media-subtitles', selectedMedia?.id] })
  }

  // Provider toggle
  const toggleProvider = (provider: string) => {
    setSelectedProviders(prev =>
      prev.includes(provider)
        ? prev.filter(p => p !== provider)
        : [...prev, provider]
    )
  }

  // Toggle search mode
  const toggleSearchMode = () => {
    setShowMediaSelector(!showMediaSelector)
    setShowSearchResults(false)
    setSearchQuery('')
  }

  return (
    <div className="p-6 max-w-7xl mx-auto">
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        className="mb-8"
      >
        <h1 className="text-3xl font-bold text-gray-900 dark:text-white mb-2">
          Subtitle Manager
        </h1>
        <p className="text-gray-600 dark:text-gray-400">
          Search, download, and manage subtitles for your media collection
        </p>
      </motion.div>

      {/* Media Selection */}
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.1 }}
        className="bg-white dark:bg-gray-800 rounded-lg shadow-lg p-6 mb-6"
      >
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-semibold flex items-center gap-2">
            <Film className="w-5 h-5" />
            Selected Media
          </h2>
          <div className="flex gap-2">
            {selectedMedia && (
              <button
                onClick={() => setUploadModalOpen(true)}
                className="px-4 py-2 bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300 rounded-lg hover:bg-green-200 dark:hover:bg-green-800 flex items-center gap-2"
              >
                <Upload className="w-4 h-4" />
                Upload Subtitle
              </button>
            )}
            <button
              onClick={toggleSearchMode}
              className="px-4 py-2 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded-lg hover:bg-blue-200 dark:hover:bg-blue-800 flex items-center gap-2"
            >
              {selectedMedia ? 'Change Media' : 'Select Media'}
            </button>
          </div>
        </div>

        {selectedMedia ? (
          <div className="border border-gray-200 dark:border-gray-700 rounded-lg p-4">
            <div className="flex items-start justify-between">
              <div>
                <h3 className="font-medium text-gray-900 dark:text-white text-lg">
                  {selectedMedia.title}
                </h3>
                <div className="flex items-center gap-4 mt-1 text-sm text-gray-600 dark:text-gray-400">
                  <span>{selectedMedia.media_type}</span>
                  {selectedMedia.year && <span>{selectedMedia.year}</span>}
                  {selectedMedia.quality && <span>{selectedMedia.quality}</span>}
                </div>
                <p className="text-sm text-gray-500 dark:text-gray-500 mt-2 truncate">
                  {selectedMedia.directory_path}
                </p>
              </div>
              <button
                onClick={() => setSelectedMedia(null)}
                className="p-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
          </div>
        ) : (
          <div className="text-center py-8 text-gray-500 dark:text-gray-400">
            <Film className="w-12 h-12 mx-auto mb-3 opacity-50" />
            <p>No media selected</p>
            <p className="text-sm mt-1">Select a media item to manage its subtitles</p>
          </div>
        )}
      </motion.div>

      {/* Media Selector */}
      <AnimatePresence>
        {showMediaSelector && (
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 20 }}
            className="bg-white dark:bg-gray-800 rounded-lg shadow-lg p-6 mb-6"
          >
            <h2 className="text-xl font-semibold mb-4 flex items-center gap-2">
              <Search className="w-5 h-5" />
              Search for Media
            </h2>

            <div className="flex gap-3 mb-4">
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Search for movies, TV shows, etc..."
                className="flex-1 min-w-[12rem] px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                onKeyPress={(e) => e.key === 'Enter' && searchQuery.trim() && setCurrentPage(1)}
              />
              <button
                onClick={() => setShowMediaSelector(false)}
                className="px-4 py-2 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600"
              >
                Cancel
              </button>
            </div>

            {mediaSearchResults && mediaSearchResults.items.length > 0 && (
              <div className="space-y-2 max-h-64 overflow-y-auto">
                {mediaSearchResults.items.map((media: MediaItem) => (
                  <div
                    key={media.id}
                    onClick={() => handleMediaSelect(media)}
                    onKeyDown={onActivateKey(() => handleMediaSelect(media))}
                    role="button"
                    tabIndex={0}
                    className="border border-gray-200 dark:border-gray-700 rounded-lg p-3 hover:bg-gray-50 dark:hover:bg-gray-750 cursor-pointer transition-colors"
                  >
                    <h4 className="font-medium text-gray-900 dark:text-white">
                      {media.title}
                    </h4>
                    <div className="flex items-center gap-3 mt-1 text-sm text-gray-600 dark:text-gray-400">
                      <span>{media.media_type}</span>
                      {media.year && <span>{media.year}</span>}
                      {media.quality && <span>{media.quality}</span>}
                    </div>
                  </div>
                ))}
              </div>
            )}

            {isSearchingMedia && (
              <div className="flex justify-center py-4">
                <RefreshCw className="w-6 h-6 animate-spin text-blue-600" />
              </div>
            )}
          </motion.div>
        )}
      </AnimatePresence>

      {/* Subtitle Search Section */}
      {!showMediaSelector && (
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2 }}
          className="bg-white dark:bg-gray-800 rounded-lg shadow-lg p-6 mb-6"
        >
          <h2 className="text-xl font-semibold mb-4 flex items-center gap-2">
            <Search className="w-5 h-5" />
            Search Subtitles
          </h2>

          {/* Search Bar */}
          <div className="flex flex-wrap gap-3 mb-4">
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder={selectedMedia ? `Search subtitles for "${selectedMedia.title}"...` : "Search subtitles by title, filename, or keywords..."}
              className="flex-1 px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              onKeyPress={(e) => e.key === 'Enter' && handleSearch()}
            />
            
            <button
              onClick={() => setShowFilters(!showFilters)}
              className="px-4 py-2 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 flex items-center gap-2"
            >
              <Filter className="w-4 h-4" />
              Filters
            </button>

            <button
              onClick={handleSearch}
              disabled={isSearching || !searchQuery.trim()}
              className="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:bg-gray-400 disabled:cursor-not-allowed flex items-center gap-2"
            >
              {isSearching ? (
                <RefreshCw className="w-4 h-4 animate-spin" />
              ) : (
                <Search className="w-4 h-4" />
              )}
              Search
            </button>
          </div>

          {/* Filters */}
          <AnimatePresence>
            {showFilters && (
              <motion.div
                initial={{ height: 0, opacity: 0 }}
                animate={{ height: 'auto', opacity: 1 }}
                exit={{ height: 0, opacity: 0 }}
                transition={{ duration: 0.2 }}
                className="border-t border-gray-200 dark:border-gray-700 pt-4"
              >
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                  {/* Language Selector */}
                  <div>
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                      <Globe className="w-4 h-4 inline mr-1" />
                      Language
                    </label>
                    <select
                      value={selectedLanguage}
                      onChange={(e) => setSelectedLanguage(e.target.value)}
                      className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                    >
                      <option value="">All Languages</option>
                      {languages.map((lang) => (
                        <option key={lang.code} value={lang.code}>
                          {lang.native_name} ({lang.name})
                        </option>
                      ))}
                    </select>
                  </div>

                  {/* Provider Selector */}
                  <div className="lg:col-span-2">
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                      Providers
                    </label>
                    <div className="flex flex-wrap gap-2">
                      {providers.map((provider) => (
                        <button
                          key={provider.name}
                          onClick={() => toggleProvider(provider.name)}
                          className={`px-3 py-1 rounded-full text-sm transition-colors ${
                            selectedProviders.includes(provider.name)
                              ? 'bg-blue-600 text-white'
                              : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'
                          } ${!provider.enabled ? 'opacity-50 cursor-not-allowed' : ''}`}
                          disabled={!provider.enabled}
                        >
                          {provider.display_name}
                        </button>
                      ))}
                    </div>
                  </div>
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </motion.div>
      )}

      {/* Search Results */}
      <AnimatePresence>
        {showSearchResults && searchResults && (
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 20 }}
            className="bg-white dark:bg-gray-800 rounded-lg shadow-lg p-6 mb-6"
          >
            <div className="flex justify-between items-center mb-4">
              <h3 className="text-lg font-semibold">
                Search Results ({searchResults.total} found)
              </h3>
              <button
                onClick={() => setShowSearchResults(false)}
                className="text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
              >
                ×
              </button>
            </div>

            <div className="space-y-3 max-h-96 overflow-y-auto">
              {searchResults.results.map((result) => (
                <motion.div
                  key={result.id}
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  className="border border-gray-200 dark:border-gray-700 rounded-lg p-4 hover:bg-gray-50 dark:hover:bg-gray-750 transition-colors"
                >
                  <div className="flex items-start justify-between">
                    <div className="flex-1">
                      <h4 className="font-medium text-gray-900 dark:text-white">
                        {result.title}
                      </h4>
                      <div className="flex items-center gap-4 mt-1 text-sm text-gray-600 dark:text-gray-400">
                        <span className="flex items-center gap-1">
                          <Globe className="w-3 h-3" />
                          {result.language_name}
                        </span>
                        <span className="flex items-center gap-1">
                          <Eye className="w-3 h-3" />
                          {result.provider}
                        </span>
                        {result.rating && (
                          <span className="flex items-center gap-1">
                            <CheckCircle className="w-3 h-3" />
                            {result.rating}/10
                          </span>
                        )}
                        {result.release && (
                          <span>Release: {result.release}</span>
                        )}
                      </div>
                      <div className="flex items-center gap-4 mt-2 text-xs text-gray-500 dark:text-gray-500">
                        {result.hearing_impaired && (
                          <span className="px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200 rounded">
                            HI
                          </span>
                        )}
                        {result.foreign_parts_only && (
                          <span className="px-2 py-1 bg-purple-100 dark:bg-purple-900 text-purple-800 dark:text-purple-200 rounded">
                            Foreign Parts Only
                          </span>
                        )}
                        {result.machine_translated && (
                          <span className="px-2 py-1 bg-orange-100 dark:bg-orange-900 text-orange-800 dark:text-orange-200 rounded">
                            Machine Translated
                          </span>
                        )}
                      </div>
                    </div>
                    <button
                      onClick={() => handleDownload(result)}
                      disabled={downloadMutation.isLoading}
                      className="ml-4 px-4 py-2 bg-green-700 text-white rounded-lg hover:bg-green-800 disabled:bg-gray-400 flex items-center gap-2"
                    >
                      {downloadMutation.isLoading ? (
                        <RefreshCw className="w-4 h-4 animate-spin" />
                      ) : (
                        <Download className="w-4 h-4" />
                      )}
                      Download
                    </button>
                  </div>
                </motion.div>
              ))}
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Media Subtitles Management */}
      {selectedMedia && (
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.3 }}
          className="bg-white dark:bg-gray-800 rounded-lg shadow-lg p-6"
        >
          <h2 className="text-xl font-semibold mb-4 flex items-center gap-2">
            <Languages className="w-5 h-5" />
            Subtitles for &quot;{selectedMedia.title}&quot;
          </h2>

          {isLoadingMediaSubtitles ? (
            <div className="flex justify-center py-8">
              <RefreshCw className="w-8 h-8 animate-spin text-blue-600" />
            </div>
          ) : mediaSubtitles && mediaSubtitles.subtitles.length > 0 ? (
            <div className="space-y-3">
              {mediaSubtitles.subtitles.map((subtitle) => (
                <motion.div
                  key={subtitle.id}
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  className="border border-gray-200 dark:border-gray-700 rounded-lg p-4"
                >
                  <div className="flex items-start justify-between">
                    <div className="flex-1">
                      <h4 className="font-medium text-gray-900 dark:text-white">
                        {subtitle.language_name}
                      </h4>
                      <div className="flex items-center gap-4 mt-1 text-sm text-gray-600 dark:text-gray-400">
                        <span className="flex items-center gap-1">
                          <Globe className="w-3 h-3" />
                          {subtitle.provider}
                        </span>
                        <span className="flex items-center gap-1">
                          <Eye className="w-3 h-3" />
                          {subtitle.format} • {subtitle.encoding}
                        </span>
                        {subtitle.sync_offset !== undefined && (
                          <span className="flex items-center gap-1">
                            <Clock className="w-3 h-3" />
                            Offset: {subtitle.sync_offset}ms
                          </span>
                        )}
                        {subtitle.verified && (
                          <span className="flex items-center gap-1 text-green-600">
                            <CheckCircle className="w-3 h-3" />
                            Verified
                          </span>
                        )}
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => handleSyncVerification(subtitle.id, subtitle.language_name)}
                        className="p-2 text-blue-600 hover:bg-blue-50 dark:hover:bg-gray-700 rounded"
                        title="Verify Sync"
                      >
                        <CheckCircle className="w-4 h-4" />
                      </button>
                      <button
                        onClick={() => handleDelete(subtitle.id)}
                        className="p-2 text-red-600 hover:bg-red-50 dark:hover:bg-gray-700 rounded"
                        title="Delete"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                </motion.div>
              ))}
            </div>
          ) : (
            <div className="text-center py-8 text-gray-500 dark:text-gray-400">
              <Languages className="w-12 h-12 mx-auto mb-3 opacity-50" />
              <p>No subtitles found for this media item</p>
              <p className="text-sm mt-1">Search and download subtitles to get started</p>
            </div>
          )}
        </motion.div>
      )}

      {/* Modals */}
      <AnimatePresence>
        {syncModalOpen && syncSubtitle && selectedMedia && (
          <SubtitleSyncModal
            isOpen={syncModalOpen}
            onClose={() => setSyncModalOpen(false)}
            subtitleId={syncSubtitle.id}
            mediaId={selectedMedia.id}
            subtitleLanguage={syncSubtitle.language}
          />
        )}
      </AnimatePresence>

      <AnimatePresence>
        {uploadModalOpen && selectedMedia && (
          <SubtitleUploadModal
            isOpen={uploadModalOpen}
            onClose={() => setUploadModalOpen(false)}
            mediaId={selectedMedia.id}
            mediaTitle={selectedMedia.title}
            onUploadSuccess={handleUploadSuccess}
          />
        )}
      </AnimatePresence>
    </div>
  )
}