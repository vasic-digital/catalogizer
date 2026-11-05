import { useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { 
  Activity, 
  Play, 
  Upload, 
  Download, 
  Users, 
  Settings,
  Eye
} from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import type { ActivityItem } from '@/types/dashboard'

interface ActivityFeedProps {
  limit?: number
  showFilters?: boolean
  /**
   * Real activity items supplied by the caller. When omitted the feed shows an
   * "unavailable" state: the server exposes no activity endpoint yet, and the
   * feed never invents sample activity (CAT-001).
   */
  activities?: ActivityItem[]
}

const ACTIVITY_ICONS = {
  media_played: Play,
  media_uploaded: Upload,
  media_downloaded: Download,
  user_login: Users,
  system_event: Settings,
  media_viewed: Eye
}

const ACTIVITY_COLORS = {
  media_played: 'text-green-600 bg-green-100',
  media_uploaded: 'text-blue-600 bg-blue-100',
  media_downloaded: 'text-purple-600 bg-purple-100',
  user_login: 'text-orange-600 bg-orange-100',
  system_event: 'text-gray-600 bg-gray-100',
  media_viewed: 'text-indigo-600 bg-indigo-100'
}

const ACTIVITY_MESSAGES = {
  media_played: (data: Record<string, unknown>) => `Started watching "${data.title as string}"`,
  media_uploaded: (data: Record<string, unknown>) => `Uploaded "${data.title as string}"`,
  media_downloaded: (data: Record<string, unknown>) => `Downloaded "${data.title as string}"`,
  user_login: (data: Record<string, unknown>) => `User "${data.username as string}" logged in`,
  system_event: (data: Record<string, unknown>) => data.message as string,
  media_viewed: (data: Record<string, unknown>) => `Viewed "${data.title as string}"`
}

/**
 * ActivityFeed displays a chronological list of recent user and system
 * activities such as media plays, uploads, and logins.
 */
export const ActivityFeed: React.FC<ActivityFeedProps> = ({
  limit = 10,
  showFilters = true,
  activities
}) => {
  const [filter, setFilter] = useState<string>('all')

  if (activities === undefined) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Activity className="h-5 w-5" />
            Recent Activity
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex flex-col items-center justify-center py-12" data-testid="activity-unavailable">
            <Activity className="h-12 w-12 text-gray-400 mb-4" />
            <p className="text-gray-500 dark:text-gray-400 text-center">Activity feed unavailable</p>
            <p className="text-sm text-gray-500 dark:text-gray-400 text-center">
              The server does not provide an activity feed yet.
            </p>
          </div>
        </CardContent>
      </Card>
    )
  }

  // Filter activities
  const filteredActivities = activities.filter(activity => {
    if (filter === 'all') return true
    return activity.type === filter
  }).slice(0, limit)

  const ActivityItem: React.FC<{ activity: ActivityItem }> = ({ activity }) => {
    const Icon = ACTIVITY_ICONS[activity.type] || Activity
    const colorClass = ACTIVITY_COLORS[activity.type] || ACTIVITY_COLORS.system_event
    const getMessage = ACTIVITY_MESSAGES[activity.type] || ((data: Record<string, unknown>) => data.title as string)

    return (
      <div className="flex items-start space-x-3 p-4 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors">
        <div className={`flex-shrink-0 w-10 h-10 rounded-full flex items-center justify-center ${colorClass}`}>
          <Icon className="w-5 h-5" />
        </div>
        
        <div className="flex-1 min-w-0">
          <div className="flex items-center justify-between">
            <p className="text-sm font-medium text-gray-900 dark:text-white truncate">
              {getMessage(activity.metadata)}
            </p>
            <span className="text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap ml-2">
              {formatDistanceToNow(activity.timestamp, { addSuffix: true })}
            </span>
          </div>
          
          <div className="flex items-center space-x-2 mt-1">
            <span className="text-xs text-gray-500 dark:text-gray-400">
              {activity.user}
            </span>
            
            {/* Additional metadata based on activity type */}
            {activity.type === 'media_played' && (
              <span className="text-xs text-gray-500 dark:text-gray-400">
                • {activity.metadata.quality} • {activity.metadata.duration}
              </span>
            )}
            
            {activity.type === 'media_uploaded' && (
              <span className="text-xs text-gray-500 dark:text-gray-400">
                • {activity.metadata.size} • {activity.metadata.format}
              </span>
            )}
          </div>
        </div>
      </div>
    )
  }

  return (
    <Card>
      <CardHeader className="flex flex-row flex-wrap gap-2 items-center justify-between">
        <CardTitle className="flex items-center gap-2">
          <Activity className="h-5 w-5" />
          Recent Activity
        </CardTitle>
        
        {showFilters && (
          <div className="flex flex-wrap items-center gap-2">
            <Button
              variant={filter === 'all' ? 'default' : 'ghost'}
              size="sm"
              onClick={() => setFilter('all')}
            >
              All
            </Button>
            <Button
              variant={filter === 'media_played' ? 'default' : 'ghost'}
              size="sm"
              onClick={() => setFilter('media_played')}
            >
              Playing
            </Button>
            <Button
              variant={filter === 'media_uploaded' ? 'default' : 'ghost'}
              size="sm"
              onClick={() => setFilter('media_uploaded')}
            >
              Uploads
            </Button>
          </div>
        )}
      </CardHeader>
      
      <CardContent className="p-0">
        {filteredActivities.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12">
            <Activity className="h-12 w-12 text-gray-400 mb-4" />
            <p className="text-gray-500 dark:text-gray-400 text-center">
              No recent activity
              {filter !== 'all' && (
                <span>
                  {' '}in the <span className="font-medium">{filter}</span> category
                </span>
              )}
            </p>
            <p className="text-sm text-gray-500 dark:text-gray-400 text-center">
              Activity will appear here as users interact with media
            </p>
          </div>
        ) : (
          <div className="max-h-96 overflow-y-auto">
            {filteredActivities.map(activity => (
              <ActivityItem key={activity.id} activity={activity} />
            ))}
          </div>
        )}
        
        {activities.length > limit && (
          <div className="p-4 border-t border-gray-100">
            <Button variant="outline" className="w-full">
              View All Activities
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  )
}