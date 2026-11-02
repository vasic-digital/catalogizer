import type { SystemInfo, User, StorageInfo, Backup } from '@/types/admin';
import { api } from './api';

// Every method propagates API errors. There are no mock or 404-swallowing
// fallbacks: a failed call must surface as an error, never as fabricated data
// or a fake success (CAT-001).
export const adminApi = {
  async getSystemInfo(): Promise<SystemInfo> {
    const response = await api.get('/admin/system-info');
    return response.data;
  },

  async getUsers(): Promise<User[]> {
    const response = await api.get('/admin/users');
    return response.data;
  },

  async getStorageInfo(): Promise<StorageInfo[]> {
    const response = await api.get('/admin/storage');
    return response.data;
  },

  async getBackups(): Promise<Backup[]> {
    const response = await api.get('/admin/backups');
    return response.data;
  },

  async createBackup(type: 'full' | 'incremental'): Promise<void> {
    await api.post('/admin/backups', { type });
  },

  async restoreBackup(id: string): Promise<void> {
    await api.post(`/admin/backups/${id}/restore`);
  },

  async scanStorage(path: string): Promise<void> {
    await api.post('/admin/storage/scan', { path });
  },

  async updateUser(id: string, updates: Partial<User>): Promise<void> {
    await api.put(`/admin/users/${id}`, updates);
  },
};
