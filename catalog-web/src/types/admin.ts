export interface SystemInfo {
  version: string;
  uptime: number;
  /** null when the host cannot measure it (never a fabricated 0); see `unavailable`. */
  cpuUsage: number | null;
  memoryUsage: number;
  diskUsage: {
    total: number;
    used: number;
    free: number;
  } | null;
  activeConnections: number;
  totalRequests: number | null;
  /** Names of the figures above that the server could not measure. */
  unavailable?: string[];
}

export interface User {
  id: string;
  username: string;
  email: string;
  role: 'admin' | 'user' | 'viewer';
  status: 'active' | 'inactive' | 'suspended';
  lastLogin?: string;
  createdAt: string;
}

export interface StorageInfo {
  path: string;
  /** null for storage whose capacity is not measurable from here (remote protocols). */
  totalSpace: number | null;
  usedSpace: number | null;
  availableSpace: number | null;
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