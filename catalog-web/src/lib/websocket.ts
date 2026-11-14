import { useEffect, useRef, useCallback } from 'react'
import { useQueryClient, type QueryClient } from '@tanstack/react-query'
import toast from 'react-hot-toast'
import type { WebSocketMessage, ConnectionState } from '@vasic-digital/websocket-client'
import { TicketWebSocketClient } from './wsTicketClient'

export type { WebSocketMessage }

export interface MediaUpdate {
  action: 'created' | 'updated' | 'deleted' | 'analyzed'
  media_id: number
  media: Record<string, unknown>
  analysis_id?: string
}

export interface SystemUpdate {
  action: 'health_check' | 'storage_update' | 'service_status'
  component: string
  status: 'healthy' | 'warning' | 'error'
  message?: string
  details?: Record<string, unknown>
}

// In dev mode, derive WS URL from current page origin so it goes through Vite proxy.
// In production or when VITE_WS_URL is set, use the explicit URL.
const WS_URL = import.meta.env.VITE_WS_URL || `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/ws`

/** Map submodule connection states to legacy state names used by ConnectionStatus */
function toLegacyState(state: ConnectionState): 'connecting' | 'open' | 'closing' | 'closed' {
  switch (state) {
    case 'connecting':
      return 'connecting'
    case 'connected':
      return 'open'
    case 'disconnecting':
      return 'closing'
    case 'disconnected':
      return 'closed'
    default:
      return 'closed'
  }
}

// React hook for WebSocket connection
export const useWebSocket = () => {
  const clientRef = useRef<TicketWebSocketClient | null>(null)
  const queryClient = useQueryClient()

  const connect = useCallback(() => {
    if (clientRef.current) {
      clientRef.current.connect()
      return
    }

    // SEC-WS-1 F2: no session token in the URL; the client mints a single-use ticket for every (re)connect (see wsTicketClient.ts).
    if (!localStorage.getItem('auth_token')) return
    const client = new TicketWebSocketClient({
      url: WS_URL,
      reconnectAttempts: 5,
      reconnectInterval: 1000,
    })

    client.on('message', (message: WebSocketMessage) => {
      switch (message.type) {
        case 'media_update':
          handleMediaUpdate(message.payload as MediaUpdate, queryClient)
          break
        case 'system_update':
          handleSystemUpdate(message.payload as SystemUpdate)
          break
        case 'analysis_complete':
          handleAnalysisComplete(message.payload as Record<string, unknown>, queryClient)
          break
        case 'asset_update':
          handleAssetUpdate(message.payload as Record<string, unknown>, queryClient)
          break
        case 'notification':
          handleNotification(message.payload as Record<string, unknown>)
          break
      }
    })

    client.on('connected', () => {
      // Subscribe to relevant channels
      client.sendJSON({ type: 'subscribe', channel: 'media_updates' })
      client.sendJSON({ type: 'subscribe', channel: 'system_updates' })
      client.sendJSON({ type: 'subscribe', channel: 'analysis_updates' })
      client.sendJSON({ type: 'subscribe', channel: 'asset_updates' })
    })

    client.on('disconnected', () => {
      // Connection status is shown in the UI via ConnectionStatus component
    })

    client.on('error', () => {
      // Silently handle WebSocket errors - connection status is shown in the UI
    })

    clientRef.current = client
    client.connect()
  }, [queryClient])

  const disconnect = useCallback(() => {
    if (clientRef.current) {
      clientRef.current.dispose()
      clientRef.current = null
    }
  }, [])

  const send = useCallback((message: unknown) => {
    clientRef.current?.sendJSON(message)
  }, [])

  const subscribe = useCallback((channel: string) => {
    clientRef.current?.sendJSON({ type: 'subscribe', channel })
  }, [])

  const unsubscribe = useCallback((channel: string) => {
    clientRef.current?.sendJSON({ type: 'unsubscribe', channel })
  }, [])

  const getConnectionState = useCallback((): 'connecting' | 'open' | 'closing' | 'closed' => {
    return toLegacyState(clientRef.current?.getState() ?? 'disconnected')
  }, [])

  useEffect(() => {
    return () => {
      clientRef.current?.dispose()
      clientRef.current = null
    }
  }, [])

  return {
    connect,
    disconnect,
    send,
    subscribe,
    unsubscribe,
    getConnectionState
  }
}

// Message handlers
const handleMediaUpdate = (update: MediaUpdate, queryClient: QueryClient) => {
  switch (update.action) {
    case 'created':
      toast.success(`New ${update.media.media_type} added: ${update.media.title}`)
      // Invalidate relevant queries
      queryClient.invalidateQueries({ queryKey: ['media-search'] })
      queryClient.invalidateQueries({ queryKey: ['media-stats'] })
      queryClient.invalidateQueries({ queryKey: ['recent-media'] })
      break

    case 'updated':
      toast.success(`${update.media.title} updated`)
      queryClient.invalidateQueries({ queryKey: ['media-search'] })
      queryClient.setQueryData(['media', update.media_id], update.media)
      break

    case 'deleted':
      toast.success(`Media item deleted`)
      queryClient.invalidateQueries({ queryKey: ['media-search'] })
      queryClient.invalidateQueries({ queryKey: ['media-stats'] })
      queryClient.removeQueries({ queryKey: ['media', update.media_id] })
      break

    case 'analyzed':
      toast.success(`Analysis complete for ${update.media.title}`)
      queryClient.invalidateQueries({ queryKey: ['media', update.media_id] })
      break
  }
}

const handleSystemUpdate = (update: SystemUpdate) => {
  const { component, status, message } = update

  switch (status) {
    case 'healthy':
      break
    case 'warning':
      toast(`${component} warning: ${message}`, {
        icon: '⚠️',
        duration: 6000,
      })
      break
    case 'error':
      toast.error(`${component} error: ${message}`)
      break
  }
}

const handleAnalysisComplete = (data: Record<string, unknown>, queryClient: QueryClient) => {
  toast.success(`Analysis complete for ${data.items_processed} items`)
  queryClient.invalidateQueries({ queryKey: ['media-search'] })
  queryClient.invalidateQueries({ queryKey: ['media-stats'] })
}

const handleAssetUpdate = (data: Record<string, unknown>, queryClient: QueryClient) => {
  if (data.action === 'asset_ready') {
    queryClient.invalidateQueries({ queryKey: ['asset', data.asset_id] })
    queryClient.invalidateQueries({ queryKey: [data.entity_type, data.entity_id] })
    queryClient.invalidateQueries({ queryKey: ['media-search'] })
  }
}

const handleNotification = (notification: Record<string, unknown>) => {
  const { message, level } = notification as { message: string; level: string }

  switch (level) {
    case 'info':
      toast(message, { icon: 'ℹ️' })
      break
    case 'success':
      toast.success(message)
      break
    case 'warning':
      toast(message, { icon: '⚠️' })
      break
    case 'error':
      toast.error(message)
      break
    default:
      toast(message)
  }
}
