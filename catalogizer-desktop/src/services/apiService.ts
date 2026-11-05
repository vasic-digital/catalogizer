import { invoke } from '@tauri-apps/api/core';
import {
  MediaItem,
  MediaSearchRequest,
  MediaSearchResponse,
  MediaStats,
  LoginRequest,
  LoginResponse,
  AuthStatus,
  User,
  SMBConfig,
  SMBStatus,
  PlaybackProgress,
  UiPlaybackProgress,
  UiPlaybackSession,
} from '../types';

class ApiService {
  private async makeRequest<T>(
    endpoint: string,
    options: {
      method?: string;
      body?: any;
      headers?: Record<string, string>;
    } = {}
  ): Promise<T> {
    const config = await invoke<any>('get_config');

    if (!config.server_url) {
      throw new Error('Server URL not configured');
    }

    const url = `${config.server_url}/api/v1${endpoint}`;
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...options.headers,
    };

    // Add auth token if available
    if (config.auth_token) {
      headers['Authorization'] = `Bearer ${config.auth_token}`;
    }

    const response = await invoke<string>('make_http_request', {
      url,
      method: options.method || 'GET',
      headers,
      body: options.body ? JSON.stringify(options.body) : undefined,
    });

    try {
      return JSON.parse(response);
    } catch (error) {
      throw new Error('Invalid response format');
    }
  }

  // Auth endpoints
  async login(credentials: LoginRequest): Promise<LoginResponse> {
    return this.makeRequest<LoginResponse>('/auth/login', {
      method: 'POST',
      body: credentials,
    });
  }

  async logout(): Promise<void> {
    return this.makeRequest<void>('/auth/logout', {
      method: 'POST',
    });
  }

  async getAuthStatus(): Promise<AuthStatus> {
    return this.makeRequest<AuthStatus>('/auth/status');
  }

  async getProfile(): Promise<User> {
    return this.makeRequest<User>('/auth/profile');
  }

  // Media endpoints
  async searchMedia(request: MediaSearchRequest = {}): Promise<MediaSearchResponse> {
    const params = new URLSearchParams();

    Object.entries(request).forEach(([key, value]) => {
      if (value !== undefined && value !== null) {
        params.append(key, value.toString());
      }
    });

    const query = params.toString();
    const endpoint = query ? `/media/search?${query}` : '/media/search';

    return this.makeRequest<MediaSearchResponse>(endpoint);
  }

  async getMediaById(id: number): Promise<MediaItem> {
    return this.makeRequest<MediaItem>(`/media/${id}`);
  }

  async getMediaStats(): Promise<MediaStats> {
    return this.makeRequest<MediaStats>('/media/stats');
  }

  async updateWatchProgress(mediaId: number, progress: PlaybackProgress): Promise<void> {
    return this.makeRequest<void>(`/media/${mediaId}/progress`, {
      method: 'PUT',
      body: progress,
    });
  }

  /**
   * GET /api/v1/entities/:id/progress — returns rolled-up reproduction
   * summary for a single entity. Returns null when the entity has never
   * been played or the backend responds with `{ progress: null }`.
   */
  async getEntityProgress(mediaItemId: number): Promise<UiPlaybackProgress | null> {
    try {
      const resp = await this.makeRequest<{ progress: Record<string, unknown> | null }>(
        `/entities/${mediaItemId}/progress`,
      );
      const raw = resp?.progress;
      if (!raw) return null;
      return {
        mediaItemId,
        positionUnit: (raw.position_unit as string) ?? 'seconds',
        durationTotal: (raw.duration_total as number | null) ?? null,
        lastPosition: (raw.last_position as number) ?? 0,
        lastSessionAmount: (raw.last_session_amount as number) ?? 0,
        totalReproductions: (raw.total_reproductions as number) ?? 0,
        aggregateAmount: (raw.aggregate_amount as number) ?? 0,
        lastSessionEndedAt: (raw.last_session_ended_at as string | null) ?? null,
      };
    } catch {
      return null;
    }
  }

  /**
   * GET /api/v1/entities/:id/history — returns up to `limit` session rows.
   * Returns an empty array on failure or when no sessions exist.
   */
  async listEntityHistory(mediaItemId: number, limit = 50): Promise<UiPlaybackSession[]> {
    try {
      const resp = await this.makeRequest<{ sessions: Array<Record<string, unknown>> }>(
        `/entities/${mediaItemId}/history?limit=${limit}`,
      );
      const raw = resp?.sessions ?? [];
      return raw.map((r) => ({
        id: (r.id as number) ?? 0,
        positionUnit: (r.position_unit as string) ?? 'seconds',
        startPosition: (r.start_position as number) ?? 0,
        endPosition: (r.end_position as number) ?? 0,
        totalAmount: (r.total_amount as number) ?? 0,
        startedAt: (r.started_at as string) ?? '',
        endedAt: (r.ended_at as string | null) ?? null,
        completed: (r.completed as boolean) ?? false,
      }));
    } catch {
      return [];
    }
  }

  async toggleFavorite(mediaId: number): Promise<void> {
    return this.makeRequest<void>(`/media/${mediaId}/favorite`, {
      method: 'POST',
    });
  }

  async getMediaUrl(mediaId: number): Promise<{ url: string }> {
    return this.makeRequest<{ url: string }>(`/media/${mediaId}/stream`);
  }

  async downloadMedia(mediaId: number): Promise<{ job_id: number }> {
    return this.makeRequest<{ job_id: number }>(`/media/${mediaId}/download`, {
      method: 'POST',
    });
  }

  // SMB endpoints
  async getSMBConfigs(): Promise<SMBConfig[]> {
    return this.makeRequest<SMBConfig[]>('/smb/configs');
  }

  async createSMBConfig(config: Omit<SMBConfig, 'id' | 'created_at' | 'updated_at'>): Promise<SMBConfig> {
    return this.makeRequest<SMBConfig>('/smb/configs', {
      method: 'POST',
      body: config,
    });
  }

  async updateSMBConfig(id: number, config: Partial<SMBConfig>): Promise<SMBConfig> {
    return this.makeRequest<SMBConfig>(`/smb/configs/${id}`, {
      method: 'PUT',
      body: config,
    });
  }

  async deleteSMBConfig(id: number): Promise<void> {
    return this.makeRequest<void>(`/smb/configs/${id}`, {
      method: 'DELETE',
    });
  }

  async getSMBStatus(configId?: number): Promise<SMBStatus[]> {
    const endpoint = configId ? `/smb/status/${configId}` : '/smb/status';
    return this.makeRequest<SMBStatus[]>(endpoint);
  }

  async connectSMB(configId: number): Promise<void> {
    return this.makeRequest<void>(`/smb/connect/${configId}`, {
      method: 'POST',
    });
  }

  async disconnectSMB(configId: number): Promise<void> {
    return this.makeRequest<void>(`/smb/disconnect/${configId}`, {
      method: 'POST',
    });
  }

  async scanSMB(configId: number): Promise<{ job_id: number }> {
    return this.makeRequest<{ job_id: number }>(`/smb/scan/${configId}`, {
      method: 'POST',
    });
  }

  // System endpoints
  async getSystemInfo(): Promise<{
    version: string;
    platform: string;
    arch: string;
  }> {
    const [version, platform, arch] = await Promise.all([
      invoke<string>('get_app_version'),
      invoke<string>('get_platform'),
      invoke<string>('get_arch'),
    ]);

    return { version, platform, arch };
  }

  async healthCheck(): Promise<{ status: string; timestamp: string }> {
    return this.makeRequest<{ status: string; timestamp: string }>('/health');
  }
}

export const apiService = new ApiService();