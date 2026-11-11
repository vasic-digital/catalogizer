/**
 * Identity Manager — list/add/edit/remove identities.
 *
 * For each identity type the scheme-specific fields are shown; the **secret**
 * is NEVER rendered back into the DOM after entry (§11.4.10). The list view
 * shows name, type, username (if applicable), domain, status badge, priority,
 * and a masked placeholder for the secret — never the secret value.
 *
 * §11.4.162 — uses OpenDesign/Catalogizer Blue tokens via Tailwind classes.
 * Every text foreground uses `text-gray-900 dark:text-white` (or equivalent
 * semantic token) for light+dark contrast; backgrounds use
 * `bg-gray-50 dark:bg-gray-800` / `bg-white dark:bg-gray-800`.
 * No element/label overlap — layout uses flex/grid with gap.
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
import { Input } from '@/components/ui/Input'
import { Badge } from '@/components/ui/Badge'
import { identitiesApi } from '@/lib/identitiesApi'
import type {
  Identity,
  IdentityType,
  IdentityRequest,
} from '@/types/identity'
import {
  KeyRound,
  Plus,
  Trash2,
  Pencil,
  Check,
  X,
  Lock,
  User,
  Server,
  Terminal,
  Globe,
  Fingerprint,
  ChevronDown,
  ArrowUpDown,
} from 'lucide-react'
import toast from 'react-hot-toast'

/* ------------------------------------------------------------------ */
/*  Identity form — add / edit                                        */
/* ------------------------------------------------------------------ */

interface IdentityFormProps {
  initial?: Identity
  onDone: () => void
}

const EMPTY_FORM: IdentityRequest = {
  name: '',
  type: 'credentials',
  username: null,
  secret: null,
  domain: null,
  key_path: null,
  enabled: true,
  priority: 10,
}

const IDENTITY_TYPE_OPTIONS: Array<{ value: IdentityType; label: string }> = [
  { value: 'credentials', label: 'Username + Password' },
  { value: 'api_token', label: 'API Token' },
  { value: 'ssh_key', label: 'SSH Key' },
  { value: 'webdav_basic', label: 'WebDAV Basic' },
  { value: 'oauth2', label: 'OAuth 2.0' },
]

const typeIcon = (t: IdentityType) => {
  switch (t) {
    case 'credentials':
      return <KeyRound className="h-4 w-4" />
    case 'api_token':
      return <Lock className="h-4 w-4" />
    case 'ssh_key':
      return <Terminal className="h-4 w-4" />
    case 'webdav_basic':
      return <Globe className="h-4 w-4" />
    case 'oauth2':
      return <Fingerprint className="h-4 w-4" />
    default:
      return <KeyRound className="h-4 w-4" />
  }
}

const IdentityForm: React.FC<IdentityFormProps> = ({ initial, onDone }) => {
  const queryClient = useQueryClient()
  const isEdit = !!initial

  const [form, setForm] = useState<IdentityRequest>(() =>
    initial
      ? {
          name: initial.name,
          type: initial.type,
          username: initial.username,
          // NEVER pre-fill the secret value from the existing identity —
          // server returns a secret_ref handle, not the raw secret.
          secret: null,
          domain: initial.domain,
          key_path: initial.key_path,
          enabled: initial.enabled,
          priority: initial.priority,
        }
      : { ...EMPTY_FORM }
  )

  const createMutation = useMutation({
    mutationFn: (body: IdentityRequest) => identitiesApi.create(body),
    onSuccess: () => {
      toast.success('Identity created')
      queryClient.invalidateQueries({ queryKey: ['identities'] })
      onDone()
    },
    onError: () => toast.error('Failed to create identity'),
  })

  const updateMutation = useMutation({
    mutationFn: (body: Partial<IdentityRequest>) =>
      identitiesApi.update(initial?.id ?? 0, body),
    onSuccess: () => {
      toast.success('Identity updated')
      queryClient.invalidateQueries({ queryKey: ['identities'] })
      onDone()
    },
    onError: () => toast.error('Failed to update identity'),
  })

  const isPending = createMutation.isPending || updateMutation.isPending

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!form.name.trim()) {
      toast.error('Name is required')
      return
    }
    if (isEdit) {
      updateMutation.mutate(form)
    } else {
      createMutation.mutate(form)
    }
  }

  const set = (field: keyof IdentityRequest, value: unknown) =>
    setForm((prev) => ({ ...prev, [field]: value }))

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Input
          label="Name"
          placeholder="e.g. nas-admin"
          value={form.name}
          onChange={(e) => set('name', e.target.value)}
          icon={<User className="h-4 w-4" />}
          required
        />

        <div className="space-y-2">
          <label className="text-sm font-medium text-gray-700 dark:text-gray-200">
            Type
          </label>
          <div className="relative">
            <select
              className="flex h-11 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm appearance-none pr-8 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 dark:border-gray-600 dark:bg-gray-800 dark:text-white dark:focus-visible:ring-blue-400 disabled:opacity-50"
              value={form.type}
              onChange={(e) => set('type', e.target.value as IdentityType)}
              disabled={isEdit}
            >
              {IDENTITY_TYPE_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
            <div className="absolute inset-y-0 right-0 flex items-center pr-2 pointer-events-none">
              <ChevronDown className="h-4 w-4 text-gray-400" />
            </div>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {(form.type === 'credentials' || form.type === 'webdav_basic') && (
          <>
            <Input
              label="Username"
              placeholder="username"
              value={form.username ?? ''}
              onChange={(e) => set('username', e.target.value || null)}
              icon={<User className="h-4 w-4" />}
            />
            <Input
              label="Password"
              type="password"
              placeholder={
                isEdit ? 'Leave blank to keep current' : 'Enter secret'
              }
              value={form.secret ?? ''}
              onChange={(e) => set('secret', e.target.value || null)}
              icon={<Lock className="h-4 w-4" />}
            />
          </>
        )}

        {form.type === 'api_token' && (
          <div className="md:col-span-2">
            <Input
              label="API Token"
              type="password"
              placeholder={
                isEdit ? 'Leave blank to keep current' : 'Enter token'
              }
              value={form.secret ?? ''}
              onChange={(e) => set('secret', e.target.value || null)}
              icon={<Lock className="h-4 w-4" />}
            />
          </div>
        )}

        {form.type === 'ssh_key' && (
          <>
            <Input
              label="Key Path"
              placeholder="/path/to/id_rsa"
              value={form.key_path ?? ''}
              onChange={(e) => set('key_path', e.target.value || null)}
              icon={<Terminal className="h-4 w-4" />}
            />
            <Input
              label="Passphrase (optional)"
              type="password"
              placeholder={
                isEdit ? 'Leave blank to keep current' : 'Enter passphrase'
              }
              value={form.secret ?? ''}
              onChange={(e) => set('secret', e.target.value || null)}
              icon={<Lock className="h-4 w-4" />}
            />
          </>
        )}

        {form.type === 'oauth2' && (
          <div className="md:col-span-2">
            <Input
              label="OAuth2 Refresh Token"
              type="password"
              placeholder={
                isEdit ? 'Leave blank to keep current' : 'Enter token'
              }
              value={form.secret ?? ''}
              onChange={(e) => set('secret', e.target.value || null)}
              icon={<Lock className="h-4 w-4" />}
            />
          </div>
        )}
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Input
          label="SMB/NFS Domain"
          placeholder="WORKGROUP"
          value={form.domain ?? ''}
          onChange={(e) => set('domain', e.target.value || null)}
          icon={<Server className="h-4 w-4" />}
        />
        <Input
          label="Priority (lower = tried first)"
          type="number"
          min={0}
          max={100}
          value={String(form.priority ?? 10)}
          onChange={(e) => set('priority', parseInt(e.target.value, 10) || 10)}
          icon={<ArrowUpDown className="h-4 w-4" />}
        />
        <div className="flex items-end pb-2">
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800"
              checked={form.enabled ?? true}
              onChange={(e) => set('enabled', e.target.checked)}
            />
            <span className="text-sm text-gray-700 dark:text-gray-200">
              Enabled
            </span>
          </label>
        </div>
      </div>

      <div className="flex gap-2 pt-2">
        <Button type="submit" loading={isPending}>
          <Check className="h-4 w-4 mr-2" />
          {isEdit ? 'Update' : 'Create'}
        </Button>
        <Button type="button" variant="outline" onClick={onDone}>
          <X className="h-4 w-4 mr-2" />
          Cancel
        </Button>
      </div>
    </form>
  )
}

/* ------------------------------------------------------------------ */
/*  Identity list item                                                */
/* ------------------------------------------------------------------ */

interface IdentityListItemProps {
  identity: Identity
  onEdit: (id: Identity) => void
}

const IdentityListItem: React.FC<IdentityListItemProps> = ({
  identity,
  onEdit,
}) => {
  const queryClient = useQueryClient()

  const removeMutation = useMutation({
    mutationFn: () => identitiesApi.remove(identity.id),
    onSuccess: () => {
      toast.success(`Identity "${identity.name}" removed`)
      queryClient.invalidateQueries({ queryKey: ['identities'] })
    },
    onError: () => toast.error('Failed to remove identity'),
  })

  const typeDisplay = (t: IdentityType): string => {
    switch (t) {
      case 'credentials':
        return 'Username + Password'
      case 'api_token':
        return 'API Token'
      case 'ssh_key':
        return 'SSH Key'
      case 'webdav_basic':
        return 'WebDAV Basic'
      case 'oauth2':
        return 'OAuth2'
      default:
        return t
    }
  }

  return (
    <div className="flex items-center justify-between p-4 rounded-lg bg-gray-50 dark:bg-gray-800">
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-3">
          <span className="p-1.5 rounded bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300">
            {typeIcon(identity.type)}
          </span>
          <div>
            <div className="flex items-center gap-2">
              <span className="font-medium text-gray-900 dark:text-white truncate">
                {identity.name}
              </span>
              <Badge
                variant={
                  identity.enabled ? 'default' : 'outline'
                }
              >
                {identity.enabled ? 'Enabled' : 'Disabled'}
              </Badge>
              <Badge variant="secondary">
                {typeDisplay(identity.type)}
              </Badge>
            </div>
            <div className="flex items-center gap-3 mt-1 text-xs text-gray-500 dark:text-gray-400">
              {identity.username && (
                <span className="flex items-center gap-1">
                  <User className="h-3 w-3" />
                  {identity.username}
                </span>
              )}
              {identity.domain && (
                <span className="flex items-center gap-1">
                  <Server className="h-3 w-3" />
                  {identity.domain}
                </span>
              )}
              {/* Show masked secret indicator — NEVER the secret value */}
              {identity.secret_ref && (
                <span className="flex items-center gap-1 text-gray-400 dark:text-gray-500">
                  <Lock className="h-3 w-3" />
                  &bull;&bull;&bull;&bull;&bull;&bull;&bull;&bull;
                </span>
              )}
              <span>
                Priority: {identity.priority}
              </span>
            </div>
          </div>
        </div>
      </div>
      <div className="flex items-center gap-2 ml-4 shrink-0">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => onEdit(identity)}
        >
          <Pencil className="h-3 w-3" />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => removeMutation.mutate()}
          loading={removeMutation.isPending}
          disabled={identity.type === 'anonymous'}
        >
          <Trash2 className="h-3 w-3 text-red-500" />
        </Button>
      </div>
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Identity Manager — master component                               */
/* ------------------------------------------------------------------ */

export const IdentityManager: React.FC = () => {
  const [showForm, setShowForm] = useState(false)
  const [editing, setEditing] = useState<Identity | null>(null)

  const { data: identities, isLoading } = useQuery({
    queryKey: ['identities'],
    queryFn: () => identitiesApi.list(),
    staleTime: 1000 * 30,
  })

  const handleEdit = (identity: Identity) => {
    setEditing(identity)
    setShowForm(true)
  }

  const handleDone = () => {
    setShowForm(false)
    setEditing(null)
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle className="flex items-center gap-2">
              <KeyRound className="h-5 w-5" />
              Identities
            </CardTitle>
            {!showForm && (
              <Button variant="outline" onClick={() => setShowForm(true)}>
                <Plus className="h-4 w-4 mr-2" />
                Add Identity
              </Button>
            )}
          </div>
        </CardHeader>
        <CardContent>
          {showForm && (
            <div className="mb-6 p-4 rounded-lg border border-gray-200 dark:border-gray-700">
              <h3 className="text-base font-semibold text-gray-900 dark:text-white mb-4">
                {editing ? `Edit "${editing.name}"` : 'New Identity'}
              </h3>
              <IdentityForm
                key={editing?.id ?? 'new'}
                initial={editing ?? undefined}
                onDone={handleDone}
              />
            </div>
          )}

          {isLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 3 }).map((_, i) => (
                <div
                  key={i}
                  className="h-16 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"
                />
              ))}
            </div>
          ) : !identities || identities.length === 0 ? (
            <p className="text-sm text-gray-500 dark:text-gray-400">
              No identities configured. Add one to start probing network
              shares.
            </p>
          ) : (
            <div className="space-y-3">
              {identities.map((id) => (
                <IdentityListItem
                  key={id.id}
                  identity={id}
                  onEdit={handleEdit}
                />
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

export default IdentityManager
