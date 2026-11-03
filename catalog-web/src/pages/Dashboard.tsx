import React, { lazy, Suspense, useState, useEffect } from 'react'
import { useAuth } from '@/contexts/AuthContext'
import { useQuery } from '@tanstack/react-query'
import { DashboardStats } from '@/components/dashboard/DashboardStats'
import { ActivityFeed } from '@/components/dashboard/ActivityFeed'

// Lazy-loaded chart component (pulls in heavy recharts library)
const MediaDistributionChart = lazy(() => import('@/components/dashboard/MediaDistributionChart').then(m => ({ default: m.MediaDistributionChart })))
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { mediaApi, entityApi } from '@/lib/mediaApi'
import { adminApi } from '@/lib/adminApi'
import { useNavigate } from 'react-router-dom'
import toast from 'react-hot-toast'
import { statsApi } from '@/lib/statsApi'
import {
  Film,
  Upload,
  Search,
  Settings,
  Activity,
  Clock,
  Tv,
  Music,
  Gamepad2,
  Monitor,
  BookOpen,
  Book,
  HardDrive,
  FolderTree,
  Copy,
  TrendingUp,
  FileType,
} from 'lucide-react'
import { motion } from 'framer-motion'
import type { QuickAction } from '@/types/dashboard'

const QuickActions: React.FC = () => {
  const navigate = useNavigate()

  // CAT-001: each action really navigates; nothing claims success it did not achieve.
  const handleUploadMedia = () => navigate('/media')
  const handleSearchMedia = () => navigate('/media')
  const handleSettings = () => navigate('/settings')

  const quickActions: Array<QuickAction & { disabledReason?: string }> = [
    {
      id: 'upload',
      title: 'Upload Media',
      description: 'Add new media to your library',
      icon: Upload,
      action: handleUploadMedia,
      variant: 'default'
    },
    {
      id: 'scan',
      title: 'Scan Library',
      description: 'Update media library with new files',
      icon: Activity,
      action: () => undefined,
      variant: 'outline',
      disabledReason: 'A scan needs a storage root and cannot be started from here yet.'
    },
    {
      id: 'search',
      title: 'Search',
      description: 'Find specific media quickly',
      icon: Search,
      action: handleSearchMedia,
      variant: 'outline'
    },
    {
      id: 'settings',
      title: 'Settings',
      description: 'Configure system preferences',
      icon: Settings,
      action: handleSettings,
      variant: 'outline'
    }
  ]

  return (
    <Card>
      <CardHeader>
        <CardTitle>Quick Actions</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          {quickActions.map(action => {
            const Icon = action.icon
            return (
              <div key={action.id} className="flex flex-col">
                <Button
                  variant={action.variant}
                  onClick={action.action}
                  disabled={!!action.disabledReason}
                  title={action.disabledReason}
                  className="h-auto p-4 flex flex-col items-center space-y-2"
                >
                  <Icon className="w-6 h-6" />
                  <span className="text-sm font-medium">{action.title}</span>
                </Button>
                {action.disabledReason && (
                  <span className="mt-1 text-xs text-gray-500 dark:text-gray-400">{action.disabledReason}</span>
                )}
              </div>
            )
          })}
        </div>
      </CardContent>
    </Card>
  )
}

const formatUptime = (seconds: number | null | undefined): string => {
  if (typeof seconds !== 'number' || !Number.isFinite(seconds) || seconds < 0) return 'Unavailable'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  return `${d}d ${h}h ${m}m`
}

const pct = (v: number | null | undefined): number | null =>
  typeof v === 'number' && Number.isFinite(v) ? Math.max(0, Math.min(100, Math.round(v))) : null

const SystemStatus: React.FC = () => {
  // CAT-001: real figures from the server (admin endpoint); Unavailable when it cannot supply them.
  const { data: info } = useQuery({
    queryKey: ['dashboard-system-info'],
    queryFn: () => adminApi.getSystemInfo(),
    retry: false,
    staleTime: 1000 * 60,
  })
  const disk = info?.diskUsage && info.diskUsage.total > 0 ? (info.diskUsage.used / info.diskUsage.total) * 100 : null
  const status = {
    cpu: pct(info?.cpuUsage),
    memory: pct(info?.memoryUsage),
    disk: pct(disk),
    uptime: formatUptime(info?.uptime),
  }
  const show = (v: number | null) => (v === null ? 'Unavailable' : `${v}%`)
  const color = (v: number | null) => (v === null ? 'bg-gray-300' : v > 80 ? 'bg-red-500' : v > 60 ? 'bg-yellow-500' : 'bg-green-500')

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Activity className="h-5 w-5" />
          System Status
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          {([['CPU Usage', status.cpu], ['API Process Memory (heap in use / reserved)', status.memory], ['Disk Usage', status.disk]] as const).map(([label, value]) => (
            <div key={label} className="space-y-2">
              <div className="flex justify-between text-sm">
                <span>{label}</span>
                <span>{show(value)}</span>
              </div>
              <div className="w-full bg-gray-200 rounded-full h-2">
                <div className={`h-2 rounded-full ${color(value)}`} style={{ width: `${value ?? 0}%` }} />
              </div>
            </div>
          ))}
          <div className="flex justify-between text-sm pt-2 border-t">
            <span>Uptime</span>
            <span>{status.uptime}</span>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

export const Dashboard: React.FC = () => {
  const { user } = useAuth()

  // Fetch media statistics
  const { 
    data: mediaStats, 
    isLoading: mediaLoading, 
    error: mediaError 
  } = useQuery(
    ['media-stats'],
    () => mediaApi.getMediaStats(),
    {
      refetchInterval: 30000, // Refresh every 30 seconds
      staleTime: 10000
    }
  )

  // Fetch entity statistics
  const { data: entityStats } = useQuery(
    ['entity-stats'],
    () => entityApi.getEntityStats(),
    {
      refetchInterval: 30000,
      staleTime: 10000
    }
  )

  // Fetch detailed statistics from stats API
  const { data: overallStats } = useQuery({
    queryKey: ['stats-overall'],
    queryFn: () => statsApi.getOverallStats(),
    staleTime: 30000,
  })

  const { data: duplicateStats } = useQuery({
    queryKey: ['stats-duplicates'],
    queryFn: () => statsApi.getDuplicateStats(),
    staleTime: 60000,
  })

  const { data: growthTrends } = useQuery({
    queryKey: ['stats-growth'],
    queryFn: () => statsApi.getGrowthTrends(7),
    staleTime: 60000,
  })

  const { data: scanHistory } = useQuery({
    queryKey: ['stats-scans'],
    queryFn: () => statsApi.getScanHistory(5),
    staleTime: 30000,
  })

  // The server has no user/session statistics endpoint; the old hard-coded numbers
  // (12 users, 24 sessions) were invented, so none are passed (CAT-001).

  // Handle errors
  useEffect(() => {
    if (mediaError) {
      toast.error('Failed to load media statistics')
    }
  }, [mediaError])

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-6">
      {/* Header */}
      <motion.div
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        className="flex items-center justify-between"
      >
        <div>
          <h1 className="text-3xl font-bold text-gray-900 dark:text-white">
            Welcome back, {user?.username || 'User'}!
          </h1>
          <p className="text-gray-600 dark:text-gray-400">
            Here&apos;s what&apos;s happening with your media library today.
          </p>
        </div>
        
        <div className="flex items-center space-x-2">
          <Button variant="outline">
            <Clock className="w-4 h-4 mr-2" />
            Last updated: {new Date().toLocaleTimeString()}
          </Button>
        </div>
      </motion.div>

      {/* Main Stats Grid */}
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.1 }}
      >
        <DashboardStats
          mediaStats={mediaStats}
          loading={mediaLoading}
        />
      </motion.div>

      {/* Entity Overview */}
      {entityStats && entityStats.by_type && Object.keys(entityStats.by_type).length > 0 && (
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.15 }}
        >
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Film className="h-5 w-5" />
                Media Entities ({entityStats.total_entities} total)
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-3">
                {Object.entries(entityStats.by_type as Record<string, number>)
                  .filter(([, count]) => count > 0)
                  .sort(([, a], [, b]) => b - a)
                  .map(([type, count]) => {
                    const iconMap: Record<string, React.ElementType> = {
                      movie: Film, tv_show: Tv, music_artist: Music, music_album: Music,
                      song: Music, game: Gamepad2, software: Monitor, book: BookOpen, comic: Book,
                    }
                    const TypeIcon = iconMap[type] || Film
                    return (
                      <div key={type} className="flex items-center gap-3 p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
                        <TypeIcon className="h-5 w-5 text-gray-500 flex-shrink-0" />
                        <div>
                          <div className="text-lg font-semibold text-gray-900 dark:text-white">{count}</div>
                          <div className="text-xs text-gray-500 capitalize">{type.replace(/_/g, ' ')}</div>
                        </div>
                      </div>
                    )
                  })}
              </div>
            </CardContent>
          </Card>
        </motion.div>
      )}

      {/* Storage & Scan Statistics */}
      {(overallStats || duplicateStats || scanHistory) && (
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.17 }}
        >
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <HardDrive className="h-5 w-5" />
                Storage Statistics
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-4">
                {overallStats && (
                  <>
                    <div className="p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
                      <FolderTree className="h-4 w-4 text-gray-500 mb-1" />
                      <div className="text-lg font-semibold text-gray-900 dark:text-white">
                        {(overallStats.total_directories ?? 0).toLocaleString()}
                      </div>
                      <div className="text-xs text-gray-500">Directories</div>
                    </div>
                    <div className="p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
                      <FileType className="h-4 w-4 text-gray-500 mb-1" />
                      <div className="text-lg font-semibold text-gray-900 dark:text-white">
                        {(overallStats.total_files ?? 0).toLocaleString()}
                      </div>
                      <div className="text-xs text-gray-500">Total Files</div>
                    </div>
                    <div className="p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
                      <HardDrive className="h-4 w-4 text-gray-500 mb-1" />
                      <div className="text-lg font-semibold text-gray-900 dark:text-white">
                        {((overallStats.total_size || 0) / (1024 ** 3)).toFixed(1)} GB
                      </div>
                      <div className="text-xs text-gray-500">Total Size</div>
                    </div>
                  </>
                )}
                {duplicateStats && (
                  <div className="p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
                    <Copy className="h-4 w-4 text-gray-500 mb-1" />
                    <div className="text-lg font-semibold text-gray-900 dark:text-white">
                      {duplicateStats.total_duplicate_groups}
                    </div>
                    <div className="text-xs text-gray-500">Duplicate Groups</div>
                  </div>
                )}
                {growthTrends && growthTrends.length > 0 && (
                  <div className="p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
                    <TrendingUp className="h-4 w-4 text-gray-500 mb-1" />
                    <div className="text-lg font-semibold text-gray-900 dark:text-white">
                      +{growthTrends.reduce((sum, t) => sum + t.files_added, 0)}
                    </div>
                    <div className="text-xs text-gray-500">Files This Week</div>
                  </div>
                )}
              </div>
              {scanHistory && scanHistory.length > 0 && (
                <div className="mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">
                  <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Recent Scans</h4>
                  <div className="space-y-2">
                    {scanHistory.slice(0, 3).map((scan) => (
                      <div key={scan.id} className="flex items-center justify-between text-sm">
                        <span className="text-gray-900 dark:text-white">{scan.storage_root_name}</span>
                        <div className="flex items-center gap-3 text-gray-500">
                          <span>{scan.files_found} files</span>
                          <span className={scan.status === 'completed' ? 'text-green-600' : scan.status === 'failed' ? 'text-red-600' : ''}>
                            {scan.status}
                          </span>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </CardContent>
          </Card>
        </motion.div>
      )}

      {/* Charts and Activity Row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 [overflow-x:clip]">
        {/* Media Distribution Chart */}
        <motion.div
          initial={{ opacity: 0, x: -20 }}
          animate={{ opacity: 1, x: 0 }}
          transition={{ delay: 0.2 }}
        >
          <Suspense fallback={<div className="h-80 bg-gray-100 dark:bg-gray-800 rounded-lg animate-pulse" />}>
            <MediaDistributionChart
              data={mediaStats?.by_type}
              loading={mediaLoading}
            />
          </Suspense>
        </motion.div>

        {/* System Status */}
        <motion.div
          initial={{ opacity: 0, x: 20 }}
          animate={{ opacity: 1, x: 0 }}
          transition={{ delay: 0.2 }}
        >
          <SystemStatus />
        </motion.div>
      </div>

      {/* Activity Feed and Quick Actions */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Activity Feed */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.3 }}
          className="lg:col-span-2"
        >
          <ActivityFeed limit={8} />
        </motion.div>

        {/* Quick Actions */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.4 }}
        >
          <QuickActions />
        </motion.div>
      </div>
    </div>
  )
}