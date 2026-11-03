import React, { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { 
  Trash2, 
  Share2, 
  Download, 
  Copy, 
  Tag, 
  Archive, 
  FolderOpen,
  MoreHorizontal,
  X
} from 'lucide-react'
import { Button } from '../ui/Button'
import { Switch } from '../ui/Switch'

interface BulkOperationsProps {
  selectedCollections: string[]
  onOperation: (operation: string, options?: unknown) => void
  onClose: () => void
}

type BulkAction = {
  id: string
  label: string
  icon: React.ComponentType<{ className?: string }>
  description: string
  requiresConfirmation: boolean
  options?: React.ComponentType<{ value: unknown; onChange: (value: unknown) => void }>
}

interface ActionOptions {
  deleteForever?: boolean
  shareWithPermissions?: {
    can_download: boolean
    can_reshare: boolean
    expires_at?: string
  }
  exportFormat?: 'json' | 'csv' | 'm3u'
  moveTo?: string
  addTags?: string[]
}

export const BulkOperations: React.FC<BulkOperationsProps> = ({
  selectedCollections,
  onOperation,
  onClose
}) => {
  const [selectedAction, setSelectedAction] = useState<string | null>(null)
  const [actionOptions, setActionOptions] = useState<ActionOptions>({})
  const [showConfirmation, setShowConfirmation] = useState(false)
  const [isActionInProgress, setIsActionInProgress] = useState(false)

  const bulkActions: BulkAction[] = [
    {
      id: 'delete',
      label: 'Delete Collections',
      icon: Trash2,
      description: 'Permanently remove selected collections',
      requiresConfirmation: true
    },
    {
      id: 'share',
      label: 'Share Collections',
      icon: Share2,
      description: 'Share selected collections with others',
      requiresConfirmation: false,
      options: ShareOptions as React.ComponentType<{ value: unknown; onChange: (value: unknown) => void }>
    },
    {
      id: 'export',
      label: 'Export Collections',
      icon: Download,
      description: 'Export selected collections to file',
      requiresConfirmation: false,
      options: ExportOptions as React.ComponentType<{ value: unknown; onChange: (value: unknown) => void }>
    },
    {
      id: 'duplicate',
      label: 'Duplicate Collections',
      icon: Copy,
      description: 'Create copies of selected collections',
      requiresConfirmation: false,
      options: DuplicateOptions as React.ComponentType<{ value: unknown; onChange: (value: unknown) => void }>
    },
    {
      id: 'tag',
      label: 'Add Tags',
      icon: Tag,
      description: 'Add tags to selected collections',
      requiresConfirmation: false,
      options: TagOptions as React.ComponentType<{ value: unknown; onChange: (value: unknown) => void }>
    },
    {
      id: 'archive',
      label: 'Archive Collections',
      icon: Archive,
      description: 'Archive selected collections to storage',
      requiresConfirmation: false,
      options: ArchiveOptions as React.ComponentType<{ value: unknown; onChange: (value: unknown) => void }>
    },
    {
      id: 'move',
      label: 'Move Collections',
      icon: FolderOpen,
      description: 'Move selected collections to folder',
      requiresConfirmation: false,
      options: MoveOptions as React.ComponentType<{ value: unknown; onChange: (value: unknown) => void }>
    }
  ]

  const handleActionSelect = (actionId: string) => {
    const action = bulkActions.find(a => a.id === actionId)
    if (!action) return

    if (action.requiresConfirmation) {
      setSelectedAction(actionId)
      setShowConfirmation(true)
    } else if (action.options) {
      setSelectedAction(actionId)
      setActionOptions({})
    } else {
      executeAction(actionId)
    }
  }

  const executeAction = async (actionId: string, options?: ActionOptions) => {
    try {
      setIsActionInProgress(true)
      onOperation(actionId, options || actionOptions)
      setSelectedAction(null)
      setActionOptions({})
      setShowConfirmation(false)
      onClose()
    } catch (error) {
      // Bulk action failure — handled by caller via onOperation
      void error;
    } finally {
      setIsActionInProgress(false)
    }
  }

  const handleConfirmAction = async () => {
    if (selectedAction) {
      await executeAction(selectedAction, actionOptions)
    }
  }

  const getActionIcon = (actionId: string) => {
    const action = bulkActions.find(a => a.id === actionId)
    return action ? action.icon : MoreHorizontal
  }

  const getActionDescription = (actionId: string) => {
    const action = bulkActions.find(a => a.id === actionId)
    return action ? action.description : ''
  }

  return (
    <AnimatePresence>
      <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
        <motion.div
          initial={{ opacity: 0, scale: 0.9 }}
          animate={{ opacity: 1, scale: 1 }}
          exit={{ opacity: 0, scale: 0.9 }}
          className="bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-4xl max-h-[80vh] overflow-hidden"
        >
          {/* Header */}
          <div className="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-700">
            <div>
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
                Bulk Operations
              </h3>
              <p className="text-sm text-gray-500 dark:text-gray-400">
                {selectedCollections.length} collections selected
              </p>
            </div>
            
            <Button
              variant="ghost"
              size="sm"
              onClick={onClose}
              disabled={isActionInProgress}
            >
              <X className="w-4 h-4" />
            </Button>
          </div>

          {/* Content */}
          {!selectedAction ? (
            <div className="p-6 overflow-y-auto max-h-[calc(80vh-120px)]">
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {bulkActions.map((action) => {
                  const Icon = action.icon
                  
                  return (
                    <motion.div
                      key={action.id}
                      whileHover={{ scale: 1.02 }}
                      whileTap={{ scale: 0.98 }}
                      className="p-4 rounded-lg border cursor-pointer transition-all border-gray-200 dark:border-gray-700 hover:border-blue-500 dark:hover:border-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/20"
                      onClick={() => handleActionSelect(action.id)}
                    >
                      <div className="flex items-center gap-3">
                        <div className="w-12 h-12 bg-gray-100 dark:bg-gray-700 rounded-lg flex items-center justify-center">
                          <Icon className="w-6 h-6 text-gray-600 dark:text-gray-400" />
                        </div>
                        
                        <div className="flex-1">
                          <h4 className="font-medium text-gray-900 dark:text-white">
                            {action.label}
                          </h4>
                          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                            {action.description}
                          </p>
                        </div>
                      </div>
                    </motion.div>
                  )
                })}
              </div>
            </div>
          ) : (
            <div className="p-6 overflow-y-auto max-h-[calc(80vh-120px)]">
              <div className="flex items-center gap-3 mb-6">
                <div className="w-10 h-10 bg-blue-100 dark:bg-blue-900 rounded-lg flex items-center justify-center">
                  {React.createElement(getActionIcon(selectedAction), { 
                    className: 'w-5 h-5 text-blue-600 dark:text-blue-400' 
                  })}
                </div>
                
                <div>
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
                    {bulkActions.find(a => a.id === selectedAction)?.label}
                  </h3>
                  <p className="text-sm text-gray-500 dark:text-gray-400">
                    {getActionDescription(selectedAction)}
                  </p>
                </div>
              </div>

              {/* Action Options */}
              <div className="mb-6">
                {selectedAction === 'share' && <ShareOptions value={actionOptions as ShareOptionsValue} onChange={(v) => setActionOptions(v as ActionOptions)} />}
                {selectedAction === 'export' && <ExportOptions value={actionOptions as ExportOptionsValue} onChange={(v) => setActionOptions(v as ActionOptions)} />}
                {selectedAction === 'duplicate' && <DuplicateOptions value={actionOptions as DuplicateOptionsValue} onChange={(v) => setActionOptions(v as ActionOptions)} />}
                {selectedAction === 'tag' && <TagOptions value={actionOptions as TagOptionsValue} onChange={(v) => setActionOptions(v as ActionOptions)} />}
                {selectedAction === 'archive' && <ArchiveOptions value={actionOptions as ArchiveOptionsValue} onChange={(v) => setActionOptions(v as ActionOptions)} />}
                {selectedAction === 'move' && <MoveOptions value={actionOptions as MoveOptionsValue} onChange={(v) => setActionOptions(v as ActionOptions)} />}
                {selectedAction === 'delete' && <DeleteOptions value={actionOptions as DeleteOptionsValue} onChange={(v) => setActionOptions(v as ActionOptions)} />}
              </div>
              
              {/* Actions */}
              <div className="flex items-center gap-3">
                <Button
                  variant="outline"
                  onClick={() => {
                    setSelectedAction(null)
                    setActionOptions({})
                  }}
                  disabled={isActionInProgress}
                >
                  Back
                </Button>
                
                <Button
                  onClick={() => {
                    if (bulkActions.find(a => a.id === selectedAction)?.requiresConfirmation) {
                      setShowConfirmation(true)
                    } else {
                      executeAction(selectedAction)
                    }
                  }}
                  disabled={isActionInProgress}
                  className="flex-1"
                >
                  {isActionInProgress ? 'Processing...' : 'Execute Action'}
                </Button>
              </div>
            </div>
          )}

          {/* Confirmation Modal */}
          {showConfirmation && (
            <div className="absolute inset-0 bg-black/70 flex items-center justify-center">
              <motion.div
                initial={{ scale: 0.9 }}
                animate={{ scale: 1 }}
                exit={{ scale: 0.9 }}
                className="bg-white dark:bg-gray-800 rounded-lg shadow-xl p-6 max-w-md w-full mx-4"
              >
                <div className="flex items-center gap-3 mb-4">
                  <div className="w-12 h-12 bg-red-100 dark:bg-red-900 rounded-full flex items-center justify-center">
                    <Trash2 className="w-6 h-6 text-red-600 dark:text-red-400" />
                  </div>
                  
                  <div>
                    <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
                      Delete Collections?
                    </h3>
                    <p className="text-sm text-gray-500 dark:text-gray-400">
                      This will permanently delete {selectedCollections.length} collection(s) and cannot be undone.
                    </p>
                  </div>
                </div>
                
                <div className="flex items-center gap-3">
                  <Button
                    variant="outline"
                    onClick={() => {
                      setShowConfirmation(false)
                      setSelectedAction(null)
                      setActionOptions({})
                    }}
                    disabled={isActionInProgress}
                  >
                    Cancel
                  </Button>
                  
                  <Button
                    variant="destructive"
                    onClick={handleConfirmAction}
                    disabled={isActionInProgress}
                  >
                    {isActionInProgress ? 'Deleting...' : 'Delete'}
                  </Button>
                </div>
              </motion.div>
            </div>
          )}
        </motion.div>
      </div>
    </AnimatePresence>
  )
}

interface ShareOptionsValue {
  shareWithPermissions?: {
    can_download?: boolean;
    can_reshare?: boolean;
  };
}

// Action Options Components
const ShareOptions: React.FC<{ value: ShareOptionsValue; onChange: (value: ShareOptionsValue) => void }> = ({ value, onChange }) => (
  <div className="space-y-4">
    <div>
      <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
        Download Permission
      </label>
      <Switch
        checked={value.shareWithPermissions?.can_download ?? true}
        onCheckedChange={(checked) => 
          onChange({
            ...value,
            shareWithPermissions: {
              ...value.shareWithPermissions,
              can_download: checked
            }
          })
        }
      />
    </div>
    
    <div>
      <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
        Reshare Permission
      </label>
      <Switch
        checked={value.shareWithPermissions?.can_reshare ?? false}
        onCheckedChange={(checked) => 
          onChange({
            ...value,
            shareWithPermissions: {
              ...value.shareWithPermissions,
              can_reshare: checked
            }
          })
        }
      />
    </div>
  </div>
)

interface ExportOptionsValue {
  exportFormat?: string;
}

const ExportOptions: React.FC<{ value: ExportOptionsValue; onChange: (value: ExportOptionsValue) => void }> = ({ value, onChange }) => (
  <div>
    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
      Export Format
    </label>
    <select
      value={value.exportFormat || 'json'}
      onChange={(e) => onChange({ ...value, exportFormat: e.target.value })}
      className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
    >
      <option value="json">JSON</option>
      <option value="csv">CSV</option>
      <option value="m3u">M3U Playlist</option>
    </select>
  </div>
)

interface DuplicateOptionsValue {
  suffix?: string;
}

const DuplicateOptions: React.FC<{ value: DuplicateOptionsValue; onChange: (value: DuplicateOptionsValue) => void }> = ({ value, onChange }) => (
  <div>
    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
      Suffix
    </label>
    <input
      type="text"
      value={value.suffix || '(Copy)'}
      onChange={(e) => onChange({ ...value, suffix: e.target.value })}
      placeholder="Add suffix to duplicated collections"
      className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
    />
  </div>
)

interface TagOptionsValue {
  addTags?: string[];
}

const TagOptions: React.FC<{ value: TagOptionsValue; onChange: (value: TagOptionsValue) => void }> = ({ value, onChange }) => (
  <div>
    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
      Tags
    </label>
    <input
      type="text"
      value={(value.addTags || []).join(', ')}
      onChange={(e) => onChange({ 
        ...value, 
        addTags: e.target.value.split(',').map(tag => tag.trim()).filter(Boolean) 
      })}
      placeholder="Enter tags separated by commas"
      className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
    />
  </div>
)

interface ArchiveOptionsValue {
  archiveLocation?: string;
  compressArchive?: boolean;
}

const ArchiveOptions: React.FC<{ value: ArchiveOptionsValue; onChange: (value: ArchiveOptionsValue) => void }> = ({ value, onChange }) => (
  <div className="space-y-4">
    <div>
      <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
        Archive Location
      </label>
      <select
        value={value.archiveLocation || 'default'}
        onChange={(e) => onChange({ ...value, archiveLocation: e.target.value })}
        className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
      >
        <option value="default">Default Archive</option>
        <option value="custom">Custom Location</option>
      </select>
    </div>
    
    <div>
      <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
        Compress Archive
      </label>
      <Switch
        checked={value.compressArchive ?? true}
        onCheckedChange={(checked) => onChange({ ...value, compressArchive: checked })}
      />
    </div>
  </div>
)

interface MoveOptionsValue {
  moveTo?: string;
}

const MoveOptions: React.FC<{ value: MoveOptionsValue; onChange: (value: MoveOptionsValue) => void }> = ({ value, onChange }) => (
  <div>
    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
      Destination Folder
    </label>
    <input
      type="text"
      value={value.moveTo || ''}
      onChange={(e) => onChange({ ...value, moveTo: e.target.value })}
      placeholder="Enter destination folder path"
      className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
    />
  </div>
)

interface DeleteOptionsValue {
  deleteForever?: boolean;
}

const DeleteOptions: React.FC<{ value: DeleteOptionsValue; onChange: (value: DeleteOptionsValue) => void }> = ({ value, onChange }) => (
  <div>
    <label className="touch-target gap-2">
      <input
        type="checkbox"
        checked={value.deleteForever ?? false}
        onChange={(e) => onChange({ ...value, deleteForever: e.target.checked })}
        className="w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500"
      />
      <span className="text-sm text-gray-700 dark:text-gray-300">
        Delete permanently (cannot be recovered)
      </span>
    </label>
  </div>
)

export default BulkOperations