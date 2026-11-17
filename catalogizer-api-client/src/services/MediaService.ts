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

export interface WatchProgressResult {
  /** true only when the server confirmed persisted:true */
  persisted: boolean;
  reason?: string;
  attempts: number;
}

export const WATCH_PROGRESS_MAX_ATTEMPTS = 3;

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
   * Update playback progress for a media item (position and duration in SECONDS).
   * Backend: PUT /api/v1/media/:id/progress. Only persisted:true is reported as saved; a 200 persisted:false
   * carries a reason (for example unknown_duration_no_row); a 409 concurrent_update is re-sent up to
   * WATCH_PROGRESS_MAX_ATTEMPTS times and then reported as not persisted.
   */
  public async updateProgress(mediaId: number, progress: PlaybackProgress | { progress: number }): Promise<WatchProgressResult> {
    let last: WatchProgressResult = { persisted: false, attempts: 0 };
    for (let attempt = 1; attempt <= WATCH_PROGRESS_MAX_ATTEMPTS; attempt++) {
      try {
        const resp = await this.http.put<{ persisted?: boolean; reason?: string } | undefined>(
          `/media/${mediaId}/progress`,
          progress
        );
        return { persisted: resp?.persisted === true, reason: resp?.reason, attempts: attempt };
      } catch (err) {
        if ((err as { status?: number })?.status !== 409) {
          throw err;
        }
        last = { persisted: false, reason: 'concurrent_update', attempts: attempt };
      }
    }
    return last;
  }

  /**
   * Mark media as watched. With the real duration (seconds) the item is stored as position == duration; without
   * it the fraction 1.0 is sent (the server never invents a duration, so a first watch of an item with no stored
   * row answers persisted:false).
   */
  public async markAsWatched(mediaId: number, durationSeconds?: number): Promise<WatchProgressResult> {
    if (durationSeconds !== undefined && durationSeconds > 0) {
      return this.updateProgress(mediaId, {
        media_id: mediaId,
        position: durationSeconds,
        duration: durationSeconds,
        timestamp: Date.now(),
      });
    }
    return this.updateProgress(mediaId, { progress: 1 });
  }

  /**
   * Set the calling user's favorite flag for a media item (per-user, idempotent)
   * Backend: PUT /api/v1/media/:id/favorite with body {"favorite": boolean}
   * (the field is required; the server answers 400 without it).
   * The server answers {"success": true, "message": "..."}; it does NOT return the new flag.
   */
  public async setFavorite(mediaId: number, favorite: boolean): Promise<{ success: boolean; message: string }> {
    return this.http.put<{ success: boolean; message: string }>(`/media/${mediaId}/favorite`, { favorite });
  }

  /**
   * @deprecated Misleading name and return type: this sets the flag, it does not toggle, and the server returns
   * {success, message}, not {is_favorite}. Use {@link setFavorite}. Kept as an alias for existing callers.
   */
  public async toggleFavorite(mediaId: number, favorite: boolean): Promise<{ success: boolean; message: string }> {
    return this.setFavorite(mediaId, favorite);
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
