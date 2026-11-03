import React, { useState, useMemo } from 'react'
import { motion } from 'framer-motion'
import {
  BarChart3,
  Play,
  Share2,
  Download,
  Star,
  Activity,
  Download as DownloadIcon,
  Upload,
  Users,
  FileVideo,
  FileAudio,
  FileImage,
  FileText,
  X
} from 'lucide-react'
import { Button } from '../ui/Button'
import { Card } from '../ui/Card'
import { SmartCollection } from '../../types/collections'
import { useCollection } from '../../hooks/useCollections'

interface CollectionAnalyticsProps {
  collection: SmartCollection
  onClose?: () => void
}

/** Only figures derived from the collection's real items. Plays, downloads, shares, views and engagement are not
 *  recorded by the server, so they are not modelled here at all (CAT-001: no fabricated analytics). */
interface AnalyticsData {
  overview: {
    totalItems: number
    totalSize: string
    totalDuration: string
    averageRating: number | null
  }
  mediaBreakdown: {
    music: number
    video: number
    image: number
    document: number
  }
  topItems: Array<{
    id: string
    title: string
    type: string
    rating: number
    size: string
  }>
}

interface CollectionItem {
  id: string
  title?: string
  name?: string
  media_type?: string
  size?: number
  rating?: number
  duration?: number
}

const MEDIA_ICONS = {
  music: FileAudio,
  video: FileVideo,
  image: FileImage,
  document: FileText
}

export const CollectionAnalytics: React.FC<CollectionAnalyticsProps> = ({
  collection,
  onClose
}) => {
  const [activeTab, setActiveTab] = useState<'overview' | 'activity' | 'content' | 'sharing' | 'engagement'>('overview')
  
  const { collectionItems, isLoading: itemsLoading } = useCollection(collection?.id || '')

  // Use collection items from props if available
  const items = useMemo(() => collectionItems?.items || [], [collectionItems])

  const formatFileSize = (bytes: number): string => {
    if (bytes <= 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.min(sizes.length - 1, Math.floor(Math.log(bytes) / Math.log(k)))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
  }

  const formatDuration = (seconds: number): string => {
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    const secs = Math.floor(seconds % 60)
    if (hours > 0) return `${hours}h ${minutes}m`
    if (minutes > 0) return `${minutes}m ${secs}s`
    return `${secs}s`
  }

  // Everything here is computed from the collection's real items.
  const analyticsData = useMemo((): AnalyticsData => {
    const totalItems = items.length
    const totalSize = items.reduce((sum: number, item: CollectionItem) => sum + (item.size || 0), 0)
    const rated = items.filter((item: CollectionItem) => typeof item.rating === 'number')
    const averageRating = rated.length > 0
      ? rated.reduce((sum: number, item: CollectionItem) => sum + (item.rating as number), 0) / rated.length
      : null

    const mediaTypes = items.reduce((acc: Record<string, number>, item: CollectionItem) => {
      const type = item.media_type || 'document'
      acc[type] = (acc[type] || 0) + 1
      return acc
    }, {} as Record<string, number>)

    const topItems = rated
      .map((item: CollectionItem) => ({
        id: item.id,
        title: item.title || item.name || 'Unknown',
        type: item.media_type || 'document',
        rating: item.rating as number,
        size: item.size ? formatFileSize(item.size) : 'Unavailable'
      }))
      .sort((a: { rating: number }, b: { rating: number }) => b.rating - a.rating)
      .slice(0, 10)

    return {
      overview: {
        totalItems,
        totalSize: formatFileSize(totalSize),
        totalDuration: formatDuration(items.reduce((sum: number, item: CollectionItem) => sum + (item.duration || 0), 0)),
        averageRating
      },
      mediaBreakdown: {
        music: mediaTypes.music || 0,
        video: mediaTypes.video || 0,
        image: mediaTypes.image || 0,
        document: mediaTypes.document || 0
      },
      topItems
    }
  }, [items])

  const handleExportAnalytics = (format: 'csv' | 'json' | 'pdf') => {
    const data = {
      collection: collection.name,
      generated: new Date().toISOString(),
      analytics: analyticsData
    }

    if (format === 'json') {
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `collection-analytics-${collection.name}-${Date.now()}.json`
      a.click()
      URL.revokeObjectURL(url)
    } else if (format === 'csv') {
      const rating = analyticsData.overview.averageRating === null ? 'Unavailable' : analyticsData.overview.averageRating.toFixed(2)
      const csv = `Metric,Value\nTotal Items,${analyticsData.overview.totalItems}\nTotal Size,${analyticsData.overview.totalSize}\nAverage Rating,${rating}`
      const blob = new Blob([csv], { type: 'text/csv' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `collection-analytics-${collection.name}-${Date.now()}.csv`
      a.click()
      URL.revokeObjectURL(url)
    }
  }

  const StatCard = ({ title, value, icon: Icon }: {
    title: string
    value: string | number
    icon: React.ComponentType<{ className?: string }>
  }) => (
    <motion.div
      whileHover={{ scale: 1.02 }}
      className="bg-white dark:bg-gray-800 rounded-lg p-6 shadow-sm border border-gray-200 dark:border-gray-700"
    >
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm text-gray-600 dark:text-gray-400 mb-1">{title}</p>
          <p className="text-2xl font-bold text-gray-900 dark:text-white">{value}</p>
        </div>
        <div className="bg-blue-50 dark:bg-blue-900/20 p-3 rounded-lg">
          <Icon className="w-6 h-6 text-blue-600 dark:text-blue-400" />
        </div>
      </div>
    </motion.div>
  )

  const UsageUnavailable = ({ what }: { what: string }) => (
    <Card className="p-6" data-testid="usage-unavailable">
      <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">{what} unavailable</h3>
      <p className="text-gray-600 dark:text-gray-400">
        The server does not record play, download, share or engagement statistics for collections, so no figures are shown.
      </p>
    </Card>
  )

  const renderOverview = () => (
    <div className="space-y-6">
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard title="Total Items" value={analyticsData.overview.totalItems.toLocaleString()} icon={BarChart3} />
        <StatCard title="Total Plays" value="Unavailable" icon={Play} />
        <StatCard title="Downloads" value="Unavailable" icon={Download} />
        <StatCard title="Shares" value="Unavailable" icon={Share2} />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card className="p-6">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Media Breakdown</h3>
          <div className="space-y-3">
            {Object.entries(analyticsData.mediaBreakdown).map(([type, count]) => {
              const Icon = MEDIA_ICONS[type as keyof typeof MEDIA_ICONS]
              const total = Object.values(analyticsData.mediaBreakdown).reduce((a, b) => a + b, 0)
              const percentage = total > 0 ? (count / total) * 100 : 0

              return (
                <div key={type} className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="bg-gray-100 dark:bg-gray-700 p-2 rounded">
                      <Icon className="w-4 h-4 text-gray-600 dark:text-gray-400" />
                    </div>
                    <div>
                      <p className="font-medium text-gray-900 dark:text-white capitalize">{type}</p>
                      <p className="text-sm text-gray-500 dark:text-gray-400">{count} items</p>
                    </div>
                  </div>
                  <div className="text-right">
                    <p className="font-semibold text-gray-900 dark:text-white">{percentage.toFixed(1)}%</p>
                    <div className="w-24 bg-gray-200 dark:bg-gray-700 rounded-full h-2 mt-1">
                      <div className="bg-blue-500 h-2 rounded-full" style={{ width: `${percentage}%` }} />
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        </Card>

        <Card className="p-6">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Collection Stats</h3>
          <div className="space-y-4">
            <div className="flex justify-between items-center">
              <span className="text-gray-600 dark:text-gray-400">Total Size</span>
              <span className="font-semibold text-gray-900 dark:text-white">{analyticsData.overview.totalSize}</span>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-gray-600 dark:text-gray-400">Total Duration</span>
              <span className="font-semibold text-gray-900 dark:text-white">{analyticsData.overview.totalDuration}</span>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-gray-600 dark:text-gray-400">Average Rating</span>
              <div className="flex items-center gap-2">
                <Star className="w-4 h-4 text-yellow-500 fill-current" />
                <span className="font-semibold text-gray-900 dark:text-white">
                  {analyticsData.overview.averageRating === null ? 'Unavailable' : analyticsData.overview.averageRating.toFixed(1)}
                </span>
              </div>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-gray-600 dark:text-gray-400">Total Views</span>
              <span className="font-semibold text-gray-900 dark:text-white">Unavailable</span>
            </div>
          </div>
        </Card>
      </div>

      {renderTopRated(5)}
    </div>
  )

  const renderTopRated = (limit?: number) => (
    <Card className="p-6">
      <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Top Rated Items</h3>
      {analyticsData.topItems.length === 0 ? (
        <p className="text-gray-500 dark:text-gray-400">No rated items in this collection.</p>
      ) : (
        <div className="space-y-3">
          {analyticsData.topItems.slice(0, limit).map((item, index) => {
            const IconComponent = MEDIA_ICONS[item.type as keyof typeof MEDIA_ICONS] || FileText
            return (
              <div key={item.id} className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                <div className="flex items-center gap-3">
                  <div className="w-8 h-8 bg-blue-100 dark:bg-blue-900 rounded-full flex items-center justify-center text-sm font-semibold text-blue-600 dark:text-blue-400">
                    {index + 1}
                  </div>
                  <div>
                    <p className="font-medium text-gray-900 dark:text-white truncate max-w-xs">{item.title}</p>
                    <div className="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
                      <IconComponent className="w-3 h-3" />
                      <span className="capitalize">{item.type}</span>
                      <span>{item.size}</span>
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-1 text-sm">
                  <Star className="w-3 h-3 text-yellow-500 fill-current" />
                  <span className="text-gray-500 dark:text-gray-400">{item.rating.toFixed(1)}</span>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </Card>
  )

  const renderActivity = () => <UsageUnavailable what="Activity history" />
  const renderSharing = () => <UsageUnavailable what="Sharing statistics" />
  const renderContent = () => <div className="space-y-6">{renderTopRated()}</div>
  const renderEngagement = () => <UsageUnavailable what="Engagement metrics" />

  const tabs = [
    { id: 'overview', label: 'Overview', icon: BarChart3 },
    { id: 'activity', label: 'Activity', icon: Activity },
    { id: 'content', label: 'Content', icon: FileVideo },
    { id: 'sharing', label: 'Sharing', icon: Share2 },
    { id: 'engagement', label: 'Engagement', icon: Users }
  ]

  return (
    <motion.div
      initial={{ opacity: 0, scale: 0.95 }}
      animate={{ opacity: 1, scale: 1 }}
      exit={{ opacity: 0, scale: 0.95 }}
      className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      onClick={onClose}
    >
      <motion.div
        initial={{ y: 20 }}
        animate={{ y: 0 }}
        className="bg-white dark:bg-gray-900 rounded-xl shadow-2xl max-w-6xl w-full max-h-[90vh] overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-700">
          <div>
            <h2 className="text-2xl font-bold text-gray-900 dark:text-white">Collection Analytics</h2>
            <p className="text-gray-600 dark:text-gray-400">{collection.name}</p>
          </div>
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-2">
              <Button
                variant="ghost"
                size="sm"
                onClick={() => handleExportAnalytics('csv')}
              >
                <DownloadIcon className="w-4 h-4" />
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => handleExportAnalytics('json')}
              >
                <Upload className="w-4 h-4" />
              </Button>
            </div>
            <Button
              variant="ghost"
              size="sm"
              onClick={onClose}
            >
              <X className="w-4 h-4" />
            </Button>
          </div>
        </div>

        {/* Tab Navigation */}
        <div className="border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center px-6 overflow-x-auto">
            {tabs.map((tab) => {
              const Icon = tab.icon
              return (
                <button
                  key={tab.id}
                  onClick={() => setActiveTab(tab.id as 'overview' | 'activity' | 'content' | 'sharing' | 'engagement')}
                  className={`flex items-center gap-2 px-4 py-3 border-b-2 transition-colors whitespace-nowrap ${
                    activeTab === tab.id
                      ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                      : 'border-transparent text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white'
                  }`}
                >
                  <Icon className="w-4 h-4" />
                  {tab.label}
                </button>
              )
            })}
          </div>
        </div>

        {/* Content */}
        <div className="overflow-y-auto p-6 max-h-[calc(90vh-140px)]">
          {itemsLoading ? (
            <div className="flex items-center justify-center h-64">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
            </div>
          ) : (
            <>
              {activeTab === 'overview' && renderOverview()}
              {activeTab === 'activity' && renderActivity()}
              {activeTab === 'content' && renderContent()}
              {activeTab === 'sharing' && renderSharing()}
              {activeTab === 'engagement' && renderEngagement()}
            </>
          )}
        </div>
      </motion.div>
    </motion.div>
  )
}