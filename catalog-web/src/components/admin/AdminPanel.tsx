import React, { useState } from 'react';
import { Users, Settings, HardDrive, Database, Shield, Activity, Download, Upload, RefreshCw } from 'lucide-react';
import { Button } from '../ui/Button';
import { Card, CardContent, CardHeader, CardTitle } from '../ui/Card';
import { Badge } from '../ui/Badge';
import { Progress } from '../ui/Progress';

interface SystemInfo {
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

interface User {
  id: string;
  username: string;
  email: string;
  role: 'admin' | 'user' | 'viewer';
  status: 'active' | 'inactive' | 'suspended' | 'pending';
  lastLogin?: string;
  createdAt: string;
}

interface StorageInfo {
  path: string;
  totalSpace: number;
  usedSpace: number;
  availableSpace: number;
  mediaCount: number;
  lastScan?: string;
}

interface Backup {
  id: string;
  filename: string;
  size: number;
  createdAt: string;
  type: 'full' | 'incremental';
  status: 'completed' | 'in-progress' | 'failed';
}

interface AdminPanelProps {
  systemInfo: SystemInfo;
  users: User[];
  storageInfo: StorageInfo[];
  backups: Backup[];
  onCreateBackup?: (type: 'full' | 'incremental') => void;
  onRestoreBackup?: (id: string) => void;
  onScanStorage?: (path: string) => void;
  onUpdateUser?: (id: string, updates: Partial<User>) => void;
}

/**
 * AdminPanel provides system administration controls including user management,
 * storage monitoring, and backup operations.
 */
export const AdminPanel: React.FC<AdminPanelProps> = ({
  systemInfo,
  users,
  storageInfo,
  backups,
  onCreateBackup,
  onRestoreBackup,
  onScanStorage,
  onUpdateUser
}) => {
  const [activeTab, setActiveTab] = useState<'overview' | 'users' | 'storage' | 'backups'>('overview');

  const formatUptime = (seconds: number) => {
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    return `${days}d ${hours}h ${minutes}m`;
  };

  const formatFileSize = (bytes: number) => {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const getUserRoleBadge = (role: User['role']) => {
    const colors = {
      admin: 'destructive',
      user: 'default',
      viewer: 'secondary'
    } as const;

    return <Badge variant={colors[role]}>{role.toUpperCase()}</Badge>;
  };

  const getUserStatusBadge = (status: User['status']) => {
    const colors = {
      active: 'default',
      inactive: 'secondary',
      suspended: 'destructive',
      pending: 'outline'
    } as const;

    return <Badge variant={colors[status]}>{status.toUpperCase()}</Badge>;
  };

  const getBackupStatusBadge = (status: Backup['status']) => {
    const colors = {
      completed: 'default',
      'in-progress': 'default',
      failed: 'destructive'
    } as const;

    return <Badge variant={colors[status]}>{status.replace('-', ' ').toUpperCase()}</Badge>;
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-3xl font-bold text-gray-900 dark:text-white">Admin Panel</h1>
        <Button variant="outline">
          <RefreshCw className="w-4 h-4 mr-2" />
          Refresh
        </Button>
      </div>

      {/* Navigation Tabs */}
      <div className="border-b border-gray-200">
        <nav className="flex gap-x-4 sm:gap-x-8 overflow-x-auto" aria-label="Admin sections">
          {[
            { id: 'overview', label: 'Overview', icon: Activity },
            { id: 'users', label: 'Users', icon: Users },
            { id: 'storage', label: 'Storage', icon: HardDrive },
            { id: 'backups', label: 'Backups', icon: Database }
          ].map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              onClick={() => setActiveTab(id as 'overview' | 'users' | 'storage' | 'backups')}
              className={`flex flex-shrink-0 items-center gap-2 whitespace-nowrap py-2 px-1 border-b-2 font-medium text-sm transition-colors ${
                activeTab === id
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
              }`}
            >
              <Icon className="w-4 h-4" />
              {label}
            </button>
          ))}
        </nav>
      </div>

      {/* Overview Tab */}
      {activeTab === 'overview' && (
        <div className="space-y-6">
          {/* System Stats */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
            <Card>
              <CardContent className="p-6">
                <div className="flex items-center">
                  <div className="flex-1">
                    <p className="text-sm font-medium text-gray-600">Version</p>
                    <p className="text-2xl font-bold text-gray-900">{systemInfo.version}</p>
                  </div>
                  <Settings className="w-8 h-8 text-blue-500" />
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardContent className="p-6">
                <div className="flex items-center">
                  <div className="flex-1">
                    <p className="text-sm font-medium text-gray-600">Uptime</p>
                    <p className="text-2xl font-bold text-gray-900">{formatUptime(systemInfo.uptime)}</p>
                  </div>
                  <Activity className="w-8 h-8 text-green-500" />
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardContent className="p-6">
                <div className="flex items-center">
                  <div className="flex-1">
                    <p className="text-sm font-medium text-gray-600">Active Connections</p>
                    <p className="text-2xl font-bold text-gray-900">{systemInfo.activeConnections}</p>
                  </div>
                  <Users className="w-8 h-8 text-purple-500" />
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardContent className="p-6">
                <div className="flex items-center">
                  <div className="flex-1">
                    <p className="text-sm font-medium text-gray-600">Total Requests</p>
                    <p className="text-2xl font-bold text-gray-900">{systemInfo.totalRequests.toLocaleString()}</p>
                  </div>
                  <Activity className="w-8 h-8 text-orange-500" />
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Resource Usage */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <Card>
              <CardHeader>
                <CardTitle>System Resources</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div>
                  <div className="flex justify-between mb-2">
                    <span className="text-sm font-medium">CPU Usage</span>
                    <span className="text-sm text-gray-600">{systemInfo.cpuUsage}%</span>
                  </div>
                  <Progress value={systemInfo.cpuUsage} />
                </div>
                <div>
                  <div className="flex justify-between mb-2">
                    <span className="text-sm font-medium">Memory Usage</span>
                    <span className="text-sm text-gray-600">{systemInfo.memoryUsage}%</span>
                  </div>
                  <Progress value={systemInfo.memoryUsage} />
                </div>
                <div>
                  <div className="flex justify-between mb-2">
                    <span className="text-sm font-medium">Disk Usage</span>
                    <span className="text-sm text-gray-600">
                      {formatFileSize(systemInfo.diskUsage.used)} / {formatFileSize(systemInfo.diskUsage.total)}
                    </span>
                  </div>
                  <Progress 
                    value={(systemInfo.diskUsage.used / systemInfo.diskUsage.total) * 100} 
                  />
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Quick Actions</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <Button 
                  onClick={() => onCreateBackup?.('full')}
                  className="w-full justify-start"
                >
                  <Download className="w-4 h-4 mr-2" />
                  Create Full Backup
                </Button>
                <Button 
                  onClick={() => onCreateBackup?.('incremental')}
                  variant="outline"
                  className="w-full justify-start"
                >
                  <Download className="w-4 h-4 mr-2" />
                  Create Incremental Backup
                </Button>
                <Button 
                  variant="outline"
                  className="w-full justify-start"
                >
                  <RefreshCw className="w-4 h-4 mr-2" />
                  Scan All Storage
                </Button>
                <Button 
                  variant="outline"
                  className="w-full justify-start"
                >
                  <Shield className="w-4 h-4 mr-2" />
                  Security Audit
                </Button>
              </CardContent>
            </Card>
          </div>
        </div>
      )}

      {/* Users Tab */}
      {activeTab === 'users' && (
        <Card>
          <CardHeader>
            <CardTitle>User Management</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b">
                    <th className="text-left p-2">Username</th>
                    <th className="text-left p-2">Email</th>
                    <th className="text-left p-2">Role</th>
                    <th className="text-left p-2">Status</th>
                    <th className="text-left p-2">Last Login</th>
                    <th className="text-left p-2">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {users.map(user => (
                    <tr key={user.id} className="border-b">
                      <td className="p-2 font-medium">{user.username}</td>
                      <td className="p-2">{user.email}</td>
                      <td className="p-2">{getUserRoleBadge(user.role || 'user')}</td>
                      <td className="p-2">{getUserStatusBadge(user.status)}</td>
                      <td className="p-2">
                        {user.lastLogin ? new Date(user.lastLogin).toLocaleDateString() : 'Never'}
                      </td>
                      <td className="p-2">
                        {user.status === 'pending' ? (
                          <div className="flex gap-2">
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => onUpdateUser?.(user.id, { status: 'active' })}
                            >
                              Approve
                            </Button>
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => onUpdateUser?.(user.id, { status: 'inactive' })}
                            >
                              Reject
                            </Button>
                          </div>
                        ) : (
                          <Button variant="outline" size="sm">
                            Edit
                          </Button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Storage Tab */}
      {activeTab === 'storage' && (
        <div className="space-y-4">
          {storageInfo.map((storage, index) => (
            <Card key={index}>
              <CardHeader>
                <CardTitle className="flex items-center justify-between">
                  <span>{storage.path}</span>
                  <Button 
                    variant="outline"
                    size="sm"
                    onClick={() => onScanStorage?.(storage.path)}
                  >
                    <RefreshCw className="w-4 h-4 mr-2" />
                    Scan
                  </Button>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                  <div>
                    <p className="text-sm font-medium text-gray-600">Total Space</p>
                    <p className="text-lg font-semibold">{formatFileSize(storage.totalSpace)}</p>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-gray-600">Used Space</p>
                    <p className="text-lg font-semibold">{formatFileSize(storage.usedSpace)}</p>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-gray-600">Available</p>
                    <p className="text-lg font-semibold">{formatFileSize(storage.availableSpace)}</p>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-gray-600">Media Count</p>
                    <p className="text-lg font-semibold">{storage.mediaCount.toLocaleString()}</p>
                  </div>
                </div>
                <div className="mt-4">
                  <Progress 
                    value={(storage.usedSpace / storage.totalSpace) * 100} 
                    className="mb-2"
                  />
                  <p className="text-xs text-gray-600">
                    {storage.lastScan ? `Last scanned: ${new Date(storage.lastScan).toLocaleString()}` : 'Not scanned yet'}
                  </p>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Backups Tab */}
      {activeTab === 'backups' && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>Backup Management</CardTitle>
              <div className="flex gap-2">
                <Button onClick={() => onCreateBackup?.('full')}>
                  <Upload className="w-4 h-4 mr-2" />
                  Full Backup
                </Button>
                <Button variant="outline" onClick={() => onCreateBackup?.('incremental')}>
                  <Upload className="w-4 h-4 mr-2" />
                  Incremental
                </Button>
              </div>
            </div>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {backups.map(backup => (
                <div key={backup.id} className="flex items-center justify-between p-4 border rounded-lg">
                  <div className="flex-1">
                    <h4 className="font-medium">{backup.filename}</h4>
                    <p className="text-sm text-gray-600">
                      {formatFileSize(backup.size)} • Created {new Date(backup.createdAt).toLocaleString()}
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    {getBackupStatusBadge(backup.status)}
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={backup.status !== 'completed'}
                      onClick={() => onRestoreBackup?.(backup.id)}
                    >
                      <Download className="w-4 h-4 mr-2" />
                      Restore
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
};