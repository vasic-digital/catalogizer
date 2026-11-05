import { create } from 'zustand';
import { invoke } from '@tauri-apps/api/core';
import { User } from '../types';

interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  error: string | null;
  serverUrl: string | null;

  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  setAuthToken: (token: string) => void;
  clearAuth: () => void;
  checkAuthStatus: () => Promise<void>;
  setServerUrl: (url: string) => void;
}

// catalog-api serves the versioned REST surface under /api/v1.
const API_PREFIX = '/api/v1';

// The authoritative server URL lives in the Tauri config state (set by configStore); the auth store's own
// copy is never populated by the app, so prefer the config and fall back to the store value.
async function resolveServerUrl(stored: string | null): Promise<string | null> {
  try {
    const config = await invoke<any>('get_config');
    if (config && config.server_url) return config.server_url;
  } catch (e) {
    // fall through to the stored value
  }
  return stored;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  isAuthenticated: false,
  isLoading: false,
  error: null,
  serverUrl: null,

  login: async (username: string, password: string) => {
    set({ isLoading: true, error: null });

    try {
      const serverUrl = await resolveServerUrl(get().serverUrl);
      const response = await invoke<string>('make_http_request', {
        url: `${serverUrl}${API_PREFIX}/auth/login`,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ username, password }),
      });

      const data = JSON.parse(response);

      // Store auth token
      await invoke('set_auth_token', { token: data.token ?? data.session_token });

      set({
        user: data.user,
        isAuthenticated: true,
        isLoading: false,
        error: null,
      });
    } catch (error) {
      set({
        isAuthenticated: false,
        isLoading: false,
        error: error instanceof Error ? error.message : 'Login failed',
      });
      throw error;
    }
  },

  logout: async () => {
    try {
      // Clear auth token from storage
      await invoke('clear_auth_token');

      // Optionally call logout endpoint
      try {
        const serverUrl = await resolveServerUrl(get().serverUrl);
        await invoke<string>('make_http_request', {
          url: `${serverUrl}${API_PREFIX}/auth/logout`,
          method: 'POST',
          headers: {},
        });
      } catch (e) {
        // Ignore logout endpoint errors
      }

      set({
        user: null,
        isAuthenticated: false,
        error: null,
      });
    } catch (error) {
      console.error('Logout error:', error);
    }
  },

  setAuthToken: (_token: string) => {
    // This is called when loading stored token on app start
    set({ isAuthenticated: true });
  },

  clearAuth: () => {
    set({
      user: null,
      isAuthenticated: false,
      error: null,
    });
  },

  checkAuthStatus: async () => {
    try {
      const config = await invoke<any>('get_config');
      if (!config.auth_token || !config.server_url) {
        set({ isAuthenticated: false });
        return;
      }

      const response = await invoke<string>('make_http_request', {
        url: `${config.server_url}${API_PREFIX}/auth/status`,
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${config.auth_token}`,
        },
      });

      const data = JSON.parse(response);

      if (data.authenticated) {
        set({
          user: data.user,
          isAuthenticated: true,
          error: null,
        });
      } else {
        set({ isAuthenticated: false });
        await invoke('clear_auth_token');
      }
    } catch (error) {
      set({ isAuthenticated: false });
      await invoke('clear_auth_token');
    }
  },

  setServerUrl: (url: string) => {
    set({ serverUrl: url });
  },
}));