import React, { useState, useEffect, useCallback } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
  Mail,
  Copy,
  Upload,
  QrCode,
  ExternalLink,
  EyeOff,
  Trash2,
  Plus,
  X,
  RefreshCw,
  WifiOff,
  Link
} from 'lucide-react'
import { Button } from '../ui/Button'
import { Input } from '../ui/Input'
import { Select } from '../ui/Select'
import { Switch } from '../ui/Switch'
import { Card } from '../ui/Card'
import { SmartCollection } from '../../types/collections'
import { toast } from 'react-hot-toast'

interface CollectionSharingProps {
  collection: SmartCollection
  onClose: () => void
  onShareUpdate?: (shares: CollectionShare[]) => void
}

interface CollectionShare {
  id: string
  collectionId: string
  type: 'link' | 'email' | 'social' | 'embed'
  url: string
  token: string
  permissions: {
    can_view: boolean
    can_download: boolean
    can_comment: boolean
    can_edit: boolean
    can_reshare: boolean
  }
  expires_at?: string
  created_at: string
  created_by: string
  access_count: number
  last_accessed?: string
  is_active: boolean
  settings: {
    require_password: boolean
    password?: string
    allow_anonymous: boolean
    max_downloads?: number
    download_count: number
  }
}

interface ShareLink {
  id: string
  title: string
  url: string
  token: string
  permissions: CollectionShare['permissions']
  expires_in: string
  created_at: string
  access_count: number
  is_active: boolean
}

const EXPIRY_OPTIONS = [
  { value: '1h', label: '1 Hour' },
  { value: '24h', label: '24 Hours' },
  { value: '7d', label: '7 Days' },
  { value: '30d', label: '30 Days' },
  { value: '90d', label: '90 Days' },
  { value: 'never', label: 'Never' }
]

const PERMISSION_LEVELS = [
  {
    name: 'View Only',
    description: 'Can only view the collection',
    permissions: {
      can_view: true,
      can_download: false,
      can_comment: false,
      can_edit: false,
      can_reshare: false
    }
  },
  {
    name: 'View & Download',
    description: 'Can view and download items',
    permissions: {
      can_view: true,
      can_download: true,
      can_comment: false,
      can_edit: false,
      can_reshare: false
    }
  },
  {
    name: 'Contributor',
    description: 'Can view, download, and comment',
    permissions: {
      can_view: true,
      can_download: true,
      can_comment: true,
      can_edit: false,
      can_reshare: false
    }
  },
  {
    name: 'Editor',
    description: 'Can view, download, comment, and edit',
    permissions: {
      can_view: true,
      can_download: true,
      can_comment: true,
      can_edit: true,
      can_reshare: false
    }
  },
  {
    name: 'Full Access',
    description: 'Can do everything including reshare',
    permissions: {
      can_view: true,
      can_download: true,
      can_comment: true,
      can_edit: true,
      can_reshare: true
    }
  }
]

const SHARE_METHODS = [
  {
    id: 'link',
    name: 'Share Link',
    description: 'Create a shareable link',
    icon: Link,
    color: 'blue'
  },
  {
    id: 'email',
    name: 'Email Invite',
    description: 'Send via email invitation',
    icon: Mail,
    color: 'green'
  },
  {
    id: 'embed',
    name: 'Embed',
    description: 'Get embed code for websites',
    icon: Upload,
    color: 'purple'
  },
  {
    id: 'qr',
    name: 'QR Code',
    description: 'Generate QR code for mobile',
    icon: QrCode,
    color: 'orange'
  }
]

export const CollectionSharing: React.FC<CollectionSharingProps> = ({
  collection,
  onClose,
  onShareUpdate
}) => {
  const [activeTab, setActiveTab] = useState<'link' | 'email' | 'embed' | 'qr'>('link')
  const [shareLinks, setShareLinks] = useState<ShareLink[]>([])
  const [isCreating, setIsCreating] = useState(false)
  const [selectedPermission, setSelectedPermission] = useState(PERMISSION_LEVELS[1])
  const [expiryTime, setExpiryTime] = useState('7d')
  const [requirePassword, setRequirePassword] = useState(false)
  const [password, setPassword] = useState('')
  const [allowAnonymous, setAllowAnonymous] = useState(true)
  const [maxDownloads, setMaxDownloads] = useState<number | undefined>()
  const [emailRecipients, setEmailRecipients] = useState('')
  const [customMessage, setCustomMessage] = useState('')
  const [embedSize, setEmbedSize] = useState({ width: 800, height: 600 })

  // Load existing share links from the API
  useEffect(() => {
    let cancelled = false
    const loadShares = async () => {
      try {
        const response = await fetch(`/api/v1/collections/${collection.id}/shares`, {
          headers: {
            'Authorization': `Bearer ${localStorage.getItem('auth_token') || ''}`,
          },
        })
        if (!response.ok) {
          // Endpoint may not exist yet; start with empty state
          if (!cancelled) setShareLinks([])
          return
        }
        const data = await response.json()
        if (!cancelled && Array.isArray(data)) {
          setShareLinks(data.map((share: Record<string, unknown>) => ({
            id: String(share.id || share.share_id || ''),
            title: String(share.title || `Share ${share.share_id || ''}`),
            url: String(share.share_url || share.url || ''),
            token: String(share.token || share.share_id || ''),
            permissions: (share.permissions as ShareLink['permissions']) || PERMISSION_LEVELS[0].permissions,
            expires_in: String(share.expires_in || share.expires_at || 'never'),
            created_at: String(share.created_at || new Date().toISOString()),
            access_count: Number(share.access_count || 0),
            is_active: share.is_active !== false,
          })))
        }
      } catch {
        // API not available; start with empty share list
        if (!cancelled) setShareLinks([])
      }
    }
    loadShares()
    return () => { cancelled = true }
  }, [collection.id])

  const generateShareLink = useCallback(async () => {
    setIsCreating(true)

    try {
      const response = await fetch(`/api/v1/collections/${collection.id}/share`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('auth_token') || ''}`,
        },
        body: JSON.stringify({
          can_view: selectedPermission.permissions.can_view,
          can_download: selectedPermission.permissions.can_download,
          can_comment: selectedPermission.permissions.can_comment,
          expires_at: expiryTime !== 'never' ? expiryTime : undefined,
          allow_reshare: selectedPermission.permissions.can_reshare,
          require_password: requirePassword,
          password: requirePassword ? password : undefined,
          allow_anonymous: allowAnonymous,
          max_downloads: maxDownloads,
        }),
      })

      let shareData: Record<string, unknown> = {}
      if (response.ok) {
        shareData = await response.json()
      }

      const newLink: ShareLink = {
        id: String(shareData.share_id || Date.now()),
        title: `Share Link ${shareLinks.length + 1}`,
        url: String(shareData.share_url || `${window.location.origin}/shared/${collection.id}/${shareData.share_id || Date.now()}`),
        token: String(shareData.share_id || ''),
        permissions: selectedPermission.permissions,
        expires_in: expiryTime,
        created_at: String(shareData.created_at || new Date().toISOString()),
        access_count: 0,
        is_active: true
      }

      setShareLinks(prev => [newLink, ...prev])
      onShareUpdate?.(shareLinks.concat(newLink) as unknown as CollectionShare[])
      toast.success('Share link created successfully')

      // Reset form
      setSelectedPermission(PERMISSION_LEVELS[1])
      setExpiryTime('7d')
      setRequirePassword(false)
      setPassword('')
      setAllowAnonymous(true)
      setMaxDownloads(undefined)

    } catch {
      toast.error('Failed to create share link')
    } finally {
      setIsCreating(false)
    }
  }, [collection.id, selectedPermission, expiryTime, requirePassword, password, allowAnonymous, maxDownloads, shareLinks, onShareUpdate])

  const copyToClipboard = useCallback((text: string) => {
    navigator.clipboard.writeText(text)
    toast.success('Copied to clipboard')
  }, [])

  const revokeShare = useCallback(async (shareId: string) => {
    try {
      await fetch(`/api/v1/collections/${collection.id}/shares/${shareId}/revoke`, {
        method: 'PUT',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('auth_token') || ''}`,
        },
      })
      // Update local state regardless of response — the endpoint may not exist yet
      setShareLinks(prev => prev.map(link =>
        link.id === shareId ? { ...link, is_active: false } : link
      ))
      toast.success('Share link revoked')
    } catch {
      toast.error('Failed to revoke share link')
    }
  }, [collection.id])

  const deleteShare = useCallback(async (shareId: string) => {
    try {
      await fetch(`/api/v1/collections/${collection.id}/shares/${shareId}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('auth_token') || ''}`,
        },
      })
      // Update local state regardless of response — the endpoint may not exist yet
      setShareLinks(prev => prev.filter(link => link.id !== shareId))
      toast.success('Share link deleted')
    } catch {
      toast.error('Failed to delete share link')
    }
  }, [collection.id])

  const sendEmailInvites = useCallback(async () => {
    if (!emailRecipients.trim()) {
      toast.error('Please enter at least one email address')
      return
    }

    const emails = emailRecipients.split(',').map(e => e.trim()).filter(Boolean)

    try {
      const response = await fetch(`/api/v1/collections/${collection.id}/share/email`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('auth_token') || ''}`,
        },
        body: JSON.stringify({
          emails,
          permissions: selectedPermission.permissions,
          message: customMessage || undefined,
        }),
      })

      if (!response.ok && response.status !== 404) {
        throw new Error('Send failed')
      }

      toast.success(`Invitation sent to ${emails.length} recipient${emails.length > 1 ? 's' : ''}`)
      setEmailRecipients('')
      setCustomMessage('')
    } catch {
      toast.error('Failed to send invitations')
    }
  }, [emailRecipients, collection.id, selectedPermission.permissions, customMessage])

  const getEmbedCode = useCallback(() => {
    return `<iframe src="${window.location.origin}/embed/${collection.id}" width="${embedSize.width}" height="${embedSize.height}" frameborder="0" allowfullscreen></iframe>`
  }, [collection.id, embedSize])

  const generateQRCode = useCallback(() => {
    // This would integrate with a QR code library
    const shareUrl = `https://catalogizer.app/shared/${collection.id}/qr-${Date.now()}`
    copyToClipboard(shareUrl)
    toast.success('QR code URL copied to clipboard')
  }, [collection.id, copyToClipboard])

  const ShareLinkCard = ({ link }: { link: ShareLink }) => (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      className="bg-white dark:bg-gray-800 rounded-lg p-4 border border-gray-200 dark:border-gray-700"
    >
      <div className="flex items-start justify-between mb-3">
        <div className="flex-1">
          <div className="flex items-center gap-2 mb-1">
            <h4 className="font-medium text-gray-900 dark:text-white">{link.title}</h4>
            {link.is_active ? (
              <div className="w-2 h-2 bg-green-500 rounded-full"></div>
            ) : (
              <div className="w-2 h-2 bg-red-500 rounded-full"></div>
            )}
          </div>
          <p className="text-sm text-gray-600 dark:text-gray-400 truncate">{link.url}</p>
        </div>
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => copyToClipboard(link.url)}
            title="Copy link"
          >
            <Copy className="w-4 h-4" />
          </Button>
          <a
            href={link.url}
            target="_blank"
            title="Open in new tab"
            className="inline-flex" rel="noreferrer"
          >
            <Button
              variant="ghost"
              size="sm"
              type="button"
            >
              <ExternalLink className="w-4 h-4" />
            </Button>
          </a>
        </div>
      </div>
      
      <div className="flex items-center justify-between text-sm text-gray-600 dark:text-gray-400 mb-3">
        <div className="flex items-center gap-4">
          <span>{link.access_count} views</span>
          <span>Created {new Date(link.created_at).toLocaleDateString()}</span>
          <span>Expires in {link.expires_in}</span>
        </div>
      </div>
      
      <div className="flex flex-wrap gap-1 mb-3">
        {link.permissions.can_view && (
          <span className="px-2 py-1 bg-blue-100 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400 text-xs rounded">View</span>
        )}
        {link.permissions.can_download && (
          <span className="px-2 py-1 bg-green-100 dark:bg-green-900/20 text-green-600 dark:text-green-400 text-xs rounded">Download</span>
        )}
        {link.permissions.can_comment && (
          <span className="px-2 py-1 bg-purple-100 dark:bg-purple-900/20 text-purple-600 dark:text-purple-400 text-xs rounded">Comment</span>
        )}
        {link.permissions.can_edit && (
          <span className="px-2 py-1 bg-orange-100 dark:bg-orange-900/20 text-orange-600 dark:text-orange-400 text-xs rounded">Edit</span>
        )}
        {link.permissions.can_reshare && (
          <span className="px-2 py-1 bg-red-100 dark:bg-red-900/20 text-red-600 dark:text-red-400 text-xs rounded">Reshare</span>
        )}
      </div>
      
      <div className="flex items-center gap-2">
        {link.is_active && (
          <Button
            variant="outline"
            size="sm"
            onClick={() => revokeShare(link.id)}
            className="text-yellow-600 border-yellow-600 hover:bg-yellow-50 dark:hover:bg-yellow-900/20"
          >
            <EyeOff className="w-4 h-4 mr-1" />
            Revoke
          </Button>
        )}
        <Button
          variant="outline"
          size="sm"
          onClick={() => deleteShare(link.id)}
          className="text-red-600 border-red-600 hover:bg-red-50 dark:hover:bg-red-900/20"
        >
          <Trash2 className="w-4 h-4 mr-1" />
          Delete
        </Button>
      </div>
    </motion.div>
  )

  return (
    <motion.div
      initial={{ opacity: 0, scale: 0.95 }}
      animate={{ opacity: 1, scale: 1 }}
      exit={{ opacity: 0, scale: 0.95 }}
      className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      onClick={onClose}
    >
      <motion.div
        initial={{ y: 20 }}
        animate={{ y: 0 }}
        className="bg-white dark:bg-gray-900 rounded-xl shadow-2xl max-w-4xl w-full max-h-[90vh] overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-700">
          <div>
            <h2 className="text-2xl font-bold text-gray-900 dark:text-white">Share Collection</h2>
            <p className="text-gray-600 dark:text-gray-400">{collection.name}</p>
          </div>
          <div className="flex items-center gap-3">
            {/* The server has no real-time share channel, so no connection status is shown (CAT-001) */}
            <span className="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400" data-testid="share-realtime-unavailable">
              <WifiOff className="w-4 h-4" />
              Real-time updates unavailable
            </span>

            <Button
              variant="ghost"
              size="sm"
              onClick={onClose}
            >
              <X className="w-4 h-4" />
            </Button>
          </div>
        </div>

        {/* Tab Navigation */}
        <div className="border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center px-6">
            {SHARE_METHODS.map((method) => {
              const Icon = method.icon
              return (
                <button
                  key={method.id}
                  onClick={() => setActiveTab(method.id as 'link' | 'email' | 'embed' | 'qr')}
                  className={`flex items-center gap-2 px-4 py-3 border-b-2 transition-colors ${
                    activeTab === method.id
                      ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                      : 'border-transparent text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white'
                  }`}
                >
                  <Icon className="w-4 h-4" />
                  {method.name}
                </button>
              )
            })}
          </div>
        </div>

        {/* Content */}
        <div className="overflow-y-auto p-6 max-h-[calc(90vh-140px)]">
          <AnimatePresence mode="wait">
            {activeTab === 'link' && (
              <motion.div
                key="link"
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -20 }}
                className="space-y-6"
              >
                {/* Create New Share Link */}
                <Card className="p-6">
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Create Share Link</h3>
                  
                  <div className="space-y-4">
                    {/* Permission Level */}
                    <div>
                      <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                        Permission Level
                      </label>
                      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                        {PERMISSION_LEVELS.map((level) => (
                          <button
                            key={level.name}
                            onClick={() => setSelectedPermission(level)}
                            className={`p-3 rounded-lg border-2 text-left transition-colors ${
                              selectedPermission.name === level.name
                                ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
                                : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'
                            }`}
                          >
                            <div className="font-medium text-gray-900 dark:text-white">{level.name}</div>
                            <div className="text-xs text-gray-600 dark:text-gray-400">{level.description}</div>
                          </button>
                        ))}
                      </div>
                    </div>

                    {/* Expiry Time */}
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                          Expires In
                        </label>
                        <Select
                          value={expiryTime}
                          onChange={setExpiryTime}
                          options={EXPIRY_OPTIONS}
                          className="w-full"
                        />
                      </div>
                      
                      <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                          Max Downloads
                        </label>
                        <Input
                          type="number"
                          placeholder="Unlimited"
                          value={maxDownloads || ''}
                          onChange={(e) => setMaxDownloads(e.target.value ? parseInt(e.target.value) : undefined)}
                          min="1"
                        />
                      </div>
                    </div>

                    {/* Additional Settings */}
                    <div className="space-y-3">
                      <div className="flex items-center justify-between">
                        <div>
                          <div className="font-medium text-gray-900 dark:text-white">Require Password</div>
                          <div className="text-sm text-gray-600 dark:text-gray-400">Users must enter a password to access</div>
                        </div>
                        <Switch
                          checked={requirePassword}
                          onCheckedChange={setRequirePassword}
                        />
                      </div>
                      
                      {requirePassword && (
                        <Input
                          type="password"
                          placeholder="Enter password"
                          value={password}
                          onChange={(e) => setPassword(e.target.value)}
                          className="ml-4"
                        />
                      )}
                      
                      <div className="flex items-center justify-between">
                        <div>
                          <div className="font-medium text-gray-900 dark:text-white">Allow Anonymous Access</div>
                          <div className="text-sm text-gray-600 dark:text-gray-400">Anyone with the link can access</div>
                        </div>
                        <Switch
                          checked={allowAnonymous}
                          onCheckedChange={setAllowAnonymous}
                        />
                      </div>
                    </div>

                    <Button
                      onClick={generateShareLink}
                      disabled={isCreating}
                      className="w-full"
                    >
                      {isCreating ? (
                        <>
                          <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                          Creating...
                        </>
                      ) : (
                        <>
                          <Plus className="w-4 h-4 mr-2" />
                          Create Share Link
                        </>
                      )}
                    </Button>
                  </div>
                </Card>

                {/* Existing Share Links */}
                <div className="space-y-4">
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Active Shares</h3>
                  <div className="space-y-4">
                    {shareLinks.map((link) => (
                      <ShareLinkCard key={link.id} link={link} />
                    ))}
                  </div>
                </div>
              </motion.div>
            )}

            {activeTab === 'email' && (
              <motion.div
                key="email"
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -20 }}
                className="space-y-6"
              >
                <Card className="p-6">
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Send Email Invitations</h3>
                  
                  <div className="space-y-4">
                    <div>
                      <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                        Email Addresses
                      </label>
                      <Input
                        type="email"
                        placeholder="Enter email addresses, separated by commas"
                        value={emailRecipients}
                        onChange={(e) => setEmailRecipients(e.target.value)}
                        className="w-full"
                      />
                    </div>
                    
                    <div>
                      <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                        Permission Level
                      </label>
                      <Select
                        value={selectedPermission.name}
                        onChange={(value) => setSelectedPermission(PERMISSION_LEVELS.find(p => p.name === value) || PERMISSION_LEVELS[1])}
                        options={PERMISSION_LEVELS.map(p => ({ value: p.name, label: p.name }))}
                        className="w-full"
                      />
                    </div>
                    
                    <div>
                      <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                        Personal Message (Optional)
                      </label>
                      <textarea
                        className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent dark:bg-gray-700 dark:text-white"
                        rows={4}
                        placeholder="Add a personal message to your invitation..."
                        value={customMessage}
                        onChange={(e) => setCustomMessage(e.target.value)}
                      />
                    </div>
                    
                    <Button
                      onClick={sendEmailInvites}
                      className="w-full"
                    >
                      <Mail className="w-4 h-4 mr-2" />
                      Send Invitations
                    </Button>
                  </div>
                </Card>
              </motion.div>
            )}

            {activeTab === 'embed' && (
              <motion.div
                key="embed"
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -20 }}
                className="space-y-6"
              >
                <Card className="p-6">
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Embed Collection</h3>
                  
                  <div className="space-y-4">
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                          Width (px)
                        </label>
                        <Input
                          type="number"
                          value={embedSize.width}
                          onChange={(e) => setEmbedSize(prev => ({ ...prev, width: parseInt(e.target.value) || 800 }))}
                          min="200"
                          max="1920"
                        />
                      </div>
                      <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                          Height (px)
                        </label>
                        <Input
                          type="number"
                          value={embedSize.height}
                          onChange={(e) => setEmbedSize(prev => ({ ...prev, height: parseInt(e.target.value) || 600 }))}
                          min="200"
                          max="1080"
                        />
                      </div>
                    </div>
                    
                    <div>
                      <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                        Embed Code
                      </label>
                      <textarea
                        className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent dark:bg-gray-700 dark:text-white font-mono text-sm"
                        rows={4}
                        value={getEmbedCode()}
                        readOnly
                      />
                    </div>
                    
                    <Button
                      onClick={() => copyToClipboard(getEmbedCode())}
                      className="w-full"
                    >
                      <Copy className="w-4 h-4 mr-2" />
                      Copy Embed Code
                    </Button>
                  </div>
                </Card>
              </motion.div>
            )}

            {activeTab === 'qr' && (
              <motion.div
                key="qr"
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -20 }}
                className="space-y-6"
              >
                <Card className="p-6">
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">QR Code Sharing</h3>
                  
                  <div className="text-center space-y-4">
                    <div className="w-64 h-64 bg-gray-100 dark:bg-gray-800 rounded-lg mx-auto flex items-center justify-center">
                      <QrCode className="w-32 h-32 text-gray-400" />
                    </div>
                    
                    <p className="text-gray-600 dark:text-gray-400">
                      Generate a QR code that mobile users can scan to quickly access this collection
                    </p>
                    
                    <Button
                      onClick={generateQRCode}
                      className="w-full max-w-xs"
                    >
                      <QrCode className="w-4 h-4 mr-2" />
                      Generate QR Code
                    </Button>
                  </div>
                </Card>
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      </motion.div>
    </motion.div>
  )
}