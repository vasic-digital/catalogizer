import { filterCollections } from '../lib/collectionFilter';
import React, { useState, useCallback } from 'react';
import { motion } from 'framer-motion';
import {
  Plus,
  Search,
  Grid,
  List,
  Clock,
  BarChart3,
  Share,
  Share2,
  Trash2,
  Copy,
  Download,
  Eye,
  Settings,
  CheckSquare,
  X,
  Users,
  Bot,
  FileText,
  Zap,
  Database,
  Activity,
  Loader2,
  Brain,
  Sparkles
} from 'lucide-react';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { Select } from '../components/ui/Select';
import { Tabs } from '../components/ui/Tabs';
import { SmartCollectionBuilder } from '../components/collections/SmartCollectionBuilder';
import { CollectionPreview } from '../components/collections/CollectionPreview';
import { BulkOperations } from '../components/collections/BulkOperations';
import { PerformanceOptimizer } from '../components/collections/PerformanceOptimizer';
import { CollectionSettings } from '../components/collections/CollectionSettings';
import { CollectionAnalytics } from '../components/collections/CollectionAnalytics';
import { CollectionSharing } from '../components/collections/CollectionSharing';
import { CollectionExport } from '../components/collections/CollectionExport';
import { CollectionRealTime } from '../components/collections/CollectionRealTime';
import { 
  preloadComponent,
  CollectionTemplates,
  AdvancedSearch,
  CollectionAutomation,
  ExternalIntegrations,
} from '../components/performance/LazyComponents';

import { usePagination } from '../components/performance/MemoCache';

import { 
  AICollectionSuggestions,
  AINaturalSearch,
  AIContentCategorizer
} from '../components/ai/AIComponents';
import { 
  AIUserBehaviorAnalytics,
  AIPredictions,
  AISmartOrganization
} from '../components/ai/AIAnalytics';
import { 
  AIMetadataExtractor,
  AIAutomationRules,
  AIContentQualityAnalyzer,
  type ExtractedMetadata
} from '../components/ai/AIMetadata';
import { useCollections } from '../hooks/useCollections';
import { SmartCollection, CollectionRule, ShareCollectionRequest } from '../types/collections';
import { toast } from 'react-hot-toast';

const COLLECTIONS_TABS = [
  { id: 'all', label: 'All Collections' },
  { id: 'smart', label: 'Smart Collections' },
  { id: 'manual', label: 'Manual Collections' },
  { id: 'favorites', label: 'Favorites' },
  { id: 'templates', label: 'Templates' },
  { id: 'automation', label: 'Automation' },
  { id: 'integrations', label: 'Integrations' },
  { id: 'ai', label: 'AI Features' },
];

const MEDIA_TYPE_OPTIONS = [
  { value: 'all', label: 'All Media' },
  { value: 'music', label: 'Music' },
  { value: 'video', label: 'Video' },
  { value: 'image', label: 'Images' },
  { value: 'document', label: 'Documents' }
];

const VIEW_OPTIONS = [
  { value: 'grid', label: 'Grid View', icon: Grid },
  { value: 'list', label: 'List View', icon: List },
];

const SORT_OPTIONS = [
  { value: 'name', label: 'Name' },
  { value: 'created_at', label: 'Date Created' },
  { value: 'updated_at', label: 'Date Updated' },
  { value: 'item_count', label: 'Item Count' },
];

export const Collections: React.FC = () => {
  const [activeTab, setActiveTab] = useState('all');
  const [searchQuery, setSearchQuery] = useState('');
  const [filterMediaType, setFilterMediaType] = useState('all');
  const [sortBy, setSortBy] = useState('name');
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid');
  const [showSmartBuilder, setShowSmartBuilder] = useState(false);
  const [selectedCollection, setSelectedCollection] = useState<SmartCollection | null>(null);
  const [previewCollection, setPreviewCollection] = useState<SmartCollection | null>(null);
  const [selectedCollections, setSelectedCollections] = useState<string[]>([]);
  const [showBulkOperations, setShowBulkOperations] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [showAnalytics, setShowAnalytics] = useState(false);
  const [showSharing, setShowSharing] = useState(false);
  const [showExport, setShowExport] = useState(false);
  const [showRealTime, setShowRealTime] = useState(false);
  const [showTemplates, setShowTemplates] = useState(false);
  const [showAdvancedSearch, setShowAdvancedSearch] = useState(false);
  const [showAutomation, setShowAutomation] = useState(false);
  const [showIntegrations, setShowIntegrations] = useState(false);
  const [showAI, setShowAI] = useState(false);
  const [selectAll, setSelectAll] = useState(false);

  // Performance metrics state
  const [performanceMetrics, setPerformanceMetrics] = useState({
    bundleSize: 0,
    renderTime: 0,
    memoryUsage: 0,
    cacheHitRate: 0
  });

  // Performance monitoring
  const measurePerformance = useCallback((_name: string) => {
    const startTime = performance.now();
    return () => {
      const endTime = performance.now();
      const duration = endTime - startTime;
      
      setPerformanceMetrics(prev => ({
        ...prev,
        renderTime: duration
      }));
      
      if (duration > 100) {
        // Slow render detected - duration exceeds 100ms threshold
      }
    };
  }, []);

  const {
    collections,
    isLoading,
    error: _error,
    refetchCollections: _refetchCollections,
    createCollection,
    updateCollection,
    deleteCollection,
    shareCollection,
    duplicateCollection,
    exportCollection: _exportCollection,
    bulkDeleteCollections,
    bulkShareCollections,
    bulkExportCollections,
    bulkUpdateCollections,
    isSharing,
    isDuplicating,
    isExporting: _isExporting,
  } = useCollections();

  // Debounced search for performance
  const [debouncedSearch, setDebouncedSearch] = useState(searchQuery);
  const [isDebouncing, setIsDebouncing] = useState(false);

  React.useEffect(() => {
    setIsDebouncing(true);
    const timer = setTimeout(() => {
      setDebouncedSearch(searchQuery);
      setIsDebouncing(false);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchQuery]);

  // The generic useOptimizedData() filter matches filter KEYS against item
  // FIELDS; activeTab / filterMediaType are not collection fields, so with
  // real data it filtered every collection out. Filter on the real semantics.
  const filteredCollections = React.useMemo(
    () => filterCollections(collections, { tab: activeTab, mediaType: filterMediaType, search: debouncedSearch, sortBy }),
    [collections, activeTab, filterMediaType, debouncedSearch, sortBy],
  );

  // Pagination for large collections
  const {
    page: currentPage,
    paginatedData: paginatedCollections,
    totalPages,
    nextPage,
    prevPage,
    goToPage
  } = usePagination(filteredCollections, 20);

  // Update search query with debouncing
  const handleSearchChange = useCallback((value: string) => {
    setSearchQuery(value);
  }, []);

  // Performance monitoring effect
  React.useEffect(() => {
    const endPerformance = measurePerformance('Collections Page');
    
    return () => {
      endPerformance();
    };
  }, [activeTab, filteredCollections.length, measurePerformance]);

  // Preload components based on active tab
  React.useEffect(() => {
    switch (activeTab) {
      case 'templates':
        preloadComponent('CollectionTemplates');
        break;
      case 'automation':
        preloadComponent('CollectionAutomation');
        break;
      case 'integrations':
        preloadComponent('ExternalIntegrations');
        break;
      default:
        break;
    }
  }, [activeTab]);

  // Selection handlers
  const handleSelectCollection = useCallback((collectionId: string, selected: boolean) => {
    setSelectedCollections(prev => {
      if (selected) {
        return [...prev, collectionId];
      } else {
        return prev.filter(id => id !== collectionId);
      }
    });
  }, []);

  const handleSelectAll = useCallback(() => {
    if (selectAll) {
      setSelectedCollections([]);
    } else {
      setSelectedCollections(filteredCollections.map(c => c.id));
    }
    setSelectAll(!selectAll);
  }, [selectAll, filteredCollections]);

  const handleClearSelection = () => {
    setSelectedCollections([]);
    setSelectAll(false);
  };

  const handlePreviewCollection = (collection: SmartCollection) => {
    setPreviewCollection(collection);
  };

  const handleClosePreview = () => {
    setPreviewCollection(null);
  };

  const handleOpenSettings = (collection: SmartCollection) => {
    setSelectedCollection(collection);
    setShowSettings(true);
  };

  const handleCreateSmartCollection = () => {
    setShowSmartBuilder(true);
  };

  const handleSaveSmartCollection = async (name: string, description: string, rules: CollectionRule[]) => {
    try {
      await createCollection({
        collection: {
          name,
          description,
          is_public: false,
          is_smart: true,
          smart_rules: rules,
        }
      });
      setShowSmartBuilder(false);
    } catch (error) {
      console.error('Failed to create smart collection:', error);
      // Error already handled by mutation's onError callback
    }
  };

  const handleShareCollection = async (collection: SmartCollection) => {
    try {
      await shareCollection({
        id: collection.id,
        shareRequest: {
          can_view: true,
          can_comment: false,
          can_download: false,
        }
      });
    } catch (error) {
      console.error('Failed to share collection:', error);
      // Error already handled by mutation's onError callback
    }
  };

  const handleDuplicateCollection = async (collection: SmartCollection) => {
    try {
      await duplicateCollection({
        id: collection.id,
        newName: `${collection.name} (Copy)`
      });
    } catch (error) {
      console.error('Failed to duplicate collection:', error);
      // Error already handled by mutation's onError callback
    }
  };

  const handleDeleteCollection = async (collection: SmartCollection) => {
    if (window.confirm(`Are you sure you want to delete "${collection.name}"? This action cannot be undone.`)) {
      try {
        await deleteCollection({
          id: collection.id
        });
        toast.success('Collection deleted successfully');
      } catch (error) {
        console.error('Failed to delete collection:', error);
        toast.error('Failed to delete collection');
      }
    }
  };

  const handleBulkOperation = async (operation: string, options?: unknown) => {
    try {
      switch (operation) {
        case 'delete':
          await bulkDeleteCollections({ collectionIds: selectedCollections });
          toast.success(`${selectedCollections.length} collections deleted`);
          break;
        case 'share':
          await bulkShareCollections({ 
            collectionIds: selectedCollections,
            shareRequest: ((options as unknown) as ShareCollectionRequest | undefined) || { can_view: true, can_comment: false, can_download: false }
          });
          toast.success(`${selectedCollections.length} collections shared`);
          break;
        case 'export':
          await bulkExportCollections({ 
            collectionIds: selectedCollections,
            format: ((options as { format?: string } | undefined)?.format as 'json' | 'csv' | 'm3u') || 'json'
          });
          toast.success(`${selectedCollections.length} collections exported`);
          break;
        case 'duplicate':
          await bulkUpdateCollections({ 
            collectionIds: selectedCollections,
            action: 'duplicate'
          });
          toast.success(`${selectedCollections.length} collections duplicated`);
          break;
        default:
          toast.error('Unknown operation');
      }
      handleClearSelection();
    } catch (error) {
      console.error('Bulk operation failed:', error);
      toast.error('Bulk operation failed');
    }
  };

  const handleShowAnalytics = (collection: SmartCollection) => {
    setSelectedCollection(collection);
    setShowAnalytics(true);
  };

  const handleShowSharing = (collection: SmartCollection) => {
    setSelectedCollection(collection);
    setShowSharing(true);
  };

  const handleShowExport = (collection: SmartCollection) => {
    setSelectedCollection(collection);
    setShowExport(true);
  };

  const handleShowRealTime = (collection: SmartCollection) => {
    setSelectedCollection(collection);
    setShowRealTime(true);
  };

  const handleShowTemplates = () => {
    setShowTemplates(true);
  };

  const handleShowAdvancedSearch = () => {
    setShowAdvancedSearch(true);
  };

  const handleShowAutomation = () => {
    setShowAutomation(true);
  };

  const handleShowIntegrations = () => {
    setShowIntegrations(true);
  };

  const handleShowAI = () => {
    setShowAI(true);
  };

  // AI Feature Handlers
  const handleAISuggestionAccept = (suggestion: { title: string }) => {
    toast.success(`AI suggestion applied: ${suggestion.title}`);
  };

  const handleAINaturalSearch = (query: { query: string }) => {
    toast.success(`AI search executed: ${query.query}`);
  };

  const handleAICategorizationComplete = (result: { category: string }) => {
    toast.success(`Content categorized as: ${result.category}`);
  };

  const handleAIActionImplement = (action: string) => {
    toast.success(`AI action implemented: ${action}`);
  };

  const handleAIPredictionAction = (predictionId: string, actionId: string) => {
    toast.success(`AI prediction action executed: ${actionId}`);
  };

  const handleAIOrganizationApply = (suggestionId: string) => {
    toast.success(`AI organization suggestion applied: ${suggestionId}`);
  };

  const handleAIMetadataExtracted = (metadata: ExtractedMetadata) => {
    toast.success(`AI metadata extraction completed for: ${metadata.title}`);
  };

  const handleAIRuleToggle = (ruleId: string, enabled: boolean) => {
    toast.success(`AI rule ${enabled ? 'enabled' : 'disabled'}: ${ruleId}`);
  };

  const handleAIRuleExecute = (ruleId: string) => {
    toast.success(`AI rule executed: ${ruleId}`);
  };

  const handleAIQualityImprovement = (improvement: string) => {
    toast.success(`AI quality improvement applied: ${improvement}`);
  };

  const renderCollectionCard = (collection: SmartCollection) => {
    const isSelected = selectedCollections.includes(collection.id);
    
    return (
      <motion.div
        key={collection.id}
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        whileHover={{ scale: 1.02 }}
        className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-4 cursor-pointer hover:shadow-md transition-shadow relative"
      >
        {/* Selection Checkbox */}
        <div className="absolute top-2 left-2 z-10">
          <button
            onClick={(e) => {
              e.stopPropagation();
              handleSelectCollection(collection.id, !isSelected);
            }}
            className={`w-6 h-6 rounded-md border-2 flex items-center justify-center transition-colors ${
              isSelected
                ? 'bg-blue-600 border-blue-600 text-white'
                : 'border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 hover:border-blue-400'
            }`}
          >
            {isSelected && <CheckSquare className="w-4 h-4" />}
          </button>
        </div>

        <div className="flex items-start justify-between mb-3">
          <div className="flex-1 ml-8">
            <h3 className="font-semibold text-gray-900 dark:text-white mb-1">
              {collection.name}
            </h3>
            {collection.description && (
              <p className="text-sm text-gray-600 dark:text-gray-400 line-clamp-2">
                {collection.description}
              </p>
            )}
          </div>
          
          <div className="flex items-center gap-1">
            {collection.is_smart && (
              <div className="w-6 h-6 bg-purple-100 dark:bg-purple-900 rounded-full flex items-center justify-center">
                <Clock className="w-3 h-3 text-purple-600 dark:text-purple-400" />
              </div>
            )}
          </div>
        </div>

        <div className="flex items-center justify-between text-sm text-gray-500 dark:text-gray-400">
          <span>{collection.item_count} items</span>
          <span>{new Date(collection.created_at).toLocaleDateString()}</span>
        </div>

        <div className="mt-3 pt-3 border-t border-gray-200 dark:border-gray-700 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              size="sm"
              onClick={(e) => {
                e.stopPropagation();
                handlePreviewCollection(collection);
              }}
              title="Preview collection"
            >
              <Eye className="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={(e) => {
                e.stopPropagation();
                handleShareCollection(collection);
              }}
              disabled={isSharing}
              title="Share collection"
            >
              <Share className="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={(e) => {
                e.stopPropagation();
                handleDuplicateCollection(collection);
              }}
              disabled={isDuplicating}
              title="Duplicate collection"
            >
              <Copy className="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={(e) => {
                e.stopPropagation();
                handleOpenSettings(collection);
              }}
              title="Collection settings"
            >
              <Settings className="w-4 h-4" />
            </Button>
          </div>
          
          <Button
            variant="ghost"
            size="sm"
            onClick={(e) => {
              e.stopPropagation();
              handleDeleteCollection(collection);
            }}
            className="text-red-600 hover:text-red-700"
            title="Delete collection"
          >
            <Trash2 className="w-4 h-4" />
          </Button>
        </div>
      </motion.div>
    );
  };

  const renderCollectionListItem = (collection: SmartCollection) => {
    const isSelected = selectedCollections.includes(collection.id);
    
    return (
      <motion.div
        key={collection.id}
        initial={{ opacity: 0, x: -20 }}
        animate={{ opacity: 1, x: 0 }}
        className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-4 hover:shadow-md transition-shadow"
      >
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4 flex-1">
            {/* Selection Checkbox */}
            <button
              onClick={() => handleSelectCollection(collection.id, !isSelected)}
              className={`w-6 h-6 rounded-md border-2 flex items-center justify-center transition-colors ${
                isSelected
                  ? 'bg-blue-600 border-blue-600 text-white'
                  : 'border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 hover:border-blue-400'
              }`}
            >
              {isSelected && <CheckSquare className="w-4 h-4" />}
            </button>
            
            <div className="w-12 h-12 bg-gradient-to-br from-purple-500 to-pink-600 rounded-lg flex items-center justify-center">
              {collection.is_smart ? (
                <Clock className="w-6 h-6 text-white" />
              ) : (
                <Grid className="w-6 h-6 text-white" />
              )}
            </div>
            
            <div className="flex-1">
              <h3 className="font-semibold text-gray-900 dark:text-white mb-1">
                {collection.name}
              </h3>
              {collection.description && (
                <p className="text-sm text-gray-600 dark:text-gray-400 line-clamp-1">
                  {collection.description}
                </p>
              )}
            </div>
            
            <div className="text-right">
              <div className="text-lg font-bold text-gray-900 dark:text-white">
                {collection.item_count.toLocaleString()}
              </div>
              <div className="text-xs text-gray-500 dark:text-gray-400">items</div>
            </div>
          </div>

          <div className="flex items-center gap-2 ml-4">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => handlePreviewCollection(collection)}
              title="Preview collection"
            >
              <Eye className="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => handleShowAnalytics(collection)}
              title="View analytics"
            >
              <BarChart3 className="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => handleShowSharing(collection)}
              title="Share collection"
            >
              <Share2 className="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => handleShowExport(collection)}
              title="Export collection"
            >
              <Download className="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => handleShowRealTime(collection)}
              title="Real-time collaboration"
            >
              <Users className="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => handleOpenSettings(collection)}
              title="Collection settings"
            >
              <Settings className="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => handleDeleteCollection(collection)}
              className="text-red-600 hover:text-red-700"
              title="Delete collection"
            >
              <Trash2 className="w-4 h-4" />
            </Button>
          </div>
        </div>
      </motion.div>
    );
  };

  if (showSmartBuilder) {
    return (
      <div className="max-w-4xl mx-auto">
        <SmartCollectionBuilder
          onSave={handleSaveSmartCollection}
          onCancel={() => setShowSmartBuilder(false)}
          className="mb-6"
        />
      </div>
    );
  }

  if (showTemplates) {
    return (
      <div className="max-w-7xl mx-auto">
        <CollectionTemplates 
          onClose={() => setShowTemplates(false)}
          onApplyTemplate={async (template, collectionName) => {
            // Implementation for applying template
            toast.success(`Template "${template.name}" applied to "${collectionName}"`);
            setShowTemplates(false);
          }}
        />
      </div>
    );
  }

  if (showAdvancedSearch) {
    return (
      <div className="max-w-7xl mx-auto">
        <AdvancedSearch />
      </div>
    );
  }

  if (showAutomation) {
    return (
      <div className="max-w-7xl mx-auto">
        <CollectionAutomation />
      </div>
    );
  }

  if (showIntegrations) {
    return (
      <div className="max-w-7xl mx-auto">
        <ExternalIntegrations />
      </div>
    );
  }

  if (showAI) {
    return (
      <div className="max-w-7xl mx-auto">
        <div className="mb-8">
          <div className="flex items-center space-x-3">
            <Brain className="w-8 h-8 text-indigo-600" />
            <h1 className="text-3xl font-bold text-gray-900 dark:text-white">AI-Powered Features</h1>
            <Sparkles className="w-6 h-6 text-yellow-500" />
          </div>
          <p className="text-gray-600 dark:text-gray-400 mt-2">
            Leverage artificial intelligence for intelligent content management
          </p>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
          <AICollectionSuggestions
            onSuggestionAccept={handleAISuggestionAccept}
            maxSuggestions={3}
          />
          <AINaturalSearch
            onSearch={handleAINaturalSearch}
            placeholder="Search naturally... try 'show me action movies' or 'find productivity tools'"
          />
        </div>

        <div className="mb-8">
          <AIContentCategorizer
            item={{
              title: 'Sample Content for AI Categorization',
              description: 'This is a sample content item to demonstrate AI-powered categorization capabilities'
            }}
            onCategorizationComplete={handleAICategorizationComplete}
          />
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
          <AIUserBehaviorAnalytics
            userId="demo-user"
            onActionImplement={handleAIActionImplement}
          />
          <AIPredictions
            onPredictionAction={handleAIPredictionAction}
          />
        </div>

        <div className="mb-8">
          <AISmartOrganization
            collections={collections}
            onSuggestionApply={handleAIOrganizationApply}
          />
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
          <AIMetadataExtractor
            content={{
              title: 'Sample Content',
              description: 'Sample description for AI metadata extraction',
              fileType: 'video/mp4',
              size: 1024000
            }}
            onMetadataExtracted={handleAIMetadataExtracted}
          />
          <AIContentQualityAnalyzer
            content={{
              title: 'Sample Content',
              description: 'Sample description for AI quality analysis'
            }}
            onQualityImprovement={handleAIQualityImprovement}
          />
        </div>

        <div>
          <AIAutomationRules
            onRuleToggle={handleAIRuleToggle}
            onRuleExecute={handleAIRuleExecute}
          />
        </div>

        <div className="mt-8 text-center">
          <button
            onClick={() => setShowAI(false)}
            className="px-6 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 transition-colors"
          >
            Back to Collections
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto">
      {/* Header */}
      <div className="mb-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold text-gray-900 dark:text-white mb-2">
              Collections
            </h1>
            <p className="text-gray-600 dark:text-gray-400">
              Organize your media with smart and manual collections
            </p>
          </div>
          
          {/* Performance Indicator */}
          <div className="flex items-center gap-4 text-sm text-gray-500 dark:text-gray-400">
            <div className="flex items-center gap-2">
              <Activity className="w-4 h-4" />
              <span>Render: {performanceMetrics.renderTime.toFixed(1)}ms</span>
            </div>
            <div className="flex items-center gap-2">
              <Database className="w-4 h-4" />
              <span>Items: {filteredCollections.length}</span>
            </div>
            <div className="flex items-center gap-2">
              <Zap className="w-4 h-4" />
              <span>Page {currentPage}/{totalPages}</span>
            </div>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <Tabs
        tabs={COLLECTIONS_TABS}
        activeTab={activeTab}
        onChangeTab={setActiveTab}
        className="mb-6"
      />

      {/* Controls */}
      <div className="mb-6 flex flex-col lg:flex-row gap-4 items-start lg:items-center justify-between">
        <div className="flex flex-col sm:flex-row gap-4 flex-1">
          {/* Search */}
          <div className="relative max-w-md">
            <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-400" />
            <Input
              placeholder="Search collections..."
              value={searchQuery}
              onChange={(e) => handleSearchChange(e.target.value)}
              className={`pl-10 ${isDebouncing ? 'border-blue-400' : ''}`}
            />
            {isDebouncing && (
              <div className="absolute right-3 top-1/2 transform -translate-y-1/2">
                <Loader2 className="w-4 h-4 text-blue-600 animate-spin" />
              </div>
            )}
          </div>

          {/* Media Type Filter */}
          <Select
            value={filterMediaType}
            onChange={setFilterMediaType}
            options={MEDIA_TYPE_OPTIONS}
            className="w-40"
          />

          {/* Sort */}
          <Select
            value={sortBy}
            onChange={setSortBy}
            options={SORT_OPTIONS}
            className="w-40"
          />
        </div>

        <div className="flex items-center gap-2">
          {/* View Mode Toggle */}
          <div className="flex items-center bg-gray-100 dark:bg-gray-800 rounded-lg p-1">
            {VIEW_OPTIONS.map((option) => {
              const IconComponent = option.icon;
              return (
                <button
                  key={option.value}
                  onClick={() => setViewMode(option.value as 'grid' | 'list')}
                  className={`p-2 rounded ${
                    viewMode === option.value
                      ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm'
                      : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white'
                  }`}
                  title={option.label}
                >
                  <IconComponent className="w-4 h-4" />
                </button>
              );
            })}
          </div>

          {/* Create Actions */}
          <Button
            onClick={handleCreateSmartCollection}
            className="flex items-center gap-2"
          >
            <Plus className="w-4 h-4" />
            Smart Collection
          </Button>
          <Button
            variant="outline"
            onClick={handleShowTemplates}
            className="flex items-center gap-2"
          >
            <FileText className="w-4 h-4" />
            Templates
          </Button>
          <Button
            variant="outline"
            onClick={handleShowAdvancedSearch}
            className="flex items-center gap-2"
          >
            <Search className="w-4 h-4" />
            Advanced Search
          </Button>
          <Button
            variant="outline"
            onClick={handleShowAutomation}
            className="flex items-center gap-2"
          >
            <Bot className="w-4 h-4" />
            Automation
          </Button>
          <Button
            variant="outline"
            onClick={handleShowIntegrations}
            className="flex items-center gap-2"
          >
            <Zap className="w-4 h-4" />
            Integrations
          </Button>
          <Button
            variant="outline"
            onClick={handleShowAI}
            className="flex items-center gap-2 bg-indigo-50 border-indigo-200 text-indigo-600 hover:bg-indigo-100"
          >
            <Brain className="w-4 h-4" />
            AI Features
          </Button>
          
          {/* Performance Tools (Development) */}
          {process.env.NODE_ENV === 'development' && (
            <Button
              variant="outline"
              onClick={() => {
                const analyzer = window.open('', '_blank', 'width=800,height=600');
                if (analyzer) {
                  analyzer.document.write('<html><head><title>Bundle Analyzer</title></head><body><div id="bundle-analyzer"></div><script src="/bundle-analyzer.js"></script></body></html>');
                }
              }}
              className="flex items-center gap-2 text-green-600 border-green-600 hover:bg-green-50"
            >
              <Activity className="w-4 h-4" />
              Bundle Analysis
            </Button>
          )}
        </div>
      </div>

      {/* Collections Display */}
      <div className="min-h-96">
        {/* Bulk Operations Bar */}
        {selectedCollections.length > 0 && (
          <div className="mb-4 p-3 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg flex items-center justify-between">
            <div className="flex items-center gap-3">
              <span className="text-sm font-medium text-blue-800 dark:text-blue-200">
                {selectedCollections.length} collection{selectedCollections.length > 1 ? 's' : ''} selected
              </span>
              <Button
                variant="ghost"
                size="sm"
                onClick={handleClearSelection}
                className="text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-200"
              >
                <X className="w-4 h-4" />
              </Button>
            </div>
            <Button
              onClick={() => setShowBulkOperations(true)}
              size="sm"
              className="bg-blue-600 hover:bg-blue-700 text-white"
            >
              Bulk Actions
            </Button>
          </div>
        )}

        {/* Selection Controls */}
        {filteredCollections.length > 0 && (
          <div className="mb-4 flex items-center gap-4">
            <label className="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400">
              <input
                type="checkbox"
                checked={selectAll}
                onChange={handleSelectAll}
                className="rounded border-gray-300 dark:border-gray-600"
              />
              Select all ({filteredCollections.length})
            </label>
          </div>
        )}

        {isLoading ? (
          <div className="flex items-center justify-center h-64">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
          </div>
        ) : filteredCollections.length === 0 ? (
          <div className="text-center py-12">
            <div className="w-16 h-16 bg-gray-100 dark:bg-gray-800 rounded-full flex items-center justify-center mx-auto mb-4">
              <Grid className="w-8 h-8 text-gray-400" />
            </div>
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">
              No collections found
            </h3>
            <p className="text-gray-600 dark:text-gray-400 mb-6">
              {searchQuery || filterMediaType !== 'all' || activeTab !== 'all'
                ? 'Try adjusting your search or filters'
                : 'Create your first collection to get started'
              }
            </p>
            {!searchQuery && filterMediaType === 'all' && activeTab === 'all' && (
              <Button
                onClick={handleCreateSmartCollection}
                className="flex items-center gap-2"
              >
                <Plus className="w-4 h-4" />
                Create Smart Collection
              </Button>
            )}
          </div>
        ) : (
          <>
            <PerformanceOptimizer
              itemCount={paginatedCollections.length}
              threshold={50}
              loadingStrategy="lazy"
              itemHeight={viewMode === 'grid' ? 200 : 80}
              containerHeight={600}
            >
              <div className={viewMode === 'grid' 
                ? 'grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4' 
                : 'space-y-4'
              }>
                {paginatedCollections.map(viewMode === 'grid' ? renderCollectionCard : renderCollectionListItem)}
              </div>
            </PerformanceOptimizer>

            {/* Pagination Controls */}
            {totalPages > 1 && (
              <div className="mt-6 flex items-center justify-between">
                <div className="text-sm text-gray-600 dark:text-gray-400">
                  Showing {((currentPage - 1) * 20) + 1} to {Math.min(currentPage * 20, filteredCollections.length)} of {filteredCollections.length} collections
                </div>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={prevPage}
                    disabled={currentPage === 1}
                  >
                    Previous
                  </Button>
                  <div className="flex items-center gap-1">
                    {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
                      let pageNum: number;
                      if (totalPages <= 5) {
                        pageNum = i + 1;
                      } else if (currentPage <= 3) {
                        pageNum = i + 1;
                      } else if (currentPage >= totalPages - 2) {
                        pageNum = totalPages - 4 + i;
                      } else {
                        pageNum = currentPage - 2 + i;
                      }
                      return (
                        <Button
                          key={pageNum}
                          variant={currentPage === pageNum ? "default" : "outline"}
                          size="sm"
                          onClick={() => goToPage(pageNum)}
                          className="min-w-[2.5rem]"
                        >
                          {pageNum}
                        </Button>
                      );
                    })}
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={nextPage}
                    disabled={currentPage === totalPages}
                  >
                    Next
                  </Button>
                </div>
              </div>
            )}
          </>
        )}
      </div>

      {/* Modals and Overlays */}
      {previewCollection && (
        <CollectionPreview
          collection={previewCollection}
          onClose={handleClosePreview}
        />
      )}

      {showBulkOperations && (
        <BulkOperations
          selectedCollections={selectedCollections}
          onOperation={handleBulkOperation}
          onClose={() => setShowBulkOperations(false)}
        />
      )}

      {showSettings && selectedCollection && (
        <CollectionSettings
          collection={selectedCollection}
          onClose={() => setShowSettings(false)}
          onSave={(settings) => {
            updateCollection({
              id: selectedCollection.id,
              updates: settings
            });
            setShowSettings(false);
          }}
        />
      )}

      {showAnalytics && selectedCollection && (
        <CollectionAnalytics
          collection={selectedCollection}
          onClose={() => setShowAnalytics(false)}
        />
      )}

      {showSharing && selectedCollection && (
        <CollectionSharing
          collection={selectedCollection}
          onClose={() => setShowSharing(false)}
        />
      )}

      {showExport && selectedCollection && (
        <CollectionExport
          collection={selectedCollection}
          onClose={() => setShowExport(false)}
        />
      )}

      {showRealTime && selectedCollection && (
        <CollectionRealTime
          collection={selectedCollection}
          onClose={() => setShowRealTime(false)}
        />
      )}
    </div>
  );
};