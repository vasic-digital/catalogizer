import { api } from './api'
import type {
  Playlist,
  PlaylistResponse,
  PlaylistItemsResponse,
  PlaylistShareInfo,
  PlaylistAnalytics,
  SmartPlaylistRule,
  PlaylistCreateRequest,
  PlaylistUpdateRequest
} from '@/types/playlists'

export const playlistsApi = {
  // Get user's playlists
  async getPlaylists(params?: {
    limit?: number
    offset?: number
    include_smart?: boolean
    type?: string
  }): Promise<PlaylistResponse> {
    const searchParams = new URLSearchParams()
    if (params?.limit) searchParams.append('limit', params.limit.toString())
    if (params?.offset) searchParams.append('offset', params.offset.toString())
    if (params?.include_smart !== undefined) searchParams.append('include_smart', params.include_smart.toString())
    if (params?.type) searchParams.append('type', params.type)

    const response = await api.get(`/playlists?${searchParams}`)
    return response.data
  },

  // Get playlist by ID
  async getPlaylist(playlistId: string): Promise<Playlist> {
    const response = await api.get(`/playlists/${playlistId}`)
    return response.data
  },

  // Get playlist items
  async getPlaylistItems(playlistId: string, params?: {
    limit?: number
    offset?: number
    sort_by?: 'position' | 'added_at' | 'title' | 'duration'
    sort_order?: 'asc' | 'desc'
  }): Promise<PlaylistItemsResponse> {
    const searchParams = new URLSearchParams()
    if (params?.limit) searchParams.append('limit', params.limit.toString())
    if (params?.offset) searchParams.append('offset', params.offset.toString())
    if (params?.sort_by) searchParams.append('sort_by', params.sort_by)
    if (params?.sort_order) searchParams.append('sort_order', params.sort_order)

    const response = await api.get(`/playlists/${playlistId}/items?${searchParams}`)
    return response.data
  },

  // Create playlist
  async createPlaylist(request: PlaylistCreateRequest): Promise<Playlist> {
    const response = await api.post('/playlists', request)
    return response.data
  },

  // Update playlist
  async updatePlaylist(playlistId: string, request: PlaylistUpdateRequest): Promise<Playlist> {
    const response = await api.put(`/playlists/${playlistId}`, request)
    return response.data
  },

  // Delete playlist
  async deletePlaylist(playlistId: string): Promise<void> {
    await api.delete(`/playlists/${playlistId}`)
  },

  // Add items to playlist
  async addItemsToPlaylist(playlistId: string, mediaIds: number[]): Promise<{ added: number; failed: number }> {
    const response = await api.post(`/playlists/${playlistId}/items`, { media_ids: mediaIds })
    return response.data
  },

  // Remove item from playlist
  async removeFromPlaylist(playlistId: string, itemId: string): Promise<void> {
    await api.delete(`/playlists/${playlistId}/items/${itemId}`)
  },

  // Reorder playlist items
  async reorderPlaylistItems(playlistId: string, itemOrders: { id: string; position: number }[]): Promise<void> {
    await api.put(`/playlists/${playlistId}/items/reorder`, { items: itemOrders })
  },

  // Share playlist
  async sharePlaylist(playlistId: string, permissions: {
    can_view?: boolean
    can_comment?: boolean
    can_download?: boolean
    expires_at?: string
  }): Promise<PlaylistShareInfo> {
    const response = await api.post(`/playlists/${playlistId}/share`, permissions)
    return response.data
  },

  // Get share info
  async getSharedPlaylist(shareToken: string): Promise<PlaylistItemsResponse> {
    const response = await api.get(`/playlists/shared/${shareToken}`)
    return response.data
  },

  // Unshare playlist
  async unsharePlaylist(playlistId: string): Promise<void> {
    await api.delete(`/playlists/${playlistId}/share`)
  },

  // Get playlist analytics
  async getPlaylistAnalytics(playlistId: string): Promise<PlaylistAnalytics> {
    const response = await api.get(`/playlists/${playlistId}/analytics`)
    return response.data
  },

  // Duplicate playlist
  async duplicatePlaylist(playlistId: string, newName?: string): Promise<Playlist> {
    const response = await api.post(`/playlists/${playlistId}/duplicate`, { name: newName })
    return response.data
  },

  // Export playlist
  async exportPlaylist(playlistId: string, format: 'json' | 'm3u' | 'csv' = 'json'): Promise<Blob> {
    const response = await api.get(`/playlists/${playlistId}/export?format=${format}`, {
      responseType: 'blob'
    })
    return response.data
  },

  // Play playlist
  async playPlaylist(playlistId: string): Promise<void> {
    await api.post(`/playlists/${playlistId}/play`)
  },

  // Shuffle playlist
  async shufflePlaylist(playlistId: string): Promise<void> {
    await api.post(`/playlists/${playlistId}/shuffle`)
  },

  // Reorder playlist items
  async reorderPlaylist(playlistId: string, itemIds: string[]): Promise<void> {
    await api.put(`/playlists/${playlistId}/reorder`, { itemIds })
  },

  // Import playlist
  async importPlaylist(file: File, name?: string): Promise<{ playlist: Playlist; imported: number; failed: number }> {
    const formData = new FormData()
    formData.append('file', file)
    if (name) formData.append('name', name)

    const response = await api.post('/playlists/import', formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      }
    })
    return response.data
  },

  // Validate smart playlist rules
  async validateSmartRules(rules: SmartPlaylistRule[]): Promise<{ valid: boolean; errors: string[] }> {
    const response = await api.post('/playlists/validate-smart-rules', { rules })
    return response.data
  }
}

// Alias for backward compatibility
export const playlistApi = playlistsApi