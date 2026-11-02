import { useState, useEffect } from 'react'
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
  showFilters = true
}) => {
  const [activities, setActivities] = useState<ActivityItem[]>([])
  const [filter, setFilter] = useState<string>('all')
  const [loading, setLoading] = useState(true)

  // Fetch initial activities
  useEffect(() => {
    const fetchActivities = async () => {
      try {
        setLoading(true)
        // Mock data for now - replace with actual API call
        const mockActivities: ActivityItem[] = [
          {
            id: '1',
            type: 'media_played',
            title: 'Sample Movie.mp4',
            user: 'John Doe',
            timestamp: new Date(Date.now() - 1000 * 60 * 5), // 5 minutes ago
            metadata: {
              title: 'Sample Movie.mp4',
              duration: '2h 15m',
              quality: '1080p'
            }
          },
          {
            id: '2',
            type: 'media_uploaded',
            title: 'New Episode.mkv',
            user: 'Jane Smith',
            timestamp: new Date(Date.now() - 1000 * 60 * 15), // 15 minutes ago
            metadata: {
              title: 'New Episode.mkv',
              size: '1.2GB',
              format: 'MKV'
            }
          },
          {
            id: '3',
            type: 'user_login',
            title: 'User Activity',
            user: 'Mike Johnson',
            timestamp: new Date(Date.now() - 1000 * 60 * 30), // 30 minutes ago
            metadata: {
              username: 'Mike Johnson',
              ip: '192.168.1.100'
            }
          },
          {
            id: '4',
            type: 'system_event',
            title: 'System Update',
            user: 'System',
            timestamp: new Date(Date.now() - 1000 * 60 * 60), // 1 hour ago
            metadata: {
              message: 'Library scan completed successfully'
            }
          }
        ]
        
        setActivities(mockActivities)
      } catch (error) {
        // Activity fetch failure — UI shows empty state
        void error;
      } finally {
        setLoading(false)
      }
    }

    fetchActivities()
  }, [])

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
      <div className="flex items-start space-x-3 p-4 hover:bg-gray-50 transition-colors">
        <div className={`flex-shrink-0 w-10 h-10 rounded-full flex items-center justify-center ${colorClass}`}>
          <Icon className="w-5 h-5" />
        </div>
        
        <div className="flex-1 min-w-0">
          <div className="flex items-center justify-between">
            <p className="text-sm font-medium text-gray-900 truncate">
              {getMessage(activity.metadata)}
            </p>
            <span className="text-xs text-gray-500 whitespace-nowrap ml-2">
              {formatDistanceToNow(activity.timestamp, { addSuffix: true })}
            </span>
          </div>
          
          <div className="flex items-center space-x-2 mt-1">
            <span className="text-xs text-gray-500">
              {activity.user}
            </span>
            
            {/* Additional metadata based on activity type */}
            {activity.type === 'media_played' && (
              <span className="text-xs text-gray-400">
                • {activity.metadata.quality} • {activity.metadata.duration}
              </span>
            )}
            
            {activity.type === 'media_uploaded' && (
              <span className="text-xs text-gray-400">
                • {activity.metadata.size} • {activity.metadata.format}
              </span>
            )}
          </div>
        </div>
      </div>
    )
  }

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Activity className="h-5 w-5" />
            Recent Activity
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="flex items-start space-x-3 animate-pulse">
                <div className="w-10 h-10 bg-gray-200 rounded-full"></div>
                <div className="flex-1 space-y-2">
                  <div className="h-4 bg-gray-200 rounded w-3/4"></div>
                  <div className="h-3 bg-gray-200 rounded w-1/2"></div>
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
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
            <p className="text-gray-500 text-center">
              No recent activity
              {filter !== 'all' && (
                <span>
                  {' '}in the <span className="font-medium">{filter}</span> category
                </span>
              )}
            </p>
            <p className="text-sm text-gray-400 text-center">
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