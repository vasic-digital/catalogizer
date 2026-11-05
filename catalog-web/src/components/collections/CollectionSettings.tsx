import React, { useState, useEffect } from 'react'
import { motion } from 'framer-motion'
import {
  Save,
  X,
  Grid,
  List,
  Layout,
  Settings,
  Play,
  Download,
  Share2,
  Shield
} from 'lucide-react'
import { SmartCollection, UpdateCollectionRequest } from '../../types/collections'
import { Button } from '../ui/Button'
import { Switch } from '../ui/Switch'
import { Select } from '../ui/Select'

interface CollectionSettingsProps {
  collection: SmartCollection
  onClose: () => void
  onSave: (settings: UpdateCollectionRequest) => void
}

interface CollectionPreferences {
  // Display Settings
  defaultView: 'grid' | 'list'
  itemsPerPage: number
  thumbnailSize: 'small' | 'medium' | 'large'
  showThumbnails: boolean
  showMetadata: boolean
  compactView: boolean
  
  // Behavior Settings
  autoRefresh: boolean
  refreshInterval: number // minutes
  sortOrder: 'name' | 'date_added' | 'date_modified' | 'size' | 'rating'
  sortDirection: 'asc' | 'desc'
  groupBy?: 'artist' | 'album' | 'genre' | 'year' | 'type'
  
  // Playback Settings
  autoPlayNext: boolean
  loopPlayback: boolean
  rememberPosition: boolean
  shuffleByDefault: boolean
  
  // Download Settings
  defaultDownloadFormat: string
  downloadQuality: 'original' | 'high' | 'medium' | 'low'
  downloadLocation: 'default' | 'custom'
  customDownloadPath?: string
  
  // Sharing Settings
  defaultSharePermissions: {
    can_download: boolean
    can_reshare: boolean
    expires_after_days: number
  }
  shareAnalytics: boolean
  
  // Notification Settings
  notifyOnNewItems: boolean
  notifyOnCollectionChanges: boolean
  notifyOnSharedAccess: boolean
  
  // Privacy Settings
  isPrivate: boolean
  requirePassword: boolean
  password?: string
  allowedUsers?: string[]
}

const DEFAULT_PREFERENCES: CollectionPreferences = {
  defaultView: 'grid',
  itemsPerPage: 20,
  thumbnailSize: 'medium',
  showThumbnails: true,
  showMetadata: true,
  compactView: false,
  autoRefresh: false,
  refreshInterval: 30,
  sortOrder: 'date_added',
  sortDirection: 'desc',
  autoPlayNext: true,
  loopPlayback: false,
  rememberPosition: true,
  shuffleByDefault: false,
  defaultDownloadFormat: 'original',
  downloadQuality: 'original',
  downloadLocation: 'default',
  defaultSharePermissions: {
    can_download: true,
    can_reshare: false,
    expires_after_days: 7
  },
  shareAnalytics: true,
  notifyOnNewItems: false,
  notifyOnCollectionChanges: true,
  notifyOnSharedAccess: true,
  isPrivate: false,
  requirePassword: false
}

const VIEW_SIZE_OPTIONS = [
  { value: 'small', label: 'Small (128px)' },
  { value: 'medium', label: 'Medium (256px)' },
  { value: 'large', label: 'Large (512px)' }
]

const ITEMS_PER_PAGE_OPTIONS = [10, 20, 50, 100]
const REFRESH_INTERVAL_OPTIONS = [5, 10, 15, 30, 60, 120]
const DOWNLOAD_QUALITY_OPTIONS = [
  { value: 'original', label: 'Original Quality' },
  { value: 'high', label: 'High Quality' },
  { value: 'medium', label: 'Medium Quality' },
  { value: 'low', label: 'Low Quality' }
]

const SORT_OPTIONS = [
  { value: 'name', label: 'Name' },
  { value: 'date_added', label: 'Date Added' },
  { value: 'date_modified', label: 'Date Modified' },
  { value: 'size', label: 'File Size' },
  { value: 'rating', label: 'Rating' }
]

const GROUP_BY_OPTIONS = [
  { value: '', label: 'No Grouping' },
  { value: 'artist', label: 'Artist' },
  { value: 'album', label: 'Album' },
  { value: 'genre', label: 'Genre' },
  { value: 'year', label: 'Year' },
  { value: 'type', label: 'Media Type' }
]

export const CollectionSettings: React.FC<CollectionSettingsProps> = ({
  collection,
  onClose,
  onSave
}) => {
  const [preferences, setPreferences] = useState<CollectionPreferences>(DEFAULT_PREFERENCES)
  const [isSaving, setIsSaving] = useState(false)
  const [activeTab, setActiveTab] = useState<'display' | 'behavior' | 'playback' | 'download' | 'sharing' | 'privacy'>('display')
  const [hasChanges, setHasChanges] = useState(false)

  // Load preferences on mount
  useEffect(() => {
    if (!collection) return

    // Load saved preferences or use defaults
    const savedPrefs = localStorage.getItem(`collection_prefs_${collection.id}`)
    if (savedPrefs) {
      try {
        const parsed = JSON.parse(savedPrefs)
        setPreferences({ ...DEFAULT_PREFERENCES, ...parsed })
      } catch (error) {
        // JSON parse failure — use defaults (already set)
        void error;
      }
    }
  }, [collection])

  // Save preferences to localStorage
  const savePreferences = async () => {
    if (!collection) return

    setIsSaving(true)
    try {
      // Save to localStorage
      localStorage.setItem(
        `collection_prefs_${collection.id}`,
        JSON.stringify(preferences)
      )

      // Update collection with relevant settings
      await onSave({
        is_public: !preferences.isPrivate
      })

      setHasChanges(false)
    } catch (error) {
      // Save failure — UI remains in unsaved state via hasChanges flag
      void error;
    } finally {
      setIsSaving(false)
    }
  }

  // Update preferences and track changes
  const updatePreference = <K extends keyof CollectionPreferences>(
    key: K,
    value: CollectionPreferences[K]
  ) => {
    setPreferences(prev => ({ ...prev, [key]: value }))
    setHasChanges(true)
  }

  // Update nested preferences
  const updateNestedPreference = (
    path: string,
    value: unknown
  ) => {
    setPreferences(prev => {
      const [parent, child] = path.split('.')
      return {
        ...prev,
        [parent]: {
          ...((prev as unknown) as Record<string, Record<string, unknown>>)[parent],
          [child]: value
        }
      }
    })
    setHasChanges(true)
  }

  const tabs = [
    { id: 'display', label: 'Display', icon: Layout },
    { id: 'behavior', label: 'Behavior', icon: Settings },
    { id: 'playback', label: 'Playback', icon: Play },
    { id: 'download', label: 'Download', icon: Download },
    { id: 'sharing', label: 'Sharing', icon: Share2 },
    { id: 'privacy', label: 'Privacy', icon: Shield }
  ]

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
    >
      <motion.div
        initial={{ scale: 0.9, y: 20 }}
        animate={{ scale: 1, y: 0 }}
        exit={{ scale: 0.9, y: 20 }}
        className="bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-4xl max-h-[90vh] overflow-hidden"
      >
        {/* Header */}
        <div className="p-6 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-2xl font-bold text-gray-900 dark:text-white">
                Collection Settings
              </h2>
              <p className="text-gray-500 dark:text-gray-400 mt-1">
                {collection.name}
              </p>
            </div>
            
            <Button
              variant="ghost"
              size="sm"
              onClick={onClose}
            >
              <X className="w-5 h-5" />
            </Button>
          </div>
        </div>

        <div className="flex flex-1 overflow-hidden">
          {/* Sidebar */}
          <div className="w-48 border-r border-gray-200 dark:border-gray-700 p-4">
            <nav className="space-y-1">
              {tabs.map(tab => {
                const Icon = tab.icon
                const isActive = activeTab === tab.id
                
                return (
                  <button
                    key={tab.id}
                    onClick={() => setActiveTab(tab.id as 'display' | 'behavior' | 'playback' | 'download' | 'sharing' | 'privacy')}
                    className={`
                      w-full flex items-center gap-3 px-3 py-2 rounded-lg text-left transition-colors
                      ${isActive 
                        ? 'bg-blue-50 text-blue-600 dark:bg-blue-900/20 dark:text-blue-400' 
                        : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'
                      }
                    `}
                  >
                    <Icon className="w-4 h-4" />
                    <span className="font-medium">{tab.label}</span>
                  </button>
                )
              })}
            </nav>
          </div>

          {/* Content */}
          <div className="flex-1 overflow-y-auto p-6">
            {/* Display Settings */}
            {activeTab === 'display' && (
              <motion.div
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                className="space-y-6"
              >
                <div>
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
                    View Settings
                  </h3>
                  
                  <div className="space-y-4">
                    <div>
                      <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                        Default View
                      </label>
                      <div className="flex gap-2">
                        <button
                          onClick={() => updatePreference('defaultView', 'grid')}
                          className={`
                            flex items-center gap-2 px-4 py-2 rounded-lg border
                            ${preferences.defaultView === 'grid'
                              ? 'bg-blue-50 border-blue-200 text-blue-700 dark:bg-blue-900/20 dark:border-blue-700 dark:text-blue-300'
                              : 'border-gray-200 dark:border-gray-600 text-gray-700 dark:text-gray-300'
                            }
                          `}
                        >
                          <Grid className="w-4 h-4" />
                          Grid
                        </button>
                        <button
                          onClick={() => updatePreference('defaultView', 'list')}
                          className={`
                            flex items-center gap-2 px-4 py-2 rounded-lg border
                            ${preferences.defaultView === 'list'
                              ? 'bg-blue-50 border-blue-200 text-blue-700 dark:bg-blue-900/20 dark:border-blue-700 dark:text-blue-300'
                              : 'border-gray-200 dark:border-gray-600 text-gray-700 dark:text-gray-300'
                            }
                          `}
                        >
                          <List className="w-4 h-4" />
                          List
                        </button>
                      </div>
                    </div>

                    <div>
                      <label htmlFor="coll-settings-select-1" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                        Items Per Page
                      </label>
                      <Select id="coll-settings-select-1"
                        value={preferences.itemsPerPage.toString()}
                        onChange={(value) => updatePreference('itemsPerPage', parseInt(value))}
                        options={ITEMS_PER_PAGE_OPTIONS.map(count => ({
                          value: count.toString(),
                          label: `${count} items`
                        }))}
                      />
                    </div>

                    <div>
                      <label htmlFor="coll-settings-select-2" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                        Thumbnail Size
                      </label>
                      <Select id="coll-settings-select-2"
                        value={preferences.thumbnailSize}
                        onChange={(value) => updatePreference('thumbnailSize', value as CollectionPreferences['thumbnailSize'])}
                        options={VIEW_SIZE_OPTIONS}
                      />
                    </div>
                  </div>
                </div>

                <div>
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
                    Display Options
                  </h3>
                  
                  <div className="space-y-4">
                    <div className="flex items-center justify-between">
                      <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                        Show Thumbnails
                      </label>
                      <Switch
                        checked={preferences.showThumbnails}
                        onCheckedChange={(checked) => updatePreference('showThumbnails', checked)}
                      />
                    </div>

                    <div className="flex items-center justify-between">
                      <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                        Show Metadata
                      </label>
                      <Switch
                        checked={preferences.showMetadata}
                        onCheckedChange={(checked) => updatePreference('showMetadata', checked)}
                      />
                    </div>

                    <div className="flex items-center justify-between">
                      <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                        Compact View
                      </label>
                      <Switch
                        checked={preferences.compactView}
                        onCheckedChange={(checked) => updatePreference('compactView', checked)}
                      />
                    </div>
                  </div>
                </div>
              </motion.div>
            )}

            {/* Behavior Settings */}
            {activeTab === 'behavior' && (
              <motion.div
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                className="space-y-6"
              >
                <div>
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
                    Auto-Refresh
                  </h3>
                  
                  <div className="space-y-4">
                    <div className="flex items-center justify-between">
                      <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                        Auto Refresh Collection
                      </label>
                      <Switch
                        checked={preferences.autoRefresh}
                        onCheckedChange={(checked) => updatePreference('autoRefresh', checked)}
                      />
                    </div>

                    {preferences.autoRefresh && (
                      <div>
                        <label htmlFor="coll-settings-select-3" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                          Refresh Interval
                        </label>
                        <Select id="coll-settings-select-3"
                          value={preferences.refreshInterval.toString()}
                          onChange={(value) => updatePreference('refreshInterval', parseInt(value))}
                          options={REFRESH_INTERVAL_OPTIONS.map(interval => ({
                            value: interval.toString(),
                            label: interval >= 60 
                              ? `${interval / 60} hours` 
                              : `${interval} minutes`
                          }))}
                        />
                      </div>
                    )}
                  </div>
                </div>

                <div>
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
                    Sort & Group
                  </h3>
                  
                  <div className="space-y-4">
                    <div>
                      <label htmlFor="coll-settings-select-4" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                        Sort By
                      </label>
                      <Select id="coll-settings-select-4"
                        value={preferences.sortOrder}
                        onChange={(value) => updatePreference('sortOrder', value as CollectionPreferences['sortOrder'])}
                        options={SORT_OPTIONS}
                      />
                    </div>

                    <div>
                      <label htmlFor="coll-settings-select-5" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                        Sort Direction
                      </label>
                      <Select id="coll-settings-select-5"
                        value={preferences.sortDirection}
                        onChange={(value) => updatePreference('sortDirection', value as CollectionPreferences['sortDirection'])}
                        options={[
                          { value: 'asc', label: 'Ascending' },
                          { value: 'desc', label: 'Descending' }
                        ]}
                      />
                    </div>

                    <div>
                      <label htmlFor="coll-settings-select-6" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                        Group By
                      </label>
                      <Select id="coll-settings-select-6"
                        value={preferences.groupBy || ''}
                        onChange={(value) => updatePreference('groupBy', value as CollectionPreferences['groupBy'])}
                        options={GROUP_BY_OPTIONS}
                      />
                    </div>
                  </div>
                </div>
              </motion.div>
            )}

            {/* Playback Settings */}
            {activeTab === 'playback' && (
              <motion.div
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                className="space-y-6"
              >
                <div className="space-y-4">
                  <div className="flex items-center justify-between">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                      Auto Play Next Item
                    </label>
                    <Switch
                      checked={preferences.autoPlayNext}
                      onCheckedChange={(checked) => updatePreference('autoPlayNext', checked)}
                    />
                  </div>

                  <div className="flex items-center justify-between">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                      Loop Playback
                    </label>
                    <Switch
                      checked={preferences.loopPlayback}
                      onCheckedChange={(checked) => updatePreference('loopPlayback', checked)}
                    />
                  </div>

                  <div className="flex items-center justify-between">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                      Remember Playback Position
                    </label>
                    <Switch
                      checked={preferences.rememberPosition}
                      onCheckedChange={(checked) => updatePreference('rememberPosition', checked)}
                    />
                  </div>

                  <div className="flex items-center justify-between">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                      Shuffle by Default
                    </label>
                    <Switch
                      checked={preferences.shuffleByDefault}
                      onCheckedChange={(checked) => updatePreference('shuffleByDefault', checked)}
                    />
                  </div>
                </div>
              </motion.div>
            )}

            {/* Download Settings */}
            {activeTab === 'download' && (
              <motion.div
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                className="space-y-6"
              >
                <div>
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
                    Download Options
                  </h3>
                  
                  <div className="space-y-4">
                    <div>
                      <label htmlFor="coll-settings-select-7" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                        Default Format
                      </label>
                      <Select id="coll-settings-select-7"
                        value={preferences.defaultDownloadFormat}
                        onChange={(value) => updatePreference('defaultDownloadFormat', value)}
                        options={[
                          { value: 'original', label: 'Original Format' },
                          { value: 'mp3', label: 'MP3' },
                          { value: 'mp4', label: 'MP4' },
                          { value: 'flac', label: 'FLAC' }
                        ]}
                      />
                    </div>

                    <div>
                      <label htmlFor="coll-settings-select-8" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                        Download Quality
                      </label>
                      <Select id="coll-settings-select-8"
                        value={preferences.downloadQuality}
                        onChange={(value) => updatePreference('downloadQuality', value as CollectionPreferences['downloadQuality'])}
                        options={DOWNLOAD_QUALITY_OPTIONS}
                      />
                    </div>

                    <div>
                      <label htmlFor="coll-settings-select-9" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                        Download Location
                      </label>
                      <Select id="coll-settings-select-9"
                        value={preferences.downloadLocation}
                        onChange={(value) => updatePreference('downloadLocation', value as CollectionPreferences['downloadLocation'])}
                        options={[
                          { value: 'default', label: 'Default Downloads Folder' },
                          { value: 'custom', label: 'Custom Location' }
                        ]}
                      />
                    </div>

                    {preferences.downloadLocation === 'custom' && (
                      <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                          Custom Path
                        </label>
                        <input
                          type="text"
                          value={preferences.customDownloadPath || ''}
                          onChange={(e) => updatePreference('customDownloadPath', e.target.value)}
                          placeholder="/path/to/downloads"
                          className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                        />
                      </div>
                    )}
                  </div>
                </div>
              </motion.div>
            )}

            {/* Sharing Settings */}
            {activeTab === 'sharing' && (
              <motion.div
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                className="space-y-6"
              >
                <div>
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
                    Default Permissions
                  </h3>
                  
                  <div className="space-y-4">
                    <div className="flex items-center justify-between">
                      <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                        Allow Download
                      </label>
                      <Switch
                        checked={preferences.defaultSharePermissions.can_download}
                        onCheckedChange={(checked) => 
                          updateNestedPreference('defaultSharePermissions.can_download', checked)
                        }
                      />
                    </div>

                    <div className="flex items-center justify-between">
                      <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                        Allow Reshare
                      </label>
                      <Switch
                        checked={preferences.defaultSharePermissions.can_reshare}
                        onCheckedChange={(checked) => 
                          updateNestedPreference('defaultSharePermissions.can_reshare', checked)
                        }
                      />
                    </div>

                    <div>
                      <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                        Default Expiry (Days)
                      </label>
                      <input
                        type="number"
                        min="1"
                        max="365"
                        value={preferences.defaultSharePermissions.expires_after_days}
                        onChange={(e) => 
                          updateNestedPreference('defaultSharePermissions.expires_after_days', parseInt(e.target.value))
                        }
                        className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                      />
                    </div>
                  </div>
                </div>

                <div>
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
                    Analytics
                  </h3>
                  
                  <div className="flex items-center justify-between">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                      Track Share Analytics
                    </label>
                    <Switch
                      checked={preferences.shareAnalytics}
                      onCheckedChange={(checked) => updatePreference('shareAnalytics', checked)}
                    />
                  </div>
                </div>
              </motion.div>
            )}

            {/* Privacy Settings */}
            {activeTab === 'privacy' && (
              <motion.div
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                className="space-y-6"
              >
                <div>
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
                    Privacy Controls
                  </h3>
                  
                  <div className="space-y-4">
                    <div className="flex items-center justify-between">
                      <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                        Private Collection
                      </label>
                      <Switch
                        checked={preferences.isPrivate}
                        onCheckedChange={(checked) => updatePreference('isPrivate', checked)}
                      />
                    </div>

                    <div className="flex items-center justify-between">
                      <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                        Require Password
                      </label>
                      <Switch
                        checked={preferences.requirePassword}
                        onCheckedChange={(checked) => updatePreference('requirePassword', checked)}
                      />
                    </div>

                    {preferences.requirePassword && (
                      <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                          Collection Password
                        </label>
                        <input
                          type="password"
                          value={preferences.password || ''}
                          onChange={(e) => updatePreference('password', e.target.value)}
                          placeholder="Enter collection password"
                          className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                        />
                      </div>
                    )}
                  </div>
                </div>
              </motion.div>
            )}
          </div>
        </div>

        {/* Footer */}
        <div className="p-6 border-t border-gray-200 dark:border-gray-700">
          <div className="flex items-center justify-between">
            <div className="text-sm text-gray-500 dark:text-gray-400">
              {hasChanges ? 'You have unsaved changes' : 'All changes saved'}
            </div>
            
            <div className="flex items-center gap-3">
              <Button
                variant="outline"
                onClick={onClose}
              >
                Cancel
              </Button>
              
              <Button
                onClick={savePreferences}
                disabled={isSaving || !hasChanges}
              >
                <Save className="w-4 h-4 mr-2" />
                {isSaving ? 'Saving...' : 'Save Settings'}
              </Button>
            </div>
          </div>
        </div>
      </motion.div>
    </motion.div>
  )
}

export default CollectionSettings