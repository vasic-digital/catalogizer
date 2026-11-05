import React, { useState, useCallback, useRef, useMemo } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
  Download,
  Upload,
  FileText,
  FileSpreadsheet,
  Music,
  Archive,
  Settings,
  X,
  FileJson,
  File,
  RefreshCw
} from 'lucide-react'
import { Button } from '../ui/Button'
import { Input } from '../ui/Input'
import { Select } from '../ui/Select'
import { Switch } from '../ui/Switch'
import { Card } from '../ui/Card'
import { SmartCollection } from '../../types/collections'
import { useCollection } from '../../hooks/useCollections'
import { toast } from 'react-hot-toast'

interface CollectionExportProps {
  collection: SmartCollection
  onClose: () => void
}

interface ExportOptions {
  format: 'json' | 'csv' | 'm3u' | 'xspf' | 'wpl' | 'zip'
  includeMetadata: boolean
  includeThumbnails: boolean
  includeFiles: boolean
  compression: 'none' | 'zip' | 'tar' | '7z'
  quality: 'original' | 'high' | 'medium' | 'low'
  maxFileSize: number | null
  fileTypes: string[]
  dateRange: {
    start: string
    end: string
  } | null
  customFields: string[]
}

interface ImportOptions {
  format: 'json' | 'csv' | 'm3u' | 'zip'
  mergeStrategy: 'replace' | 'merge' | 'append'
  duplicateHandling: 'skip' | 'replace' | 'rename' | 'merge'
  preserveIds: boolean
  validateData: boolean
  createBackup: boolean
  mapping: Record<string, string>
}

interface ExportProgress {
  stage: string
  progress: number
  total: number
  currentFile?: string
  estimatedTime?: number
}

interface ImportPreview {
  totalItems: number
  validItems: number
  invalidItems: number
  duplicates: number
  newItems: number
  sampleItems: Array<{
    title?: string;
    name?: string;
    id?: string;
    path?: string;
    type?: string;
  }>
  conflicts: Array<{
    type: string;
    description: string;
  }>
}

interface CollectionItem {
  id: string;
  title?: string;
  name?: string;
  media_type?: string;
  path?: string;
  size?: number;
  duration?: number;
  rating?: number;
  metadata?: Record<string, unknown>;
  thumbnail?: string;
}

const EXPORT_FORMATS = [
  {
    value: 'json',
    label: 'JSON',
    description: 'Structured data format with full metadata',
    icon: FileJson,
    extensions: ['.json'],
    supportsMetadata: true,
    supportsFiles: true
  },
  {
    value: 'csv',
    label: 'CSV',
    description: 'Spreadsheet format for basic data',
    icon: FileSpreadsheet,
    extensions: ['.csv'],
    supportsMetadata: true,
    supportsFiles: false
  },
  {
    value: 'm3u',
    label: 'M3U Playlist',
    description: 'Standard playlist format for media players',
    icon: Music,
    extensions: ['.m3u', '.m3u8'],
    supportsMetadata: false,
    supportsFiles: false
  },
  {
    value: 'xspf',
    label: 'XSPF Playlist',
    description: 'XML Shareable Playlist Format',
    icon: FileText,
    extensions: ['.xspf'],
    supportsMetadata: true,
    supportsFiles: false
  },
  {
    value: 'wpl',
    label: 'WPL Playlist',
    description: 'Windows Media Player playlist',
    icon: File,
    extensions: ['.wpl'],
    supportsMetadata: false,
    supportsFiles: false
  },
  {
    value: 'zip',
    label: 'ZIP Archive',
    description: 'Complete collection with files',
    icon: Archive,
    extensions: ['.zip'],
    supportsMetadata: true,
    supportsFiles: true
  }
]

const COMPRESSION_OPTIONS = [
  { value: 'none', label: 'No Compression', description: 'Original file size' },
  { value: 'zip', label: 'ZIP', description: 'Standard compression' },
  { value: 'tar', label: 'TAR', description: 'Unix archive format' },
  { value: '7z', label: '7-Zip', description: 'Best compression ratio' }
]

const QUALITY_OPTIONS = [
  { value: 'original', label: 'Original Quality', description: 'No quality loss' },
  { value: 'high', label: 'High Quality', description: 'Slight compression for better size' },
  { value: 'medium', label: 'Medium Quality', description: 'Balanced quality and size' },
  { value: 'low', label: 'Low Quality', description: 'Maximum compression' }
]

const FILE_TYPES = [
  { value: 'audio', label: 'Audio Files', extensions: ['.mp3', '.wav', '.flac', '.m4a'] },
  { value: 'video', label: 'Video Files', extensions: ['.mp4', '.avi', '.mkv', '.mov'] },
  { value: 'image', label: 'Image Files', extensions: ['.jpg', '.png', '.gif', '.webp'] },
  { value: 'document', label: 'Documents', extensions: ['.pdf', '.doc', '.txt'] }
]

export const CollectionExport: React.FC<CollectionExportProps> = ({
  collection,
  onClose
}) => {
  const [activeTab, setActiveTab] = useState<'export' | 'import'>('export')
  const [exportOptions, setExportOptions] = useState<ExportOptions>({
    format: 'json',
    includeMetadata: true,
    includeThumbnails: false,
    includeFiles: false,
    compression: 'none',
    quality: 'original',
    maxFileSize: null,
    fileTypes: [],
    dateRange: null,
    customFields: []
  })
  const [importOptions, setImportOptions] = useState<ImportOptions>({
    format: 'json',
    mergeStrategy: 'merge',
    duplicateHandling: 'skip',
    preserveIds: false,
    validateData: true,
    createBackup: true,
    mapping: {}
  })
  const [isExporting, setIsExporting] = useState(false)
  const [isImporting, setIsImporting] = useState(false)
  const [exportProgress, setExportProgress] = useState<ExportProgress | null>(null)
  const [importPreview, setImportPreview] = useState<ImportPreview | null>(null)
  const [importFile, setImportFile] = useState<File | null>(null)
  const [showAdvanced, setShowAdvanced] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  
  const { collectionItems, isLoading } = useCollection(collection?.id || '')

  // Use collection items from props if available
  const items = useMemo(() => collectionItems?.items || [], [collectionItems])

  const selectedFormat = EXPORT_FORMATS.find(f => f.value === exportOptions.format)

  const handleExport = useCallback(async () => {
    setIsExporting(true)
    setExportProgress({
      stage: 'Generating export',
      progress: 0,
      total: items.length
    })

    try {
      // Generate actual export data
      const exportData = {
        collection: {
          id: collection.id,
          name: collection.name,
          description: collection.description,
          created_at: collection.created_at,
          updated_at: collection.updated_at
        },
        items: items.map((item: CollectionItem) => ({
          id: item.id,
          title: item.title || item.name,
          type: item.media_type,
          path: item.path,
          size: item.size,
          duration: item.duration,
          rating: item.rating,
          metadata: exportOptions.includeMetadata ? item.metadata : undefined,
          thumbnail: exportOptions.includeThumbnails ? item.thumbnail : undefined
        })),
        exported_at: new Date().toISOString(),
        export_options: exportOptions
      }

      // Create and download file
      let filename = `${collection.name.replace(/[^a-z0-9]/gi, '_')}_${Date.now()}`
      let mimeType = 'application/json'
      let content: string | Blob = JSON.stringify(exportData, null, 2)

      if (exportOptions.format === 'csv') {
        // Simple CSV generation
        const headers = ['id', 'title', 'type', 'path', 'size', 'duration', 'rating']
        const csvContent = [
          headers.join(','),
          ...items.map((item: CollectionItem) => [
            item.id,
            `"${item.title || item.name}"`,
            item.media_type,
            `"${item.path}"`,
            item.size || 0,
            item.duration || 0,
            item.rating || 0
          ].join(','))
        ].join('\n')
        
        content = csvContent
        mimeType = 'text/csv'
        filename += '.csv'
      } else if (exportOptions.format === 'm3u') {
        // M3U playlist generation
        const m3uContent = [
          '#EXTM3U',
          ...items.map((item: CollectionItem) => `#EXTINF:${item.duration || 0},${item.title || item.name}\n${item.path}`)
        ].join('\n')
        
        content = m3uContent
        mimeType = 'audio/x-mpegurl'
        filename += '.m3u'
      } else {
        filename += '.json'
      }

      const blob = new Blob([content], { type: mimeType })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = filename
      a.click()
      URL.revokeObjectURL(url)

      toast.success('Collection exported successfully')
    } catch (error) {
      toast.error('Export failed')
    } finally {
      setIsExporting(false)
      setExportProgress(null)
    }
  }, [collection, items, exportOptions])

  const handleImportFile = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (file) {
      setImportFile(file)
      
      // Generate import preview
      const reader = new FileReader()
      reader.onload = (e) => {
        try {
          const content = e.target?.result as string
          let parsedContent: { items?: Array<Record<string, string>> } | null = null
          
          if (file.name.endsWith('.json')) {
            parsedContent = JSON.parse(content) as { items?: Array<Record<string, string>> }
          } else if (file.name.endsWith('.csv')) {
            // Simple CSV parsing
            const lines = content.split('\n')
            const headers = lines[0].split(',')
            parsedContent = {
              items: lines.slice(1).map(line => {
                const values = line.split(',')
                return headers.reduce<Record<string, string>>((obj, header, index) => {
                  obj[header.trim()] = values[index]?.replace(/"/g, '') || ''
                  return obj
                }, {})
              }).filter(item => item.title)
            }
          }
          
          const preview: ImportPreview = {
            totalItems: parsedContent?.items?.length || 0,
            validItems: parsedContent?.items?.length || 0,
            invalidItems: 0,
            duplicates: 0,
            newItems: parsedContent?.items?.length || 0,
            sampleItems: (parsedContent?.items || []).slice(0, 5),
            conflicts: []
          }
          
          setImportPreview(preview)
        } catch (error) {
          toast.error('Failed to parse import file')
          setImportPreview(null)
        }
      }
      
      reader.readAsText(file)
    }
  }, [])

  const handleImport = useCallback(async () => {
    if (!importFile || !importPreview) return
    
    // The server has no collection import endpoint. The earlier version waited two
    // seconds and reported a successful import without importing anything (CAT-001).
    toast.error('Import is not available: the server has no collection import endpoint. Nothing was imported.')
  }, [importFile, importPreview])

  const formatFileSize = (bytes: number): string => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
  }

  const FormatCard = ({ format, isSelected, onClick }: {
    format: typeof EXPORT_FORMATS[0]
    isSelected: boolean
    onClick: () => void
  }) => (
    <motion.button
      whileHover={{ scale: 1.02 }}
      whileTap={{ scale: 0.98 }}
      onClick={onClick}
      className={`p-4 rounded-lg border-2 text-left transition-colors ${
        isSelected
          ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
          : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'
      }`}
    >
      <div className="flex items-start gap-3">
        <div className={`p-2 rounded-lg ${
          isSelected ? 'bg-blue-100 dark:bg-blue-900/40' : 'bg-gray-100 dark:bg-gray-800'
        }`}>
          <format.icon className={`w-5 h-5 ${
            isSelected ? 'text-blue-600 dark:text-blue-400' : 'text-gray-600 dark:text-gray-400'
          }`} />
        </div>
        <div className="flex-1">
          <h4 className="font-medium text-gray-900 dark:text-white">{format.label}</h4>
          <p className="text-sm text-gray-600 dark:text-gray-400 mt-1">{format.description}</p>
          <div className="flex flex-wrap gap-1 mt-2">
            {format.extensions.map(ext => (
              <span key={ext} className="px-2 py-1 bg-gray-100 dark:bg-gray-800 text-xs rounded text-gray-600 dark:text-gray-400">
                {ext}
              </span>
            ))}
          </div>
        </div>
      </div>
    </motion.button>
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
            <h2 className="text-2xl font-bold text-gray-900 dark:text-white">Export / Import Collection</h2>
            <p className="text-gray-600 dark:text-gray-400">{collection.name}</p>
          </div>
          <Button
            variant="ghost"
            size="sm"
            onClick={onClose}
          >
            <X className="w-4 h-4" />
          </Button>
        </div>

        {/* Tab Navigation */}
        <div className="border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center px-6">
            <button
              onClick={() => setActiveTab('export')}
              className={`flex items-center gap-2 px-4 py-3 border-b-2 transition-colors ${
                activeTab === 'export'
                  ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                  : 'border-transparent text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white'
              }`}
            >
              <Download className="w-4 h-4" />
              Export
            </button>
            <button
              onClick={() => setActiveTab('import')}
              className={`flex items-center gap-2 px-4 py-3 border-b-2 transition-colors ${
                activeTab === 'import'
                  ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                  : 'border-transparent text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white'
              }`}
            >
              <Upload className="w-4 h-4" />
              Import
            </button>
          </div>
        </div>

        {/* Content */}
        <div className="overflow-y-auto p-6 max-h-[calc(90vh-140px)]">
          <AnimatePresence mode="wait">
            {activeTab === 'export' && (
              <motion.div
                key="export"
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -20 }}
                className="space-y-6"
              >
                {/* Format Selection */}
                <Card className="p-6">
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Export Format</h3>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    {EXPORT_FORMATS.map((format) => (
                      <FormatCard
                        key={format.value}
                        format={format}
                        isSelected={exportOptions.format === format.value}
                        onClick={() => setExportOptions(prev => ({ ...prev, format: format.value as ExportOptions['format'] }))}
                      />
                    ))}
                  </div>
                </Card>

                {/* Export Options */}
                <Card className="p-6">
                  <div className="flex items-center justify-between mb-4">
                    <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Export Options</h3>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setShowAdvanced(!showAdvanced)}
                      className="flex items-center gap-2"
                    >
                      <Settings className="w-4 h-4" />
                      {showAdvanced ? 'Hide' : 'Show'} Advanced
                    </Button>
                  </div>
                  
                  <div className="space-y-4">
                    {/* Basic Options */}
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <div className="flex items-center justify-between">
                        <div>
                          <div className="font-medium text-gray-900 dark:text-white">Include Metadata</div>
                          <div className="text-sm text-gray-600 dark:text-gray-400">Export tags, ratings, and custom fields</div>
                        </div>
                        <Switch
                          checked={exportOptions.includeMetadata}
                          onCheckedChange={(checked) => setExportOptions(prev => ({ ...prev, includeMetadata: checked }))}
                          disabled={!selectedFormat?.supportsMetadata}
                        />
                      </div>
                      
                      <div className="flex items-center justify-between">
                        <div>
                          <div className="font-medium text-gray-900 dark:text-white">Include Thumbnails</div>
                          <div className="text-sm text-gray-600 dark:text-gray-400">Export thumbnail images</div>
                        </div>
                        <Switch
                          checked={exportOptions.includeThumbnails}
                          onCheckedChange={(checked) => setExportOptions(prev => ({ ...prev, includeThumbnails: checked }))}
                        />
                      </div>
                      
                      <div className="flex items-center justify-between">
                        <div>
                          <div className="font-medium text-gray-900 dark:text-white">Include Files</div>
                          <div className="text-sm text-gray-600 dark:text-gray-400">Export actual media files</div>
                        </div>
                        <Switch
                          checked={exportOptions.includeFiles}
                          onCheckedChange={(checked) => setExportOptions(prev => ({ ...prev, includeFiles: checked }))}
                          disabled={!selectedFormat?.supportsFiles}
                        />
                      </div>
                      
                      <div className="flex items-center justify-between">
                        <div>
                          <div className="font-medium text-gray-900 dark:text-white">Create Backup</div>
                          <div className="text-sm text-gray-600 dark:text-gray-400">Keep original data intact</div>
                        </div>
                        <Switch
                          checked={true}
                          disabled
                        />
                      </div>
                    </div>

                    {/* Advanced Options */}
                    <AnimatePresence>
                      {showAdvanced && (
                        <motion.div
                          initial={{ opacity: 0, height: 0 }}
                          animate={{ opacity: 1, height: 'auto' }}
                          exit={{ opacity: 0, height: 0 }}
                          className="space-y-4 border-t border-gray-200 dark:border-gray-700 pt-4"
                        >
                          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            <div>
                              <label htmlFor="coll-export-select-1" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                Compression
                              </label>
                              <Select id="coll-export-select-1"
                                value={exportOptions.compression}
                                onChange={(value) => setExportOptions(prev => ({ ...prev, compression: value as ExportOptions['compression'] }))}
                                options={COMPRESSION_OPTIONS}
                                className="w-full"
                              />
                            </div>
                            
                            <div>
                              <label htmlFor="coll-export-select-2" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                Quality
                              </label>
                              <Select id="coll-export-select-2"
                                value={exportOptions.quality}
                                onChange={(value) => setExportOptions(prev => ({ ...prev, quality: value as ExportOptions['quality'] }))}
                                options={QUALITY_OPTIONS}
                                className="w-full"
                              />
                            </div>
                            
                            <div>
                              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                Max File Size (MB)
                              </label>
                              <Input
                                type="number"
                                placeholder="No limit"
                                value={exportOptions.maxFileSize || ''}
                                onChange={(e) => setExportOptions(prev => ({ 
                                  ...prev, 
                                  maxFileSize: e.target.value ? parseInt(e.target.value) : null 
                                }))}
                                min="1"
                              />
                            </div>
                            
                            <div>
                              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                File Types
                              </label>
                              <div className="space-y-2">
                                {FILE_TYPES.map((type) => (
                                  <label key={type.value} className="flex items-center gap-2">
                                    <input
                                      type="checkbox"
                                      checked={exportOptions.fileTypes.includes(type.value)}
                                      onChange={(e) => {
                                        if (e.target.checked) {
                                          setExportOptions(prev => ({ 
                                            ...prev, 
                                            fileTypes: [...prev.fileTypes, type.value] 
                                          }))
                                        } else {
                                          setExportOptions(prev => ({ 
                                            ...prev, 
                                            fileTypes: prev.fileTypes.filter(t => t !== type.value) 
                                          }))
                                        }
                                      }}
                                      className="rounded border-gray-300 dark:border-gray-600"
                                    />
                                    <span className="text-sm text-gray-700 dark:text-gray-300">{type.label}</span>
                                  </label>
                                ))}
                              </div>
                            </div>
                          </div>
                        </motion.div>
                      )}
                    </AnimatePresence>

                    {/* Collection Summary */}
                    <div className="bg-gray-50 dark:bg-gray-800 rounded-lg p-4">
                      <div className="flex items-center justify-between">
                        <div>
                          <div className="font-medium text-gray-900 dark:text-white">Collection Summary</div>
                          <div className="text-sm text-gray-600 dark:text-gray-400">
                            {items.length} items • {formatFileSize(items.reduce((sum: number, item: CollectionItem) => sum + (item.size || 0), 0))}
                          </div>
                        </div>
                        <div className="text-right">
                          <div className="font-medium text-gray-900 dark:text-white">Est. Export Size</div>
                          <div className="text-sm text-gray-600 dark:text-gray-400">
                            ~{formatFileSize(items.reduce((sum: number, item: CollectionItem) => sum + (item.size || 0), 0))}
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </Card>

                {/* Export Progress */}
                {exportProgress && (
                  <Card className="p-6">
                    <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Export Progress</h3>
                    <div className="space-y-4">
                      <div className="flex items-center justify-between">
                        <span className="text-sm text-gray-600 dark:text-gray-400">{exportProgress.stage}</span>
                        <span className="text-sm text-gray-600 dark:text-gray-400">
                          {exportProgress.progress} / {exportProgress.total}
                        </span>
                      </div>
                      <div className="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2">
                        <motion.div
                          className="bg-blue-500 h-2 rounded-full"
                          initial={{ width: 0 }}
                          animate={{ width: `${(exportProgress.progress / exportProgress.total) * 100}%` }}
                          transition={{ duration: 0.3 }}
                        />
                      </div>
                      {exportProgress.currentFile && (
                        <div className="text-sm text-gray-600 dark:text-gray-400">
                          Processing: {exportProgress.currentFile}
                        </div>
                      )}
                    </div>
                  </Card>
                )}

                {/* Export Button */}
                <Button
                  onClick={handleExport}
                  disabled={isExporting || isLoading}
                  className="w-full"
                >
                  {isExporting ? (
                    <>
                      <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                      Exporting...
                    </>
                  ) : (
                    <>
                      <Download className="w-4 h-4 mr-2" />
                      Export Collection
                    </>
                  )}
                </Button>
              </motion.div>
            )}

            {activeTab === 'import' && (
              <motion.div
                key="import"
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -20 }}
                className="space-y-6"
              >
                {/* File Upload */}
                <Card className="p-6">
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Import File</h3>
                  
                  <div className="space-y-4">
                    <div
                      onClick={() => fileInputRef.current?.click()}
                      className="border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg p-8 text-center cursor-pointer hover:border-blue-500 transition-colors"
                    >
                      <Upload className="w-12 h-12 text-gray-400 mx-auto mb-4" />
                      <div className="text-gray-900 dark:text-white font-medium mb-2">
                        Click to upload or drag and drop
                      </div>
                      <div className="text-sm text-gray-600 dark:text-gray-400">
                        JSON, CSV, M3U, or ZIP files
                      </div>
                      <input
                        ref={fileInputRef}
                        type="file"
                        accept=".json,.csv,.m3u,.m3u8,.zip"
                        onChange={handleImportFile}
                        className="hidden"
                      />
                    </div>
                    
                    {importFile && (
                      <div className="bg-gray-50 dark:bg-gray-800 rounded-lg p-4">
                        <div className="flex items-center justify-between">
                          <div className="flex items-center gap-3">
                            <FileText className="w-5 h-5 text-gray-600 dark:text-gray-400" />
                            <div>
                              <div className="font-medium text-gray-900 dark:text-white">{importFile.name}</div>
                              <div className="text-sm text-gray-600 dark:text-gray-400">
                                {formatFileSize(importFile.size)}
                              </div>
                            </div>
                          </div>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => {
                              setImportFile(null)
                              setImportPreview(null)
                              if (fileInputRef.current) {
                                fileInputRef.current.value = ''
                              }
                            }}
                          >
                            <X className="w-4 h-4" />
                          </Button>
                        </div>
                      </div>
                    )}
                  </div>
                </Card>

                {/* Import Options */}
                {importFile && (
                  <Card className="p-6">
                    <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Import Options</h3>
                    
                    <div className="space-y-4">
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div>
                          <label htmlFor="coll-export-select-3" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                            Merge Strategy
                          </label>
                          <Select id="coll-export-select-3"
                            value={importOptions.mergeStrategy}
                            onChange={(value) => setImportOptions(prev => ({ ...prev, mergeStrategy: value as ImportOptions['mergeStrategy'] }))}
                            options={[
                              { value: 'replace', label: 'Replace Collection' },
                              { value: 'merge', label: 'Merge with Existing' },
                              { value: 'append', label: 'Append to Collection' }
                            ]}
                            className="w-full"
                          />
                        </div>
                        
                        <div>
                          <label htmlFor="coll-export-select-4" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                            Duplicate Handling
                          </label>
                          <Select id="coll-export-select-4"
                            value={importOptions.duplicateHandling}
                            onChange={(value) => setImportOptions(prev => ({ ...prev, duplicateHandling: value as ImportOptions['duplicateHandling'] }))}
                            options={[
                              { value: 'skip', label: 'Skip Duplicates' },
                              { value: 'replace', label: 'Replace Existing' },
                              { value: 'rename', label: 'Rename Imports' },
                              { value: 'merge', label: 'Merge Data' }
                            ]}
                            className="w-full"
                          />
                        </div>
                      </div>
                      
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div className="flex items-center justify-between">
                          <div>
                            <div className="font-medium text-gray-900 dark:text-white">Preserve IDs</div>
                            <div className="text-sm text-gray-600 dark:text-gray-400">Keep original item IDs</div>
                          </div>
                          <Switch
                            checked={importOptions.preserveIds}
                            onCheckedChange={(checked) => setImportOptions(prev => ({ ...prev, preserveIds: checked }))}
                          />
                        </div>
                        
                        <div className="flex items-center justify-between">
                          <div>
                            <div className="font-medium text-gray-900 dark:text-white">Validate Data</div>
                            <div className="text-sm text-gray-600 dark:text-gray-400">Check for errors</div>
                          </div>
                          <Switch
                            checked={importOptions.validateData}
                            onCheckedChange={(checked: boolean) => setImportOptions(prev => ({ ...prev, validateData: checked }))}
                          />
                        </div>
                      </div>
                    </div>
                  </Card>
                )}

                {/* Import Preview */}
                {importPreview && (
                  <Card className="p-6">
                    <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Import Preview</h3>
                    
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
                      <div className="text-center">
                        <div className="text-2xl font-bold text-blue-600 dark:text-blue-400">
                          {importPreview.totalItems}
                        </div>
                        <div className="text-sm text-gray-600 dark:text-gray-400">Total Items</div>
                      </div>
                      <div className="text-center">
                        <div className="text-2xl font-bold text-green-600 dark:text-green-400">
                          {importPreview.newItems}
                        </div>
                        <div className="text-sm text-gray-600 dark:text-gray-400">New Items</div>
                      </div>
                      <div className="text-center">
                        <div className="text-2xl font-bold text-orange-600 dark:text-orange-400">
                          {importPreview.duplicates}
                        </div>
                        <div className="text-sm text-gray-600 dark:text-gray-400">Duplicates</div>
                      </div>
                      <div className="text-center">
                        <div className="text-2xl font-bold text-red-600 dark:text-red-400">
                          {importPreview.invalidItems}
                        </div>
                        <div className="text-sm text-gray-600 dark:text-gray-400">Invalid Items</div>
                      </div>
                    </div>
                    
                    {importPreview.sampleItems.length > 0 && (
                      <div>
                        <h4 className="font-medium text-gray-900 dark:text-white mb-3">Sample Items</h4>
                        <div className="space-y-2">
                          {importPreview.sampleItems.map((item, index) => (
                            <div key={index} className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-800 rounded-lg">
                              <div className="flex-1">
                                <div className="font-medium text-gray-900 dark:text-white">
                                  {item.title || item.name || 'Unknown'}
                                </div>
                                <div className="text-sm text-gray-600 dark:text-gray-400">
                                  {item.path || 'No path'}
                                </div>
                              </div>
                              <div className="text-sm text-gray-500 dark:text-gray-400">
                                {item.type || 'Unknown type'}
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}
                  </Card>
                )}

                {/* Import Button */}
                {importFile && importPreview && (
                  <Button
                    onClick={handleImport}
                    disabled={isImporting}
                    className="w-full"
                  >
                    {isImporting ? (
                      <>
                        <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                        Importing...
                      </>
                    ) : (
                      <>
                        <Upload className="w-4 h-4 mr-2" />
                        Import Collection
                      </>
                    )}
                  </Button>
                )}
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      </motion.div>
    </motion.div>
  )
}