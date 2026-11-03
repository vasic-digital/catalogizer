import React, { useEffect, useState } from 'react'
import { useWebSocketContext } from '@/contexts/WebSocketContext'
import { useAuth } from '@/contexts/AuthContext'
import { Wifi, WifiOff, Loader2 } from 'lucide-react'
import { motion, AnimatePresence } from 'framer-motion'

export const ConnectionStatus: React.FC = () => {
  // Must read the SAME connection the provider opened. Calling useWebSocket()
  // here created a second, never-connected client (own ref) and the banner
  // read 'Disconnected' forever while the real socket was open.
  const { getConnectionState } = useWebSocketContext()
  const { isAuthenticated } = useAuth()
  const [connectionState, setConnectionState] = useState<'connecting' | 'open' | 'closing' | 'closed'>('closed')
  const [showStatus, setShowStatus] = useState(false)

  useEffect(() => {
    // The WebSocket only connects after authentication, so showing
    // a red "Disconnected" banner on /login (or any pre-auth route)
    // is misleading — users on the public login page have no idea
    // why their connection is "broken". Surfaced by the 2026-04-29
    // anti-bluff sweep against catalog-web.
    if (!isAuthenticated) {
      setShowStatus(false)
      return
    }
    const checkConnection = () => {
      const state = getConnectionState()
      setConnectionState(state)

      // Show status when not connected or connecting
      setShowStatus(state !== 'open')
    }

    checkConnection()
    const interval = setInterval(checkConnection, 1000)

    return () => clearInterval(interval)
  }, [getConnectionState, isAuthenticated])

  const getStatusConfig = () => {
    switch (connectionState) {
      case 'connecting':
        return {
          icon: <Loader2 className="h-4 w-4 animate-spin" />,
          text: 'Connecting...',
          color: 'bg-yellow-500',
          textColor: 'text-yellow-100'
        }
      case 'open':
        return {
          icon: <Wifi className="h-4 w-4" />,
          text: 'Connected',
          color: 'bg-green-500',
          textColor: 'text-green-100'
        }
      case 'closing':
        return {
          icon: <WifiOff className="h-4 w-4" />,
          text: 'Disconnecting...',
          color: 'bg-orange-500',
          textColor: 'text-orange-100'
        }
      case 'closed':
        return {
          icon: <WifiOff className="h-4 w-4" />,
          text: 'Disconnected',
          color: 'bg-red-500',
          textColor: 'text-red-100'
        }
    }
  }

  const config = getStatusConfig()

  return (
    <AnimatePresence>
      {showStatus && (
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: 20 }}
          className="fixed z-50 pointer-events-none bottom-10 left-4 right-4 flex justify-end sm:bottom-auto sm:left-auto sm:top-20 sm:right-4 sm:max-w-[calc(100vw-2rem)]"
        >
          <div className={`${config.color} ${config.textColor} px-3 py-2 rounded-lg shadow-lg flex items-center space-x-2 text-sm font-medium`}>
            {config.icon}
            <span>{config.text}</span>
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}