export interface SystemInfo {
  version: string;
  uptime: number;
  cpuUsage: number;
  memoryUsage: number;
  diskUsage: {
    total: number;
    used: number;
    free: number;
  };
  activeConnections: number;
  totalRequests: number;
}

export interface User {
  id: string;
  username: string;
  email: string;
  role: 'admin' | 'user' | 'viewer';
  status: 'active' | 'inactive' | 'suspended' | 'pending';
  lastLogin?: string;
  createdAt: string;
}

export interface StorageInfo {
  path: string;
  totalSpace: number;
  usedSpace: number;
  availableSpace: number;
  mediaCount: number;
  lastScan?: string;
}

export interface Backup {
  id: string;
  filename: string;
  size: number;
  createdAt: string;
  type: 'full' | 'incremental';
  status: 'completed' | 'in-progress' | 'failed';
}