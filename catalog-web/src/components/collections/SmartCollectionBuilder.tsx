import React, { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Plus,
  X,
  Save,
  Filter,
  Clock,
  Star,
  Music,
  Film,
  FileText,
  Image,
  Heart,
  HardDrive,
  Mic,
  Play,
  ChevronDown,
  ChevronUp,
  Trash2,
  Copy,
  TestTube,
  Lightbulb,
  Zap
} from 'lucide-react';
import { Button } from '../ui/Button';
import { Input } from '../ui/Input';
import { Select } from '../ui/Select';
import { Textarea } from '../ui/Textarea';
import { Switch } from '../ui/Switch';
import { 
  COLLECTION_TEMPLATES, 
  COLLECTION_FIELD_OPTIONS, 
  COLLECTION_OPERATORS, 
  getFieldOptions, 
  getFieldLabel, 
  getFieldType, 
  validateRules 
} from '../../lib/collectionRules';
import { CollectionRule, CollectionTemplate } from '../../types/collections';
import { toast } from 'react-hot-toast';

interface SmartCollectionBuilderProps {
  initialName?: string;
  initialDescription?: string;
  initialRules?: CollectionRule[];
  onSave: (name: string, description: string, rules: CollectionRule[]) => void;
  onCancel: () => void;
  className?: string;
}

const FIELD_ICONS = {
  music: Music,
  video: Film,
  image: Image,
  document: FileText,
};

const TEMPLATE_ICONS = {
  Clock: Clock,
  Star: Star,
  Film: Film,
  Music: Music,
  Heart: Heart,
  HardDrive: HardDrive,
  Guitar: Mic,
  Play: Play,
};

/**
 * SmartCollectionBuilder provides a rule-based editor for creating smart
 * collections with field/operator/value criteria and preset templates.
 */
export const SmartCollectionBuilder: React.FC<SmartCollectionBuilderProps> = ({
  initialName = '',
  initialDescription = '',
  initialRules = [],
  onSave,
  onCancel,
  className = ''
}) => {
  const [name, setName] = useState(initialName);
  const [description, setDescription] = useState(initialDescription);
  const [rules, setRules] = useState<CollectionRule[]>(initialRules);
  const [showTemplates, setShowTemplates] = useState(false);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [testResults, setTestResults] = useState<{
    total_items: number;
    matched_items: number;
    performance_ms: number;
    sample_items?: Array<{ id: string; title: string; artist?: string; media_type: string }>;
  } | null>(null);
  const [isTesting, setIsTesting] = useState(false);
  const [refreshFrequency, setRefreshFrequency] = useState('daily');
  const [sortOrder, setSortOrder] = useState('date_added');
  const [maxItems, setMaxItems] = useState('0');
  const [cacheResults, setCacheResults] = useState(true);

  // Generate unique ID for new rules
  const generateRuleId = () => `rule_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;

  // Initialize with at least one rule if none provided
  useEffect(() => {
    if (rules.length === 0) {
      addRule();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const addRule = (parentId?: string, index?: number) => {
    const newRule: CollectionRule = {
      id: generateRuleId(),
      field: 'title',
      operator: 'contains',
      value: '',
      field_type: 'text',
      label: 'Title',
    };

    if (parentId) {
      setRules(prev => prev.map(rule => {
        if (rule.id === parentId) {
          return {
            ...rule,
            nested_rules: [...(rule.nested_rules || []), newRule]
          };
        }
        return rule;
      }));
    } else {
      setRules(prev => {
        if (index !== undefined) {
          const newRules = [...prev];
          newRules.splice(index + 1, 0, newRule);
          return newRules;
        }
        return [...prev, newRule];
      });
    }
  };

  const removeRule = (ruleId: string) => {
    setRules(prev => {
      const removeNestedRules = (rules: CollectionRule[]): CollectionRule[] => {
        return rules.filter(rule => rule.id !== ruleId).map(rule => ({
          ...rule,
          nested_rules: rule.nested_rules ? removeNestedRules(rule.nested_rules) : undefined
        }));
      };
      return removeNestedRules(prev);
    });
  };

  const updateRule = (ruleId: string, updates: Partial<CollectionRule>) => {
    setRules(prev => {
      const updateNestedRules = (rules: CollectionRule[]): CollectionRule[] => {
        return rules.map(rule => {
          if (rule.id === ruleId) {
            const fieldType = getFieldType(updates.field || rule.field);
            const validFieldType: 'text' | 'number' | 'date' | 'boolean' | 'select' | 'multiselect' = 
              ['text', 'number', 'date', 'boolean', 'select', 'multiselect'].includes(fieldType)
                ? fieldType as 'text' | 'number' | 'date' | 'boolean' | 'select' | 'multiselect'
                : 'text';
            
            const updatedRule = {
              field: rule.field,
              operator: rule.operator,
              value: rule.value,
              condition: rule.condition,
              nested_rules: rule.nested_rules,
              label: rule.label,
              id: rule.id,
              field_type: validFieldType,
              ...updates
            };
            
            // Update field_type and label if field changed
            if (updates.field) {
              updatedRule.field_type = validFieldType;
              updatedRule.label = getFieldLabel(updates.field);
            }
            
            return updatedRule;
          }
          if (rule.nested_rules) {
            return { ...rule, nested_rules: updateNestedRules(rule.nested_rules) };
          }
          return rule;
        });
      };
      return updateNestedRules(prev);
    });
  };

  const loadTemplate = (template: CollectionTemplate) => {
    const rulesWithIds: CollectionRule[] = template.rules.map((rule): CollectionRule => {
      const validFieldType = ['text', 'number', 'date', 'boolean', 'select', 'multiselect'].includes(rule.field_type) 
        ? rule.field_type as 'text' | 'number' | 'date' | 'boolean' | 'select' | 'multiselect'
        : 'text';
      
      return {
        field: rule.field,
        operator: rule.operator,
        value: rule.value,
        condition: rule.condition,
        nested_rules: rule.nested_rules,
        label: rule.label,
        id: generateRuleId(),
        field_type: validFieldType,
      };
    });
    setRules(rulesWithIds);
    setName(template.name);
    setDescription(template.description);
    setShowTemplates(false);
    toast.success(`Loaded template: ${template.name}`);
  };

  const duplicateRule = (ruleId: string) => {
    const originalRule = rules.find(r => r.id === ruleId);
    if (originalRule) {
      const ruleIndex = rules.findIndex(r => r.id === ruleId);
      const duplicatedRule = { ...originalRule, id: generateRuleId() };
      const newRules = [...rules];
      newRules.splice(ruleIndex + 1, 0, duplicatedRule);
      setRules(newRules);
    }
  };

  // The server has no endpoint that evaluates smart-collection rules against the library.
  // The earlier version returned canned counts and sample items (CAT-001); it now says so.
  const testRules = async () => {
    setTestResults(null);
    toast.error('Rule testing is not available: the server cannot evaluate rules against the library.');
  };

  const handleSave = () => {
    const errors = validateRules(rules);
    if (errors.length > 0) {
      toast.error(errors[0]);
      return;
    }

    if (!name.trim()) {
      toast.error('Collection name is required');
      return;
    }

    onSave(name.trim(), description.trim(), rules);
  };

  const renderRule = (rule: CollectionRule, level = 0, _parentRule?: CollectionRule) => {
    const fieldType = getFieldType(rule.field);
    const operators = COLLECTION_OPERATORS[fieldType as keyof typeof COLLECTION_OPERATORS] || [];
    const fieldOptions = getFieldOptions(rule.field);

    return (
      <motion.div
        key={rule.id}
        initial={{ opacity: 0, x: -20 }}
        animate={{ opacity: 1, x: 0 }}
        exit={{ opacity: 0, x: 20 }}
        className={`border border-gray-200 dark:border-gray-700 rounded-lg p-4 mb-3 ${
          level > 0 ? 'ml-6 border-l-4 border-l-blue-500' : ''
        }`}
      >
        <div className="flex items-start gap-3">
          {/* Drag Handle */}
          <div className="flex items-center justify-center mt-2">
            <div className="w-4 h-4 bg-gray-300 dark:bg-gray-600 rounded cursor-move" />
          </div>

          <div className="flex-1 grid grid-cols-1 md:grid-cols-4 gap-3">
            {/* Field Select */}
            <Select
              aria-label="Rule field"
              value={rule.field}
              onChange={(value) => updateRule(rule.id, { field: value })}
              options={COLLECTION_FIELD_OPTIONS}
              className="w-full"
            />

            {/* Operator Select */}
            <Select
              aria-label="Rule operator"
              value={rule.operator}
              onChange={(value) => updateRule(rule.id, { operator: value })}
              options={operators}
              className="w-full"
            />

            {/* Value Input */}
            <div className="relative">
              {fieldType === 'boolean' ? (
                <Switch
                  checked={rule.value === true}
                  onCheckedChange={(checked) => updateRule(rule.id, { value: checked })}
                />
              ) : fieldOptions.length > 0 ? (
                <Select
                  aria-label="Rule value"
                  value={Array.isArray(rule.value) ? rule.value[0] : String(rule.value ?? '')}
                  onChange={(value) => updateRule(rule.id, {
                    value: rule.operator === 'is_any' || rule.operator === 'is_not_any' ? [value] : value
                  })}
                  options={fieldOptions}
                  className="w-full"
                />
              ) : (
                <Input
                  value={String(rule.value ?? '')}
                  onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateRule(rule.id, { value: e.target.value })}
                  placeholder="Enter value"
                  type={fieldType === 'number' ? 'number' : fieldType === 'date' ? 'date' : 'text'}
                  className="w-full"
                />
              )}
            </div>

            {/* Actions */}
            <div className="flex items-center gap-2">
              <Button
                variant="ghost"
                size="sm"
                onClick={() => duplicateRule(rule.id)}
                title="Duplicate rule"
              >
                <Copy className="w-4 h-4" />
              </Button>
              
              <Button
                variant="ghost"
                size="sm"
                onClick={() => addRule(rule.id)}
                title="Add nested rule"
              >
                <Plus className="w-4 h-4" />
              </Button>

              <Button
                variant="ghost"
                size="sm"
                onClick={() => removeRule(rule.id)}
                title="Remove rule"
                className="text-red-600 hover:text-red-700"
              >
                <Trash2 className="w-4 h-4" />
              </Button>
            </div>
          </div>
        </div>

        {/* Nested Rules */}
        {rule.nested_rules && rule.nested_rules.length > 0 && (
          <div className="mt-3 border-t border-gray-200 dark:border-gray-700 pt-3">
            {rule.nested_rules.map(nestedRule => renderRule(nestedRule, level + 1, rule))}
          </div>
        )}
      </motion.div>
    );
  };

  return (
    <div className={`bg-white dark:bg-gray-900 rounded-lg shadow-lg ${className}`}>
      <div className="p-6 border-b border-gray-200 dark:border-gray-700">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 bg-gradient-to-br from-purple-500 to-pink-600 rounded-lg flex items-center justify-center">
              <Zap className="w-5 h-5 text-white" />
            </div>
            <div>
              <h2 className="text-xl font-bold text-gray-900 dark:text-white">
                Smart Collection Builder
              </h2>
              <p className="text-sm text-gray-600 dark:text-gray-400">
                Create collections that automatically update based on rules
              </p>
            </div>
          </div>
          
          <Button
            variant="ghost"
            size="sm"
            onClick={onCancel}
          >
            <X className="w-4 h-4" />
          </Button>
        </div>
      </div>

      <div className="p-6">
        {/* Basic Info */}
        <div className="mb-6">
          <Input
            label="Collection Name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Enter collection name"
            className="mb-4"
          />
          
          <Textarea
            label="Description"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Optional description for this collection"
            rows={3}
            className="mb-4"
          />
        </div>

        {/* Templates */}
        <div className="mb-6">
          <div className="flex items-center justify-between mb-3">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
              <Lightbulb className="w-5 h-5 text-yellow-500" />
              Quick Start Templates
            </h3>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setShowTemplates(!showTemplates)}
            >
              {showTemplates ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
            </Button>
          </div>

          <AnimatePresence>
            {showTemplates && (
              <motion.div
                initial={{ height: 0, opacity: 0 }}
                animate={{ height: 'auto', opacity: 1 }}
                exit={{ height: 0, opacity: 0 }}
                className="overflow-hidden"
              >
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                  {COLLECTION_TEMPLATES.map(template => {
                    const IconComponent = (TEMPLATE_ICONS as Record<string, React.ComponentType<{ className?: string }>>)[template.icon || 'Star'];
                    return (
                      <motion.div
                        key={template.id}
                        whileHover={{ scale: 1.02 }}
                        whileTap={{ scale: 0.98 }}
                        className="border border-gray-200 dark:border-gray-700 rounded-lg p-4 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800"
                        onClick={() => loadTemplate(template)}
                      >
                        <div className="flex items-center gap-3 mb-2">
                          <div className="w-8 h-8 bg-blue-100 dark:bg-blue-900 rounded-full flex items-center justify-center">
                            {IconComponent && <IconComponent className="w-4 h-4 text-blue-600 dark:text-blue-400" />}
                          </div>
                          <div>
                            <h4 className="font-semibold text-gray-900 dark:text-white text-sm">
                              {template.name}
                            </h4>
                            <span className="text-xs text-gray-500 dark:text-gray-400">
                              {template.category}
                            </span>
                          </div>
                        </div>
                        <p className="text-xs text-gray-600 dark:text-gray-400">
                          {template.description}
                        </p>
                      </motion.div>
                    );
                  })}
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </div>

        {/* Rules Section */}
        <div className="mb-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
              <Filter className="w-5 h-5" />
              Collection Rules
            </h3>
            
            <div className="flex items-center gap-2">
              <Button
                variant="ghost"
                size="sm"
                onClick={testRules}
                disabled={isTesting}
                className="flex items-center gap-2"
              >
                <TestTube className="w-4 h-4" />
                {isTesting ? 'Testing...' : 'Test Rules'}
              </Button>
              
              <Button
                variant="outline"
                size="sm"
                onClick={() => addRule()}
              >
                <Plus className="w-4 h-4 mr-1" />
                Add Rule
              </Button>
            </div>
          </div>

          <AnimatePresence>
            {rules.map((rule) => renderRule(rule))}
          </AnimatePresence>

          {rules.length === 0 && (
            <div className="text-center py-8 border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg">
              <Filter className="w-12 h-12 text-gray-400 mx-auto mb-3" />
              <p className="text-gray-600 dark:text-gray-400 mb-3">
                No rules defined yet. Start by adding a rule above.
              </p>
              <Button
                variant="outline"
                onClick={() => addRule()}
              >
                <Plus className="w-4 h-4 mr-1" />
                Add First Rule
              </Button>
            </div>
          )}
        </div>

        {/* Test Results */}
        {testResults && (
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            className="mb-6 p-4 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg"
          >
            <h4 className="font-semibold text-green-800 dark:text-green-200 mb-3">
              Test Results
            </h4>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-4">
              <div>
                <p className="text-sm text-green-600 dark:text-green-400">Total Items</p>
                <p className="text-lg font-bold text-green-800 dark:text-green-200">
                  {testResults.total_items.toLocaleString()}
                </p>
              </div>
              <div>
                <p className="text-sm text-green-600 dark:text-green-400">Matched Items</p>
                <p className="text-lg font-bold text-green-800 dark:text-green-200">
                  {testResults.matched_items.toLocaleString()}
                </p>
              </div>
              <div>
                <p className="text-sm text-green-600 dark:text-green-400">Match Rate</p>
                <p className="text-lg font-bold text-green-800 dark:text-green-200">
                  {((testResults.matched_items / testResults.total_items) * 100).toFixed(1)}%
                </p>
              </div>
              <div>
                <p className="text-sm text-green-600 dark:text-green-400">Performance</p>
                <p className="text-lg font-bold text-green-800 dark:text-green-200">
                  {testResults.performance_ms}ms
                </p>
              </div>
            </div>
            
            {testResults.sample_items && testResults.sample_items.length > 0 && (
              <div>
                <p className="text-sm text-green-600 dark:text-green-400 mb-2">Sample Items:</p>
                <div className="space-y-1">
                  {testResults.sample_items.map((item) => (
                    <div key={item.id} className="flex items-center gap-2 text-sm">
                      {FIELD_ICONS[item.media_type as keyof typeof FIELD_ICONS] && 
                        React.createElement(FIELD_ICONS[item.media_type as keyof typeof FIELD_ICONS], { className: 'w-4 h-4' })
                      }
                      <span className="text-green-800 dark:text-green-200">{item.title}</span>
                      {item.artist && <span className="text-green-600 dark:text-green-400">by {item.artist}</span>}
                    </div>
                  ))}
                </div>
              </div>
            )}
          </motion.div>
        )}

        {/* Actions */}
        <div className="flex items-center justify-between pt-6 border-t border-gray-200 dark:border-gray-700">
          <div className="flex items-center gap-2">
            <label className="text-sm text-gray-600 dark:text-gray-400">Advanced options</label>
            <Switch
              checked={showAdvanced}
              onCheckedChange={setShowAdvanced}
            />
          </div>
          
          <div className="flex items-center gap-3">
            <Button
              variant="ghost"
              onClick={onCancel}
            >
              Cancel
            </Button>
            
            <Button
              onClick={handleSave}
              className="flex items-center gap-2"
            >
              <Save className="w-4 h-4" />
              Create Collection
            </Button>
          </div>
        </div>

        {/* Advanced Options */}
        <AnimatePresence>
          {showAdvanced && (
            <motion.div
              initial={{ height: 0, opacity: 0 }}
              animate={{ height: 'auto', opacity: 1 }}
              exit={{ height: 0, opacity: 0 }}
              className="overflow-hidden mt-6 pt-6 border-t border-gray-200 dark:border-gray-700"
            >
              <h4 className="font-semibold text-gray-900 dark:text-white mb-4">Advanced Options</h4>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="space-y-4">
                  <div>
                    <label htmlFor="smart-coll-select-1" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                      Auto-refresh frequency
                    </label>
                    <Select id="smart-coll-select-1"
                      value={refreshFrequency}
                      onChange={(value) => setRefreshFrequency(value)}
                      options={[
                        { value: 'hourly', label: 'Hourly' },
                        { value: 'daily', label: 'Daily' },
                        { value: 'weekly', label: 'Weekly' },
                        { value: 'manual', label: 'Manual only' },
                      ]}
                      className="w-full"
                    />
                  </div>
                  
                  <div>
                    <label htmlFor="smart-coll-select-2" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                      Sort order
                    </label>
                    <Select id="smart-coll-select-2"
                      value={sortOrder}
                      onChange={(value) => setSortOrder(value)}
                      options={[
                        { value: 'date_added', label: 'Date Added' },
                        { value: 'title', label: 'Title' },
                        { value: 'artist', label: 'Artist' },
                        { value: 'rating', label: 'Rating' },
                        { value: 'play_count', label: 'Play Count' },
                      ]}
                      className="w-full"
                    />
                  </div>
                </div>
                
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                      Maximum items (0 = unlimited)
                    </label>
                    <Input
                      type="number"
                      value={maxItems}
                      onChange={(e) => setMaxItems(e.target.value)}
                      placeholder="0"
                      className="w-full"
                    />
                  </div>
                  
                  <div>
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                      Cache results
                    </label>
                    <Switch
                      checked={cacheResults}
                      onCheckedChange={setCacheResults}
                    />
                    <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                      Improve performance by caching query results
                    </p>
                  </div>
                </div>
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </div>
  );
};