import { HttpClient } from '../utils/http';
import {
  MediaItem,
  MediaSearchRequest,
  PaginatedResponse,
  MediaStats,
  PlaybackProgress,
  StreamInfo,
  DownloadJob,
} from '../types';

export class MediaService {
  constructor(private http: HttpClient) {}

  /**
   * Search for media items
   * Backend: GET /api/v1/media/search
   */
  public async search(request: MediaSearchRequest = {}): Promise<PaginatedResponse<MediaItem>> {
    const params = new URLSearchParams();

    // Build query parameters
    Object.entries(request).forEach(([key, value]) => {
      if (value !== undefined && value !== null) {
        params.append(key, value.toString());
      }
    });

    const query = params.toString();
    const endpoint = query ? `/media/search?${query}` : '/media/search';

    return this.http.get<PaginatedResponse<MediaItem>>(endpoint);
  }

  /**
   * Get a specific media item by ID
   * Backend: GET /api/v1/media/:id
   */
  public async getById(id: number): Promise<MediaItem> {
    return this.http.get<MediaItem>(`/media/${id}`);
  }

  /**
   * Get media statistics
   * Backend: GET /api/v1/media/stats
   */
  public async getStats(): Promise<MediaStats> {
    return this.http.get<MediaStats>('/media/stats');
  }

  /**
   * Get recently added media
   */
  public async getRecentlyAdded(limit = 20): Promise<MediaItem[]> {
    const response = await this.search({
      sort_by: 'created_at',
      sort_order: 'desc',
      limit,
    });
    return response.items;
  }

  /**
   * Get trending/popular media
   * Backend: GET /api/v1/recommendations/trending
   */
  public async getTrending(limit = 20): Promise<MediaItem[]> {
    return this.http.get<MediaItem[]>(`/recommendations/trending?limit=${limit}`);
  }

  /**
   * Get media by type
   */
  public async getByType(mediaType: string, limit = 20): Promise<MediaItem[]> {
    const response = await this.search({
      media_type: mediaType,
      sort_by: 'updated_at',
      sort_order: 'desc',
      limit,
    });
    return response.items;
  }

  /**
   * Get user's favorite media
   * Backend: GET /api/v1/favorites
   */
  public async getFavorites(limit = 50): Promise<MediaItem[]> {
    return this.http.get<MediaItem[]>(`/favorites?limit=${limit}`);
  }

  /**
   * Add media item to favorites
   * Backend: POST /api/v1/favorites
   */
  public async addFavorite(entityType: string, entityId: number): Promise<void> {
    return this.http.post<void>('/favorites', { entity_type: entityType, entity_id: entityId });
  }

  /**
   * Remove media item from favorites
   * Backend: DELETE /api/v1/favorites/:entity_type/:entity_id
   */
  public async removeFavorite(entityType: string, entityId: number): Promise<void> {
    return this.http.delete<void>(`/favorites/${entityType}/${entityId}`);
  }

  /**
   * Check if media item is favorited
   * Backend: GET /api/v1/favorites/check/:entity_type/:entity_id
   */
  public async checkFavorite(entityType: string, entityId: number): Promise<{ is_favorite: boolean }> {
    return this.http.get<{ is_favorite: boolean }>(`/favorites/check/${entityType}/${entityId}`);
  }

  /**
   * Update playback progress for a media item
   * Backend: PUT /api/v1/media/:id/progress
   */
  public async updateProgress(mediaId: number, progress: PlaybackProgress): Promise<void> {
    return this.http.put<void>(`/media/${mediaId}/progress`, progress);
  }

  /**
   * Mark media as watched (100% progress)
   */
  public async markAsWatched(mediaId: number): Promise<void> {
    const progress: PlaybackProgress = {
      media_id: mediaId,
      position: 100,
      duration: 100,
      timestamp: Date.now(),
    };
    return this.updateProgress(mediaId, progress);
  }

  /**
   * Set the calling user's favorite flag for a media item (per-user, idempotent)
   * Backend: PUT /api/v1/media/:id/favorite with body {"favorite": boolean}
   * (the field is required; the server answers 400 without it)
   */
  public async toggleFavorite(mediaId: number, favorite: boolean): Promise<{ is_favorite: boolean }> {
    return this.http.put<{ is_favorite: boolean }>(`/media/${mediaId}/favorite`, { favorite });
  }

  /**
   * Get streaming URL for a media entity
   * Backend: GET /api/v1/entities/:id/stream
   */
  public async getStreamUrl(entityId: number): Promise<StreamInfo> {
    return this.http.get<StreamInfo>(`/entities/${entityId}/stream`);
  }

  /**
   * Get download URL for a media entity
   * Backend: GET /api/v1/entities/:id/download
   */
  public async getDownloadUrl(entityId: number): Promise<{ url: string; expires_at: string }> {
    return this.http.get<{ url: string; expires_at: string }>(`/entities/${entityId}/download`);
  }

  /**
   * Download a file by ID
   * Backend: GET /api/v1/download/file/:id
   */
  public async downloadFile(fileId: number): Promise<ArrayBuffer> {
    return this.http.downloadStream(`/download/file/${fileId}`);
  }

  /**
   * Get asset (cover art, thumbnail) for a media entity
   * Backend: GET /api/v1/assets/by-entity/:type/:id
   */
  public async getEntityAsset(entityType: string, entityId: number): Promise<ArrayBuffer> {
    return this.http.downloadStream(`/assets/by-entity/${entityType}/${entityId}`);
  }

  /**
   * Refresh metadata for a media entity
   * Backend: POST /api/v1/entities/:id/metadata/refresh
   */
  public async refreshMetadata(entityId: number): Promise<MediaItem> {
    return this.http.post<MediaItem>(`/entities/${entityId}/metadata/refresh`);
  }

  /**
   * Get similar media items
   * Backend: GET /api/v1/recommendations/similar/:media_id
   */
  public async getSimilar(mediaId: number, limit = 10): Promise<MediaItem[]> {
    return this.http.get<MediaItem[]>(`/recommendations/similar/${mediaId}?limit=${limit}`);
  }

  /**
   * Get personalized recommendations for a user
   * Backend: GET /api/v1/recommendations/personalized/:user_id
   */
  public async getRecommendations(userId: number, limit = 20): Promise<MediaItem[]> {
    return this.http.get<MediaItem[]>(`/recommendations/personalized/${userId}?limit=${limit}`);
  }

  /**
   * Get media entity children (e.g., seasons of a show)
   * Backend: GET /api/v1/entities/:id/children
   */
  public async getChildren(entityId: number): Promise<MediaItem[]> {
    return this.http.get<MediaItem[]>(`/entities/${entityId}/children`);
  }

  /**
   * Get files associated with a media entity
   * Backend: GET /api/v1/entities/:id/files
   */
  public async getEntityFiles(entityId: number): Promise<unknown[]> {
    return this.http.get<unknown[]>(`/entities/${entityId}/files`);
  }

  /**
   * Get entity metadata from external providers
   * Backend: GET /api/v1/entities/:id/metadata
   */
  public async getEntityMetadata(entityId: number): Promise<unknown> {
    return this.http.get<unknown>(`/entities/${entityId}/metadata`);
  }

  /**
   * Update user metadata for an entity
   * Backend: PUT /api/v1/entities/:id/user-metadata
   */
  public async updateUserMetadata(entityId: number, metadata: Record<string, unknown>): Promise<void> {
    return this.http.put<void>(`/entities/${entityId}/user-metadata`, metadata);
  }
}
