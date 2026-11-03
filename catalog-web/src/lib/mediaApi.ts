import api from './api'
import { asList } from './apiShapes'
import type {
  MediaItem,
  MediaSearchRequest,
  MediaSearchResponse,
  ExternalMetadata,
  QualityInfo,
  StorageRoot,
  MediaEntity,
  MediaEntityDetail,
  MediaTypeInfo,
  EntityListResponse,
  EntityStatsResponse,
  EntityFile,
  EntityUserMetadata,
  EntityExternalMetadata,
} from '@/types/media'

function normalizeListedMedia(raw: Record<string, unknown>): MediaItem {
  return {
    ...raw,
    media_type: (raw.media_type ?? raw.type_name ?? '') as string,
    created_at: (raw.created_at ?? raw.last_updated) as string,
  } as unknown as MediaItem
}

export const mediaApi = {
  searchMedia: (params: MediaSearchRequest): Promise<MediaSearchResponse> =>
    api.get('/media/search', { params }).then((res) => res.data),

  getMediaById: (id: number): Promise<MediaItem> =>
    api.get(`/media/${id}`).then((res) => res.data),

  getMediaByPath: (path: string): Promise<MediaItem> =>
    api.get('/media/by-path', { params: { path } }).then((res) => res.data),

  analyzeDirectory: (path: string): Promise<{ message: string; analysis_id: string }> =>
    api.post('/media/analyze', { directory_path: path }).then((res) => res.data),

  getExternalMetadata: (mediaId: number): Promise<ExternalMetadata[]> =>
    api.get(`/media/${mediaId}/metadata`).then((res) => res.data),

  refreshMetadata: (mediaId: number): Promise<{ message: string }> =>
    api.post(`/media/${mediaId}/refresh`).then((res) => res.data),

  getQualityInfo: (mediaId: number): Promise<QualityInfo> =>
    api.get(`/media/${mediaId}/quality`).then((res) => res.data),

  getMediaStats: (): Promise<{
    total_items: number
    by_type: Record<string, number>
    by_quality: Record<string, number>
    total_size: number
    recent_additions: number
  }> =>
    api.get('/media/stats').then((res) => res.data),

  // The backend answers `{items:[…], total}` and names the type `type_name` /
  // the timestamp `last_updated`; map to the MediaItem fields pages read.
  getRecentMedia: (limit = 10): Promise<MediaItem[]> =>
    api.get('/media/recent', { params: { limit } }).then((res) =>
      asList<Record<string, unknown>>(res.data, 'items').map(normalizeListedMedia)
    ),

  getPopularMedia: (limit = 10): Promise<MediaItem[]> =>
    api.get('/media/popular', { params: { limit } }).then((res) =>
      asList<Record<string, unknown>>(res.data, 'items').map(normalizeListedMedia)
    ),

  deleteMedia: (id: number): Promise<void> =>
    api.delete(`/media/${id}`).then(() => {/* no content */}),

  updateMedia: (id: number, data: Partial<MediaItem>): Promise<MediaItem> =>
    api.put(`/media/${id}`, data).then((res) => res.data),

  // Storage root management
  getStorageRoots: (): Promise<StorageRoot[]> =>
    api.get('/storage/roots').then((res) => asList<StorageRoot>(res.data, 'roots')),

  getStorageRoot: (id: number): Promise<StorageRoot> =>
    api.get(`/storage/roots/${id}`).then((res) => res.data),

  createStorageRoot: (data: Omit<StorageRoot, 'id' | 'created_at' | 'updated_at'>): Promise<StorageRoot> =>
    api.post('/storage/roots', data).then((res) => res.data),

  updateStorageRoot: (id: number, data: Partial<StorageRoot>): Promise<StorageRoot> =>
    api.put(`/storage/roots/${id}`, data).then((res) => res.data),

  deleteStorageRoot: (id: number): Promise<void> =>
    api.delete(`/storage/roots/${id}`).then(() => {/* no content */}),

  testStorageRoot: (id: number): Promise<{ success: boolean; message: string }> =>
    api.post(`/storage/roots/${id}/test`).then((res) => res.data),

  // Download media file
  downloadMedia: async (media: MediaItem): Promise<void> => {
    const response = await api.get(`/download`, {
      params: {
        path: media.directory_path,
        storage: media.storage_root_name,
      },
      responseType: 'blob',
    })

    // Create a download link and trigger it
    const url = window.URL.createObjectURL(new Blob([response.data]))
    const link = document.createElement('a')
    link.href = url

    // Extract filename from path or use title
    const filename = media.directory_path?.split('/').pop() || `${media.title}.${media.media_type}`
    link.setAttribute('download', filename)

    document.body.appendChild(link)
    link.click()

    // Cleanup
    link.parentNode?.removeChild(link)
    window.URL.revokeObjectURL(url)
  },
}

// --- Entity API (structured media browsing) ---

export const entityApi = {
  getEntities: (params: {
    query?: string
    type?: string
    limit?: number
    offset?: number
  }): Promise<EntityListResponse> =>
    api.get('/entities', { params }).then((res) => res.data),

  getEntity: (id: number): Promise<MediaEntityDetail> =>
    api.get(`/entities/${id}`).then((res) => res.data),

  getEntityChildren: (id: number, params?: {
    limit?: number
    offset?: number
  }): Promise<EntityListResponse> =>
    api.get(`/entities/${id}/children`, { params }).then((res) => res.data),

  getEntityFiles: (id: number): Promise<{ files: EntityFile[]; total: number }> =>
    api.get(`/entities/${id}/files`).then((res) => res.data),

  getEntityMetadata: (id: number): Promise<{ metadata: EntityExternalMetadata[] }> =>
    api.get(`/entities/${id}/metadata`).then((res) => res.data),

  getEntityDuplicates: (id: number): Promise<{ duplicates: MediaEntity[]; total: number }> =>
    api.get(`/entities/${id}/duplicates`).then((res) => res.data),

  getEntityTypes: (): Promise<{ types: MediaTypeInfo[] }> =>
    api.get('/entities/types').then((res) => res.data),

  browseByType: (type: string, params?: {
    limit?: number
    offset?: number
  }): Promise<EntityListResponse & { type: string }> =>
    api.get(`/entities/browse/${type}`, { params }).then((res) => res.data),

  getEntityStats: (): Promise<EntityStatsResponse> =>
    api.get('/entities/stats').then((res) => res.data),

  refreshEntityMetadata: (id: number): Promise<{ message: string; entity_id: number }> =>
    api.post(`/entities/${id}/metadata/refresh`).then((res) => res.data),

  updateUserMetadata: (id: number, data: EntityUserMetadata): Promise<{ message: string }> =>
    api.put(`/entities/${id}/user-metadata`, data).then((res) => res.data),
}

export default mediaApi