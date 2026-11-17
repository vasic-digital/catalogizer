import { describe, it, expect, vi, beforeEach } from 'vitest';
import { MediaService } from '../MediaService';
import { HttpClient } from '../../utils/http';

// Mock HttpClient
vi.mock('../../utils/http');

describe('MediaService', () => {
  let mediaService: MediaService;
  let mockHttp: any;

  beforeEach(() => {
    vi.clearAllMocks();

    mockHttp = {
      get: vi.fn(),
      post: vi.fn(),
      put: vi.fn(),
      patch: vi.fn(),
      delete: vi.fn(),
      downloadStream: vi.fn(),
    };

    mediaService = new MediaService(mockHttp as any);
  });

  describe('search', () => {
    it('searches with query parameters', async () => {
      const response = { items: [{ id: 1, title: 'Movie' }], total: 1, limit: 20, offset: 0, has_next: false, has_previous: false };
      mockHttp.get.mockResolvedValueOnce(response);

      const result = await mediaService.search({ query: 'movie', media_type: 'movie', limit: 10 });

      expect(mockHttp.get).toHaveBeenCalledWith('/media/search?query=movie&media_type=movie&limit=10');
      expect(result).toEqual(response);
    });

    it('searches with empty params', async () => {
      const response = { items: [], total: 0, limit: 20, offset: 0, has_next: false, has_previous: false };
      mockHttp.get.mockResolvedValueOnce(response);

      await mediaService.search({});

      expect(mockHttp.get).toHaveBeenCalledWith('/media/search');
    });

    it('searches with default empty request', async () => {
      const response = { items: [], total: 0, limit: 20, offset: 0, has_next: false, has_previous: false };
      mockHttp.get.mockResolvedValueOnce(response);

      await mediaService.search();

      expect(mockHttp.get).toHaveBeenCalledWith('/media/search');
    });

    it('skips undefined and null values in query params', async () => {
      const response = { items: [], total: 0, limit: 20, offset: 0, has_next: false, has_previous: false };
      mockHttp.get.mockResolvedValueOnce(response);

      await mediaService.search({ query: 'test', media_type: undefined });

      expect(mockHttp.get).toHaveBeenCalledWith('/media/search?query=test');
    });
  });

  describe('getById', () => {
    it('gets media by ID', async () => {
      const media = { id: 1, title: 'Test Movie', media_type: 'movie' };
      mockHttp.get.mockResolvedValueOnce(media);

      const result = await mediaService.getById(1);

      expect(mockHttp.get).toHaveBeenCalledWith('/media/1');
      expect(result).toEqual(media);
    });
  });

  describe('getStats', () => {
    it('gets media statistics', async () => {
      const stats = { total_items: 100, by_type: { movie: 50 }, by_quality: {}, total_size: 1000, recent_additions: 5 };
      mockHttp.get.mockResolvedValueOnce(stats);

      const result = await mediaService.getStats();

      expect(mockHttp.get).toHaveBeenCalledWith('/media/stats');
      expect(result).toEqual(stats);
    });
  });

  describe('getRecentlyAdded', () => {
    it('gets recently added with default limit', async () => {
      const response = { items: [{ id: 1 }], total: 1, limit: 20, offset: 0, has_next: false, has_previous: false };
      mockHttp.get.mockResolvedValueOnce(response);

      const result = await mediaService.getRecentlyAdded();

      expect(mockHttp.get).toHaveBeenCalledWith('/media/search?sort_by=created_at&sort_order=desc&limit=20');
      expect(result).toEqual(response.items);
    });

    it('gets recently added with custom limit', async () => {
      const response = { items: [], total: 0, limit: 5, offset: 0, has_next: false, has_previous: false };
      mockHttp.get.mockResolvedValueOnce(response);

      await mediaService.getRecentlyAdded(5);

      expect(mockHttp.get).toHaveBeenCalledWith('/media/search?sort_by=created_at&sort_order=desc&limit=5');
    });
  });

  describe('getTrending', () => {
    it('gets trending media from recommendations endpoint', async () => {
      const items = [{ id: 1, rating: 9.0 }];
      mockHttp.get.mockResolvedValueOnce(items);

      const result = await mediaService.getTrending();

      expect(mockHttp.get).toHaveBeenCalledWith('/recommendations/trending?limit=20');
      expect(result).toEqual(items);
    });
  });

  describe('getByType', () => {
    it('gets media filtered by type', async () => {
      const response = { items: [{ id: 1, media_type: 'tv' }], total: 1, limit: 20, offset: 0, has_next: false, has_previous: false };
      mockHttp.get.mockResolvedValueOnce(response);

      const result = await mediaService.getByType('tv');

      expect(mockHttp.get).toHaveBeenCalledWith('/media/search?media_type=tv&sort_by=updated_at&sort_order=desc&limit=20');
      expect(result).toEqual(response.items);
    });
  });

  describe('favorites', () => {
    it('gets user favorites', async () => {
      const favorites = [{ id: 1, title: 'Fav Movie' }];
      mockHttp.get.mockResolvedValueOnce(favorites);

      const result = await mediaService.getFavorites();

      expect(mockHttp.get).toHaveBeenCalledWith('/favorites?limit=50');
      expect(result).toEqual(favorites);
    });

    it('adds a favorite', async () => {
      mockHttp.post.mockResolvedValueOnce(undefined);

      await mediaService.addFavorite('movie', 42);

      expect(mockHttp.post).toHaveBeenCalledWith('/favorites', { entity_type: 'movie', entity_id: 42 });
    });

    it('removes a favorite', async () => {
      mockHttp.delete.mockResolvedValueOnce(undefined);

      await mediaService.removeFavorite('movie', 42);

      expect(mockHttp.delete).toHaveBeenCalledWith('/favorites/movie/42');
    });

    it('checks favorite status', async () => {
      mockHttp.get.mockResolvedValueOnce({ is_favorite: true });

      const result = await mediaService.checkFavorite('movie', 42);

      expect(mockHttp.get).toHaveBeenCalledWith('/favorites/check/movie/42');
      expect(result).toEqual({ is_favorite: true });
    });

    it('sets favorite status via media endpoint (server answers {success,message})', async () => {
      const serverReply = { success: true, message: 'Favorite status updated successfully' };
      mockHttp.put.mockResolvedValueOnce(serverReply);

      const result = await mediaService.setFavorite(42, true);

      expect(mockHttp.put).toHaveBeenCalledWith('/media/42/favorite', { favorite: true });
      expect(result).toEqual(serverReply);
    });

    it('deprecated toggleFavorite alias still sends the same request', async () => {
      mockHttp.put.mockResolvedValueOnce({ success: true, message: 'ok' });

      const result = await mediaService.toggleFavorite(42, false);

      expect(mockHttp.put).toHaveBeenCalledWith('/media/42/favorite', { favorite: false });
      expect(result).toEqual({ success: true, message: 'ok' });
    });
  });

  describe('playback progress', () => {
    it('updates playback progress', async () => {
      mockHttp.put.mockResolvedValueOnce(undefined);

      const progress = { media_id: 1, position: 300, duration: 7200, timestamp: Date.now() };
      await mediaService.updateProgress(1, progress);

      expect(mockHttp.put).toHaveBeenCalledWith('/media/1/progress', progress);
    });

    it('marks media as watched', async () => {
      mockHttp.put.mockResolvedValueOnce(undefined);

      await mediaService.markAsWatched(1);

      expect(mockHttp.put).toHaveBeenCalledWith('/media/1/progress', { progress: 1 });
    });
  });

  describe('streaming and downloads', () => {
    it('gets stream URL via entities endpoint', async () => {
      const streamInfo = { url: 'http://example.com/stream', mime_type: 'video/mp4', file_size: 1000000 };
      mockHttp.get.mockResolvedValueOnce(streamInfo);

      const result = await mediaService.getStreamUrl(1);

      expect(mockHttp.get).toHaveBeenCalledWith('/entities/1/stream');
      expect(result).toEqual(streamInfo);
    });

    it('gets download URL via entities endpoint', async () => {
      const downloadInfo = { url: 'http://example.com/dl', expires_at: '2024-01-01' };
      mockHttp.get.mockResolvedValueOnce(downloadInfo);

      const result = await mediaService.getDownloadUrl(1);

      expect(mockHttp.get).toHaveBeenCalledWith('/entities/1/download');
      expect(result).toEqual(downloadInfo);
    });

    it('downloads a file by ID', async () => {
      const buffer = new ArrayBuffer(100);
      mockHttp.downloadStream.mockResolvedValueOnce(buffer);

      const result = await mediaService.downloadFile(5);

      expect(mockHttp.downloadStream).toHaveBeenCalledWith('/download/file/5');
      expect(result).toBe(buffer);
    });
  });

  describe('entity assets', () => {
    it('gets entity asset', async () => {
      const buffer = new ArrayBuffer(100);
      mockHttp.downloadStream.mockResolvedValueOnce(buffer);

      const result = await mediaService.getEntityAsset('movie', 1);

      expect(mockHttp.downloadStream).toHaveBeenCalledWith('/assets/by-entity/movie/1');
      expect(result).toBe(buffer);
    });
  });

  describe('metadata operations', () => {
    it('refreshes entity metadata', async () => {
      const refreshed = { id: 1, title: 'Refreshed' };
      mockHttp.post.mockResolvedValueOnce(refreshed);

      const result = await mediaService.refreshMetadata(1);

      expect(mockHttp.post).toHaveBeenCalledWith('/entities/1/metadata/refresh');
      expect(result).toEqual(refreshed);
    });

    it('updates user metadata for an entity', async () => {
      mockHttp.put.mockResolvedValueOnce(undefined);

      await mediaService.updateUserMetadata(1, { rating: 8, notes: 'Great film' });

      expect(mockHttp.put).toHaveBeenCalledWith('/entities/1/user-metadata', { rating: 8, notes: 'Great film' });
    });

    it('gets entity metadata', async () => {
      const meta = { provider: 'tmdb', data: {} };
      mockHttp.get.mockResolvedValueOnce(meta);

      const result = await mediaService.getEntityMetadata(1);

      expect(mockHttp.get).toHaveBeenCalledWith('/entities/1/metadata');
      expect(result).toEqual(meta);
    });
  });

  describe('recommendations', () => {
    it('gets similar media', async () => {
      const similar = [{ id: 2, title: 'Similar' }];
      mockHttp.get.mockResolvedValueOnce(similar);

      const result = await mediaService.getSimilar(1);

      expect(mockHttp.get).toHaveBeenCalledWith('/recommendations/similar/1?limit=10');
      expect(result).toEqual(similar);
    });

    it('gets personalized recommendations', async () => {
      const recs = [{ id: 3, title: 'Recommended' }];
      mockHttp.get.mockResolvedValueOnce(recs);

      const result = await mediaService.getRecommendations(42);

      expect(mockHttp.get).toHaveBeenCalledWith('/recommendations/personalized/42?limit=20');
      expect(result).toEqual(recs);
    });
  });

  describe('entity hierarchy', () => {
    it('gets entity children', async () => {
      const children = [{ id: 2, title: 'Season 1' }];
      mockHttp.get.mockResolvedValueOnce(children);

      const result = await mediaService.getChildren(1);

      expect(mockHttp.get).toHaveBeenCalledWith('/entities/1/children');
      expect(result).toEqual(children);
    });

    it('gets entity files', async () => {
      const files = [{ id: 10, path: '/media/file.mkv' }];
      mockHttp.get.mockResolvedValueOnce(files);

      const result = await mediaService.getEntityFiles(1);

      expect(mockHttp.get).toHaveBeenCalledWith('/entities/1/files');
      expect(result).toEqual(files);
    });
  });
});
