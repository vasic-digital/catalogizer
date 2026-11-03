import React, { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Plus,
  X,
  Save,
  Filter,
  Calendar,
  Clock,
  Star,
  Tag,
  Music,
  Film,
  Image,
  FileText,
  ChevronDown,
  ChevronUp,
  HelpCircle
} from 'lucide-react';
import { Button } from '../ui/Button';
import { Input } from '../ui/Input';
import { Select } from '../ui/Select';
import { SmartPlaylistRule } from '../../types/playlists';
import { toast } from 'react-hot-toast';

interface SmartPlaylistBuilderProps {
  onSave: (name: string, description: string, rules: SmartPlaylistRule[]) => void;
  onCancel: () => void;
  initialData?: {
    name: string;
    description: string;
    rules: SmartPlaylistRule[];
  };
  className?: string;
}

const FIELD_OPTIONS = [
  { value: 'media_type', label: 'Media Type', icon: Music },
  { value: 'year', label: 'Year', icon: Calendar },
  { value: 'rating', label: 'Rating', icon: Star },
  { value: 'quality', label: 'Quality', icon: Filter },
  { value: 'genre', label: 'Genre', icon: Tag },
  { value: 'created_at', label: 'Date Added', icon: Clock },
] as const;

const OPERATOR_OPTIONS = [
  { value: 'equals', label: 'Is', description: 'Exactly matches the value' },
  { value: 'not_equals', label: 'Is not', description: 'Does not match the value' },
  { value: 'greater_than', label: 'Greater than', description: 'More than the value' },
  { value: 'less_than', label: 'Less than', description: 'Less than the value' },
  { value: 'contains', label: 'Contains', description: 'Contains the text' },
  { value: 'starts_with', label: 'Starts with', description: 'Begins with the text' },
  { value: 'ends_with', label: 'Ends with', description: 'Ends with the text' },
  { value: 'in', label: 'Is one of', description: 'Matches any of the values' },
  { value: 'not_in', label: 'Is not one of', description: 'Does not match any of the values' },
] as const;

const MEDIA_TYPE_OPTIONS = [
  { value: 'video', label: 'Video', icon: Film },
  { value: 'music', label: 'Music', icon: Music },
  { value: 'image', label: 'Image', icon: Image },
  { value: 'document', label: 'Document', icon: FileText },
] as const;

const QUALITY_OPTIONS = [
  { value: 'cam', label: 'CAM' },
  { value: 'ts', label: 'TS' },
  { value: 'dvdrip', label: 'DVDrip' },
  { value: 'brrip', label: 'BRRip' },
  { value: '720p', label: '720p' },
  { value: '1080p', label: '1080p' },
  { value: '4k', label: '4K' },
  { value: 'hdr', label: 'HDR' },
  { value: 'dolby_vision', label: 'Dolby Vision' },
] as const;

const GENRE_OPTIONS = [
  'Action', 'Comedy', 'Drama', 'Horror', 'Sci-Fi', 'Romance', 'Thriller', 
  'Documentary', 'Animation', 'Fantasy', 'Mystery', 'Adventure', 'Crime',
  'Family', 'Biography', 'History', 'War', 'Western', 'Musical', 'Sport'
];

const COMMON_PRESETS = [
  {
    name: 'Recently Added',
    description: 'Items added in the last 30 days',
    rules: [
      {
        field: 'created_at',
        operator: 'greater_than',
        value: new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString().split('T')[0],
        condition: 'and'
      }
    ]
  },
  {
    name: 'High Rated Movies',
    description: 'Movies with rating 8 or higher',
    rules: [
      {
        field: 'media_type',
        operator: 'equals',
        value: 'video',
        condition: 'and'
      },
      {
        field: 'rating',
        operator: 'greater_than',
        value: 8,
        condition: 'and'
      }
    ]
  },
  {
    name: 'HD Movies Only',
    description: 'Movies in 1080p or higher quality',
    rules: [
      {
        field: 'media_type',
        operator: 'equals',
        value: 'video',
        condition: 'and'
      },
      {
        field: 'quality',
        operator: 'in',
        value: ['1080p', '4k', 'hdr', 'dolby_vision'],
        condition: 'and'
      }
    ]
  }
] as Array<{
  name: string;
  description: string;
  rules: SmartPlaylistRule[];
}>;

export const SmartPlaylistBuilder: React.FC<SmartPlaylistBuilderProps> = ({
  onSave,
  onCancel,
  initialData,
  className = ''
}) => {
  const [name, setName] = useState(initialData?.name || '');
  const [description, setDescription] = useState(initialData?.description || '');
  const [rules, setRules] = useState<SmartPlaylistRule[]>(
    initialData?.rules || [{ field: 'media_type', operator: 'equals', value: '', condition: 'and' }]
  );
  const [expandedRules, setExpandedRules] = useState<Set<number>>(new Set([0]));
  const [showAdvanced, setShowAdvanced] = useState(false);

  const addRule = () => {
    const newRule: SmartPlaylistRule = {
      field: 'media_type',
      operator: 'equals',
      value: '',
      condition: 'and'
    };
    setRules([...rules, newRule]);
    setExpandedRules(new Set([...expandedRules, rules.length]));
  };

  const removeRule = (index: number) => {
    setRules(rules.filter((_, i) => i !== index));
    setExpandedRules(new Set([...expandedRules].filter(i => i !== index)));
  };

  const updateRule = (index: number, updates: Partial<SmartPlaylistRule>) => {
    setRules(rules.map((rule, i) => 
      i === index ? { ...rule, ...updates } : rule
    ));
  };

  const moveRule = (index: number, direction: 'up' | 'down') => {
    const newIndex = direction === 'up' ? index - 1 : index + 1;
    if (newIndex >= 0 && newIndex < rules.length) {
      const newRules = [...rules];
      [newRules[index], newRules[newIndex]] = [newRules[newIndex], newRules[index]];
      setRules(newRules);
    }
  };

  const toggleRuleExpanded = (index: number) => {
    setExpandedRules(prev => {
      const newSet = new Set(prev);
      if (newSet.has(index)) {
        newSet.delete(index);
      } else {
        newSet.add(index);
      }
      return newSet;
    });
  };

  const applyPreset = (preset: typeof COMMON_PRESETS[0]) => {
    setName(preset.name);
    setDescription(preset.description);
    setRules(preset.rules);
    setExpandedRules(new Set(preset.rules.map((_, index) => index)));
  };

  const validateRules = (): string | null => {
    if (!name.trim()) {
      return 'Playlist name is required';
    }
    
    if (rules.length === 0) {
      return 'At least one rule is required';
    }

    for (let i = 0; i < rules.length; i++) {
      const rule = rules[i];
      if (!rule.field || !rule.operator) {
        return `Rule ${i + 1} is incomplete`;
      }
      
      if (rule.value === '' || rule.value === null || rule.value === undefined) {
        return `Rule ${i + 1} has no value`;
      }
    }

    return null;
  };

  const handleSave = () => {
    const error = validateRules();
    if (error) {
      toast.error(error);
      return;
    }

    onSave(name, description, rules);
  };

  const renderRuleInput = (rule: SmartPlaylistRule, index: number) => {
    const fieldInfo = FIELD_OPTIONS.find(f => f.value === rule.field);
    const operatorInfo = OPERATOR_OPTIONS.find(o => o.value === rule.operator);
    const isExpanded = expandedRules.has(index);

    const renderValueInput = () => {
      switch (rule.field) {
        case 'media_type':
          return (
            <Select
              value={String(rule.value)}
              onChange={(value) => updateRule(index, { value: value })}
              className="flex-1"
            >
              <option value="">Select media type...</option>
              {MEDIA_TYPE_OPTIONS.map(option => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </Select>
          );
        
        case 'quality':
          return (
            <Select
              value={Array.isArray(rule.value) ? rule.value[0] : String(rule.value)}
              onChange={(value) => updateRule(index, { value: value })}
              className="flex-1"
            >
              <option value="">Select quality...</option>
              {QUALITY_OPTIONS.map(option => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </Select>
          );
        
        case 'rating':
          return (
            <Input
              type="number"
              min="0"
              max="10"
              step="0.1"
              value={String(rule.value)}
              onChange={(e) => updateRule(index, { value: parseFloat(e.target.value) || 0 })}
              placeholder="Rating"
              className="flex-1"
            />
          );
        
        case 'year':
          return (
            <Input
              type="number"
              min="1900"
              max="2030"
              value={String(rule.value)}
              onChange={(e) => updateRule(index, { value: parseInt(e.target.value) || 0 })}
              placeholder="Year"
              className="flex-1"
            />
          );
        
        case 'genre':
          return (
            <Select
              value={String(rule.value)}
              onChange={(value) => updateRule(index, { value: value })}
              className="flex-1"
            >
              <option value="">Select genre...</option>
              {GENRE_OPTIONS.map(genre => (
                <option key={genre} value={genre}>
                  {genre}
                </option>
              ))}
            </Select>
          );
        
        case 'created_at':
          return (
            <Input
              type="date"
              value={String(rule.value)}
              onChange={(e) => updateRule(index, { value: e.target.value })}
              className="flex-1"
            />
          );
        
        default:
          return (
            <Input
              value={String(rule.value || '')}
              onChange={(e) => updateRule(index, { value: e.target.value })}
              placeholder="Enter value..."
              className="flex-1"
            />
          );
      }
    };

    return (
      <motion.div
        layout
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        exit={{ opacity: 0, y: -20 }}
        className="bg-gray-50 dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4"
      >
        <div className="flex items-start gap-3">
          {/* Drag Handle */}
          <div className="flex flex-col gap-1 pt-2">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => moveRule(index, 'up')}
              disabled={index === 0}
              className="p-1"
            >
              <ChevronUp className="w-3 h-3" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => moveRule(index, 'down')}
              disabled={index === rules.length - 1}
              className="p-1"
            >
              <ChevronDown className="w-3 h-3" />
            </Button>
          </div>

          {/* Rule Content */}
          <div className="flex-1 space-y-3">
            {/* Header */}
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                {fieldInfo && <fieldInfo.icon className="w-4 h-4 text-gray-600 dark:text-gray-400" />}
                <span className="font-medium text-gray-900 dark:text-white">
                  Rule {index + 1}
                </span>
              </div>
              
              <div className="flex items-center gap-2">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => toggleRuleExpanded(index)}
                  className="p-2"
                >
                  {isExpanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
                </Button>
                {rules.length > 1 && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => removeRule(index)}
                    className="text-red-600 hover:text-red-700 p-2"
                  >
                    <X className="w-4 h-4" />
                  </Button>
                )}
              </div>
            </div>

            {/* Rule Builder */}
            <AnimatePresence>
              {isExpanded && (
                <motion.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  exit={{ opacity: 0, height: 0 }}
                  className="space-y-3"
                >
                  <div className="flex items-center gap-3">
                    {/* Field */}
                    <div className="flex-1">
                      <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                        Field
                      </label>
                      <Select
                        value={rule.field}
                        onChange={(value) => updateRule(index, { 
                          field: value as SmartPlaylistRule['field'],
                          value: '' // Reset value when field changes
                        })}
                      >
                        {FIELD_OPTIONS.map(option => (
                          <option key={option.value} value={option.value}>
                            {option.label}
                          </option>
                        ))}
                      </Select>
                    </div>

                    {/* Operator */}
                    <div className="flex-1">
                      <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                        Condition
                      </label>
                      <Select
                        value={rule.operator}
                        onChange={(value) => updateRule(index, { 
                          operator: value as SmartPlaylistRule['operator']
                        })}
                      >
                        {OPERATOR_OPTIONS.map(option => (
                          <option key={option.value} value={option.value}>
                            {option.label}
                          </option>
                        ))}
                      </Select>
                    </div>

                    {/* Value */}
                    <div className="flex-2">
                      <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                        Value
                      </label>
                      {renderValueInput()}
                    </div>
                  </div>

                  {operatorInfo && (
                    <div className="flex items-start gap-2 text-sm text-gray-600 dark:text-gray-400">
                      <HelpCircle className="w-4 h-4 mt-0.5 flex-shrink-0" />
                      <span>{operatorInfo.description}</span>
                    </div>
                  )}

                  {/* Advanced Options */}
                  {showAdvanced && (
                    <div className="flex items-center gap-4">
                      <label className="flex items-center gap-2 text-sm">
                        <span className="text-gray-700 dark:text-gray-300">Logic:</span>
                        <Select
                          value={rule.condition}
                          onChange={(value) => updateRule(index, { 
                            condition: value as SmartPlaylistRule['condition']
                          })}
                          className="w-20"
                        >
                          <option value="and">AND</option>
                          <option value="or">OR</option>
                        </Select>
                      </label>
                    </div>
                  )}
                </motion.div>
              )}
            </AnimatePresence>
          </div>
        </div>
      </motion.div>
    );
  };

  return (
    <div className={`space-y-6 ${className}`}>
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
          Smart Playlist Builder
        </h2>
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setShowAdvanced(!showAdvanced)}
          >
            <Filter className="w-4 h-4" />
            {showAdvanced ? 'Simple' : 'Advanced'}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={onCancel}
          >
            <X className="w-4 h-4" />
            Cancel
          </Button>
        </div>
      </div>

      {/* Basic Info */}
      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Playlist Name
          </label>
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Enter playlist name..."
            className="w-full"
          />
        </div>
        
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Description (optional)
          </label>
          <Input
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Describe your smart playlist..."
            className="w-full"
          />
        </div>
      </div>

      {/* Presets */}
      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
          Quick Presets
        </label>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
          {COMMON_PRESETS.map((preset, index) => (
            <motion.button
              key={index}
              whileHover={{ scale: 1.02 }}
              whileTap={{ scale: 0.98 }}
              onClick={() => applyPreset(preset)}
              className="text-left p-4 bg-gray-50 dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 hover:border-blue-300 dark:hover:border-blue-600 transition-colors"
            >
              <h3 className="font-medium text-gray-900 dark:text-white mb-1">
                {preset.name}
              </h3>
              <p className="text-sm text-gray-600 dark:text-gray-400">
                {preset.description}
              </p>
            </motion.button>
          ))}
        </div>
      </div>

      {/* Rules Section */}
      <div>
        <div className="flex items-center justify-between mb-4">
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">
            Rules ({rules.length})
          </label>
          <Button
            onClick={addRule}
            size="sm"
          >
            <Plus className="w-4 h-4" />
            Add Rule
          </Button>
        </div>

        {/* Rules List */}
        <div className="space-y-3">
          <AnimatePresence>
            {rules.map((rule, index) => (
              <div key={index}>
                {renderRuleInput(rule, index)}
              </div>
            ))}
          </AnimatePresence>
        </div>
      </div>

      {/* Actions */}
      <div className="flex items-center justify-end gap-3 pt-6 border-t border-gray-200 dark:border-gray-700">
        <Button
          variant="ghost"
          onClick={onCancel}
        >
          Cancel
        </Button>
        <Button
          onClick={handleSave}
          className="min-w-32"
        >
          <Save className="w-4 h-4" />
          Save Playlist
        </Button>
      </div>
    </div>
  );
};