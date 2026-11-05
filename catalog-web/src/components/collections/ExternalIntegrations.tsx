import React, { useState, useEffect, useCallback } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Globe,
  Cloud,
  Plus,
  Trash2,
  Edit,
  CheckCircle,
  AlertCircle,
  XCircle,
  Clock,
  RefreshCw,
  Link,
  Key,
  Zap,
  Database,
  FolderSync,
  Share2,
  Info,
  ExternalLink,
  TestTube,
  Activity
} from 'lucide-react';
import { Button } from '../ui/Button';
import { Input } from '../ui/Input';
import { Badge } from '../ui/Badge';
import { Switch } from '../ui/Switch';
import { toast } from 'react-hot-toast';

// Types
interface ExternalIntegration {
  id: string;
  name: string;
  provider: string;
  type: 'storage' | 'metadata' | 'analytics' | 'automation' | 'sharing';
  status: 'connected' | 'disconnected' | 'error' | 'connecting';
  description: string;
  config: IntegrationConfig;
  syncSettings: SyncSettings;
  statistics: IntegrationStats;
  lastSync?: string;
  createdAt: string;
  enabled: boolean;
}

interface IntegrationConfig {
  apiKey?: string;
  apiSecret?: string;
  endpoint?: string;
  username?: string;
  password?: string;
  webhookUrl?: string;
  customFields?: Record<string, unknown>;
}

interface SyncSettings {
  enabled: boolean;
  frequency: 'realtime' | 'hourly' | 'daily' | 'weekly' | 'manual';
  direction: 'import' | 'export' | 'bidirectional';
  filters: SyncFilter[];
  mapping?: Record<string, string>;
}

interface SyncFilter {
  field: string;
  operator: string;
  value: unknown;
}

interface IntegrationStats {
  totalSyncs: number;
  successfulSyncs: number;
  failedSyncs: number;
  lastSyncStatus: 'success' | 'error' | 'pending';
  itemsProcessed: number;
  lastSyncDuration?: number;
  bytesTransferred?: number;
}

const INTEGRATION_EXAMPLES = [
  {
    name: 'Google Drive',
    type: 'storage',
    description: 'Access files from Google Drive',
    features: ['File access', 'Metadata sync', 'Real-time updates'],
    setupUrl: 'https://console.developers.google.com/'
  },
  {
    name: 'TMDB',
    type: 'metadata',
    description: 'Fetch movie and TV metadata from The Movie Database',
    features: ['Movie metadata', 'TV metadata', 'Image posters'],
    setupUrl: 'https://www.themoviedb.org/settings/api'
  },
  {
    name: 'Plex',
    type: 'analytics',
    description: 'Sync with Plex media server',
    features: ['Library sync', 'Watch status', 'Analytics'],
    setupUrl: 'https://www.plex.tv/'
  },
  {
    name: 'Discord',
    type: 'sharing',
    description: 'Share collections to Discord channels',
    features: ['Channel notifications', 'File sharing', 'Status updates'],
    setupUrl: 'https://discord.com/developers/applications/'
  }
];

const ExternalIntegrations: React.FC = () => {
  const [integrations, setIntegrations] = useState<ExternalIntegration[]>([]);
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [editingIntegration, setEditingIntegration] = useState<ExternalIntegration | null>(null);
  const [testingIntegration, setTestingIntegration] = useState<string | null>(null);
  const [expandedIntegration, setExpandedIntegration] = useState<string | null>(null);
  const [filter, setFilter] = useState<'all' | 'connected' | 'disconnected'>('all');
  const [typeFilter, setTypeFilter] = useState<string>('all');
  const [searchQuery, setSearchQuery] = useState('');

  // Load existing integrations from the API
  const loadIntegrations = useCallback(async () => {
    try {
      const response = await fetch('/api/v1/integrations', {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('auth_token') || ''}`,
        },
      });
      if (response.ok) {
        const data = await response.json();
        if (Array.isArray(data)) {
          setIntegrations(data.map((item: Record<string, unknown>) => ({
            id: String(item.id || ''),
            name: String(item.name || ''),
            provider: String(item.provider || ''),
            type: (item.type as ExternalIntegration['type']) || 'storage',
            status: (item.status as ExternalIntegration['status']) || 'disconnected',
            description: String(item.description || ''),
            config: (item.config as IntegrationConfig) || {},
            syncSettings: (item.syncSettings as SyncSettings) || (item.sync_settings as SyncSettings) || {
              enabled: false, frequency: 'manual', direction: 'import', filters: [],
            },
            statistics: (item.statistics as IntegrationStats) || {
              totalSyncs: 0, successfulSyncs: 0, failedSyncs: 0,
              lastSyncStatus: 'pending', itemsProcessed: 0,
            },
            lastSync: item.last_sync as string | undefined,
            createdAt: String(item.created_at || item.createdAt || new Date().toISOString()),
            enabled: item.enabled !== false,
          })));
          return;
        }
      }
      // Endpoint does not exist yet or returned non-array; start with empty state
      setIntegrations([]);
    } catch {
      // API not available; start with empty list
      setIntegrations([]);
    }
  }, []);

  useEffect(() => {
    loadIntegrations();
  }, [loadIntegrations]);

  // Filter integrations
  const filteredIntegrations = React.useMemo(() => {
    let filtered = [...integrations];

    // Apply connection status filter
    if (filter === 'connected') {
      filtered = filtered.filter(integration => integration.status === 'connected');
    } else if (filter === 'disconnected') {
      filtered = filtered.filter(integration => integration.status !== 'connected');
    }

    // Apply type filter
    if (typeFilter !== 'all') {
      filtered = filtered.filter(integration => integration.type === typeFilter);
    }

    // Apply search
    if (searchQuery) {
      filtered = filtered.filter(integration =>
        integration.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        integration.provider.toLowerCase().includes(searchQuery.toLowerCase()) ||
        integration.description.toLowerCase().includes(searchQuery.toLowerCase())
      );
    }

    return filtered;
  }, [integrations, filter, typeFilter, searchQuery]);

  // Toggle integration enabled/disabled
  const toggleIntegrationStatus = async (integrationId: string) => {
    const target = integrations.find(i => i.id === integrationId);
    if (!target) return;

    const newEnabled = !target.enabled;
    setIntegrations(prev => prev.map(integration =>
      integration.id === integrationId
        ? { ...integration, enabled: newEnabled }
        : integration
    ));

    // CAT-001: report the server's real answer; never claim a save that did not happen.
    let saved = false;
    try {
      const res = await fetch(`/api/v1/integrations/${integrationId}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('auth_token') || ''}`,
        },
        body: JSON.stringify({ enabled: newEnabled }),
      });
      saved = res.ok;
    } catch {
      saved = false;
    }
    if (saved) {
      toast.success('Integration status updated');
    } else {
      setIntegrations(prev => prev.map(integration =>
        integration.id === integrationId ? { ...integration, enabled: target.enabled } : integration
      ));
      toast.error('Integration status was not changed: the server has no integrations API.');
    }
  };

  // Test integration connection
  const testIntegration = async (integrationId: string) => {
    setTestingIntegration(integrationId);

    try {
      const response = await fetch(`/api/v1/integrations/${integrationId}/test`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('auth_token') || ''}`,
        },
      });

      let newStatus: ExternalIntegration['status'] = 'error';
      if (response.ok) {
        const data = await response.json();
        newStatus = data.success ? 'connected' : 'error';
      } else if (response.status === 404) {
        // Endpoint not implemented; mark as disconnected gracefully
        newStatus = 'disconnected';
      }

      setIntegrations(prev => prev.map(integration =>
        integration.id === integrationId
          ? { ...integration, status: newStatus, lastSync: new Date().toISOString() }
          : integration
      ));
      if (newStatus === 'connected') {
        toast.success('Connection test succeeded');
      } else if (newStatus === 'disconnected') {
        toast.error('Connection test unavailable: the server has no integration test endpoint. The connection was not tested.');
      } else {
        toast.error('Connection test failed');
      }
    } catch {
      setIntegrations(prev => prev.map(integration =>
        integration.id === integrationId
          ? { ...integration, status: 'error' }
          : integration
      ));
      toast.error('Connection test failed');
    } finally {
      setTestingIntegration(null);
    }
  };

  // Manual sync
  const syncIntegration = async (integrationId: string) => {
    const integration = integrations.find(i => i.id === integrationId);
    if (!integration || integration.status !== 'connected') return;

    // Update status to show syncing
    setIntegrations(prev => prev.map(i =>
      i.id === integrationId
        ? {
            ...i,
            lastSync: new Date().toISOString(),
            statistics: { ...i.statistics, lastSyncStatus: 'pending' as const }
          }
        : i
    ));

    try {
      const response = await fetch(`/api/v1/integrations/${integrationId}/sync`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('auth_token') || ''}`,
        },
      });

      if (!response.ok) {
        // CAT-001: no sync happened; do not count it or report success.
        setIntegrations(prev => prev.map(i =>
          i.id === integrationId
            ? { ...i, lastSync: integration.lastSync, statistics: { ...i.statistics, lastSyncStatus: integration.statistics.lastSyncStatus } }
            : i
        ));
        toast.error('Sync unavailable: the server has no integration sync endpoint. Nothing was synced.');
        return;
      }
      const data = await response.json();
      const itemsProcessedDelta = Number(data.items_processed || 0);
      const syncStatus: 'success' | 'error' = data.success === false ? 'error' : 'success';

      setIntegrations(prev => prev.map(i =>
        i.id === integrationId
          ? {
              ...i,
              statistics: {
                ...i.statistics,
                totalSyncs: i.statistics.totalSyncs + 1,
                successfulSyncs: syncStatus === 'success' ? i.statistics.successfulSyncs + 1 : i.statistics.successfulSyncs,
                failedSyncs: syncStatus === 'error' ? i.statistics.failedSyncs + 1 : i.statistics.failedSyncs,
                lastSyncStatus: syncStatus,
                itemsProcessed: i.statistics.itemsProcessed + itemsProcessedDelta,
              }
            }
          : i
      ));
      toast.success(syncStatus === 'success' ? 'Sync completed successfully' : 'Sync completed with errors');
    } catch {
      setIntegrations(prev => prev.map(i =>
        i.id === integrationId
          ? {
              ...i,
              statistics: {
                ...i.statistics,
                totalSyncs: i.statistics.totalSyncs + 1,
                failedSyncs: i.statistics.failedSyncs + 1,
                lastSyncStatus: 'error' as const,
              }
            }
          : i
      ));
      toast.error('Sync failed');
    }
  };

  // Delete integration
  const deleteIntegration = async (integrationId: string) => {
    const removed = integrations.find(i => i.id === integrationId);
    setIntegrations(prev => prev.filter(integration => integration.id !== integrationId));

    let deleted = false;
    try {
      const res = await fetch(`/api/v1/integrations/${integrationId}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('auth_token') || ''}`,
        },
      });
      deleted = res.ok;
    } catch {
      deleted = false;
    }
    if (deleted) {
      toast.success('Integration deleted successfully');
    } else {
      // CAT-001: the server did not delete it; put it back instead of claiming success.
      if (removed) setIntegrations(prev => [...prev, removed]);
      toast.error('Integration was not deleted: the server has no integrations API.');
    }
  };

  // Get status icon and color
  const getStatusInfo = (status: string) => {
    switch (status) {
      case 'connected':
        return { icon: CheckCircle, color: 'text-green-700 dark:text-green-400', bg: 'bg-green-50 dark:bg-green-900/20' };
      case 'disconnected':
        return { icon: XCircle, color: 'text-red-700 dark:text-red-400', bg: 'bg-red-50 dark:bg-red-900/20' };
      case 'error':
        return { icon: AlertCircle, color: 'text-red-700 dark:text-red-400', bg: 'bg-red-50 dark:bg-red-900/20' };
      case 'connecting':
        return { icon: RefreshCw, color: 'text-blue-700 dark:text-blue-400', bg: 'bg-blue-50 dark:bg-blue-900/20' };
      default:
        return { icon: Clock, color: 'text-gray-600 dark:text-gray-300', bg: 'bg-gray-50 dark:bg-gray-800' };
    }
  };

  // Get type icon
  const getTypeIcon = (type: string) => {
    switch (type) {
      case 'storage':
        return Cloud;
      case 'metadata':
        return Database;
      case 'analytics':
        return Activity;
      case 'automation':
        return Zap;
      case 'sharing':
        return Share2;
      default:
        return Globe;
    }
  };

  // Format bytes
  const formatBytes = (bytes?: number) => {
    if (!bytes) return 'N/A';
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${sizes[i]}`;
  };

  // Format duration
  const formatDuration = (seconds?: number) => {
    if (!seconds) return 'N/A';
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
    return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`;
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white">External Integrations</h3>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
            Connect with external services to extend functionality
          </p>
        </div>
        <Button
          onClick={() => setIsCreateModalOpen(true)}
          className="flex items-center gap-2"
        >
          <Plus className="w-4 h-4" />
          Add Integration
        </Button>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <div className="bg-white dark:bg-gray-800 rounded-lg border p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-gray-500 dark:text-gray-400">Total Integrations</p>
              <p className="text-2xl font-bold text-gray-900 dark:text-white">{integrations.length}</p>
            </div>
            <Globe className="w-8 h-8 text-blue-500" />
          </div>
        </div>
        
        <div className="bg-white dark:bg-gray-800 rounded-lg border p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-gray-500 dark:text-gray-400">Connected</p>
              <p className="text-2xl font-bold text-green-600">
                {integrations.filter(i => i.status === 'connected').length}
              </p>
            </div>
            <Link className="w-8 h-8 text-green-500" />
          </div>
        </div>
        
        <div className="bg-white dark:bg-gray-800 rounded-lg border p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-gray-500 dark:text-gray-400">Active Syncs</p>
              <p className="text-2xl font-bold text-blue-600 dark:text-blue-400">
                {integrations.filter(i => i.syncSettings.enabled).length}
              </p>
            </div>
            <FolderSync className="w-8 h-8 text-blue-500" />
          </div>
        </div>
        
        <div className="bg-white dark:bg-gray-800 rounded-lg border p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-gray-500 dark:text-gray-400">Success Rate</p>
              <p className="text-2xl font-bold text-gray-900 dark:text-white">
                {integrations.length > 0 && integrations.some(i => i.statistics.totalSyncs > 0)
                  ? Math.round(
                      (integrations.reduce((sum, i) => sum + i.statistics.successfulSyncs, 0) / 
                       integrations.reduce((sum, i) => sum + i.statistics.totalSyncs, 0)) * 100
                    )
                  : 0}%
            </p>
            </div>
            <CheckCircle className="w-8 h-8 text-green-500" />
          </div>
        </div>
      </div>

      {/* Filters and Search */}
      <div className="flex flex-col sm:flex-row gap-4 items-start sm:items-center justify-between bg-white dark:bg-gray-800 rounded-lg border p-4">
        <div className="flex flex-wrap gap-2">
          {(['all', 'connected', 'disconnected'] as const).map((filterOption) => (
            <Button
              key={filterOption}
              variant={filter === filterOption ? 'default' : 'outline'}
              size="sm"
              onClick={() => setFilter(filterOption)}
            >
              {filterOption.charAt(0).toUpperCase() + filterOption.slice(1)}
            </Button>
          ))}
          
          <select
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
            className="px-3 py-1 text-sm border rounded-lg bg-white dark:bg-gray-700 dark:text-white"
          >
            <option value="all">All Types</option>
            <option value="storage">Storage</option>
            <option value="metadata">Metadata</option>
            <option value="analytics">Analytics</option>
            <option value="automation">Automation</option>
            <option value="sharing">Sharing</option>
          </select>
        </div>
        
        <Input
          placeholder="Search integrations..."
          value={searchQuery}
          onChange={(e: React.ChangeEvent<HTMLInputElement>) => setSearchQuery(e.target.value)}
          className="w-full sm:w-64"
        />
      </div>

      {/* Integrations List */}
      <div className="space-y-4">
        <AnimatePresence>
          {filteredIntegrations.map((integration) => {
            const StatusIcon = getStatusInfo(integration.status).icon;
            const TypeIcon = getTypeIcon(integration.type);
            const isExpanded = expandedIntegration === integration.id;
            
            return (
              <motion.div
                key={integration.id}
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -10 }}
                className="bg-white dark:bg-gray-800 rounded-lg border hover:shadow-md transition-shadow"
              >
                {/* Integration Header */}
                <div className="p-4">
                  <div className="flex items-start justify-between">
                    <div className="flex-1">
                      <div className="flex items-center gap-3 mb-2">
                        <h4 className="font-semibold text-gray-900 dark:text-white">{integration.name}</h4>
                        <Badge variant="outline">{integration.provider}</Badge>
                        <Badge variant="secondary">{integration.type}</Badge>
                        <div className={`flex items-center gap-1 px-2 py-1 rounded-full text-xs ${
                          getStatusInfo(integration.status).bg
                        } ${getStatusInfo(integration.status).color}`}>
                          <StatusIcon className="w-3 h-3" />
                          {integration.status}
                        </div>
                      </div>
                      
                      <p className="text-sm text-gray-600 dark:text-gray-300 mb-3">{integration.description}</p>
                      
                      <div className="flex flex-wrap items-center gap-4 text-xs text-gray-500 dark:text-gray-400">
                        <div className="flex items-center gap-1">
                          <TypeIcon className="w-3 h-3" />
                          {integration.type}
                        </div>
                        
                        <div className="flex items-center gap-1">
                          <FolderSync className="w-3 h-3" />
                          {integration.syncSettings.enabled ? `${integration.syncSettings.frequency} sync` : 'Sync disabled'}
                        </div>
                        
                        {integration.lastSync && (
                          <div className="flex items-center gap-1">
                            <Clock className="w-3 h-3" />
                            Last sync: {new Date(integration.lastSync).toLocaleDateString()}
                          </div>
                        )}
                        
                        <div className="flex items-center gap-1">
                          <Activity className="w-3 h-3" />
                          {integration.statistics.itemsProcessed} items processed
                        </div>
                      </div>
                    </div>
                    
                    <div className="flex items-center gap-2 ml-4">
                      <Switch
                        checked={integration.enabled}
                        onCheckedChange={() => toggleIntegrationStatus(integration.id)}
                      />
                      
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setExpandedIntegration(isExpanded ? null : integration.id)}
                      >
                        <Info className="w-4 h-4" />
                      </Button>
                      
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => testIntegration(integration.id)}
                        disabled={testingIntegration === integration.id}
                      >
                        {testingIntegration === integration.id ? (
                          <RefreshCw className="w-4 h-4 animate-spin" />
                        ) : (
                          <TestTube className="w-4 h-4" />
                        )}
                      </Button>
                      
                      {integration.status === 'connected' && (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => syncIntegration(integration.id)}
                        >
                          <RefreshCw className="w-4 h-4" />
                        </Button>
                      )}
                      
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setEditingIntegration(integration)}
                      >
                        <Edit className="w-4 h-4" />
                      </Button>
                      
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => deleteIntegration(integration.id)}
                      >
                        <Trash2 className="w-4 h-4" />
                      </Button>
                    </div>
                  </div>
                </div>
                
                {/* Expanded Content */}
                <AnimatePresence>
                  {isExpanded && (
                    <motion.div
                      initial={{ height: 0 }}
                      animate={{ height: 'auto' }}
                      exit={{ height: 0 }}
                      transition={{ duration: 0.2 }}
                      className="border-t overflow-hidden"
                    >
                      <div className="p-4 bg-gray-50 dark:bg-gray-900 space-y-4">
                        {/* Configuration */}
                        <div>
                          <h5 className="font-medium text-sm text-gray-700 dark:text-gray-300 mb-2">Configuration</h5>
                          <div className="bg-white dark:bg-gray-800 rounded p-3 text-sm space-y-2">
                            {integration.config.endpoint && (
                              <div className="flex items-center gap-2">
                                <ExternalLink className="w-3 h-3 text-gray-400" />
                                <span className="text-gray-600 dark:text-gray-300">Endpoint: </span>
                                <code className="bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded text-xs">
                                  {integration.config.endpoint}
                                </code>
                              </div>
                            )}
                            {integration.config.webhookUrl && (
                              <div className="flex items-center gap-2">
                                <Link className="w-3 h-3 text-gray-400" />
                                <span className="text-gray-600 dark:text-gray-300">Webhook: </span>
                                <code className="bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded text-xs truncate max-w-xs">
                                  {integration.config.webhookUrl}
                                </code>
                              </div>
                            )}
                            {integration.config.apiKey && (
                              <div className="flex items-center gap-2">
                                <Key className="w-3 h-3 text-gray-400" />
                                <span className="text-gray-600 dark:text-gray-300">API Key: </span>
                                <code className="bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded text-xs">
                                  {integration.config.apiKey}
                                </code>
                              </div>
                            )}
                          </div>
                        </div>
                        
                        {/* Sync Settings */}
                        <div>
                          <h5 className="font-medium text-sm text-gray-700 dark:text-gray-300 mb-2">Sync Settings</h5>
                          <div className="bg-white dark:bg-gray-800 rounded p-3 text-sm">
                            <div className="grid grid-cols-2 gap-4 text-xs">
                              <div>
                                <span className="text-gray-500 dark:text-gray-400">Enabled: </span>
                                <span className={integration.syncSettings.enabled ? 'text-green-600' : 'text-gray-600 dark:text-gray-300'}>
                                  {integration.syncSettings.enabled ? 'Yes' : 'No'}
                                </span>
                              </div>
                              <div>
                                <span className="text-gray-500 dark:text-gray-400">Frequency: </span>
                                <span className="text-gray-600 dark:text-gray-300">{integration.syncSettings.frequency}</span>
                              </div>
                              <div>
                                <span className="text-gray-500 dark:text-gray-400">Direction: </span>
                                <span className="text-gray-600 dark:text-gray-300">{integration.syncSettings.direction}</span>
                              </div>
                              <div>
                                <span className="text-gray-500 dark:text-gray-400">Filters: </span>
                                <span className="text-gray-600 dark:text-gray-300">{integration.syncSettings.filters.length}</span>
                              </div>
                            </div>
                          </div>
                        </div>
                        
                        {/* Statistics */}
                        <div>
                          <h5 className="font-medium text-sm text-gray-700 dark:text-gray-300 mb-2">Statistics</h5>
                          <div className="bg-white dark:bg-gray-800 rounded p-3">
                            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
                              <div className="text-center">
                                <p className="text-2xl font-bold text-gray-900 dark:text-white">{integration.statistics.totalSyncs}</p>
                                <p className="text-xs text-gray-500 dark:text-gray-400">Total Syncs</p>
                              </div>
                              <div className="text-center">
                                <p className="text-2xl font-bold text-green-600">{integration.statistics.successfulSyncs}</p>
                                <p className="text-xs text-gray-500 dark:text-gray-400">Successful</p>
                              </div>
                              <div className="text-center">
                                <p className="text-2xl font-bold text-red-600">{integration.statistics.failedSyncs}</p>
                                <p className="text-xs text-gray-500 dark:text-gray-400">Failed</p>
                              </div>
                              <div className="text-center">
                                <p className="text-2xl font-bold text-blue-600 dark:text-blue-400">{integration.statistics.itemsProcessed}</p>
                                <p className="text-xs text-gray-500 dark:text-gray-400">Items</p>
                              </div>
                            </div>
                            {integration.statistics.lastSyncDuration && (
                              <div className="mt-3 pt-3 border-t text-xs text-gray-600 dark:text-gray-300">
                                Last sync duration: {formatDuration(integration.statistics.lastSyncDuration)}
                                {integration.statistics.bytesTransferred && (
                                  <span className="ml-3">
                                    Data transferred: {formatBytes(integration.statistics.bytesTransferred)}
                                  </span>
                                )}
                              </div>
                            )}
                          </div>
                        </div>
                      </div>
                    </motion.div>
                  )}
                </AnimatePresence>
              </motion.div>
            );
          })}
        </AnimatePresence>
        
        {filteredIntegrations.length === 0 && (
          <div className="text-center py-12 bg-white dark:bg-gray-800 rounded-lg border">
            <Globe className="w-12 h-12 text-gray-400 mx-auto mb-4" />
            <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">No integrations found</h3>
            <p className="text-gray-500 dark:text-gray-400 mb-4">
              Connect with external services to extend your collection management capabilities
            </p>
            <Button onClick={() => setIsCreateModalOpen(true)}>
              <Plus className="w-4 h-4 mr-2" />
              Add Your First Integration
            </Button>
          </div>
        )}
      </div>

      {/* Create Modal */}
      {isCreateModalOpen && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
          <div className="bg-white dark:bg-gray-800 rounded-lg max-w-4xl w-full max-h-[90vh] overflow-y-auto p-6">
            <h3 className="text-xl font-bold mb-4">Add External Integration</h3>

            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Select Service</label>
                <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
                  {INTEGRATION_EXAMPLES.map(example => (
                    <button
                      key={example.name}
                      onClick={() => {
                        const newIntegration: ExternalIntegration = {
                          id: Date.now().toString(),
                          name: example.name,
                          provider: example.name.toLowerCase().replace(/\s+/g, '_'),
                          type: example.type as ExternalIntegration['type'],
                          status: 'disconnected',
                          description: example.description,
                          config: {},
                          syncSettings: {
                            enabled: false,
                            frequency: 'daily',
                            direction: 'import',
                            filters: [],
                          },
                          statistics: {
                            totalSyncs: 0,
                            successfulSyncs: 0,
                            failedSyncs: 0,
                            lastSyncStatus: 'pending',
                            itemsProcessed: 0,
                          },
                          createdAt: new Date().toISOString(),
                          enabled: true,
                        }
                        setIntegrations(prev => [...prev, newIntegration])
                        setIsCreateModalOpen(false)
                        setEditingIntegration(newIntegration)
                        toast.success(`${example.name} integration added locally. It is not saved on the server (no integrations API).`)
                      }}
                      className="p-4 border rounded-lg hover:border-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/20 text-left transition-colors"
                    >
                      <div className="font-medium text-sm">{example.name}</div>
                      <div className="text-xs text-gray-500 dark:text-gray-400 mt-1">{example.description}</div>
                      <div className="flex flex-wrap gap-1 mt-2">
                        {example.features.slice(0, 2).map(f => (
                          <span key={f} className="text-xs bg-gray-100 dark:bg-gray-700 px-1.5 py-0.5 rounded">{f}</span>
                        ))}
                      </div>
                    </button>
                  ))}
                </div>
              </div>
            </div>

            <div className="flex justify-end gap-2 mt-6 pt-4 border-t">
              <Button variant="outline" onClick={() => setIsCreateModalOpen(false)}>Cancel</Button>
            </div>
          </div>
        </div>
      )}

      {/* Edit Modal */}
      {editingIntegration && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
          <div className="bg-white dark:bg-gray-800 rounded-lg max-w-4xl w-full max-h-[90vh] overflow-y-auto p-6">
            <h3 className="text-xl font-bold mb-4">Edit Integration</h3>
            <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">Configure {editingIntegration.name}</p>

            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">API Key</label>
                <Input
                  type="password"
                  placeholder="Enter API key"
                  defaultValue={editingIntegration.config.apiKey || ''}
                  onChange={(e) => {
                    setEditingIntegration(prev => prev ? { ...prev, config: { ...prev.config, apiKey: e.target.value } } : null)
                  }}
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">API Endpoint (optional)</label>
                <Input
                  placeholder="https://api.example.com"
                  defaultValue={editingIntegration.config.endpoint || ''}
                  onChange={(e) => {
                    setEditingIntegration(prev => prev ? { ...prev, config: { ...prev.config, endpoint: e.target.value } } : null)
                  }}
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Webhook URL (optional)</label>
                <Input
                  placeholder="https://hooks.example.com/callback"
                  defaultValue={editingIntegration.config.webhookUrl || ''}
                  onChange={(e) => {
                    setEditingIntegration(prev => prev ? { ...prev, config: { ...prev.config, webhookUrl: e.target.value } } : null)
                  }}
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Sync Frequency</label>
                <select
                  className="w-full border rounded-lg p-2 text-sm"
                  defaultValue={editingIntegration.syncSettings.frequency}
                  onChange={(e) => {
                    setEditingIntegration(prev => prev ? { ...prev, syncSettings: { ...prev.syncSettings, frequency: e.target.value as SyncSettings['frequency'] } } : null)
                  }}
                >
                  <option value="realtime">Real-time</option>
                  <option value="hourly">Hourly</option>
                  <option value="daily">Daily</option>
                  <option value="weekly">Weekly</option>
                  <option value="manual">Manual only</option>
                </select>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Sync Direction</label>
                <select
                  className="w-full border rounded-lg p-2 text-sm"
                  defaultValue={editingIntegration.syncSettings.direction}
                  onChange={(e) => {
                    setEditingIntegration(prev => prev ? { ...prev, syncSettings: { ...prev.syncSettings, direction: e.target.value as SyncSettings['direction'] } } : null)
                  }}
                >
                  <option value="import">Import only</option>
                  <option value="export">Export only</option>
                  <option value="bidirectional">Bidirectional</option>
                </select>
              </div>
            </div>

            <div className="flex justify-end gap-2 mt-6 pt-4 border-t">
              <Button variant="outline" onClick={() => setEditingIntegration(null)}>Cancel</Button>
              <Button onClick={() => {
                setIntegrations(prev => prev.map(i => i.id === editingIntegration.id ? editingIntegration : i))
                setEditingIntegration(null)
                toast.success('Integration updated locally. It is not saved on the server (no integrations API).')
              }}>Save Changes</Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default ExternalIntegrations;