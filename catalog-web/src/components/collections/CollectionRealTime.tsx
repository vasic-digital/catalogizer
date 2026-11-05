import React, { useState, useEffect, useCallback, useRef } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
  Wifi,
  WifiOff,
  Bell,
  BellOff,
  RefreshCw,
  UserPlus,
  UserMinus,
  FilePlus,
  FileMinus,
  Edit,
  Trash2,
  Download,
  Play,
  Eye,
  EyeOff,
  MessageSquare,
  Heart,
  Share2,
  Activity,
  X
} from 'lucide-react'
import { Button } from '../ui/Button'
import { Switch } from '../ui/Switch'
import { Card } from '../ui/Card'
import { SmartCollection } from '../../types/collections'
import { toast } from 'react-hot-toast'

interface CollectionRealTimeProps {
  collection: SmartCollection
  onClose?: () => void
}

interface RealtimeEvent {
  id: string
  type: 'user_joined' | 'user_left' | 'item_added' | 'item_removed' | 'item_updated' | 'item_deleted' | 'download_started' | 'playback_started' | 'comment_added' | 'share_created' | 'rating_added'
  timestamp: string
  userId: string
  userName: string
  userAvatar?: string
  data: {
    itemId?: string
    itemTitle?: string
    itemType?: string
    action?: string
    details?: unknown
  }
  collectionId: string
  collectionName: string
}

interface OnlineUser {
  id: string
  name: string
  avatar?: string
  status: 'active' | 'away' | 'idle'
  lastSeen: string
  currentActivity?: string
  isCurrentUser?: boolean
}

interface ConnectionStatus {
  status: 'connecting' | 'connected' | 'disconnected' | 'reconnecting' | 'error'
  latency?: number
  messageCount?: number
  lastConnected?: string
}

interface NotificationSettings {
  userActivity: boolean
  itemChanges: boolean
  playbackActivity: boolean
  commentsAndShares: boolean
  systemNotifications: boolean
  soundEnabled: boolean
  desktopNotifications: boolean
}

const EVENT_ICONS = {
  user_joined: UserPlus,
  user_left: UserMinus,
  item_added: FilePlus,
  item_removed: FileMinus,
  item_updated: Edit,
  item_deleted: Trash2,
  download_started: Download,
  playback_started: Play,
  comment_added: MessageSquare,
  share_created: Share2,
  rating_added: Heart
}

const EVENT_DESCRIPTIONS = {
  user_joined: (userName: string) => `${userName} joined the collection`,
  user_left: (userName: string) => `${userName} left the collection`,
  item_added: (userName: string, itemTitle: string) => `${userName} added "${itemTitle}"`,
  item_removed: (userName: string, itemTitle: string) => `${userName} removed "${itemTitle}"`,
  item_updated: (userName: string, itemTitle: string) => `${userName} updated "${itemTitle}"`,
  item_deleted: (userName: string, itemTitle: string) => `${userName} deleted "${itemTitle}"`,
  download_started: (userName: string, itemTitle: string) => `${userName} started downloading "${itemTitle}"`,
  playback_started: (userName: string, itemTitle: string) => `${userName} started playing "${itemTitle}"`,
  comment_added: (userName: string, itemTitle: string) => `${userName} commented on "${itemTitle}"`,
  share_created: (userName: string) => `${userName} shared this collection`,
  rating_added: (userName: string, itemTitle: string, rating: number) => `${userName} rated "${itemTitle}" ${rating} stars`
}

export const CollectionRealTime: React.FC<CollectionRealTimeProps> = ({
  collection,
  onClose
}) => {
  const [isConnected, setIsConnected] = useState(false)
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>({
    status: 'disconnected'
  })
  const [onlineUsers, setOnlineUsers] = useState<OnlineUser[]>([])
  const [recentEvents, setRecentEvents] = useState<RealtimeEvent[]>([])
  const [notificationsEnabled, setNotificationsEnabled] = useState(true)
  const [notificationSettings, setNotificationSettings] = useState<NotificationSettings>({
    userActivity: true,
    itemChanges: true,
    playbackActivity: true,
    commentsAndShares: true,
    systemNotifications: true,
    soundEnabled: true,
    desktopNotifications: false
  })
  const [showUserList, setShowUserList] = useState(true)
  const [showEventHistory, setShowEventHistory] = useState(true)
  const [activityStats, setActivityStats] = useState({
    activeUsers: 0,
    totalInteractions: 0,
    recentActivity: 0,
    peakConcurrent: 0
  })
  
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null)
  const heartbeatIntervalRef = useRef<NodeJS.Timeout | null>(null)
  const connectionTimeoutRef = useRef<NodeJS.Timeout | null>(null)
  const audioRef = useRef<HTMLAudioElement | null>(null)

  // The server has no real-time channel (no WebSocket or SSE endpoint for collections), so
  // there is nothing to connect to. The earlier version faked a connection with invented
  // users and events (CAT-001); it now reports the real state instead.
  const connectWebSocket = useCallback(() => {
    setIsConnected(false)
    setOnlineUsers([])
    setRecentEvents([])
    setConnectionStatus({ status: 'error' })
  }, [])

  const disconnectWebSocket = useCallback(() => {
    setIsConnected(false)
    setConnectionStatus({ status: 'disconnected' })
    setOnlineUsers([])
    
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current)
    }
    
    if (heartbeatIntervalRef.current) {
      clearInterval(heartbeatIntervalRef.current)
    }
    
    if (connectionTimeoutRef.current) {
      clearTimeout(connectionTimeoutRef.current)
    }
  }, [])

  const toggleConnection = useCallback(() => {
    if (isConnected) {
      disconnectWebSocket()
    } else {
      connectWebSocket()
    }
  }, [isConnected, connectWebSocket, disconnectWebSocket])

  // Auto-connect on component mount
  useEffect(() => {
    connectWebSocket()
    
    return () => {
      disconnectWebSocket()
    }
  }, [connectWebSocket, disconnectWebSocket])

  const getEventCategory = (eventType: string): string => {
    if (['user_joined', 'user_left'].includes(eventType)) return 'userActivity'
    if (['item_added', 'item_removed', 'item_updated', 'item_deleted'].includes(eventType)) return 'itemChanges'
    if (['download_started', 'playback_started'].includes(eventType)) return 'playbackActivity'
    if (['comment_added', 'share_created', 'rating_added'].includes(eventType)) return 'commentsAndShares'
    return 'systemNotifications'
  }

  const getEventDescription = (event: RealtimeEvent): string => {
    const descriptionFn = EVENT_DESCRIPTIONS[event.type]
    if (descriptionFn) {
      if (event.type === 'rating_added') {
        const details = event.data.details as { rating?: number } | undefined
        return (descriptionFn as (user: string, item: string, rating: number) => string)(event.userName, event.data.itemTitle || '', details?.rating || 5)
      } else if (event.type === 'user_joined' || event.type === 'user_left' || event.type === 'share_created') {
        return (descriptionFn as (user: string) => string)(event.userName)
      } else {
        return (descriptionFn as (user: string, item: string) => string)(event.userName, event.data.itemTitle || '')
      }
    }
    return `${event.userName} performed an action`
  }

  const formatTimestamp = (timestamp: string): string => {
    const date = new Date(timestamp)
    const now = new Date()
    const diffMs = now.getTime() - date.getTime()
    const diffMins = Math.floor(diffMs / 60000)
    
    if (diffMins < 1) return 'Just now'
    if (diffMins < 60) return `${diffMins}m ago`
    
    const diffHours = Math.floor(diffMins / 60)
    if (diffHours < 24) return `${diffHours}h ago`
    
    return date.toLocaleDateString()
  }

  const requestDesktopNotifications = useCallback(async () => {
    if ('Notification' in window && Notification.permission === 'default') {
      await Notification.requestPermission()
      setNotificationSettings(prev => ({ ...prev, desktopNotifications: true }))
    }
  }, [])

  const EventItem = ({ event }: { event: RealtimeEvent }) => {
    const Icon = EVENT_ICONS[event.type]
    
    return (
      <motion.div
        initial={{ opacity: 0, x: -20 }}
        animate={{ opacity: 1, x: 0 }}
        className="flex items-start gap-3 p-3 bg-gray-50 dark:bg-gray-800 rounded-lg"
      >
        <div className="p-2 bg-blue-100 dark:bg-blue-900/20 rounded-lg">
          <Icon className="w-4 h-4 text-blue-600 dark:text-blue-400" />
        </div>
        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium text-gray-900 dark:text-white">
            {getEventDescription(event)}
          </p>
          <p className="text-xs text-gray-600 dark:text-gray-400">
            {formatTimestamp(event.timestamp)}
          </p>
        </div>
      </motion.div>
    )
  }

  const OnlineUserItem = ({ user }: { user: OnlineUser }) => (
    <div className="flex items-center gap-3 p-2">
      <div className="relative">
        <div className="w-8 h-8 bg-blue-500 rounded-full flex items-center justify-center text-white text-sm font-medium">
          {user.name.charAt(0)}
        </div>
        <div className={`absolute bottom-0 right-0 w-2.5 h-2.5 rounded-full border-2 border-white dark:border-gray-900 ${
          user.status === 'active' ? 'bg-green-500' :
          user.status === 'away' ? 'bg-yellow-500' : 'bg-gray-400'
        }`} />
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-gray-900 dark:text-white">
          {user.name} {user.isCurrentUser && '(You)'}
        </p>
        {user.currentActivity && (
          <p className="text-xs text-gray-600 dark:text-gray-400 truncate">
            {user.currentActivity}
          </p>
        )}
      </div>
    </div>
  )

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
        className="bg-white dark:bg-gray-900 rounded-xl shadow-2xl max-w-5xl w-full max-h-[90vh] overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-700">
          <div>
            <h2 className="text-2xl font-bold text-gray-900 dark:text-white">Real-time Collection</h2>
            <p className="text-gray-600 dark:text-gray-400">{collection.name}</p>
          </div>
          <div className="flex items-center gap-3">
            {/* Connection Status */}
            <div className="flex items-center gap-2">
              <div className={`flex items-center gap-2 px-3 py-1.5 rounded-lg ${
                connectionStatus.status === 'connected' 
                  ? 'bg-green-100 dark:bg-green-900/20 text-green-700 dark:text-green-400'
                  : connectionStatus.status === 'connecting'
                  ? 'bg-yellow-100 dark:bg-yellow-900/20 text-yellow-800 dark:text-yellow-400'
                  : 'bg-red-100 dark:bg-red-900/20 text-red-700 dark:text-red-400'
              }`}>
                {connectionStatus.status === 'connected' ? (
                  <Wifi className="w-4 h-4" />
                ) : connectionStatus.status === 'connecting' ? (
                  <RefreshCw className="w-4 h-4 animate-spin" />
                ) : (
                  <WifiOff className="w-4 h-4" />
                )}
                <span className="text-sm font-medium">
                  {connectionStatus.status.charAt(0).toUpperCase() + connectionStatus.status.slice(1)}
                </span>
              </div>
              
              {connectionStatus.latency && (
                <span className="text-sm text-gray-600 dark:text-gray-400">
                  {connectionStatus.latency}ms
                </span>
              )}
            </div>

            {/* Notifications Toggle */}
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setNotificationsEnabled(!notificationsEnabled)}
              className={`flex items-center gap-2 ${
                notificationsEnabled ? 'text-blue-600 dark:text-blue-400' : 'text-gray-600 dark:text-gray-400'
              }`}
            >
              {notificationsEnabled ? (
                <Bell className="w-4 h-4" />
              ) : (
                <BellOff className="w-4 h-4" />
              )}
            </Button>

            {onClose && (
              <Button
                variant="ghost"
                size="sm"
                onClick={onClose}
              >
                <X className="w-4 h-4" />
              </Button>
            )}
          </div>
        </div>

        {/* Content */}
        <div className="overflow-y-auto p-6 max-h-[calc(90vh-140px)]">
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            {/* Left Panel - Activity Stats */}
            <div className="space-y-6">
              {/* Connection Controls */}
              <Card className="p-4">
                <h3 className="font-semibold text-gray-900 dark:text-white mb-3">Connection</h3>
                <div className="space-y-3">
                  <Button
                    onClick={toggleConnection}
                    className={`w-full ${
                      isConnected 
                        ? 'bg-red-600 hover:bg-red-700' 
                        : 'bg-green-700 hover:bg-green-800'
                    }`}
                  >
                    {isConnected ? (
                      <>
                        <WifiOff className="w-4 h-4 mr-2" />
                        Disconnect
                      </>
                    ) : (
                      <>
                        <Wifi className="w-4 h-4 mr-2" />
                        Connect
                      </>
                    )}
                  </Button>
                  
                  {connectionStatus.lastConnected && (
                    <div className="text-sm text-gray-600 dark:text-gray-400">
                      Last connected: {formatTimestamp(connectionStatus.lastConnected)}
                    </div>
                  )}
                </div>
              </Card>

              {/* Activity Stats */}
              <Card className="p-4">
                <h3 className="font-semibold text-gray-900 dark:text-white mb-3">Activity Stats</h3>
                {connectionStatus.status === 'error' && (
                  <p className="text-sm text-gray-500 dark:text-gray-400 mb-3" data-testid="realtime-unavailable">
                    Real-time updates are not available: the server has no real-time channel.
                  </p>
                )}
                <div className="space-y-3">
                  <div className="flex justify-between items-center">
                    <span className="text-sm text-gray-600 dark:text-gray-400">Active Users</span>
                    <span className="font-medium text-gray-900 dark:text-white">
                      {connectionStatus.status === 'error' ? 'Unavailable' : activityStats.activeUsers}
                    </span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-sm text-gray-600 dark:text-gray-400">Total Interactions</span>
                    <span className="font-medium text-gray-900 dark:text-white">
                      {connectionStatus.status === 'error' ? 'Unavailable' : activityStats.totalInteractions}
                    </span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-sm text-gray-600 dark:text-gray-400">Recent Activity</span>
                    <span className="font-medium text-gray-900 dark:text-white">
                      {connectionStatus.status === 'error' ? 'Unavailable' : activityStats.recentActivity}
                    </span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-sm text-gray-600 dark:text-gray-400">Peak Concurrent</span>
                    <span className="font-medium text-gray-900 dark:text-white">
                      {connectionStatus.status === 'error' ? 'Unavailable' : activityStats.peakConcurrent}
                    </span>
                  </div>
                </div>
              </Card>

              {/* Notification Settings */}
              <Card className="p-4">
                <h3 className="font-semibold text-gray-900 dark:text-white mb-3">Notifications</h3>
                <div className="space-y-3">
                  {Object.entries({
                    userActivity: 'User Activity',
                    itemChanges: 'Item Changes',
                    playbackActivity: 'Playback Activity',
                    commentsAndShares: 'Comments & Shares'
                  }).map(([key, label]) => (
                    <div key={key} className="flex items-center justify-between">
                      <span className="text-sm text-gray-700 dark:text-gray-300">{label}</span>
                      <Switch
                        checked={notificationSettings[key as keyof NotificationSettings]}
                        onCheckedChange={(checked: boolean) => setNotificationSettings(prev => ({
                          ...prev,
                          [key]: checked
                        }))}
                      />
                    </div>
                  ))}
                  
                  <div className="border-t border-gray-200 dark:border-gray-700 pt-3 space-y-3">
                    <div className="flex items-center justify-between">
                      <span className="text-sm text-gray-700 dark:text-gray-300">Sound</span>
                      <Switch
                        checked={notificationSettings.soundEnabled}
                        onCheckedChange={(checked: boolean) => setNotificationSettings(prev => ({
                          ...prev,
                          soundEnabled: checked
                        }))}
                      />
                    </div>
                    
                    <div className="flex items-center justify-between">
                      <span className="text-sm text-gray-700 dark:text-gray-300">Desktop</span>
                      {notificationSettings.desktopNotifications ? (
                        <Switch
                          checked={true}
                          onCheckedChange={() => setNotificationSettings(prev => ({
                            ...prev,
                            desktopNotifications: false
                          }))}
                        />
                      ) : (
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={requestDesktopNotifications}
                          className="text-xs"
                        >
                          Enable
                        </Button>
                      )}
                    </div>
                  </div>
                </div>
              </Card>
            </div>

            {/* Middle Panel - Online Users */}
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <h3 className="font-semibold text-gray-900 dark:text-white">
                  Online Users ({onlineUsers.length})
                </h3>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setShowUserList(!showUserList)}
                >
                  {showUserList ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                </Button>
              </div>
              
              <AnimatePresence>
                {showUserList && (
                  <motion.div
                    initial={{ opacity: 0, height: 0 }}
                    animate={{ opacity: 1, height: 'auto' }}
                    exit={{ opacity: 0, height: 0 }}
                  >
                    <Card className="p-4">
                      <div className="space-y-2">
                        {onlineUsers.map((user) => (
                          <OnlineUserItem key={user.id} user={user} />
                        ))}
                      </div>
                    </Card>
                  </motion.div>
                )}
              </AnimatePresence>
            </div>

            {/* Right Panel - Event History */}
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <h3 className="font-semibold text-gray-900 dark:text-white">
                  Recent Events
                </h3>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setShowEventHistory(!showEventHistory)}
                >
                  {showEventHistory ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                </Button>
              </div>
              
              <AnimatePresence>
                {showEventHistory && (
                  <motion.div
                    initial={{ opacity: 0, height: 0 }}
                    animate={{ opacity: 1, height: 'auto' }}
                    exit={{ opacity: 0, height: 0 }}
                  >
                    <Card className="p-4">
                      <div className="space-y-3 max-h-96 overflow-y-auto">
                        {recentEvents.length === 0 ? (
                          <div className="text-center text-gray-500 dark:text-gray-400 py-8">
                            <Activity className="w-8 h-8 mx-auto mb-2" />
                            <p>No recent activity</p>
                          </div>
                        ) : (
                          recentEvents.slice(0, 20).map((event) => (
                            <EventItem key={event.id} event={event} />
                          ))
                        )}
                      </div>
                    </Card>
                  </motion.div>
                )}
              </AnimatePresence>
            </div>
          </div>
        </div>
      </motion.div>
    </motion.div>
  )
}