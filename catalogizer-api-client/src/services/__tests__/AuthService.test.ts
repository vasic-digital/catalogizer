import { describe, it, expect, vi, beforeEach } from 'vitest';
import { AuthService } from '../AuthService';
import { HttpClient } from '../../utils/http';

// Mock HttpClient
vi.mock('../../utils/http');

describe('AuthService', () => {
  let authService: AuthService;
  let mockHttp: any;

  beforeEach(() => {
    vi.clearAllMocks();

    mockHttp = {
      get: vi.fn(),
      post: vi.fn(),
      put: vi.fn(),
      patch: vi.fn(),
      delete: vi.fn(),
      setAuthToken: vi.fn(),
      clearAuthToken: vi.fn(),
      getAuthToken: vi.fn(),
    };

    authService = new AuthService(mockHttp as any);
  });

  describe('login', () => {
    it('sends credentials and returns login response', async () => {
      const loginResponse = {
        token: 'jwt-token',
        refresh_token: 'refresh-token',
        expires_in: 3600,
        user: { id: 1, username: 'testuser', email: 'test@test.com' },
      };
      mockHttp.post.mockResolvedValueOnce(loginResponse);

      const result = await authService.login({
        username: 'testuser',
        password: 'password123',
      });

      expect(mockHttp.post).toHaveBeenCalledWith('/auth/login', {
        username: 'testuser',
        password: 'password123',
      });
      expect(result).toEqual(loginResponse);
    });

    it('sets auth token after successful login', async () => {
      const loginResponse = { token: 'new-jwt-token', refresh_token: 'r', expires_in: 3600, user: {} };
      mockHttp.post.mockResolvedValueOnce(loginResponse);

      await authService.login({ username: 'user', password: 'pass' });

      expect(mockHttp.setAuthToken).toHaveBeenCalledWith('new-jwt-token');
    });

    // Article XI §11.5 regression guard for the contract bluff fixed
    // in commit a82ace8a (2026-04-29). The catalog-api login endpoint
    // returns the bearer token under `session_token` (canonical), but
    // the previous LoginResponse type required `token` and the
    // AuthService unconditionally read `response.token` — so the http
    // client never stored the bearer token, and isAuthenticated()
    // silently always returned false even after a successful login.
    // These three tests pin the dual-read behaviour so it can't
    // regress.
    it('extracts token from session_token (canonical current API)', async () => {
      const loginResponse: any = {
        session_token: 'canonical-jwt-token',
        refresh_token: 'r',
        expires_at: '2026-04-30T17:41:24Z',
        user: { id: 1, username: 'admin' },
      };
      mockHttp.post.mockResolvedValueOnce(loginResponse);

      await authService.login({ username: 'admin', password: 'Test-Admin-Pw-9xK2-local' });

      expect(mockHttp.setAuthToken).toHaveBeenCalledWith('canonical-jwt-token');
    });

    it('extracts token from legacy `token` field (back-compat)', async () => {
      const loginResponse: any = {
        token: 'legacy-jwt-token',
        refresh_token: 'r',
        expires_in: 3600,
        user: { id: 1, username: 'admin' },
      };
      mockHttp.post.mockResolvedValueOnce(loginResponse);

      await authService.login({ username: 'admin', password: 'Test-Admin-Pw-9xK2-local' });

      expect(mockHttp.setAuthToken).toHaveBeenCalledWith('legacy-jwt-token');
    });

    it('throws when neither session_token nor token is present', async () => {
      const loginResponse: any = {
        refresh_token: 'r',
        user: { id: 1 },
      };
      mockHttp.post.mockResolvedValueOnce(loginResponse);

      await expect(
        authService.login({ username: 'admin', password: 'Test-Admin-Pw-9xK2-local' }),
      ).rejects.toThrow(/missing both session_token and token/i);
      expect(mockHttp.setAuthToken).not.toHaveBeenCalled();
    });

    it('propagates errors on login failure', async () => {
      mockHttp.post.mockRejectedValueOnce(new Error('Invalid credentials'));

      await expect(authService.login({ username: 'wrong', password: 'wrong' }))
        .rejects.toThrow('Invalid credentials');
    });
  });

  describe('register', () => {
    it('sends registration data and returns user', async () => {
      const user = { id: 1, username: 'newuser', email: 'new@test.com' };
      mockHttp.post.mockResolvedValueOnce(user);

      const result = await authService.register({
        username: 'newuser',
        email: 'new@test.com',
        password: 'securepass',
        first_name: 'New',
        last_name: 'User',
      });

      expect(mockHttp.post).toHaveBeenCalledWith('/auth/register', {
        username: 'newuser',
        email: 'new@test.com',
        password: 'securepass',
        first_name: 'New',
        last_name: 'User',
      });
      expect(result).toEqual(user);
    });
  });

  describe('logout', () => {
    it('calls logout endpoint and clears token', async () => {
      mockHttp.post.mockResolvedValueOnce(undefined);

      await authService.logout();

      expect(mockHttp.post).toHaveBeenCalledWith('/auth/logout');
      expect(mockHttp.clearAuthToken).toHaveBeenCalled();
    });

    it('clears token even if logout request fails', async () => {
      mockHttp.post.mockRejectedValueOnce(new Error('Network error'));

      // logout uses try/finally (no catch), so the error propagates
      await expect(authService.logout()).rejects.toThrow('Network error');

      // But clearAuthToken should still be called via finally
      expect(mockHttp.clearAuthToken).toHaveBeenCalled();
    });
  });

  describe('getStatus', () => {
    it('returns authentication status', async () => {
      const status = { authenticated: true, user: { id: 1, username: 'testuser' } };
      mockHttp.get.mockResolvedValueOnce(status);

      const result = await authService.getStatus();

      expect(mockHttp.get).toHaveBeenCalledWith('/auth/status');
      expect(result).toEqual(status);
    });
  });

  describe('refreshToken', () => {
    it('refreshes token and updates auth header', async () => {
      const refreshResponse = { token: 'refreshed-token', expires_in: 7200 };
      mockHttp.post.mockResolvedValueOnce(refreshResponse);

      const result = await authService.refreshToken();

      expect(mockHttp.post).toHaveBeenCalledWith('/auth/refresh');
      expect(mockHttp.setAuthToken).toHaveBeenCalledWith('refreshed-token');
      expect(result).toEqual(refreshResponse);
    });
  });

  describe('profile operations', () => {
    it('gets user profile', async () => {
      const profile = { id: 1, username: 'testuser', email: 'test@test.com' };
      mockHttp.get.mockResolvedValueOnce(profile);

      const result = await authService.getProfile();

      expect(mockHttp.get).toHaveBeenCalledWith('/auth/profile');
      expect(result).toEqual(profile);
    });

    it('updates user profile', async () => {
      const updatedUser = { id: 1, first_name: 'Updated', last_name: 'Name' };
      mockHttp.put.mockResolvedValueOnce(updatedUser);

      const result = await authService.updateProfile({ first_name: 'Updated' });

      expect(mockHttp.put).toHaveBeenCalledWith('/auth/profile', { first_name: 'Updated' });
      expect(result).toEqual(updatedUser);
    });
  });

  describe('password operations', () => {
    it('changes password', async () => {
      mockHttp.post.mockResolvedValueOnce(undefined);

      await authService.changePassword({
        current_password: 'oldpass',
        new_password: 'newpass',
      });

      expect(mockHttp.post).toHaveBeenCalledWith('/auth/password', {
        current_password: 'oldpass',
        new_password: 'newpass',
      });
    });

    it('requests password reset', async () => {
      mockHttp.post.mockResolvedValueOnce(undefined);

      await authService.requestPasswordReset('test@test.com');

      expect(mockHttp.post).toHaveBeenCalledWith('/auth/password/reset', { email: 'test@test.com' });
    });

    it('resets password with token', async () => {
      mockHttp.post.mockResolvedValueOnce(undefined);

      await authService.resetPassword('reset-token-123', 'newpassword');

      expect(mockHttp.post).toHaveBeenCalledWith('/auth/password/reset/confirm', {
        token: 'reset-token-123',
        password: 'newpassword',
      });
    });
  });

  describe('email verification', () => {
    it('verifies email with token', async () => {
      mockHttp.post.mockResolvedValueOnce(undefined);

      await authService.verifyEmail('verify-token');

      expect(mockHttp.post).toHaveBeenCalledWith('/auth/email/verify', { token: 'verify-token' });
    });

    it('resends email verification', async () => {
      mockHttp.post.mockResolvedValueOnce(undefined);

      await authService.resendEmailVerification();

      expect(mockHttp.post).toHaveBeenCalledWith('/auth/email/verify/resend');
    });
  });

  describe('availability checks', () => {
    it('checks username availability', async () => {
      mockHttp.get.mockResolvedValueOnce({ available: true });

      const result = await authService.checkUsernameAvailability('newuser');

      expect(mockHttp.get).toHaveBeenCalledWith('/auth/username/check?username=newuser');
      expect(result).toEqual({ available: true });
    });

    it('checks email availability with URL encoding', async () => {
      mockHttp.get.mockResolvedValueOnce({ available: false });

      const result = await authService.checkEmailAvailability('user@example.com');

      expect(mockHttp.get).toHaveBeenCalledWith('/auth/email/check?email=user%40example.com');
      expect(result).toEqual({ available: false });
    });
  });

  describe('permissions', () => {
    it('gets user permissions', async () => {
      mockHttp.get.mockResolvedValueOnce(['read', 'write', 'admin:system']);

      const result = await authService.getPermissions();

      expect(mockHttp.get).toHaveBeenCalledWith('/auth/permissions');
      expect(result).toEqual(['read', 'write', 'admin:system']);
    });

    it('returns true for existing permission via hasPermission', async () => {
      mockHttp.get.mockResolvedValueOnce(['read', 'write']);

      const result = await authService.hasPermission('read');

      expect(result).toBe(true);
    });

    it('returns true for admin:system wildcard via hasPermission', async () => {
      mockHttp.get.mockResolvedValueOnce(['admin:system']);

      const result = await authService.hasPermission('any_permission');

      expect(result).toBe(true);
    });

    it('returns false for missing permission via hasPermission', async () => {
      mockHttp.get.mockResolvedValueOnce(['read']);

      const result = await authService.hasPermission('write');

      expect(result).toBe(false);
    });
  });

  describe('API keys', () => {
    it('generates an API key', async () => {
      const apiKey = { key: 'api-key-123', name: 'My Key', created_at: '2024-01-01T00:00:00Z' };
      mockHttp.post.mockResolvedValueOnce(apiKey);

      const result = await authService.generateApiKey('My Key');

      expect(mockHttp.post).toHaveBeenCalledWith('/auth/api-keys', { name: 'My Key' });
      expect(result).toEqual(apiKey);
    });

    it('lists API keys', async () => {
      const keys = [{ id: 1, name: 'Key 1', created_at: '2024-01-01T00:00:00Z' }];
      mockHttp.get.mockResolvedValueOnce(keys);

      const result = await authService.listApiKeys();

      expect(mockHttp.get).toHaveBeenCalledWith('/auth/api-keys');
      expect(result).toEqual(keys);
    });

    it('revokes an API key', async () => {
      mockHttp.delete.mockResolvedValueOnce(undefined);

      await authService.revokeApiKey(42);

      expect(mockHttp.delete).toHaveBeenCalledWith('/auth/api-keys/42');
    });
  });
});
