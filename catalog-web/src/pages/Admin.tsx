import React from 'react';
import { AdminPanel } from '@/components/admin/AdminPanel';
import { adminApi } from '@/lib/adminApi';
import { reportsApi } from '@/lib/reportsApi';
import { useQuery } from '@tanstack/react-query';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card';
import { FileBarChart, Gauge } from 'lucide-react';
import toast from 'react-hot-toast';
import type { User, SystemInfo } from '@/types/admin';

// Shown only while the system-info request is loading or has failed. Every
// figure is null / zero-with-unavailable, never an invented healthy-looking value.
const UNAVAILABLE_SYSTEM_INFO: SystemInfo = {
  version: 'Unavailable',
  uptime: 0,
  cpuUsage: null,
  memoryUsage: 0,
  diskUsage: null,
  activeConnections: 0,
  totalRequests: null,
  unavailable: ['all'],
};

/** Format a metric the backend may not measure: null/undefined/NaN render as an em dash, never a crash. */
const fmtNum = (v: number | null | undefined, unit = '', digits?: number): string => {
  if (typeof v !== 'number' || !Number.isFinite(v)) return '—';
  const text = digits === undefined ? v.toLocaleString() : v.toFixed(digits);
  return `${text}${unit}`;
};

export const Admin: React.FC = () => {
  const { data: systemInfo, isLoading: _systemLoading } = useQuery({
    queryKey: ['admin-system-info'],
    queryFn: () => adminApi.getSystemInfo(),
    staleTime: 1000 * 60 * 2,
    refetchInterval: 1000 * 30, // Refresh every 30 seconds
  });

  const { data: users, isLoading: _usersLoading } = useQuery({
    queryKey: ['admin-users'],
    queryFn: () => adminApi.getUsers(),
    staleTime: 1000 * 60 * 5,
  });

  const { data: storageInfo, isLoading: _storageLoading } = useQuery({
    queryKey: ['admin-storage'],
    queryFn: () => adminApi.getStorageInfo(),
    staleTime: 1000 * 60 * 5,
  });

  const { data: backups, isLoading: _backupsLoading } = useQuery({
    queryKey: ['admin-backups'],
    queryFn: () => adminApi.getBackups(),
    staleTime: 1000 * 60 * 2,
  });

  const { data: usageReport } = useQuery({
    queryKey: ['reports-usage'],
    queryFn: () => reportsApi.getUsageReport(),
    staleTime: 1000 * 60 * 5,
  });

  const { data: performanceReport } = useQuery({
    queryKey: ['reports-performance'],
    queryFn: () => reportsApi.getPerformanceReport(),
    staleTime: 1000 * 60 * 5,
  });

  const handleCreateBackup = async (type: 'full' | 'incremental') => {
    try {
      await adminApi.createBackup(type);
      toast.success(`${type === 'full' ? 'Full' : 'Incremental'} backup started successfully`);
    } catch (error) {
      toast.error(`Failed to create backup: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  };

  const handleRestoreBackup = async (id: string) => {
    try {
      await adminApi.restoreBackup(id);
      toast.success('Backup restore initiated successfully');
    } catch (error) {
      toast.error(`Failed to restore backup: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  };

  const handleScanStorage = async (path: string) => {
    try {
      await adminApi.scanStorage(path);
      toast.success(`Storage scan initiated for ${path}`);
    } catch (error) {
      toast.error(`Failed to scan storage: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  };

  const handleUpdateUser = async (id: string, updates: Partial<User>) => {
    try {
      await adminApi.updateUser(id, updates);
      toast.success('User updated successfully');
    } catch (error) {
      toast.error(`Failed to update user: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  };

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <AdminPanel
        systemInfo={systemInfo || UNAVAILABLE_SYSTEM_INFO}
        users={users || []}
        storageInfo={storageInfo || []}
        backups={backups || []}
        onCreateBackup={handleCreateBackup}
        onRestoreBackup={handleRestoreBackup}
        onScanStorage={handleScanStorage}
        onUpdateUser={handleUpdateUser}
      />

      {/* Reports Section */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mt-8">
        {/* Usage Report */}
        {usageReport && (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <FileBarChart className="h-5 w-5" />
                Usage Report
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                <div className="grid grid-cols-2 gap-3">
                  <div className="p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
                    <div className="text-xl font-bold text-gray-900 dark:text-white">
                      {fmtNum(usageReport.active_users)}
                    </div>
                    <div className="text-xs text-gray-500">Active Users</div>
                  </div>
                  <div className="p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
                    <div className="text-xl font-bold text-gray-900 dark:text-white">
                      {fmtNum(usageReport.total_media_accessed)}
                    </div>
                    <div className="text-xs text-gray-500">Media Accessed</div>
                  </div>
                  <div className="p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
                    <div className="text-xl font-bold text-gray-900 dark:text-white">
                      {fmtNum(usageReport.total_downloads)}
                    </div>
                    <div className="text-xs text-gray-500">Downloads</div>
                  </div>
                  <div className="p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
                    <div className="text-xl font-bold text-gray-900 dark:text-white">
                      {usageReport.storage_growth_bytes == null ? '—' : `${(usageReport.storage_growth_bytes / (1024 ** 3)).toFixed(1)} GB`}
                    </div>
                    <div className="text-xs text-gray-500">Storage Growth</div>
                  </div>
                </div>
                {usageReport.top_media && usageReport.top_media.length > 0 && (
                  <div>
                    <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Top Media</h4>
                    <div className="space-y-1">
                      {usageReport.top_media.slice(0, 5).map((m) => (
                        <div key={m.id} className="flex items-center justify-between text-sm">
                          <span className="text-gray-900 dark:text-white truncate flex-1">{m.title}</span>
                          <span className="text-gray-500 ml-2">{m.access_count} accesses</span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        )}

        {/* Performance Report */}
        {performanceReport && (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Gauge className="h-5 w-5" />
                Performance Report
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                <div className="grid grid-cols-2 gap-3">
                  <div className="p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
                    <div className="text-xl font-bold text-gray-900 dark:text-white">
                      {fmtNum(performanceReport.p95_response_time_ms, 'ms')}
                    </div>
                    <div className="text-xs text-gray-500">P95 Response</div>
                  </div>
                  <div className="p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
                    <div className="text-xl font-bold text-gray-900 dark:text-white">
                      {fmtNum(performanceReport.total_requests)}
                    </div>
                    <div className="text-xs text-gray-500">Total Requests</div>
                  </div>
                  <div className="p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
                    <div className={`text-xl font-bold ${(performanceReport.error_rate ?? 0) > 5 ? 'text-red-600' : 'text-green-600'}`}>
                      {fmtNum(performanceReport.error_rate, '%', 1)}
                    </div>
                    <div className="text-xs text-gray-500">Error Rate</div>
                  </div>
                </div>
                <div className="p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
                  <div className="text-xl font-bold text-green-600">
                    {fmtNum(performanceReport.uptime_percentage, '%', 2)}
                  </div>
                  <div className="text-xs text-gray-500">Uptime</div>
                </div>
                {performanceReport.slowest_endpoints && performanceReport.slowest_endpoints.length > 0 && (
                  <div>
                    <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Slowest Endpoints</h4>
                    <div className="space-y-1">
                      {performanceReport.slowest_endpoints.slice(0, 5).map((ep, idx) => (
                        <div key={idx} className="flex items-center justify-between text-sm">
                          <span className="text-gray-900 dark:text-white truncate flex-1 font-mono text-xs">{ep.path}</span>
                          <span className="text-gray-500 ml-2">{ep.avg_ms}ms ({ep.count}x)</span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  );
};