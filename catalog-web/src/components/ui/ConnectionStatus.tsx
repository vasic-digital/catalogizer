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

  // Docked in the page header (see Header.tsx), in normal flow: it can never float over page content or
  // other header controls. Icon-only below `sm` (the text stays available to screen readers), icon + text from `sm`.
  return (
    <AnimatePresence>
      {showStatus && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          role="status"
          aria-live="polite"
          data-testid="connection-status"
          className="flex-none"
        >
          <div className={`${config.color} ${config.textColor} px-2 sm:px-3 py-1.5 rounded-lg shadow flex items-center gap-2 text-sm font-medium`}>
            {config.icon}
            <span className="sr-only sm:not-sr-only whitespace-nowrap">{config.text}</span>
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}
