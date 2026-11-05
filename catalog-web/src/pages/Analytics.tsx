import React from 'react'
import { useQuery } from '@tanstack/react-query'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { mediaApi } from '@/lib/mediaApi'
import { statsApi } from '@/lib/statsApi'
import { analyticsApi } from '@/lib/analyticsApi'
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
  LineChart,
  Line,
  Area,
  AreaChart
} from 'recharts'
import {
  TrendingUp,
  Database,
  HardDrive,
  Star,
  Film,
  Music,
  Gamepad2,
  Monitor,
  Copy,
  FileType,
} from 'lucide-react'
import { motion } from 'framer-motion'

const COLORS = [
  '#3B82F6', '#EF4444', '#10B981', '#F59E0B',
  '#8B5CF6', '#EC4899', '#14B8A6', '#F97316'
]

const StatCard: React.FC<{
  title: string
  value: string
  icon: React.ReactNode
  change?: string
  changeType?: 'positive' | 'negative'
  trend?: number[]
}> = ({ title, value, icon, change, changeType, trend }) => (
  <Card className="relative overflow-hidden">
    <CardContent className="p-6">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm text-gray-600 dark:text-gray-400">{title}</p>
          <p className="text-2xl font-bold text-gray-900 dark:text-white">{value}</p>
          {change && (
            <p className={`text-sm ${
              changeType === 'positive' ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'
            }`}>
              {change}
            </p>
          )}
        </div>
        <div className="text-blue-600 dark:text-blue-400">
          {icon}
        </div>
      </div>
      {trend && (
        <div className="mt-4 h-8">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={trend.map((value, index) => ({ value, index }))}>
              <Line
                type="monotone"
                dataKey="value"
                stroke={changeType === 'positive' ? '#10B981' : '#EF4444'}
                strokeWidth={2}
                dot={false}
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}
    </CardContent>
  </Card>
)

export const Analytics: React.FC = () => {
  const { data: stats, isLoading: statsLoading } = useQuery({
    queryKey: ['media-stats'],
    queryFn: mediaApi.getMediaStats,
    staleTime: 1000 * 60 * 5,
  })

  const { data: recentMedia } = useQuery({
    queryKey: ['recent-media', 20],
    queryFn: () => mediaApi.getRecentMedia(20),
    staleTime: 1000 * 60 * 5,
  })

  const { data: fileTypeStats } = useQuery({
    queryKey: ['stats-filetypes'],
    queryFn: () => statsApi.getFileTypeStats(),
    staleTime: 1000 * 60 * 5,
  })

  const { data: duplicateStats } = useQuery({
    queryKey: ['stats-duplicates'],
    queryFn: () => statsApi.getDuplicateStats(),
    staleTime: 1000 * 60 * 5,
  })

  const { data: apiGrowthTrends } = useQuery({
    queryKey: ['stats-growth-30'],
    queryFn: () => statsApi.getGrowthTrends(30),
    staleTime: 1000 * 60 * 5,
  })

  const { data: systemAnalytics } = useQuery({
    queryKey: ['analytics-system'],
    queryFn: () => analyticsApi.getSystemAnalytics(),
    staleTime: 1000 * 60 * 5,
    retry: false, // admin-only endpoint: a non-admin gets 403 and the panel stays hidden (no retries)
  })

  // Transform stats data for charts
  const mediaTypeData = stats ? Object.entries(stats.by_type).map(([type, count]) => ({
    name: type.replace('_', ' ').toUpperCase(),
    value: count,
    count
  })) : []

  const qualityData = stats ? Object.entries(stats.by_quality).map(([quality, count]) => ({
    name: quality.toUpperCase(),
    value: count,
    count
  })) : []

  // Use real growth trends from API; with none, the charts are empty
  const growthTrend = apiGrowthTrends && apiGrowthTrends.length > 0
    ? apiGrowthTrends.map((t, i) => ({
        day: i + 1,
        items: t.cumulative_files,
        size: Math.round(t.cumulative_size / (1024 ** 2)),
      }))
    : [] // no growth data from the API: show an empty chart, never invented points (CAT-001)

  const weeklyAdditions = apiGrowthTrends && apiGrowthTrends.length >= 7
    ? apiGrowthTrends.slice(-7).map((t, i) => ({
        day: ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'][i],
        additions: t.files_added,
      }))
    : []

  // File type chart data from API
  const fileTypeChartData = fileTypeStats
    ? fileTypeStats.slice(0, 10).map((ft) => ({
        name: ft.extension || 'unknown',
        value: ft.count,
        size: Math.round(ft.total_size / (1024 ** 2)),
      }))
    : []

  if (statsLoading) {
    return (
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="animate-pulse">
          <div className="h-8 bg-gray-300 dark:bg-gray-600 rounded w-1/4 mb-4" />
          <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mb-8">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="h-32 bg-gray-300 dark:bg-gray-600 rounded-xl" />
            ))}
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5 }}
      >
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-gray-900 dark:text-white mb-2">
            Analytics Dashboard
          </h1>
          <p className="text-gray-600 dark:text-gray-400">
            Insights and statistics about your media collection
          </p>
        </div>

        {/* Key Metrics */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
          <StatCard
            title="Total Media Items"
            value={stats?.total_items?.toLocaleString() || '0'}
            icon={<Database className="h-8 w-8" />}
          />
          <StatCard
            title="Storage Used"
            value={`${((stats?.total_size || 0) / (1024 ** 3)).toFixed(1)} GB`}
            icon={<HardDrive className="h-8 w-8" />}
          />
          <StatCard
            title="Recent Additions"
            value={stats?.recent_additions?.toString() || '0'}
            icon={<TrendingUp className="h-8 w-8" />}
          />
          <StatCard
            title="Media Types"
            value={Object.keys(stats?.by_type || {}).length.toString()}
            icon={<Star className="h-8 w-8" />}
          />
        </div>

        {/* Charts Row 1 */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
          {/* Media Types Distribution */}
          <Card>
            <CardHeader>
              <CardTitle>Media Types Distribution</CardTitle>
              <CardDescription>
                Breakdown of your media collection by type
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="h-80">
                <ResponsiveContainer width="100%" height="100%">
                  <PieChart>
                    <Pie
                      data={mediaTypeData}
                      cx="50%"
                      cy="50%"
                      labelLine={false}
                      label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}
                      outerRadius={80}
                      fill="#8884d8"
                      dataKey="value"
                    >
                      {mediaTypeData.map((entry, index) => (
                        <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} aria-label={`${entry.name}: ${entry.value}`} />
                      ))}
                    </Pie>
                    <Tooltip formatter={(value) => [value, 'Items']} />
                  </PieChart>
                </ResponsiveContainer>
              </div>
            </CardContent>
          </Card>

          {/* Quality Distribution */}
          <Card>
            <CardHeader>
              <CardTitle>Quality Distribution</CardTitle>
              <CardDescription>
                Media items by quality level
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="h-80">
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart data={qualityData}>
                    <CartesianGrid strokeDasharray="3 3" />
                    <XAxis dataKey="name" />
                    <YAxis />
                    <Tooltip formatter={(value) => [value, 'Items']} />
                    <Bar dataKey="value" fill="#3B82F6" />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Charts Row 2 */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
          {/* Growth Trend */}
          <Card>
            <CardHeader>
              <CardTitle>Collection Growth</CardTitle>
              <CardDescription>
                Media items and storage growth over time
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="h-80">
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={growthTrend}>
                    <CartesianGrid strokeDasharray="3 3" />
                    <XAxis dataKey="day" />
                    <YAxis />
                    <Tooltip />
                    <Area
                      type="monotone"
                      dataKey="items"
                      stackId="1"
                      stroke="#3B82F6"
                      fill="#3B82F6"
                      fillOpacity={0.6}
                    />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            </CardContent>
          </Card>

          {/* Weekly Additions */}
          <Card>
            <CardHeader>
              <CardTitle>Weekly Activity</CardTitle>
              <CardDescription>
                New media items added each day this week
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="h-80">
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart data={weeklyAdditions}>
                    <CartesianGrid strokeDasharray="3 3" />
                    <XAxis dataKey="day" />
                    <YAxis />
                    <Tooltip formatter={(value) => [value, 'Items Added']} />
                    <Bar dataKey="additions" fill="#10B981" />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* File Types & Duplicates Row */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
          {/* File Type Distribution */}
          {fileTypeChartData.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <FileType className="h-5 w-5" />
                  File Types
                </CardTitle>
                <CardDescription>
                  Top file extensions in your collection
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="h-80">
                  <ResponsiveContainer width="100%" height="100%">
                    <BarChart data={fileTypeChartData} layout="vertical">
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis type="number" />
                      <YAxis dataKey="name" type="category" width={60} />
                      <Tooltip formatter={(value, name) => [value, name === 'value' ? 'Files' : 'Size (MB)']} />
                      <Bar dataKey="value" fill="#8B5CF6" />
                    </BarChart>
                  </ResponsiveContainer>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Duplicate & System Stats */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Copy className="h-5 w-5" />
                Storage Insights
              </CardTitle>
              <CardDescription>
                Duplicate detection and system analytics
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="space-y-6">
                {duplicateStats && (
                  <div>
                    <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Duplicates</h4>
                    <div className="grid grid-cols-3 gap-3">
                      <div className="p-3 rounded-lg bg-gray-50 dark:bg-gray-800 text-center">
                        <div className="text-xl font-bold text-gray-900 dark:text-white">
                          {duplicateStats.total_duplicate_groups}
                        </div>
                        <div className="text-xs text-gray-500">Groups</div>
                      </div>
                      <div className="p-3 rounded-lg bg-gray-50 dark:bg-gray-800 text-center">
                        <div className="text-xl font-bold text-gray-900 dark:text-white">
                          {duplicateStats.total_duplicate_files}
                        </div>
                        <div className="text-xs text-gray-500">Files</div>
                      </div>
                      <div className="p-3 rounded-lg bg-gray-50 dark:bg-gray-800 text-center">
                        <div className="text-xl font-bold text-orange-600">
                          {((duplicateStats.wasted_space || 0) / (1024 ** 3)).toFixed(1)} GB
                        </div>
                        <div className="text-xs text-gray-500">Wasted</div>
                      </div>
                    </div>
                  </div>
                )}
                {systemAnalytics && (
                  <div>
                    <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">System Activity</h4>
                    <div className="grid grid-cols-2 gap-3">
                      <div className="p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
                        <div className="text-xl font-bold text-gray-900 dark:text-white">
                          {systemAnalytics.total_events.toLocaleString()}
                        </div>
                        <div className="text-xs text-gray-500">Total Events</div>
                      </div>
                      <div className="p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
                        <div className="text-xl font-bold text-green-600">
                          {systemAnalytics.active_users_24h}
                        </div>
                        <div className="text-xs text-gray-500">Active Users (24h)</div>
                      </div>
                    </div>
                    {systemAnalytics.popular_media && systemAnalytics.popular_media.length > 0 && (
                      <div className="mt-3">
                        <h5 className="text-xs font-medium text-gray-500 mb-2">Most Popular</h5>
                        <div className="space-y-1">
                          {systemAnalytics.popular_media.slice(0, 5).map((m) => (
                            <div key={m.media_id} className="flex items-center justify-between text-sm">
                              <span className="text-gray-900 dark:text-white truncate flex-1">{m.title}</span>
                              <span className="text-gray-500 ml-2">{m.access_count} views</span>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Recent Activity */}
        <Card>
          <CardHeader>
            <CardTitle>Recently Added Media</CardTitle>
            <CardDescription>
              Your latest additions to the collection
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {recentMedia?.slice(0, 10).map((item) => {
                const getIcon = (type: string) => {
                  switch (type.toLowerCase()) {
                    case 'movie':
                    case 'tv_show':
                      return <Film className="h-4 w-4" />
                    case 'music':
                      return <Music className="h-4 w-4" />
                    case 'game':
                      return <Gamepad2 className="h-4 w-4" />
                    case 'software':
                      return <Monitor className="h-4 w-4" />
                    default:
                      return <Database className="h-4 w-4" />
                  }
                }

                return (
                  <div key={item.id} className="flex items-center space-x-4 p-3 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors">
                    <div className="text-blue-600 dark:text-blue-400">
                      {getIcon(item.media_type ?? '')}
                    </div>
                    <div className="flex-1">
                      <div className="flex items-center space-x-2">
                        <span className="font-medium text-gray-900 dark:text-white">
                          {item.title}
                        </span>
                        {item.year && (
                          <span className="text-sm text-gray-500 dark:text-gray-400">
                            ({item.year})
                          </span>
                        )}
                      </div>
                      <div className="flex items-center space-x-4 text-sm text-gray-600 dark:text-gray-400">
                        <span className="px-2 py-1 bg-blue-100 text-blue-800 rounded-full text-xs dark:bg-blue-900 dark:text-blue-200">
                          {item.media_type?.replace('_', ' ')}
                        </span>
                        {item.quality && (
                          <span>{item.quality.toUpperCase()}</span>
                        )}
                        {item.file_size && (
                          <span>{((item.file_size || 0) / (1024 ** 2)).toFixed(0)} MB</span>
                        )}
                      </div>
                    </div>
                    <div className="text-xs text-gray-500 dark:text-gray-400">
                      {item.created_at ? new Date(item.created_at).toLocaleDateString() : 'Unknown'}
                    </div>
                  </div>
                )
              })}
            </div>
          </CardContent>
        </Card>
      </motion.div>
    </div>
  )
}