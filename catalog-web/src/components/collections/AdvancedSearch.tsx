import React, { useState } from 'react';
import {
  Search,
  Plus,
  Minus,
  Settings,
  Video,
  Image,
  Eye,
  Heart,
  Save,
  RotateCcw,
  Bookmark,
  Folder
} from 'lucide-react';
import { Button } from '../ui/Button';
import { Input } from '../ui/Input';
import { Select } from '../ui/Select';
import { Switch } from '../ui/Switch';
import { Badge } from '../ui/Badge';
import { toast } from 'react-hot-toast';
import { randomId } from '../../lib/randomId';
import { onActivateKey } from '../../lib/keyboard';

interface SearchRule {
  id: string;
  field: string;
  operator: string;
  value: unknown;
  enabled: boolean;
  condition?: 'and' | 'or';
}

interface SearchSettings {
  sortBy: string;
  sortOrder: 'asc' | 'desc';
  itemsPerPage: number;
  caseSensitive: boolean;
  includeMetadata: boolean;
  viewMode: 'grid' | 'list';
}

interface SavedSearch {
  id: string;
  name: string;
  description: string;
  rules: SearchRule[];
  settings: SearchSettings;
  created_at: string;
  updated_at: string;
}

interface SearchPreset {
  id: string;
  name: string;
  description: string;
  category: string;
  icon: React.ReactNode;
  rules: Partial<SearchRule>[];
  tags: string[];
}

const SEARCH_FIELDS = [
  { value: 'title', label: 'Title', type: 'text' },
  { value: 'description', label: 'Description', type: 'text' },
  { value: 'file_type', label: 'File Type', type: 'select', options: ['video', 'audio', 'image', 'document'] },
  { value: 'size', label: 'File Size', type: 'number' },
  { value: 'duration', label: 'Duration', type: 'number' },
  { value: 'created_at', label: 'Created Date', type: 'date' },
  { value: 'updated_at', label: 'Updated Date', type: 'date' },
  { value: 'rating', label: 'Rating', type: 'number' },
  { value: 'tags', label: 'Tags', type: 'text' },
  { value: 'year', label: 'Year', type: 'number' },
  { value: 'genre', label: 'Genre', type: 'text' },
  { value: 'resolution', label: 'Resolution', type: 'select', options: ['720p', '1080p', '4K', '8K'] },
  { value: 'codec', label: 'Codec', type: 'text' },
  { value: 'language', label: 'Language', type: 'text' },
  { value: 'watch_count', label: 'Watch Count', type: 'number' },
  { value: 'download_count', label: 'Download Count', type: 'number' },
  { value: 'is_favorite', label: 'Favorite', type: 'boolean' },
  { value: 'is_archived', label: 'Archived', type: 'boolean' }
];

const OPERATORS: Record<string, Array<{ value: string; label: string; requiresArray?: boolean }>> = {
  text: [
    { value: 'equals', label: 'Equals' },
    { value: 'not_equals', label: 'Not Equals' },
    { value: 'contains', label: 'Contains' },
    { value: 'not_contains', label: 'Does Not Contain' },
    { value: 'starts_with', label: 'Starts With' },
    { value: 'ends_with', label: 'Ends With' },
    { value: 'in', label: 'In', requiresArray: true },
    { value: 'not_in', label: 'Not In', requiresArray: true }
  ],
  number: [
    { value: 'equals', label: 'Equals' },
    { value: 'not_equals', label: 'Not Equals' },
    { value: 'greater_than', label: 'Greater Than' },
    { value: 'less_than', label: 'Less Than' },
    { value: 'between', label: 'Between', requiresArray: true },
    { value: 'not_between', label: 'Not Between', requiresArray: true }
  ],
  date: [
    { value: 'equals', label: 'Equals' },
    { value: 'not_equals', label: 'Not Equals' },
    { value: 'greater_than', label: 'After' },
    { value: 'less_than', label: 'Before' },
    { value: 'between', label: 'Between', requiresArray: true },
    { value: 'not_between', label: 'Not Between', requiresArray: true }
  ],
  boolean: [
    { value: 'equals', label: 'Is' },
    { value: 'not_equals', label: 'Is Not' }
  ],
  select: [
    { value: 'equals', label: 'Equals' },
    { value: 'not_equals', label: 'Not Equals' },
    { value: 'in', label: 'In', requiresArray: true },
    { value: 'not_in', label: 'Not In', requiresArray: true }
  ]
};

const SEARCH_PRESETS: SearchPreset[] = [
  {
    id: '1',
    name: 'HD Movies',
    description: 'Find high-definition movies',
    category: 'media',
    icon: <Video className="w-4 h-4" />,
    rules: [
      { field: 'file_type', operator: 'equals', value: 'video' },
      { field: 'resolution', operator: 'in', value: ['1080p', '4K'] }
    ],
    tags: ['video', 'quality']
  },
  {
    id: '2',
    name: 'Recent Photos',
    description: 'Photos from the last 30 days',
    category: 'media',
    icon: <Image className="w-4 h-4" />,
    rules: [
      { field: 'file_type', operator: 'equals', value: 'image' },
      { field: 'created_at', operator: 'greater_than', value: new Date(Date.now() - 30 * 24 * 60 * 60 * 1000) }
    ],
    tags: ['images', 'recent']
  },
  {
    id: '3',
    name: 'Large Files',
    description: 'Files larger than 1GB',
    category: 'media',
    icon: <Folder className="w-4 h-4" />,
    rules: [
      { field: 'size', operator: 'greater_than', value: 1073741824 }
    ],
    tags: ['size', 'storage']
  },
  {
    id: '4',
    name: 'Favorites',
    description: 'All marked as favorite',
    category: 'media',
    icon: <Heart className="w-4 h-4" />,
    rules: [
      { field: 'is_favorite', operator: 'equals', value: true }
    ],
    tags: ['favorites']
  },
  {
    id: '5',
    name: 'Unwatched Videos',
    description: 'Videos you haven\'t watched yet',
    category: 'media',
    icon: <Eye className="w-4 h-4" />,
    rules: [
      { field: 'file_type', operator: 'in', value: ['video', 'audio'] },
      { field: 'watch_count', operator: 'equals', value: 0 }
    ],
    tags: ['video', 'watchlist']
  }
];

const AdvancedSearch: React.FC = () => {
  const [rules, setRules] = useState<SearchRule[]>([]);
  const [searchSettings, setSearchSettings] = useState<SearchSettings>({
    sortBy: 'relevance',
    sortOrder: 'desc',
    itemsPerPage: 24,
    caseSensitive: false,
    includeMetadata: true,
    viewMode: 'grid'
  });
  const [savedSearches, setSavedSearches] = useState<SavedSearch[]>([]);
  const [_showSaveDialog, setShowSaveDialog] = useState(false);
  const [saveName, setSaveName] = useState('');
  const [_isSaving, setIsSaving] = useState(false);
  const [activeTab, setActiveTab] = useState<'builder' | 'presets' | 'saved'>('builder');
  const [showAdvanced, setShowAdvanced] = useState(false);

  const generateId = () => randomId();

  const getFieldType = (field: string): string => {
    const fieldDef = SEARCH_FIELDS.find(f => f.value === field);
    return fieldDef?.type || 'text';
  };

  const getOperatorsForField = (field: string) => {
    const fieldType = getFieldType(field);
    return OPERATORS[fieldType] || [];
  };

  const addRule = () => {
    const newRule: SearchRule = {
      id: generateId(),
      field: 'title',
      operator: 'contains',
      value: '',
      enabled: true,
      condition: rules.length > 0 ? 'and' : undefined
    };
    setRules([...rules, newRule]);
  };

  const updateRule = (id: string, updates: Partial<SearchRule>) => {
    setRules(rules.map(rule => 
      rule.id === id ? { ...rule, ...updates } : rule
    ));
  };

  const removeRule = (id: string) => {
    setRules(rules.filter(rule => rule.id !== id));
  };

  const duplicateRule = (id: string) => {
    const ruleToDuplicate = rules.find(rule => rule.id === id);
    if (ruleToDuplicate) {
      const newRule = {
        ...ruleToDuplicate,
        id: generateId(),
        condition: 'and' as const
      };
      setRules([...rules, newRule]);
    }
  };

  const saveSearch = async () => {
    setIsSaving(true);
    try {
      const newSavedSearch: SavedSearch = {
        id: generateId(),
        name: saveName,
        description: `Custom search with ${rules.length} rules`,
        rules: [...rules],
        settings: { ...searchSettings },
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString()
      };
      
      setSavedSearches([...savedSearches, newSavedSearch]);
      setShowSaveDialog(false);
      setSaveName('');
      toast.success('Search saved for this session only. It is not stored on the server.');
    } catch (error) {
      toast.error('Failed to save search');
    } finally {
      setIsSaving(false);
    }
  };

  const applyPreset = (preset: SearchPreset) => {
    const newRules: SearchRule[] = preset.rules.map((rule, index) => ({
      id: generateId(),
      field: rule.field || 'title',
      operator: rule.operator || 'contains',
      value: rule.value || '',
      enabled: true,
      condition: index > 0 ? 'and' : undefined
    }));
    setRules(newRules);
    toast.success(`Applied preset: ${preset.name}`);
  };

  const loadSavedSearch = (search: SavedSearch) => {
    setRules([...search.rules]);
    setSearchSettings({ ...search.settings });
    toast.success(`Loaded search: ${search.name}`);
  };

  const clearRules = () => {
    setRules([]);
    toast.success('Search rules cleared');
  };

  return (
    <div className="max-w-7xl mx-auto">
      <div className="bg-white dark:bg-gray-800 rounded-lg border p-6">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Advanced Search</h3>
        <p className="text-sm text-gray-600 dark:text-gray-300 mb-6">
          Build complex search queries with multiple rules and conditions
        </p>

        {/* Tabs */}
        <div className="border-b border-gray-200 mb-6">
          <div className="flex gap-4">
            {['builder', 'presets', 'saved'].map((tab) => (
              <button
                key={tab}
                onClick={() => setActiveTab(tab as 'builder' | 'presets' | 'saved')}
                className={`py-2 px-1 border-b-2 font-medium text-sm ${
                  activeTab === tab
                    ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                    : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 hover:border-gray-300'
                }`}
              >
                {tab.charAt(0).toUpperCase() + tab.slice(1)}
              </button>
            ))}
          </div>
        </div>

        {/* Tab Content */}
        {activeTab === 'builder' && (
          <div className="space-y-6">
            {/* Rules List */}
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <h4 className="text-md font-medium text-gray-900 dark:text-white">Search Rules</h4>
                <Button onClick={addRule} size="sm">
                  <Plus className="w-4 h-4 mr-2" />
                  Add Rule
                </Button>
              </div>

              {rules.length === 0 ? (
                <div className="text-center py-8 border-2 border-dashed border-gray-300 rounded-lg">
                  <Search className="w-12 h-12 text-gray-400 mx-auto mb-4" />
                  <p className="text-gray-600 dark:text-gray-300 mb-2">No search rules defined</p>
                  <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">Add rules to build your search query</p>
                  <Button onClick={addRule}>
                    <Plus className="w-4 h-4 mr-2" />
                    Add First Rule
                  </Button>
                </div>
              ) : (
                <div className="space-y-3">
                  {rules.map((rule, index) => (
                    <div key={rule.id} className="border rounded-lg p-4">
                      <div className="flex items-start gap-4">
                        <div className="pt-2">
                          <Switch
                            checked={rule.enabled}
                            onCheckedChange={(checked) => updateRule(rule.id, { enabled: checked })}
                          />
                          {index > 0 && (
                            <Select
                              aria-label="Rule condition"
                              value={rule.condition || 'and'}
                              onChange={(value) => updateRule(rule.id, { condition: value as 'and' | 'or' })}
                              options={[
                                { value: 'and', label: 'AND' },
                                { value: 'or', label: 'OR' }
                              ]}
                            />
                          )}
                        </div>

                        <div className="flex-1 grid grid-cols-1 md:grid-cols-4 gap-3">
                          <Select
                            aria-label="Rule field"
                            value={rule.field}
                            onChange={(value) => updateRule(rule.id, { 
                              field: value, 
                              value: '', 
                              operator: OPERATORS[getFieldType(value)][0].value 
                            })}
                            options={SEARCH_FIELDS.map(f => ({ value: f.value, label: f.label }))}
                          />

                          <Select
                            aria-label="Rule operator"
                            value={rule.operator}
                            onChange={(value) => updateRule(rule.id, { operator: value, value: '' })}
                            options={getOperatorsForField(rule.field)}
                          />

                          <div className="md:col-span-2">
                            <Input
                              value={rule.value as string}
                              onChange={(e) => updateRule(rule.id, { value: e.target.value })}
                              placeholder={rule.operator === 'in' || rule.operator === 'not_in' ? 'comma,separated,values' : 'Enter value...'}
                            />
                          </div>
                        </div>

                        <div className="flex items-center gap-2 pt-2">
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => duplicateRule(rule.id)}
                            title="Duplicate rule"
                          >
                            <Plus className="w-3 h-3" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => removeRule(rule.id)}
                            className="text-red-600 hover:text-red-700"
                            title="Remove rule"
                          >
                            <Minus className="w-3 h-3" />
                          </Button>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* Search Settings */}
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <h4 className="text-md font-medium text-gray-900 dark:text-white">Search Settings</h4>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setShowAdvanced(!showAdvanced)}
                >
                  <Settings className="w-4 h-4 mr-2" />
                  {showAdvanced ? 'Hide' : 'Show'} Advanced
                </Button>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <div>
                  <label htmlFor="adv-search-select-1" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Sort By</label>
                  <Select id="adv-search-select-1"
                    value={searchSettings.sortBy}
                    onChange={(value) => setSearchSettings({ ...searchSettings, sortBy: value })}
                    options={[
                      { value: 'relevance', label: 'Relevance' },
                      { value: 'title', label: 'Title' },
                      { value: 'created_at', label: 'Date Created' },
                      { value: 'rating', label: 'Rating' }
                    ]}
                  />
                </div>

                <div>
                  <label htmlFor="adv-search-select-2" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Order</label>
                  <Select id="adv-search-select-2"
                    value={searchSettings.sortOrder}
                    onChange={(value) => setSearchSettings({ ...searchSettings, sortOrder: value as 'asc' | 'desc' })}
                    options={[
                      { value: 'asc', label: 'Ascending' },
                      { value: 'desc', label: 'Descending' }
                    ]}
                  />
                </div>

                <div>
                  <label htmlFor="adv-search-select-3" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Items per page</label>
                  <Select id="adv-search-select-3"
                    value={searchSettings.itemsPerPage.toString()}
                    onChange={(value) => setSearchSettings({ ...searchSettings, itemsPerPage: Number(value) })}
                    options={[
                      { value: '12', label: '12' },
                      { value: '24', label: '24' },
                      { value: '48', label: '48' },
                      { value: '96', label: '96' }
                    ]}
                  />
                </div>
              </div>
            </div>
          </div>
        )}

        {activeTab === 'presets' && (
          <div className="space-y-4">
            <h4 className="text-md font-medium text-gray-900 dark:text-white mb-4">Search Presets</h4>
            <p className="text-sm text-gray-600 dark:text-gray-300 mb-6">Quick start with pre-configured search patterns</p>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {SEARCH_PRESETS.map((preset) => (
                <div
                  key={preset.id}
                  className="border rounded-lg p-4 cursor-pointer hover:shadow-md transition-shadow"
                  onClick={() => applyPreset(preset)}
                  onKeyDown={onActivateKey(() => applyPreset(preset))}
                  role="button"
                  tabIndex={0}
                >
                  <div className="flex items-start gap-3">
                    <div className="p-2 rounded-lg bg-blue-100">
                      {preset.icon}
                    </div>
                    <div className="flex-1">
                      <h5 className="font-semibold text-gray-900 dark:text-white text-sm">{preset.name}</h5>
                      <p className="text-xs text-gray-500 dark:text-gray-400 mb-2">{preset.category}</p>
                      <p className="text-sm text-gray-600 dark:text-gray-300 mb-2">{preset.description}</p>
                      <div className="flex items-center gap-2">
                        <Badge variant="outline">{preset.rules.length} rules</Badge>
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {activeTab === 'saved' && (
          <div className="space-y-4">
            <h4 className="text-md font-medium text-gray-900 dark:text-white mb-4">Saved Searches</h4>
            <p className="text-sm text-gray-600 dark:text-gray-300 mb-6">Access your previously saved search configurations</p>

            {savedSearches.length === 0 ? (
              <div className="text-center py-8 border-2 border-dashed border-gray-300 rounded-lg">
                <Bookmark className="w-12 h-12 text-gray-400 mx-auto mb-4" />
                <p className="text-gray-600 dark:text-gray-300 mb-2">No saved searches yet</p>
                <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">Save your search rules to reuse them later</p>
                <Button onClick={() => setActiveTab('builder')}>
                  Create Search Rule
                </Button>
              </div>
            ) : (
              <div className="space-y-4">
                {savedSearches.map((search) => (
                  <div
                    key={search.id}
                    className="border rounded-lg p-4 cursor-pointer hover:shadow-md transition-shadow"
                    onClick={() => loadSavedSearch(search)}
                    onKeyDown={onActivateKey(() => loadSavedSearch(search))}
                    role="button"
                    tabIndex={0}
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex-1">
                        <h5 className="font-semibold text-gray-900 dark:text-white">{search.name}</h5>
                        <p className="text-sm text-gray-600 dark:text-gray-300">{search.description}</p>
                        <div className="flex items-center gap-4 mt-2 text-xs text-gray-500 dark:text-gray-400">
                          <span>{search.rules.length} rules</span>
                          <span>Created {new Date(search.created_at).toLocaleDateString()}</span>
                        </div>
                      </div>
                      <Button variant="outline" size="sm">
                        Load
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {/* Action Bar */}
        <div className="flex items-center justify-between pt-6 border-t">
          <div className="text-sm text-gray-600 dark:text-gray-300">
            {rules.filter(rule => rule.enabled).length} active rule{rules.filter(rule => rule.enabled).length !== 1 ? 's' : ''}
          </div>
          <div className="flex items-center gap-2">
            {rules.length > 0 && (
              <Button variant="outline" onClick={clearRules}>
                <RotateCcw className="w-4 h-4 mr-2" />
                Clear All
              </Button>
            )}
            
            <Button onClick={saveSearch} disabled={rules.length === 0}>
              <Save className="w-4 h-4 mr-2" />
              Save Search
            </Button>
            
            {/* CAT-001: the server has no rule-based search endpoint, so no search can be run. */}
            <Button
              disabled
              title="Rule-based search is not available: the server has no endpoint that evaluates these rules."
            >
              <Search className="w-4 h-4 mr-2" />
              Search
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
};

export default AdvancedSearch;