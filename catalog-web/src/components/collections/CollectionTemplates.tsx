import React, { useState, useMemo } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Search,
  TrendingUp,
  Folder,
  FileText,
  Grid,
  List,
  Calendar,
  ChevronRight,
  Zap,
  Film,
  Tv,
  Headphones,
  Camera,
  Users,
  Sparkles,
  X
} from 'lucide-react';
import { Button } from '../ui/Button';
import { Input } from '../ui/Input';
import { Select } from '../ui/Select';
import { Badge } from '../ui/Badge';

import { toast } from 'react-hot-toast';

interface CollectionTemplate {
  id: string;
  name: string;
  description: string;
  category: 'media' | 'workflow' | 'organization' | 'automation';
  icon: React.ReactNode;
  rules?: Array<Record<string, unknown>>;
  settings?: Record<string, unknown>;
  metrics?: {
    complexity: 'simple' | 'medium' | 'advanced';
    lastUpdated?: string;
  };
  tags: string[];
  preview?: {
    type: string;
  };
}

// Smart collections now work on the server, but the server's rule language is closed (title, genre, media_type,
// year, rating, size, tags, path, added, ...). The templates below use fields it does not have (category,
// ai_score, trending_score, ...) and values it rejects (30_days_ago, audio), so the server would answer 400.
// Applying stays disabled with a reason until the templates are rewritten in the supported language.
const TEMPLATE_APPLY_AVAILABLE = false;
const TEMPLATE_APPLY_REASON = 'Creating from a template is not available: the templates use rule fields the smart rule language does not have yet. Build a smart collection with the Smart Collections builder instead. Nothing was created.';

interface CollectionTemplatesProps {
  onClose: () => void;
  onApplyTemplate: (template: CollectionTemplate, collectionName: string) => Promise<void>;
}

const CATEGORIES = [
  { id: 'media', name: 'Media Collections', icon: <Folder className="w-4 h-4" />, color: 'blue' },
  { id: 'workflow', name: 'Workflow Templates', icon: <Zap className="w-4 h-4" />, color: 'purple' },
  { id: 'organization', name: 'Organization', icon: <Grid className="w-4 h-4" />, color: 'green' },
  { id: 'automation', name: 'Automation', icon: <Sparkles className="w-4 h-4" />, color: 'orange' }
];

const COMPLEXITY_COLORS = {
  simple: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
  medium: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
  advanced: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
};

const COLLECTION_TEMPLATES: CollectionTemplate[] = [
  // Media Collections
  {
    id: 'recent-movies',
    name: 'Recent Movies',
    description: 'Movies added in the last 30 days with ratings above 4.0',
    category: 'media',
    icon: <Film className="w-6 h-6" />,
    rules: [
      { field: 'media_type', operator: 'equals', value: 'video' },
      { field: 'category', operator: 'equals', value: 'movie' },
      { field: 'created_at', operator: 'greater_than', value: '30_days_ago', timeBased: true },
      { field: 'rating', operator: 'greater_than', value: 4.0 }
    ],
    metrics: {
      complexity: 'simple'
    },
    tags: ['movies', 'recent', 'high-rated'],
    preview: {
      type: 'Video'
    }
  },
  {
    id: 'music-genres',
    name: 'Music by Genres',
    description: 'Organized collection of music sorted by genres and sub-genres',
    category: 'media',
    icon: <Headphones className="w-6 h-6" />,
    rules: [
      { field: 'media_type', operator: 'equals', value: 'audio' },
      { field: 'genre', operator: 'is_not_empty', value: '' }
    ],
    settings: {
      groupBy: 'genre',
      sortBy: 'artist',
      includeAlbums: true,
      includePlaylists: false
    },
    metrics: {
      complexity: 'medium'
    },
    tags: ['music', 'genres', 'organized'],
    preview: {
      type: 'Audio'
    }
  },
  {
    id: 'photo-library',
    name: 'Photo Library',
    description: 'Complete photo collection with date and location organization',
    category: 'media',
    icon: <Camera className="w-6 h-6" />,
    rules: [
      { field: 'media_type', operator: 'equals', value: 'image' },
      { field: 'format', operator: 'in', value: ['jpg', 'png', 'raw', 'heic'] }
    ],
    settings: {
      groupBy: 'date_taken',
      includeLocation: true,
      includeAlbums: true,
      faceDetection: true
    },
    metrics: {
      complexity: 'advanced'
    },
    tags: ['photos', 'images', 'library'],
    preview: {
      type: 'Images'
    }
  },
  // Workflow Templates
  {
    id: 'watchlist',
    name: 'Watchlist Manager',
    description: 'Dynamic watchlist with progress tracking and recommendations',
    category: 'workflow',
    icon: <Tv className="w-6 h-6" />,
    rules: [
      { field: 'watchlist_status', operator: 'equals', value: 'to_watch' },
      { field: 'media_type', operator: 'in', value: ['video'] }
    ],
    settings: {
      autoRemoveWatched: true,
      includeRecommendations: true,
      sortBy: 'priority',
      progressTracking: true
    },
    metrics: {
      complexity: 'medium'
    },
    tags: ['watchlist', 'tracking', 'workflow'],
    preview: {
      type: 'Mixed'
    }
  },
  {
    id: 'content-review',
    name: 'Content Review Queue',
    description: 'Items pending review with quality checks and metadata validation',
    category: 'workflow',
    icon: <FileText className="w-6 h-6" />,
    rules: [
      { field: 'review_status', operator: 'equals', value: 'pending' },
      { field: 'quality_score', operator: 'less_than', value: 0.8 }
    ],
    settings: {
      includeQualityCheck: true,
      includeMetadataValidation: true,
      autoCategorization: false,
      notificationEnabled: true
    },
    metrics: {
      complexity: 'advanced'
    },
    tags: ['review', 'quality', 'workflow'],
    preview: {
      type: 'Mixed'
    }
  },
  // Organization Templates
  {
    id: 'by-decade',
    name: 'Decades Collection',
    description: 'Media organized by decades for nostalgic browsing',
    category: 'organization',
    icon: <Calendar className="w-6 h-6" />,
    rules: [
      { field: 'media_type', operator: 'in', value: ['video', 'audio', 'image'] }
    ],
    settings: {
      groupBy: 'decade',
      includeUnknownDates: false,
      createSubCollections: true
    },
    metrics: {
      complexity: 'simple'
    },
    tags: ['organization', 'timeline', 'decades'],
    preview: {
      type: 'Mixed'
    }
  },
  {
    id: 'workspace-projects',
    name: 'Workspace Projects',
    description: 'Professional projects and work-related media',
    category: 'organization',
    icon: <Users className="w-6 h-6" />,
    rules: [
      { field: 'category', operator: 'equals', value: 'work' },
      { field: 'project_type', operator: 'is_not_empty', value: '' }
    ],
    settings: {
      groupBy: 'project',
      includeArchived: false,
      includeCollaborative: true
    },
    metrics: {
      complexity: 'medium'
    },
    tags: ['work', 'projects', 'organization'],
    preview: {
      type: 'Documents'
    }
  },
  // Automation Templates
  {
    id: 'smart-suggestions',
    name: 'Smart Suggestions',
    description: 'AI-powered collection with suggested items based on preferences',
    category: 'automation',
    icon: <Sparkles className="w-6 h-6" />,
    rules: [
      { field: 'ai_score', operator: 'greater_than', value: 0.7 },
      { field: 'user_preference_match', operator: 'greater_than', value: 0.8 }
    ],
    settings: {
      updateFrequency: 'daily',
      includeFeedback: true,
      learningEnabled: true,
      maxItems: 100
    },
    metrics: {
      complexity: 'advanced'
    },
    tags: ['ai', 'suggestions', 'automation'],
    preview: {
      type: 'Mixed'
    }
  },
  {
    id: 'trending-content',
    name: 'Trending Now',
    description: 'Currently popular and trending content in your library',
    category: 'automation',
    icon: <TrendingUp className="w-6 h-6" />,
    rules: [
      { field: 'recent_plays', operator: 'greater_than', value: 10 },
      { field: 'trending_score', operator: 'greater_than', value: 0.6 }
    ],
    settings: {
      updateFrequency: 'hourly',
      timeWindow: '7_days',
      includeSocialSignals: true
    },
    metrics: {
      complexity: 'medium'
    },
    tags: ['trending', 'popular', 'automation'],
    preview: {
      type: 'Mixed'
    }
  }
];

export const CollectionTemplates: React.FC<CollectionTemplatesProps> = ({
  onClose,
  onApplyTemplate
}) => {
  const [selectedCategory, setSelectedCategory] = useState('all');
  const [searchQuery, setSearchQuery] = useState('');
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid');
  const [selectedTemplate, setSelectedTemplate] = useState<CollectionTemplate | null>(null);
  const [collectionName, setCollectionName] = useState('');
  const [isCreating, setIsCreating] = useState(false);
  const [sortBy, setSortBy] = useState<'name' | 'complexity' | 'updated'>('name');

  const filteredTemplates = useMemo(() => {
    const filtered = COLLECTION_TEMPLATES.filter(template => {
      const matchesCategory = selectedCategory === 'all' || template.category === selectedCategory;
      const matchesSearch = !searchQuery || 
        template.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        template.description.toLowerCase().includes(searchQuery.toLowerCase()) ||
        template.tags.some(tag => tag.toLowerCase().includes(searchQuery.toLowerCase()));
      
      return matchesCategory && matchesSearch;
    });

    // Sort templates
    filtered.sort((a, b) => {
      switch (sortBy) {
        case 'name':
          return a.name.localeCompare(b.name);
        case 'complexity': {
          const complexityOrder = { simple: 1, medium: 2, advanced: 3 };
          return complexityOrder[a.metrics?.complexity || 'simple'] - complexityOrder[b.metrics?.complexity || 'simple'];
        }
        case 'updated':
          return (b.metrics?.lastUpdated || '').localeCompare(a.metrics?.lastUpdated || '');
        default:
          return 0;
      }
    });

    return filtered;
  }, [selectedCategory, searchQuery, sortBy]);

  const handleApplyTemplate = async (template: CollectionTemplate) => {
    if (!collectionName.trim()) {
      toast.error('Please enter a collection name');
      return;
    }

    setIsCreating(true);
    try {
      await onApplyTemplate(template, collectionName.trim());
      toast.success(`Collection "${collectionName}" created from template`);
      handleClose();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to create collection');
    } finally {
      setIsCreating(false);
    }
  };

  const handleClose = () => {
    setSelectedTemplate(null);
    setCollectionName('');
    onClose();
  };

  const getComplexityLabel = (complexity: string) => {
    return complexity.charAt(0).toUpperCase() + complexity.slice(1);
  };

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4"
    >
      <motion.div
        initial={{ opacity: 0, scale: 0.95 }}
        animate={{ opacity: 1, scale: 1 }}
        exit={{ opacity: 0, scale: 0.95 }}
        className="bg-white dark:bg-gray-900 rounded-xl shadow-2xl w-full max-w-7xl max-h-[90vh] overflow-hidden"
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-700">
          <div>
            <h2 className="text-2xl font-bold text-gray-900 dark:text-white">Collection Templates</h2>
            <p className="text-gray-600 dark:text-gray-400 mt-1">Choose from pre-built templates to get started</p>
          </div>
          <Button
            variant="ghost"
            size="sm"
            onClick={handleClose}
          >
            <X className="w-4 h-4" />
          </Button>
        </div>

        {/* Controls */}
        <div className="p-6 border-b border-gray-200 dark:border-gray-700">
          <div className="flex flex-wrap items-center gap-4">
            <div className="flex-1 min-w-[200px]">
              <Input
                placeholder="Search templates..."
                value={searchQuery}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setSearchQuery(e.target.value)}
                icon={<Search className="w-4 h-4" />}
              />
            </div>
            
            <Select
              aria-label="Filter by category"
              value={selectedCategory}
              onChange={setSelectedCategory}
              options={[
                { value: 'all', label: 'All Categories' },
                ...CATEGORIES.map(cat => ({ value: cat.id, label: cat.name }))
              ]}
            />
            
            <Select
              aria-label="Sort templates"
              value={sortBy}
              onChange={(value) => setSortBy(value as 'name' | 'complexity' | 'updated')}
              options={[
                { value: 'name', label: 'Name' },
                { value: 'complexity', label: 'Complexity' },
                { value: 'updated', label: 'Recently Updated' }
              ]}
            />
            
            <div className="flex items-center gap-2">
              <Button
                variant={viewMode === 'grid' ? 'default' : 'ghost'}
                size="sm"
                onClick={() => setViewMode('grid')}
              >
                <Grid className="w-4 h-4" />
              </Button>
              <Button
                variant={viewMode === 'list' ? 'default' : 'ghost'}
                size="sm"
                onClick={() => setViewMode('list')}
              >
                <List className="w-4 h-4" />
              </Button>
            </div>
          </div>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-6">
          <div className="mb-4">
            <div className="flex items-center gap-2">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
                {selectedCategory === 'all' ? 'All Templates' : CATEGORIES.find(c => c.id === selectedCategory)?.name}
              </h3>
              <Badge variant="outline">{filteredTemplates.length} templates</Badge>
            </div>
          </div>

          {viewMode === 'grid' ? (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
              {filteredTemplates.map((template) => (
                <motion.button
                  type="button"
                  key={template.id}
                  whileHover={{ y: -2 }}
                  className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4 cursor-pointer hover:shadow-lg transition-shadow text-left"
                  onClick={() => setSelectedTemplate(template)}
                >
                  <div className="flex items-start justify-between mb-3">
                    <div className="flex items-center gap-2">
                      <div className={`p-2 rounded-lg bg-${CATEGORIES.find(c => c.id === template.category)?.color}-100 dark:bg-${CATEGORIES.find(c => c.id === template.category)?.color}-900`}>
                        {template.icon}
                      </div>
                      <div className="flex-1">
                        <h4 className="font-semibold text-gray-900 dark:text-white text-sm">{template.name}</h4>
                        <p className="text-xs text-gray-500 dark:text-gray-400">
                          {CATEGORIES.find(c => c.id === template.category)?.name}
                        </p>
                      </div>
                    </div>
                  </div>
                  
                  <p className="text-sm text-gray-600 dark:text-gray-300 mb-3 line-clamp-2">
                    {template.description}
                  </p>
                  
                  <div className="flex items-center justify-between mb-3">
                    <Badge className={COMPLEXITY_COLORS[template.metrics?.complexity || 'simple']}>
                      {getComplexityLabel(template.metrics?.complexity || 'simple')}
                    </Badge>
                  </div>
                  
                  {template.preview && (
                    <div className="text-xs text-gray-600 dark:text-gray-400">
                      <div className="flex items-center justify-between">
                        <span>{template.preview.type}</span>
                      </div>
                    </div>
                  )}
                </motion.button>
              ))}
            </div>
          ) : (
            <div className="space-y-2">
              {filteredTemplates.map((template) => (
                <button type="button"
                  key={template.id}
                  className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4 cursor-pointer hover:shadow-lg transition-shadow w-full text-left"
                  onClick={() => setSelectedTemplate(template)}
                >
                  <div className="flex items-center gap-4">
                    <div className={`p-3 rounded-lg bg-${CATEGORIES.find(c => c.id === template.category)?.color}-100 dark:bg-${CATEGORIES.find(c => c.id === template.category)?.color}-900`}>
                      {template.icon}
                    </div>
                    <div className="flex-1">
                      <div className="flex items-center justify-between mb-1">
                        <h4 className="font-semibold text-gray-900 dark:text-white">{template.name}</h4>
                        <div className="flex items-center gap-2">
                          <Badge className={COMPLEXITY_COLORS[template.metrics?.complexity || 'simple']}>
                            {getComplexityLabel(template.metrics?.complexity || 'simple')}
                          </Badge>
                        </div>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-300">{template.description}</p>
                      <div className="flex items-center gap-4 mt-2 text-xs text-gray-500 dark:text-gray-400">
                        {template.metrics?.lastUpdated && <span>Updated {template.metrics.lastUpdated}</span>}
                      </div>
                    </div>
                    <ChevronRight className="w-5 h-5 text-gray-400" />
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Preview Modal */}
        <AnimatePresence>
          {selectedTemplate && (
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4"
              onClick={() => setSelectedTemplate(null)}
            >
              <motion.div
                initial={{ opacity: 0, scale: 0.95 }}
                animate={{ opacity: 1, scale: 1 }}
                exit={{ opacity: 0, scale: 0.95 }}
                className="bg-white dark:bg-gray-900 rounded-xl max-w-2xl w-full max-h-[80vh] overflow-y-auto"
                onClick={(e) => e.stopPropagation()}
              >
                <div className="p-6">
                  <div className="flex items-center justify-between mb-6">
                    <div className="flex items-center gap-3">
                      <div className={`p-3 rounded-lg bg-${CATEGORIES.find(c => c.id === selectedTemplate.category)?.color}-100 dark:bg-${CATEGORIES.find(c => c.id === selectedTemplate.category)?.color}-900`}>
                        {selectedTemplate.icon}
                      </div>
                      <div>
                        <h3 className="text-xl font-bold text-gray-900 dark:text-white">{selectedTemplate.name}</h3>
                        <p className="text-gray-600 dark:text-gray-400">
                          {CATEGORIES.find(c => c.id === selectedTemplate.category)?.name}
                        </p>
                      </div>
                    </div>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setSelectedTemplate(null)}
                    >
                      <X className="w-4 h-4" />
                    </Button>
                  </div>

                  <div className="space-y-6">
                    <div>
                      <h4 className="font-semibold text-gray-900 dark:text-white mb-2">Description</h4>
                      <p className="text-gray-600 dark:text-gray-300">{selectedTemplate.description}</p>
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <h4 className="font-semibold text-gray-900 dark:text-white mb-2">Metrics</h4>
                        <div className="space-y-2">
                          <div className="flex items-center justify-between">
                            <span className="text-sm text-gray-600 dark:text-gray-400">Complexity</span>
                            <Badge className={COMPLEXITY_COLORS[selectedTemplate.metrics?.complexity || 'simple']}>
                              {getComplexityLabel(selectedTemplate.metrics?.complexity || 'simple')}
                            </Badge>
                          </div>
                        </div>
                      </div>

                      <div>
                        <h4 className="font-semibold text-gray-900 dark:text-white mb-2">Tags</h4>
                        <div className="flex flex-wrap gap-2">
                          {selectedTemplate.tags.map((tag) => (
                            <Badge key={tag} variant="outline">{tag}</Badge>
                          ))}
                        </div>
                      </div>
                    </div>

                    {selectedTemplate.preview && (
                      <div>
                        <h4 className="font-semibold text-gray-900 dark:text-white mb-2">Preview</h4>
                        <div className="bg-gray-50 dark:bg-gray-800 rounded-lg p-4">
                          <div className="grid grid-cols-1 gap-4 text-center">
                            <div>
                              <div className="text-2xl font-bold text-gray-900 dark:text-white">{selectedTemplate.preview.type}</div>
                              <div className="text-sm text-gray-600 dark:text-gray-400">Type</div>
                            </div>
                          </div>
                        </div>
                      </div>
                    )}

                    {selectedTemplate.rules && selectedTemplate.rules.length > 0 && (
                      <div>
                        <h4 className="font-semibold text-gray-900 dark:text-white mb-2">Rules</h4>
                        <div className="bg-gray-50 dark:bg-gray-800 rounded-lg p-4 space-y-2">
                          {selectedTemplate.rules.map((rule, index) => {
                            const r = rule as Record<string, unknown>;
                            const timeBasedText = r.timeBased ? <span className="ml-2 text-xs text-blue-600">(Time-based)</span> : null;
                            return (
                              <div key={index} className="text-sm text-gray-600 dark:text-gray-300">
                                {String(r.field)} {String(r.operator)} {String(r.value)}
                                {timeBasedText}
                              </div>
                            );
                          })}
                        </div>
                      </div>
                    )}

                    {selectedTemplate.settings && (
                      <div>
                        <h4 className="font-semibold text-gray-900 dark:text-white mb-2">Settings</h4>
                        <div className="bg-gray-50 dark:bg-gray-800 rounded-lg p-4">
                          <div className="grid grid-cols-2 gap-4">
                            {Object.entries(selectedTemplate.settings).map(([key, value]) => (
                              <div key={key} className="text-sm">
                                <span className="text-gray-600 dark:text-gray-400">{key}: </span>
                                <span className="font-medium text-gray-900 dark:text-white">
                                  {typeof value === 'boolean' ? (value ? 'Yes' : 'No') : String(value)}
                                </span>
                              </div>
                            ))}
                          </div>
                        </div>
                      </div>
                    )}
                  </div>

                  <div className="flex items-center gap-4 mt-6">
                    <div className="flex-1">
                      <Input
                        placeholder="Enter collection name..."
                        value={collectionName}
                        onChange={(e) => setCollectionName(e.target.value)}
                      />
                    </div>
                    <Button
                      variant="outline"
                      onClick={() => setSelectedTemplate(null)}
                    >
                      Cancel
                    </Button>
                    <Button
                      onClick={() => handleApplyTemplate(selectedTemplate)}
                      disabled={!TEMPLATE_APPLY_AVAILABLE || !collectionName.trim() || isCreating}
                      title={TEMPLATE_APPLY_AVAILABLE ? undefined : TEMPLATE_APPLY_REASON}
                    >
                      {isCreating ? 'Creating...' : 'Create Collection'}
                    </Button>
                    {!TEMPLATE_APPLY_AVAILABLE && (
                      <p data-testid="template-apply-unavailable" className="text-sm text-gray-500">{TEMPLATE_APPLY_REASON}</p>
                    )}
                  </div>
                </div>
              </motion.div>
            </motion.div>
          )}
        </AnimatePresence>
      </motion.div>
    </motion.div>
  );
};

export default CollectionTemplates;