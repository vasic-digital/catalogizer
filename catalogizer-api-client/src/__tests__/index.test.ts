import { describe, it, expect, vi, beforeEach, type Mock } from 'vitest';

// Mock axios
vi.mock('axios');
import axios from 'axios';
const mockAxios = axios as unknown as { create: Mock };

// Mock axios.create to return a mock instance with interceptors
const mockAxiosInstance = {
  interceptors: {
    request: { use: vi.fn() },
    response: { use: vi.fn() },
  },
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  delete: vi.fn(),
  patch: vi.fn(),
};

mockAxios.create.mockReturnValue(mockAxiosInstance as any);

import { CatalogizerClient, CatalogizerError } from '../index';

describe('CatalogizerClient', () => {
  let client: CatalogizerClient;

  beforeEach(() => {
    vi.clearAllMocks();
    // Re-setup mockAxios.create since clearAllMocks clears it
    mockAxios.create.mockReturnValue(mockAxiosInstance as any);
    // Re-clear the instance mocks
    mockAxiosInstance.interceptors.request.use.mockClear();
    mockAxiosInstance.interceptors.response.use.mockClear();
    mockAxiosInstance.get.mockReset();
    mockAxiosInstance.post.mockReset();
    mockAxiosInstance.put.mockReset();
    mockAxiosInstance.delete.mockReset();
    mockAxiosInstance.patch.mockReset();

    client = new CatalogizerClient({
      baseURL: 'http://localhost:8080'
    });
  });

  describe('initialization', () => {
    it('creates client with custom config', () => {
      expect(client).toBeDefined();
    });
  });

  describe('media service', () => {
    describe('search', () => {
      it('searches media successfully', async () => {
        const mockResponse = {
          data: {
            items: [
              {
                id: 1,
                title: 'Test Movie',
                directory_path: '/media/movies/',
                media_type: 'movie'
              }
            ],
            total: 1,
            limit: 10,
            offset: 0
          }
        };

        // Mock the axios response structure
        const axiosResponse = { data: mockResponse.data };

        mockAxiosInstance.get.mockResolvedValueOnce(axiosResponse);

        const result = await client.media.search({
          query: 'test movie',
          media_type: 'movie',
          limit: 10
        });

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/media/search?query=test+movie&media_type=movie&limit=10', undefined);
        expect(result).toEqual(mockResponse.data);
      });

      it('handles search errors', async () => {
        mockAxiosInstance.get.mockRejectedValueOnce(new Error('Network error'));

        await expect(client.media.search({ query: 'test' })).rejects.toThrow('Network error');
      });

      it('searches with empty params', async () => {
        const mockResponse = { data: { items: [], total: 0, limit: 20, offset: 0 } };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        await client.media.search({});

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/media/search', undefined);
      });
    });

    describe('getById', () => {
      it('gets media by id', async () => {
        const mockResponse = {
          data: {
            id: 1,
            title: 'Test Movie',
            directory_path: '/media/movies/',
            media_type: 'movie',
            file_size: 1000000
          }
        };

        const axiosResponse = { data: mockResponse.data };
        mockAxiosInstance.get.mockResolvedValueOnce(axiosResponse);

        const result = await client.media.getById(1);

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/media/1', undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('getStats', () => {
      it('gets media statistics', async () => {
        const mockResponse = {
          data: {
            total_items: 100,
            total_size: 1000000000,
            by_type: { movie: 50, tv: 30, music: 20 }
          }
        };

        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.media.getStats();

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/media/stats', undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('getRecentlyAdded', () => {
      it('gets recently added media with default limit', async () => {
        const mockResponse = {
          data: {
            items: [{ id: 1, title: 'New Movie', media_type: 'movie' }],
            total: 1, limit: 20, offset: 0
          }
        };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.media.getRecentlyAdded();

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/media/search?sort_by=created_at&sort_order=desc&limit=20', undefined);
        expect(result).toEqual(mockResponse.data.items);
      });

      it('gets recently added media with custom limit', async () => {
        const mockResponse = {
          data: { items: [], total: 0, limit: 5, offset: 0 }
        };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        await client.media.getRecentlyAdded(5);

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/media/search?sort_by=created_at&sort_order=desc&limit=5', undefined);
      });
    });

    describe('getTrending', () => {
      it('gets trending media from recommendations endpoint', async () => {
        const mockResponse = {
          data: [{ id: 1, title: 'Popular Movie', rating: 9.0 }]
        };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.media.getTrending();

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/recommendations/trending?limit=20', undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('getByType', () => {
      it('gets media by type', async () => {
        const mockResponse = {
          data: {
            items: [{ id: 1, title: 'TV Show', media_type: 'tv' }],
            total: 1, limit: 20, offset: 0
          }
        };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.media.getByType('tv');

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/media/search?media_type=tv&sort_by=updated_at&sort_order=desc&limit=20', undefined);
        expect(result).toEqual(mockResponse.data.items);
      });
    });

    describe('getFavorites', () => {
      it('gets user favorites', async () => {
        const mockResponse = {
          data: [{ id: 1, title: 'Favorite Movie' }]
        };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.media.getFavorites();

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/favorites?limit=50', undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('toggleFavorite', () => {
      it('toggles favorite status', async () => {
        const mockResponse = { data: { is_favorite: true } };
        mockAxiosInstance.put.mockResolvedValueOnce(mockResponse);

        const result = await client.media.toggleFavorite(1, false);

        expect(mockAxiosInstance.put).toHaveBeenCalledWith('/media/1/favorite', { favorite: false }, undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('updateProgress', () => {
      it('updates playback progress', async () => {
        mockAxiosInstance.put.mockResolvedValueOnce({ data: {} });

        const progress = { media_id: 1, position: 300, duration: 7200, timestamp: Date.now() };
        await client.media.updateProgress(1, progress);

        expect(mockAxiosInstance.put).toHaveBeenCalledWith('/media/1/progress', progress, undefined);
      });
    });

    describe('markAsWatched', () => {
      it('marks media as watched', async () => {
        mockAxiosInstance.put.mockResolvedValueOnce({ data: {} });

        await client.media.markAsWatched(1);

        expect(mockAxiosInstance.put).toHaveBeenCalledWith('/media/1/progress', expect.objectContaining({
          media_id: 1,
          position: 100,
          duration: 100
        }), undefined);
      });
    });

    describe('getStreamUrl', () => {
      it('gets stream URL via entities endpoint', async () => {
        const mockResponse = { data: { url: 'http://example.com/stream', quality: '1080p' } };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.media.getStreamUrl(1);

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/entities/1/stream', undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('getDownloadUrl', () => {
      it('gets download URL via entities endpoint', async () => {
        const mockResponse = { data: { url: 'http://example.com/download', expires_at: '2024-01-01T00:00:00Z' } };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.media.getDownloadUrl(1);

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/entities/1/download', undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('downloadFile', () => {
      it('downloads a file by ID', async () => {
        const buffer = new ArrayBuffer(100);
        mockAxiosInstance.get.mockResolvedValueOnce({ data: buffer });

        const result = await client.media.downloadFile(5);

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/download/file/5', { responseType: 'arraybuffer' });
      });
    });

    describe('getEntityAsset', () => {
      it('gets entity asset', async () => {
        const buffer = new ArrayBuffer(100);
        mockAxiosInstance.get.mockResolvedValueOnce({ data: buffer });

        const result = await client.media.getEntityAsset('movie', 1);

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/assets/by-entity/movie/1', { responseType: 'arraybuffer' });
      });
    });

    describe('refreshMetadata', () => {
      it('refreshes entity metadata', async () => {
        const mockResponse = { data: { id: 1, title: 'Refreshed Movie' } };
        mockAxiosInstance.post.mockResolvedValueOnce(mockResponse);

        const result = await client.media.refreshMetadata(1);

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/entities/1/metadata/refresh', undefined, undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('getSimilar', () => {
      it('gets similar media items', async () => {
        const mockResponse = { data: [{ id: 2, title: 'Similar Movie' }] };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.media.getSimilar(1);

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/recommendations/similar/1?limit=10', undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('getRecommendations', () => {
      it('gets personalized recommendations', async () => {
        const mockResponse = { data: [{ id: 1, title: 'Recommended Movie' }] };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.media.getRecommendations(42);

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/recommendations/personalized/42?limit=20', undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('addFavorite', () => {
      it('adds a favorite', async () => {
        mockAxiosInstance.post.mockResolvedValueOnce({ data: {} });

        await client.media.addFavorite('movie', 1);

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/favorites', { entity_type: 'movie', entity_id: 1 }, undefined);
      });
    });

    describe('removeFavorite', () => {
      it('removes a favorite', async () => {
        mockAxiosInstance.delete.mockResolvedValueOnce({ data: {} });

        await client.media.removeFavorite('movie', 1);

        expect(mockAxiosInstance.delete).toHaveBeenCalledWith('/favorites/movie/1', undefined);
      });
    });

    describe('entity operations', () => {
      it('gets entity children', async () => {
        const mockResponse = { data: [{ id: 2, title: 'Season 1' }] };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.media.getChildren(1);

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/entities/1/children', undefined);
        expect(result).toEqual(mockResponse.data);
      });

      it('gets entity files', async () => {
        const mockResponse = { data: [{ id: 10, path: '/media/file.mkv' }] };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.media.getEntityFiles(1);

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/entities/1/files', undefined);
        expect(result).toEqual(mockResponse.data);
      });

      it('updates user metadata', async () => {
        mockAxiosInstance.put.mockResolvedValueOnce({ data: {} });

        await client.media.updateUserMetadata(1, { rating: 9 });

        expect(mockAxiosInstance.put).toHaveBeenCalledWith('/entities/1/user-metadata', { rating: 9 }, undefined);
      });
    });
  });

  describe('auth service', () => {
    describe('login', () => {
      it('logs in successfully', async () => {
        const mockResponse = {
          data: {
            user: { id: 1, username: 'testuser' },
            token: 'jwt-token',
            refresh_token: 'refresh-token',
            expires_in: 3600
          }
        };

        const axiosResponse = { data: mockResponse.data };
        mockAxiosInstance.post.mockResolvedValueOnce(axiosResponse);

        const result = await client.auth.login({
          username: 'testuser',
          password: 'password'
        });

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/auth/login', {
          username: 'testuser',
          password: 'password'
        }, undefined);

        expect(result).toEqual(mockResponse.data);
      });

      it('handles login failure', async () => {
        const catalogizerError = new CatalogizerError('Invalid credentials', 401, 'AUTH_ERROR');
        mockAxiosInstance.post.mockRejectedValueOnce(catalogizerError);

        await expect(client.auth.login({
          username: 'wrong',
          password: 'wrong'
        })).rejects.toThrow(CatalogizerError);
      });
    });

    describe('refreshToken', () => {
      it('refreshes token', async () => {
        const mockResponse = {
          data: {
            token: 'new-jwt-token',
            expires_in: 3600
          }
        };

        const axiosResponse = { data: mockResponse.data };
        mockAxiosInstance.post.mockResolvedValueOnce(axiosResponse);

        const result = await client.auth.refreshToken();

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/auth/refresh', undefined, undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('logout', () => {
      it('logs out successfully', async () => {
        const axiosResponse = { data: {} };
        mockAxiosInstance.post.mockResolvedValueOnce(axiosResponse);

        await expect(client.auth.logout()).resolves.toBeUndefined();

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/auth/logout', undefined, undefined);
      });
    });

    describe('getStatus', () => {
      it('gets authentication status', async () => {
        const mockResponse = {
          data: {
            authenticated: true,
            user: { id: 1, username: 'testuser' }
          }
        };

        const axiosResponse = { data: mockResponse.data };
        mockAxiosInstance.get.mockResolvedValueOnce(axiosResponse);

        const result = await client.auth.getStatus();

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/auth/status', undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('register', () => {
      it('registers new user', async () => {
        const mockResponse = { data: { id: 1, username: 'newuser', email: 'new@test.com' } };
        mockAxiosInstance.post.mockResolvedValueOnce(mockResponse);

        const result = await client.auth.register({
          username: 'newuser',
          email: 'new@test.com',
          password: 'password123',
          first_name: 'New',
          last_name: 'User'
        });

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/auth/register', {
          username: 'newuser',
          email: 'new@test.com',
          password: 'password123',
          first_name: 'New',
          last_name: 'User'
        }, undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('getProfile', () => {
      it('gets user profile', async () => {
        const mockResponse = { data: { id: 1, username: 'testuser', email: 'test@test.com' } };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.auth.getProfile();

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/auth/profile', undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('updateProfile', () => {
      it('updates user profile', async () => {
        const mockResponse = { data: { id: 1, username: 'testuser', first_name: 'New' } };
        mockAxiosInstance.put.mockResolvedValueOnce(mockResponse);

        const result = await client.auth.updateProfile({ first_name: 'New' });

        expect(mockAxiosInstance.put).toHaveBeenCalledWith('/auth/profile', { first_name: 'New' }, undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('changePassword', () => {
      it('changes user password', async () => {
        mockAxiosInstance.post.mockResolvedValueOnce({ data: {} });

        await client.auth.changePassword({
          current_password: 'oldpass',
          new_password: 'newpass'
        });

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/auth/password', {
          current_password: 'oldpass',
          new_password: 'newpass'
        }, undefined);
      });
    });

    describe('requestPasswordReset', () => {
      it('requests password reset', async () => {
        mockAxiosInstance.post.mockResolvedValueOnce({ data: {} });

        await client.auth.requestPasswordReset('test@test.com');

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/auth/password/reset', { email: 'test@test.com' }, undefined);
      });
    });

    describe('resetPassword', () => {
      it('resets password with token', async () => {
        mockAxiosInstance.post.mockResolvedValueOnce({ data: {} });

        await client.auth.resetPassword('reset-token', 'newpassword');

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/auth/password/reset/confirm', {
          token: 'reset-token',
          password: 'newpassword'
        }, undefined);
      });
    });

    describe('verifyEmail', () => {
      it('verifies email with token', async () => {
        mockAxiosInstance.post.mockResolvedValueOnce({ data: {} });

        await client.auth.verifyEmail('verify-token');

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/auth/email/verify', { token: 'verify-token' }, undefined);
      });
    });

    describe('resendEmailVerification', () => {
      it('resends email verification', async () => {
        mockAxiosInstance.post.mockResolvedValueOnce({ data: {} });

        await client.auth.resendEmailVerification();

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/auth/email/verify/resend', undefined, undefined);
      });
    });

    describe('checkUsernameAvailability', () => {
      it('checks username availability', async () => {
        const mockResponse = { data: { available: true } };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.auth.checkUsernameAvailability('newuser');

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/auth/username/check?username=newuser', undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('checkEmailAvailability', () => {
      it('checks email availability', async () => {
        const mockResponse = { data: { available: false } };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.auth.checkEmailAvailability('test@test.com');

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/auth/email/check?email=test%40test.com', undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('getPermissions', () => {
      it('gets user permissions', async () => {
        const mockResponse = { data: { role: 'User', permissions: ['read', 'write', 'system.admin'], is_admin: false } };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.auth.getPermissions();

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/auth/permissions', undefined);
        expect(result).toEqual(mockResponse.data.permissions);
      });
    });

    describe('hasPermission', () => {
      it('returns true for existing permission', async () => {
        const mockResponse = { data: { role: 'User', permissions: ['read', 'write'], is_admin: false } };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.auth.hasPermission('read');

        expect(result).toBe(true);
      });

      it('returns true for system.admin override', async () => {
        const mockResponse = { data: { role: 'User', permissions: ['system.admin'], is_admin: false } };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.auth.hasPermission('any_permission');

        expect(result).toBe(true);
      });

      it('returns false for missing permission', async () => {
        const mockResponse = { data: { role: 'User', permissions: ['read'], is_admin: false } };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.auth.hasPermission('write');

        expect(result).toBe(false);
      });
    });

    describe('generateApiKey', () => {
      it('generates API key', async () => {
        const mockResponse = { data: { key: 'api-key-123', name: 'My Key', created_at: '2024-01-01T00:00:00Z' } };
        mockAxiosInstance.post.mockResolvedValueOnce(mockResponse);

        const result = await client.auth.generateApiKey('My Key');

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/auth/api-keys', { name: 'My Key' }, undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('listApiKeys', () => {
      it('lists API keys', async () => {
        const mockResponse = { data: [{ id: 1, name: 'Key 1', created_at: '2024-01-01T00:00:00Z' }] };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.auth.listApiKeys();

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/auth/api-keys', undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('revokeApiKey', () => {
      it('revokes API key', async () => {
        mockAxiosInstance.delete.mockResolvedValueOnce({ data: {} });

        await client.auth.revokeApiKey(1);

        expect(mockAxiosInstance.delete).toHaveBeenCalledWith('/auth/api-keys/1', undefined);
      });
    });
  });

  describe('SMB service', () => {
    describe('getConfigs', () => {
      it('gets SMB configurations', async () => {
        const mockResponse = {
          data: [
            {
              id: 1,
              name: 'Media Server',
              host: '192.168.1.100',
              share_name: 'media',
              username: 'user'
            }
          ]
        };

        const axiosResponse = { data: mockResponse.data };
        mockAxiosInstance.get.mockResolvedValueOnce(axiosResponse);

        const result = await client.smb.getConfigs();

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/smb/configs', undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('getStatus', () => {
      it('gets SMB status', async () => {
        const mockResponse = {
          data: [
            {
              id: 1,
              config_id: 1,
              status: 'connected',
              last_connected: '2024-01-01T00:00:00Z'
            }
          ]
        };

        const axiosResponse = { data: mockResponse.data };
        mockAxiosInstance.get.mockResolvedValueOnce(axiosResponse);

        const result = await client.smb.getStatus();

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/smb/status', undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('connect', () => {
      it('connects to SMB share', async () => {
        const mockResponse = {
          data: { success: true, message: 'Connected successfully' }
        };

        const axiosResponse = { data: mockResponse.data };
        mockAxiosInstance.post.mockResolvedValueOnce(axiosResponse);

        const result = await client.smb.connect(1);

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/smb/connect/1', undefined, undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('createConfig', () => {
      it('creates SMB configuration', async () => {
        const config = {
          name: 'Test Share',
          host: '192.168.1.100',
          port: 445,
          share_name: 'media',
          username: 'user',
          password: 'pass',
          mount_point: '/mnt/media'
        };

        const mockResponse = {
          data: { id: 1, ...config }
        };

        const axiosResponse = { data: mockResponse.data };
        mockAxiosInstance.post.mockResolvedValueOnce(axiosResponse);

        const result = await client.smb.createConfig(config);

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/smb/configs', config, undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('getConfig', () => {
      it('gets specific SMB configuration', async () => {
        const mockResponse = { data: { id: 1, name: 'Media Server', host: '192.168.1.100' } };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.smb.getConfig(1);

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/smb/configs/1', undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('updateConfig', () => {
      it('updates SMB configuration', async () => {
        const mockResponse = { data: { id: 1, name: 'Updated Name' } };
        mockAxiosInstance.put.mockResolvedValueOnce(mockResponse);

        const result = await client.smb.updateConfig(1, { name: 'Updated Name' });

        expect(mockAxiosInstance.put).toHaveBeenCalledWith('/smb/configs/1', { name: 'Updated Name' }, undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('deleteConfig', () => {
      it('deletes SMB configuration', async () => {
        mockAxiosInstance.delete.mockResolvedValueOnce({ data: {} });

        await client.smb.deleteConfig(1);

        expect(mockAxiosInstance.delete).toHaveBeenCalledWith('/smb/configs/1', undefined);
      });
    });

    describe('testConnection', () => {
      it('tests SMB connection', async () => {
        const mockResponse = { data: { success: true, message: 'Connection successful' } };
        mockAxiosInstance.post.mockResolvedValueOnce(mockResponse);

        const config = { name: 'Test', host: '192.168.1.100', port: 445, share_name: 'media', username: 'user', password: 'pass', mount_point: '/mnt/media' };
        const result = await client.smb.testConnection(config);

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/smb/test', config, undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('testExistingConfig', () => {
      it('tests existing SMB configuration', async () => {
        const mockResponse = { data: { success: true, message: 'Connection successful' } };
        mockAxiosInstance.post.mockResolvedValueOnce(mockResponse);

        const result = await client.smb.testExistingConfig(1);

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/smb/configs/1/test', undefined, undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('getConfigStatus', () => {
      it('gets specific config status', async () => {
        const mockResponse = { data: { config_id: 1, status: 'connected' } };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.smb.getConfigStatus(1);

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/smb/status/1', undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('disconnect', () => {
      it('disconnects from SMB share', async () => {
        const mockResponse = { data: { success: true, message: 'Disconnected' } };
        mockAxiosInstance.post.mockResolvedValueOnce(mockResponse);

        const result = await client.smb.disconnect(1);

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/smb/disconnect/1', undefined, undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('reconnect', () => {
      it('reconnects to SMB share', async () => {
        const disconnectResponse = { data: { success: true, message: 'Disconnected' } };
        const connectResponse = { data: { success: true, message: 'Connected' } };
        mockAxiosInstance.post.mockResolvedValueOnce(disconnectResponse);
        mockAxiosInstance.post.mockResolvedValueOnce(connectResponse);

        const result = await client.smb.reconnect(1);

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/smb/disconnect/1', undefined, undefined);
        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/smb/connect/1', undefined, undefined);
        expect(result).toEqual(connectResponse.data);
      });
    });

    describe('scan', () => {
      it('starts SMB scan', async () => {
        const mockResponse = { data: { job_id: 1, message: 'Scan started' } };
        mockAxiosInstance.post.mockResolvedValueOnce(mockResponse);

        const result = await client.smb.scan(1, { deep_scan: true });

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/smb/scan/1', { deep_scan: true }, undefined);
        expect(result).toEqual(mockResponse.data);
      });

      it('starts scan with default options', async () => {
        const mockResponse = { data: { job_id: 1, message: 'Scan started' } };
        mockAxiosInstance.post.mockResolvedValueOnce(mockResponse);

        await client.smb.scan(1);

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/smb/scan/1', {}, undefined);
      });
    });

    describe('getScanStatus', () => {
      it('gets scan job status', async () => {
        const mockResponse = { data: { id: 1, status: 'in_progress', progress: 50 } };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.smb.getScanStatus(1);

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/smb/scan-jobs/1', undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('cancelScan', () => {
      it('cancels scan job', async () => {
        mockAxiosInstance.post.mockResolvedValueOnce({ data: {} });

        await client.smb.cancelScan(1);

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/smb/scan-jobs/1/cancel', undefined, undefined);
      });
    });

    describe('getScanJobs', () => {
      it('gets all scan jobs', async () => {
        const mockResponse = { data: [{ id: 1, status: 'completed' }] };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.smb.getScanJobs();

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/smb/scan-jobs', undefined);
        expect(result).toEqual(mockResponse.data);
      });

      it('gets scan jobs for specific config', async () => {
        const mockResponse = { data: [{ id: 1, config_id: 1, status: 'completed' }] };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.smb.getScanJobs(1);

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/smb/scan-jobs?config_id=1', undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('browse', () => {
      it('browses SMB directory', async () => {
        const mockResponse = {
          data: {
            current_path: '/media',
            directories: [{ name: 'movies', path: '/media/movies' }],
            files: [{ name: 'file.txt', path: '/media/file.txt', size: 100, modified: '2024-01-01' }]
          }
        };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.smb.browse(1, '/media');

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/smb/configs/1/browse?path=%2Fmedia', undefined);
        expect(result).toEqual(mockResponse.data);
      });

      it('browses root directory', async () => {
        const mockResponse = { data: { current_path: '/', directories: [], files: [] } };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        await client.smb.browse(1);

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/smb/configs/1/browse', undefined);
      });
    });

    describe('toggleConfig', () => {
      it('toggles config active state', async () => {
        const mockResponse = { data: { id: 1, is_active: false } };
        mockAxiosInstance.patch.mockResolvedValueOnce(mockResponse);

        const result = await client.smb.toggleConfig(1, false);

        expect(mockAxiosInstance.patch).toHaveBeenCalledWith('/smb/configs/1', { is_active: false }, undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('getShareInfo', () => {
      it('gets SMB share info', async () => {
        const mockResponse = {
          data: {
            total_space: 1000000000,
            free_space: 500000000,
            used_space: 500000000,
            mount_point: '/mnt/media',
            share_name: 'media',
            server_name: 'nas'
          }
        };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.smb.getShareInfo(1);

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/smb/configs/1/info', undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('refreshAllConnections', () => {
      it('refreshes all SMB connections', async () => {
        const mockResponse = {
          data: {
            refreshed: 2,
            failed: 1,
            results: [
              { config_id: 1, success: true, message: 'OK' },
              { config_id: 2, success: true, message: 'OK' },
              { config_id: 3, success: false, message: 'Failed' }
            ]
          }
        };
        mockAxiosInstance.post.mockResolvedValueOnce(mockResponse);

        const result = await client.smb.refreshAllConnections();

        expect(mockAxiosInstance.post).toHaveBeenCalledWith('/smb/refresh-all', undefined, undefined);
        expect(result).toEqual(mockResponse.data);
      });
    });

    describe('getLogs', () => {
      it('gets SMB logs', async () => {
        const mockResponse = {
          data: [{ id: 1, config_id: 1, level: 'info', message: 'Connected', timestamp: '2024-01-01' }]
        };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        const result = await client.smb.getLogs();

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/smb/logs?limit=100', undefined);
        expect(result).toEqual(mockResponse.data);
      });

      it('gets logs for specific config', async () => {
        const mockResponse = { data: [] };
        mockAxiosInstance.get.mockResolvedValueOnce(mockResponse);

        await client.smb.getLogs(1, 50);

        expect(mockAxiosInstance.get).toHaveBeenCalledWith('/smb/logs?config_id=1&limit=50', undefined);
      });
    });
  });

  describe('error handling', () => {
    it('handles network errors', async () => {
      mockAxiosInstance.get.mockRejectedValueOnce(new Error('Network Error'));

      await expect(client.media.search({ query: 'test' })).rejects.toThrow('Network Error');
    });

    it('handles HTTP errors', async () => {
      const catalogizerError = new CatalogizerError('Not found', 404, 'NOT_FOUND');
      mockAxiosInstance.get.mockRejectedValueOnce(catalogizerError);

      await expect(client.media.getById(999)).rejects.toThrow(CatalogizerError);
    });

    it('handles timeout errors', async () => {
      const networkError = new Error('Network connection failed');
      (networkError as any).code = 'ECONNABORTED';
      mockAxiosInstance.get.mockRejectedValueOnce(networkError);

      await expect(client.media.search({ query: 'test' })).rejects.toThrow('Network connection failed');
    });
  });

  describe('configuration', () => {
    it('uses custom timeout', () => {
      const customClient = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
        timeout: 5000
      });

      expect(customClient).toBeDefined();
    });

    it('handles invalid base URL', () => {
      expect(() => {
        new CatalogizerClient({
          baseURL: 'invalid-url',
        });
      }).not.toThrow();
    });
  });
});
