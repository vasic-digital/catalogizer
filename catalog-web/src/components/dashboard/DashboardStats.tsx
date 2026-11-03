import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { 
  Database, 
  HardDrive, 
  PlusCircle, 
  Zap,
  TrendingUp,
  TrendingDown,
  Users,
  PlayCircle,
  Clock
} from 'lucide-react'

interface StatCardProps {
  title: string
  value: string | number
  icon: React.ComponentType<{ className?: string }>
  trend?: {
    value: number
    isPositive: boolean
  }
  description?: string
  loading?: boolean
}

/**
 * StatCard renders a single dashboard statistic with an icon, value, optional trend indicator, and description.
 */
export const StatCard: React.FC<StatCardProps> = ({
  title,
  value,
  icon: Icon,
  trend,
  description,
  loading = false
}) => {
  return (
    <Card className="relative overflow-hidden">
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">{title}</CardTitle>
        <Icon className="h-4 w-4 text-muted-foreground" />
      </CardHeader>
      <CardContent>
        <div className="text-2xl font-bold">
          {loading ? (
            <div className="h-8 w-20 bg-gray-200 animate-pulse rounded" />
          ) : (
            value
          )}
        </div>
        {(trend || description) && (
          <div className="flex items-center space-x-2 text-xs text-muted-foreground mt-1">
            {trend && (
              <div className={`flex items-center space-x-1 ${
                trend.isPositive ? 'text-green-600' : 'text-red-600'
              }`}>
                {trend.isPositive ? (
                  <TrendingUp className="h-3 w-3" />
                ) : (
                  <TrendingDown className="h-3 w-3" />
                )}
                <span>{Math.abs(trend.value)}%</span>
              </div>
            )}
            {description && (
              <span>{description}</span>
            )}
          </div>
        )}
      </CardContent>
      
      {/* Decorative background pattern */}
      <div className="absolute top-0 right-0 -z-10 opacity-10">
        <Icon className="h-16 w-16" />
      </div>
    </Card>
  )
}

interface DashboardStatsProps {
  mediaStats?: {
    total_items: number
    total_size: number
    recent_additions: number
    by_type: Record<string, number>
    by_quality: Record<string, number>
  }
  userStats?: {
    active_users: number
    total_users: number
    sessions_today: number
    avg_session_duration: number
  }
  loading?: boolean
}

/**
 * DashboardStats displays an overview of media library and user statistics
 * as a row of StatCard components.
 */
// Most common quality bucket from real counts; 'Unavailable' when none are known.
function dominantQuality(byQuality?: Record<string, number>): string {
  const entries = Object.entries(byQuality ?? {}).filter(([, n]) => n > 0)
  if (entries.length === 0) return 'Unavailable'
  return entries.sort((a, b) => b[1] - a[1])[0][0]
}

export const DashboardStats: React.FC<DashboardStatsProps> = ({
  mediaStats,
  userStats,
  loading = false
}) => {
  const formatBytes = (bytes: number) => {
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    if (bytes === 0) return '0 B'
    const i = Math.floor(Math.log(bytes) / Math.log(1024))
    return `${Math.round(bytes / Math.pow(1024, i) * 100) / 100} ${sizes[i]}`
  }

  const formatDuration = (minutes: number) => {
    if (minutes < 60) return `${minutes}m`
    const hours = Math.floor(minutes / 60)
    const mins = minutes % 60
    return `${hours}h ${mins}m`
  }

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
      {/* Total Media Items */}
      <StatCard
        title="Total Media"
        value={mediaStats?.total_items?.toLocaleString() ?? '0'}
        icon={Database}
        loading={loading}
      />
      
      {/* Storage Used */}
      <StatCard
        title="Storage Used"
        value={formatBytes(mediaStats?.total_size || 0)}
        icon={HardDrive}
        loading={loading}
      />
      
      {/* Recent Additions */}
      <StatCard
        title="Recent Additions"
        value={mediaStats?.recent_additions || 0}
        icon={PlusCircle}
        description="last 7 days"
        loading={loading}
      />
      
      {/* Average Quality Score */}
      <StatCard
        title="Quality Score"
        value={dominantQuality(mediaStats?.by_quality)}
        icon={Zap}
        description="average quality"
        loading={loading}
      />
      
      {/* Active Users */}
      <StatCard
        title="Active Users"
        value={userStats?.active_users?.toLocaleString() ?? 'Unavailable'}
        icon={Users}
        description="online now"
        loading={loading}
      />
      
      {/* Total Users */}
      <StatCard
        title="Total Users"
        value={userStats?.total_users?.toLocaleString() ?? 'Unavailable'}
        icon={Users}
        description="registered users"
        loading={loading}
      />
      
      {/* Sessions Today */}
      <StatCard
        title="Sessions Today"
        value={userStats?.sessions_today?.toLocaleString() ?? 'Unavailable'}
        icon={PlayCircle}
        description="viewing sessions"
        loading={loading}
      />
      
      {/* Avg Session Duration */}
      <StatCard
        title="Avg Session"
        value={userStats?.avg_session_duration != null ? formatDuration(userStats.avg_session_duration) : 'Unavailable'}
        icon={Clock}
        description="viewing duration"
        loading={loading}
      />
    </div>
  )
}