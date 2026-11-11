/**
 * Discovered Shares — list of hosts + their discovered shares with the
 * working-identity label and probe/scan buttons.
 *
 * Grouped by host (using HostWithShares shape). Each share shows its
 * protocol, share name, auth status chip, and a Probe button. The toolbar
 * offers "Scan Network" + filter-by-protocol.
 *
 * §11.4.162 — OpenDesign/Catalogizer Blue tokens via Tailwind for light+dark.
 * §11.4.143 — the "Scan Network" button drives the real network-sweep
 * endpoint (STUB until backend lands); an honest error surfaces.
 */

import React, { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { identitiesApi } from '@/lib/identitiesApi'
import type { DiscoveredShare } from '@/types/identity'
import {
  Network,
  Server,
  FolderOpen,
  Wifi,
  RefreshCw,
  Search,
  Radio,
  Globe,
  HardDrive,
  Monitor,
} from 'lucide-react'
import toast from 'react-hot-toast'

/* ------------------------------------------------------------------ */
/*  Protocol icon helper                                               */
/* ------------------------------------------------------------------ */

const ProtocolIcon: React.FC<{ protocol: string }> = ({ protocol }) => {
  const cls = 'h-4 w-4 text-gray-500'
  switch (protocol?.toLowerCase()) {
    case 'smb':
      return <HardDrive className={cls} />
    case 'nfs':
      return <Monitor className={cls} />
    case 'ftp':
      return <Globe className={cls} />
    case 'webdav':
      return <Radio className={cls} />
    default:
      return <FolderOpen className={cls} />
  }
}

/* ------------------------------------------------------------------ */
/*  Share row within a host group                                      */
/* ------------------------------------------------------------------ */

interface ShareRowProps {
  share: DiscoveredShare
}

const ShareRow: React.FC<ShareRowProps> = ({ share }) => {
  const queryClient = useQueryClient()

  const probeMutation = useMutation({
    mutationFn: () => identitiesApi.probeShare(share.id),
    onSuccess: (data) => {
      if (data.status === 'ok') {
        toast.success(
          data.anonymous_ok
            ? `Share "${share.share_name}" accessible anonymously`
            : `Share "${share.share_name}" — bound to ${data.bound_identity_name}`
        )
      } else {
        toast.error(
          `Share "${share.share_name}": ${data.status}`
        )
      }
      queryClient.invalidateQueries({ queryKey: ['identities-bindings'] })
    },
    onError: () => toast.error('Probe failed'),
  })

  // We render basic share info; binding info will come from a future
  // join query once the backend is implemented.
  return (
    <div className="flex items-center justify-between py-2 px-3 rounded-md hover:bg-gray-50 dark:hover:bg-gray-750 transition-colors">
      <div className="flex items-center gap-3 min-w-0">
        <ProtocolIcon protocol={share.protocol} />
        <div className="min-w-0">
          <span className="text-sm font-medium text-gray-900 dark:text-white">
            {share.share_name}
          </span>
          <span className="text-xs text-gray-500 dark:text-gray-400 ml-2">
            :{share.port}
          </span>
          {share.last_seen && (
            <span className="text-xs text-gray-600 dark:text-gray-400 ml-3">
              seen {new Date(share.last_seen).toLocaleDateString()}
            </span>
          )}
        </div>
      </div>
      <div className="flex items-center gap-2 shrink-0 ml-3">
        <Badge variant="secondary" className="text-xs">
          {share.protocol.toUpperCase()}
        </Badge>
        <Button
          variant="outline"
          size="sm"
          onClick={() => probeMutation.mutate()}
          loading={probeMutation.isPending}
        >
          <Radio className="h-3 w-3 mr-1" />
          Probe
        </Button>
      </div>
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Host group card                                                    */
/* ------------------------------------------------------------------ */

interface HostGroupProps {
  host: {
    id: number
    ip: string
    hostname: string | null
    reachable: boolean
    oui_vendor?: string | null
  }
  shares: DiscoveredShare[]
}

const HostGroup: React.FC<HostGroupProps> = ({ host, shares }) => {
  const [expanded, setExpanded] = useState(true)

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <button type="button"
            className="flex items-center gap-3 cursor-pointer select-none text-left bg-transparent border-0 p-0"
            onClick={() => setExpanded(!expanded)}
            aria-expanded={expanded}
          >
            <Server
              className={`h-5 w-5 ${
                host.reachable
                  ? 'text-green-500'
                  : 'text-gray-400'
              }`}
            />
            <div>
              <div className="flex items-center gap-2">
                <span className="font-medium text-gray-900 dark:text-white">
                  {host.hostname || host.ip}
                </span>
                {host.hostname && host.hostname !== host.ip && (
                  <span className="text-sm text-gray-500 dark:text-gray-400">
                    ({host.ip})
                  </span>
                )}
                <Badge
                  variant={host.reachable ? 'default' : 'outline'}
                  className="text-xs"
                >
                  {host.reachable ? 'Online' : 'Offline'}
                </Badge>
                {host.oui_vendor && (
                  <Badge variant="secondary" className="text-xs">
                    {host.oui_vendor}
                  </Badge>
                )}
              </div>
            </div>
          </button>
          <div className="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
            <span>{shares.length} share{shares.length !== 1 ? 's' : ''}</span>
          </div>
        </div>
      </CardHeader>
      {expanded && shares.length > 0 && (
        <CardContent className="pt-0">
          <div className="divide-y divide-gray-100 dark:divide-gray-700">
            {shares.map((share) => (
              <ShareRow key={share.id} share={share} />
            ))}
          </div>
        </CardContent>
      )}
      {expanded && shares.length === 0 && (
        <CardContent className="pt-0">
          <p className="text-sm text-gray-500 dark:text-gray-400 py-3">
            No shares enumerated for this host.
          </p>
        </CardContent>
      )}
    </Card>
  )
}

/* ------------------------------------------------------------------ */
/*  Discovered Shares — main component                                 */
/* ------------------------------------------------------------------ */

export const DiscoveredShares: React.FC = () => {
  const queryClient = useQueryClient()
  const [protocolFilter, setProtocolFilter] = useState<string>('')
  const [searchHost, setSearchHost] = useState('')

  const { data: hosts, isLoading: hostsLoading } = useQuery({
    queryKey: ['discovery-hosts'],
    queryFn: () => identitiesApi.listHosts(),
    staleTime: 1000 * 15,
  })

  const { data: shares, isLoading: sharesLoading } = useQuery({
    queryKey: ['discovery-shares'],
    queryFn: () => identitiesApi.listShares(),
    staleTime: 1000 * 15,
  })

  const scanMutation = useMutation({
    mutationFn: () => identitiesApi.scanNetwork(),
    onSuccess: (data) => {
      toast.success(
        `Scan complete: ${data.hosts_found} hosts, ${data.shares_found} shares`
      )
      queryClient.invalidateQueries({ queryKey: ['discovery-hosts'] })
      queryClient.invalidateQueries({ queryKey: ['discovery-shares'] })
    },
    onError: () =>
      toast.error(
        'Scan failed. The discovery endpoint may not be implemented yet.'
      ),
  })

  // Client-side filter: by protocol and host search
  const filteredShares = React.useMemo(() => {
    if (!shares) return []
    let list = shares
    if (protocolFilter) {
      list = list.filter(
        (s) => s.protocol.toLowerCase() === protocolFilter.toLowerCase()
      )
    }
    if (searchHost.trim()) {
      const q = searchHost.trim().toLowerCase()
      list = list.filter(
        (s) =>
          s.host_ip.toLowerCase().includes(q) ||
          (s.host_hostname && s.host_hostname.toLowerCase().includes(q))
      )
    }
    return list
  }, [shares, protocolFilter, searchHost])

  // Group filtered shares by host IP
  const grouped = React.useMemo(() => {
    const map = new Map<
      string,
      {
        host: {
          id: number
          ip: string
          hostname: string | null
          reachable: boolean
          oui_vendor: string | null
        }
        shares: DiscoveredShare[]
      }
    >()
    for (const share of filteredShares) {
      const key = share.host_ip
      if (!map.has(key)) {
        const h = hosts?.find((h) => h.id === share.host_id)
        map.set(key, {
          host: {
            id: share.host_id,
            ip: share.host_ip,
            hostname: share.host_hostname,
            reachable: h?.reachable ?? true,
            oui_vendor: h?.oui_vendor ?? null,
          },
          shares: [],
        })
      }
      const entry = map.get(key)
      if (entry) {
        entry.shares.push(share)
      }
    }
    return Array.from(map.values())
  }, [filteredShares, hosts])

  // Unique protocol values for the filter dropdown
  const protocols = React.useMemo(() => {
    if (!shares) return []
    return Array.from(new Set(shares.map((s) => s.protocol.toLowerCase())))
  }, [shares])

  const isLoading = hostsLoading || sharesLoading

  return (
    <div className="space-y-6">
      {/* Toolbar */}
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <CardTitle className="flex items-center gap-2">
              <Network className="h-5 w-5" />
              Discovered Shares
            </CardTitle>
            <div className="flex items-center gap-2">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-400" />
                <input
                  type="text"
                  placeholder="Search host..."
                  className="h-9 pl-9 pr-3 rounded-lg border border-gray-300 bg-white text-sm text-gray-900 placeholder-gray-400 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-white dark:placeholder-gray-500 dark:focus-visible:ring-blue-400"
                  value={searchHost}
                  onChange={(e) => setSearchHost(e.target.value)}
                />
              </div>
              <select
                className="h-9 rounded-lg border border-gray-300 bg-white px-3 text-sm text-gray-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-white dark:focus-visible:ring-blue-400"
                value={protocolFilter}
                onChange={(e) => setProtocolFilter(e.target.value)}
              >
                <option value="">All protocols</option>
                {protocols.map((p) => (
                  <option key={p} value={p}>
                    {p.toUpperCase()}
                  </option>
                ))}
              </select>
              <Button
                onClick={() => scanMutation.mutate()}
                loading={scanMutation.isPending}
              >
                <Wifi className="h-4 w-4 mr-2" />
                Scan Network
              </Button>
            </div>
          </div>
        </CardHeader>
      </Card>

      {/* Host groups */}
      {isLoading ? (
        <div className="space-y-4">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="h-24 bg-gray-200 dark:bg-gray-700 rounded animate-pulse" />
          ))}
        </div>
      ) : grouped.length === 0 ? (
        <Card>
          <CardContent className="py-12">
            <div className="flex flex-col items-center justify-center text-center">
              <Network className="h-12 w-12 text-gray-300 dark:text-gray-600 mb-4" />
              <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-1">
                No shares discovered
              </h3>
              <p className="text-sm text-gray-500 dark:text-gray-400 max-w-md">
                Click &quot;Scan Network&quot; to discover available hosts and
                shares on your LAN. The scan will probe anonymously first,
                then try each identity in priority order.
              </p>
            </div>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          {grouped.map((g) => (
            <HostGroup key={g.host.ip} host={g.host} shares={g.shares} />
          ))}
        </div>
      )}

      {/* Summary card */}
      {!isLoading && grouped.length > 0 && (
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center justify-between text-sm text-gray-500 dark:text-gray-400">
              <span className="flex items-center gap-2">
                <Server className="h-4 w-4" />
                {grouped.length} host{grouped.length !== 1 ? 's' : ''}
              </span>
              <span className="flex items-center gap-2">
                <FolderOpen className="h-4 w-4" />
                {filteredShares.length} share{filteredShares.length !== 1 ? 's' : ''}
                {filteredShares.length !== (shares?.length ?? 0) && (
                  <span className="text-xs text-gray-600 dark:text-gray-400">
                    (filtered from {shares?.length ?? 0})
                  </span>
                )}
              </span>
              <span className="flex items-center gap-2">
                <RefreshCw className="h-4 w-4" />
                Auto-refreshes every 15s
              </span>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}

export default DiscoveredShares
