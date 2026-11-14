import React, { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Badge } from '@/components/ui/Badge'
import { Progress } from '@/components/ui/Progress'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/Tabs'
import { scansApi } from '@/lib/scansApi'
import { smbApi } from '@/lib/smbApi'
import { syncApi } from '@/lib/syncApi'
import type { QueueScanRequest } from '@/lib/scansApi'
import type { SmbTestRequest } from '@/lib/smbApi'
import type { CreateEndpointRequest } from '@/lib/syncApi'
import { mediaApi } from '@/lib/mediaApi'
import { useAuth } from '@/contexts/AuthContext'
import toast from 'react-hot-toast'
import {
  Scan,
  Network,
  RefreshCw,
  Plus,
  Play,
  Trash2,
  CheckCircle,
  XCircle,
  Clock,
  Server,
  FolderSearch,
  Wifi,
} from 'lucide-react'
import { motion } from 'framer-motion'

// Storage, scan, SMB and sync-endpoint actions are administrator-only on the server (403 for everyone else): say so instead of
// reporting a generic failure.
const failureMessage = (err: unknown, fallback: string): string =>
  (err as { response?: { status?: number } } | null)?.response?.status === 403
    ? 'Administrator access required'
    : fallback

// --- Scans Section ---
const ScansSection: React.FC = () => {
  const queryClient = useQueryClient()
  const [selectedRootId, setSelectedRootId] = useState<number>(0)

  const { data: scans, isLoading: scansLoading } = useQuery({
    queryKey: ['scans'],
    queryFn: () => scansApi.listScans(),
    refetchInterval: 5000,
  })

  const { data: storageRoots } = useQuery({
    queryKey: ['storage-roots'],
    queryFn: () => mediaApi.getStorageRoots(),
  })

  const queueMutation = useMutation({
    mutationFn: (body: QueueScanRequest) => scansApi.queueScan(body),
    onSuccess: () => {
      toast.success('Scan queued successfully')
      queryClient.invalidateQueries({ queryKey: ['scans'] })
    },
    onError: (err) => toast.error(failureMessage(err, 'Failed to queue scan')),
  })

  const handleStartScan = () => {
    if (!selectedRootId) {
      toast.error('Please select a storage root')
      return
    }
    queueMutation.mutate({ storage_root_id: selectedRootId, full_scan: true })
  }

  const statusBadge = (status: string) => {
    const variant = status === 'completed' ? 'default'
      : status === 'running' ? 'secondary'
      : status === 'failed' ? 'destructive'
      : 'outline'
    return <Badge variant={variant}>{status}</Badge>
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Plus className="h-5 w-5" />
            Start New Scan
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-end gap-4">
            <div className="flex-1">
              <label htmlFor="scan-root-select" className="text-sm font-medium text-gray-700 dark:text-gray-200 block mb-2">
                Storage Root
              </label>
              <select
                id="scan-root-select"
                className="w-full h-11 rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800 dark:text-white"
                value={selectedRootId}
                onChange={(e) => setSelectedRootId(Number(e.target.value))}
              >
                <option value={0}>Select a storage root...</option>
                {storageRoots?.map((root) => (
                  <option key={root.id} value={root.id}>
                    {root.name} ({root.protocol})
                  </option>
                ))}
              </select>
            </div>
            <Button
              onClick={handleStartScan}
              loading={queueMutation.isPending}
              disabled={!selectedRootId}
            >
              <Play className="h-4 w-4 mr-2" />
              Start Scan
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Scan className="h-5 w-5" />
            Scan History
          </CardTitle>
        </CardHeader>
        <CardContent>
          {scansLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 3 }).map((_, i) => (
                <div key={i} className="h-16 bg-gray-200 dark:bg-gray-700 rounded animate-pulse" />
              ))}
            </div>
          ) : !scans || scans.length === 0 ? (
            <p className="text-sm text-gray-500 dark:text-gray-400">No scans recorded yet.</p>
          ) : (
            <div className="space-y-3">
              {scans.map((scan) => (
                <div key={scan.job_id} className="flex items-center justify-between p-4 rounded-lg bg-gray-50 dark:bg-gray-800">
                  <div className="flex-1">
                    <div className="flex items-center gap-3">
                      <span className="font-medium text-gray-900 dark:text-white">
                        {scan.storage_root_name || `Root #${scan.storage_root_id}`}
                      </span>
                      {statusBadge(scan.status)}
                    </div>
                    <div className="flex items-center gap-4 mt-1 text-xs text-gray-500 dark:text-gray-400">
                      <span>{scan.files_found} files found</span>
                      <span>{scan.files_processed} processed</span>
                      {scan.errors > 0 && <span className="text-red-500">{scan.errors} errors</span>}
                      {scan.started_at && <span>{new Date(scan.started_at).toLocaleString()}</span>}
                    </div>
                    {scan.status === 'running' && (
                      <Progress value={scan.progress} className="mt-2" />
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

// --- SMB Discovery Section ---
const SmbDiscoverySection: React.FC = () => {
  const [testForm, setTestForm] = useState<SmbTestRequest>({
    host: '',
    share: '',
    username: '',
    password: '',
  })

  const discoverMutation = useMutation({
    mutationFn: () => smbApi.discover(),
    onSuccess: (data) => {
      toast.success(`Discovered ${data.length} SMB shares`)
    },
    onError: (err) => toast.error(failureMessage(err, 'SMB discovery failed')),
  })

  const testMutation = useMutation({
    mutationFn: (body: SmbTestRequest) => smbApi.testConnection(body),
    onSuccess: (data) => {
      if (data.success) {
        toast.success(`Connection successful (${data.latency_ms}ms)`)
      } else {
        toast.error(`Connection failed: ${data.message}`)
      }
    },
    onError: (err) => toast.error(failureMessage(err, 'Connection test failed')),
  })

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <FolderSearch className="h-5 w-5" />
            Network Discovery
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <p className="text-sm text-gray-600 dark:text-gray-400">
              Scan your network for available SMB shares.
            </p>
            <Button
              onClick={() => discoverMutation.mutate()}
              loading={discoverMutation.isPending}
            >
              <Network className="h-4 w-4 mr-2" />
              Discover Shares
            </Button>
            {discoverMutation.data && discoverMutation.data.length > 0 && (
              <div className="space-y-2 mt-4">
                {discoverMutation.data.map((share, idx) => (
                  <div key={idx} className="flex items-center gap-3 p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
                    <Server className="h-4 w-4 text-gray-500 dark:text-gray-400" />
                    <div>
                      <span className="font-medium text-gray-900 dark:text-white">{share.name}</span>
                      <span className="text-sm text-gray-500 dark:text-gray-400 ml-2">{share.host}{share.path}</span>
                    </div>
                  </div>
                ))}
              </div>
            )}
            {discoverMutation.data && discoverMutation.data.length === 0 && (
              <p className="text-sm text-gray-500 dark:text-gray-400 mt-2">No shares discovered.</p>
            )}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Wifi className="h-5 w-5" />
            Test Connection
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Input
              label="Host"
              placeholder="192.168.1.100"
              value={testForm.host}
              onChange={(e) => setTestForm(prev => ({ ...prev, host: e.target.value }))}
            />
            <Input
              label="Share Name"
              placeholder="media"
              value={testForm.share}
              onChange={(e) => setTestForm(prev => ({ ...prev, share: e.target.value }))}
            />
            <Input
              label="Username (optional)"
              placeholder="guest"
              value={testForm.username}
              onChange={(e) => setTestForm(prev => ({ ...prev, username: e.target.value }))}
            />
            <Input
              label="Password (optional)"
              type="password"
              placeholder=""
              value={testForm.password}
              onChange={(e) => setTestForm(prev => ({ ...prev, password: e.target.value }))}
            />
          </div>
          <div className="mt-4">
            <Button
              onClick={() => testMutation.mutate(testForm)}
              loading={testMutation.isPending}
              disabled={!testForm.host || !testForm.share}
            >
              Test Connection
            </Button>
          </div>
          {testMutation.data && (
            <div className={`mt-4 flex items-center gap-2 text-sm ${testMutation.data.success ? 'text-green-600' : 'text-red-600'}`}>
              {testMutation.data.success
                ? <CheckCircle className="h-4 w-4" />
                : <XCircle className="h-4 w-4" />}
              {testMutation.data.message}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

// --- Sync Section ---
// Every GET /sync/* is administrator-only on the server (SSE-F4): for anyone else the tab is not offered and, as a second
// line of defence, no query is issued even if the section were mounted (no 403 polling, no 10 s sessions refetch).
export const SyncSection: React.FC = () => {
  const { isAdmin } = useAuth()
  const queryClient = useQueryClient()
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState<CreateEndpointRequest>({ name: '', url: '' })

  const { data: endpoints, isLoading: endpointsLoading } = useQuery({
    queryKey: ['sync-endpoints'],
    queryFn: () => syncApi.listEndpoints(),
    enabled: isAdmin,
  })

  const { data: stats } = useQuery({
    queryKey: ['sync-statistics'],
    queryFn: () => syncApi.getStatistics(),
    enabled: isAdmin,
  })

  const { data: sessions } = useQuery({
    queryKey: ['sync-sessions'],
    queryFn: () => syncApi.getSessions(),
    enabled: isAdmin,
    refetchInterval: isAdmin ? 10000 : false,
  })

  const createMutation = useMutation({
    mutationFn: (body: CreateEndpointRequest) => syncApi.createEndpoint(body),
    onSuccess: () => {
      toast.success('Sync endpoint created')
      setShowForm(false)
      setForm({ name: '', url: '' })
      queryClient.invalidateQueries({ queryKey: ['sync-endpoints'] })
    },
    onError: (err) => toast.error(failureMessage(err, 'Failed to create endpoint')),
  })

  const syncMutation = useMutation({
    mutationFn: (id: number) => syncApi.startSync(id),
    onSuccess: () => {
      toast.success('Sync started')
      queryClient.invalidateQueries({ queryKey: ['sync-sessions'] })
    },
    onError: (err) => toast.error(failureMessage(err, 'Failed to start sync')),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => syncApi.deleteEndpoint(id),
    onSuccess: () => {
      toast.success('Endpoint deleted')
      queryClient.invalidateQueries({ queryKey: ['sync-endpoints'] })
    },
    onError: (err) => toast.error(failureMessage(err, 'Failed to delete endpoint')),
  })

  return (
    <div className="space-y-6">
      {stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <Card>
            <CardContent className="p-4">
              <div className="text-2xl font-bold text-gray-900 dark:text-white">{stats.total_syncs}</div>
              <div className="text-sm text-gray-500 dark:text-gray-400">Total Syncs</div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="p-4">
              <div className="text-2xl font-bold text-green-600">{stats.successful_syncs}</div>
              <div className="text-sm text-gray-500 dark:text-gray-400">Successful</div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="p-4">
              <div className="text-2xl font-bold text-red-600">{stats.failed_syncs}</div>
              <div className="text-sm text-gray-500 dark:text-gray-400">Failed</div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="p-4">
              <div className="text-2xl font-bold text-gray-900 dark:text-white">{stats.total_items_synced}</div>
              <div className="text-sm text-gray-500 dark:text-gray-400">Items Synced</div>
            </CardContent>
          </Card>
        </div>
      )}

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle className="flex items-center gap-2">
              <RefreshCw className="h-5 w-5" />
              Sync Endpoints
            </CardTitle>
            <Button variant="outline" onClick={() => setShowForm(!showForm)}>
              <Plus className="h-4 w-4 mr-2" />
              Add Endpoint
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {showForm && (
            <div className="mb-6 p-4 rounded-lg border border-gray-200 dark:border-gray-700 space-y-4">
              <Input
                label="Name"
                placeholder="My Sync Target"
                value={form.name}
                onChange={(e) => setForm(prev => ({ ...prev, name: e.target.value }))}
              />
              <Input
                label="URL"
                placeholder="https://remote-catalogizer.example.com"
                value={form.url}
                onChange={(e) => setForm(prev => ({ ...prev, url: e.target.value }))}
              />
              <div className="flex gap-2">
                <Button
                  onClick={() => createMutation.mutate(form)}
                  loading={createMutation.isPending}
                  disabled={!form.name || !form.url}
                >
                  Create
                </Button>
                <Button variant="outline" onClick={() => setShowForm(false)}>
                  Cancel
                </Button>
              </div>
            </div>
          )}

          {endpointsLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 2 }).map((_, i) => (
                <div key={i} className="h-16 bg-gray-200 dark:bg-gray-700 rounded animate-pulse" />
              ))}
            </div>
          ) : !endpoints || endpoints.length === 0 ? (
            <p className="text-sm text-gray-500 dark:text-gray-400">No sync endpoints configured.</p>
          ) : (
            <div className="space-y-3">
              {endpoints.map((ep) => (
                <div key={ep.id} className="flex items-center justify-between p-4 rounded-lg bg-gray-50 dark:bg-gray-800">
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-gray-900 dark:text-white">{ep.name}</span>
                      <Badge variant={ep.enabled ? 'default' : 'outline'}>
                        {ep.enabled ? 'Active' : 'Disabled'}
                      </Badge>
                    </div>
                    <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                      {ep.url}
                      {ep.last_sync_at && (
                        <span className="ml-3">Last sync: {new Date(ep.last_sync_at).toLocaleString()}</span>
                      )}
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => syncMutation.mutate(ep.id)}
                      loading={syncMutation.isPending}
                    >
                      <Play className="h-3 w-3 mr-1" />
                      Sync
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => deleteMutation.mutate(ep.id)}
                    >
                      <Trash2 className="h-3 w-3" />
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {sessions && sessions.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Clock className="h-5 w-5" />
              Recent Sessions
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {sessions.slice(0, 10).map((session) => (
                <div key={session.id} className="flex items-center justify-between p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
                  <div className="flex items-center gap-3">
                    {session.status === 'completed' && <CheckCircle className="h-4 w-4 text-green-500" />}
                    {session.status === 'running' && <RefreshCw className="h-4 w-4 text-blue-500 animate-spin" />}
                    {session.status === 'failed' && <XCircle className="h-4 w-4 text-red-500" />}
                    <div>
                      <span className="text-sm text-gray-900 dark:text-white">
                        Endpoint #{session.endpoint_id}
                      </span>
                      <span className="text-xs text-gray-500 dark:text-gray-400 ml-2">
                        {session.items_synced} items synced
                      </span>
                    </div>
                  </div>
                  <span className="text-xs text-gray-500 dark:text-gray-400">
                    {new Date(session.started_at).toLocaleString()}
                  </span>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}

// --- Main Settings Page ---
export const Settings: React.FC = () => {
  const { isAdmin } = useAuth()
  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5 }}
      >
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-gray-900 dark:text-white mb-2">
            Settings
          </h1>
          <p className="text-gray-600 dark:text-gray-400">
            Manage scans, network discovery, and synchronization
          </p>
        </div>

        <Tabs defaultValue="scans">
          <TabsList className="mb-6">
            <TabsTrigger value="scans">
              <Scan className="h-4 w-4 mr-2" />
              Scans
            </TabsTrigger>
            <TabsTrigger value="smb">
              <Network className="h-4 w-4 mr-2" />
              SMB Discovery
            </TabsTrigger>
            {isAdmin && (
              <TabsTrigger value="sync">
                <RefreshCw className="h-4 w-4 mr-2" />
                Sync
              </TabsTrigger>
            )}
          </TabsList>

          <TabsContent value="scans">
            <ScansSection />
          </TabsContent>

          <TabsContent value="smb">
            <SmbDiscoverySection />
          </TabsContent>

          {isAdmin && (
            <TabsContent value="sync">
              <SyncSection />
            </TabsContent>
          )}
        </Tabs>
      </motion.div>
    </div>
  )
}

export default Settings
