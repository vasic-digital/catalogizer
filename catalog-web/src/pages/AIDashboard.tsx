import React, { lazy, Suspense, useState, useEffect, useCallback } from 'react';
import { Brain, Search, TrendingUp, Settings, FileText, Zap, Sparkles, Activity, RefreshCw, Info, ChevronDown, ChevronRight } from 'lucide-react';

// Type-only imports (no runtime cost)
import type { AISuggestion, AICategorizationResult, AISearchQuery } from '../components/ai/AIComponents';
import type { ExtractedMetadata } from '../components/ai/AIMetadata';

// Lazy-loaded AI components (heavy sub-components, loaded per-tab)
const AICollectionSuggestions = lazy(() => import('../components/ai/AIComponents').then(m => ({ default: m.AICollectionSuggestions })));
const AINaturalSearch = lazy(() => import('../components/ai/AIComponents').then(m => ({ default: m.AINaturalSearch })));
const AIContentCategorizer = lazy(() => import('../components/ai/AIComponents').then(m => ({ default: m.AIContentCategorizer })));
const AIUserBehaviorAnalytics = lazy(() => import('../components/ai/AIAnalytics').then(m => ({ default: m.AIUserBehaviorAnalytics })));
const AIPredictions = lazy(() => import('../components/ai/AIAnalytics').then(m => ({ default: m.AIPredictions })));
const AISmartOrganization = lazy(() => import('../components/ai/AIAnalytics').then(m => ({ default: m.AISmartOrganization })));
const AIMetadataExtractor = lazy(() => import('../components/ai/AIMetadata').then(m => ({ default: m.AIMetadataExtractor })));
const AIAutomationRules = lazy(() => import('../components/ai/AIMetadata').then(m => ({ default: m.AIAutomationRules })));
const AIContentQualityAnalyzer = lazy(() => import('../components/ai/AIMetadata').then(m => ({ default: m.AIContentQualityAnalyzer })));

// Types for AI Dashboard state
interface AIDashboardState {
  activeSection: 'overview' | 'suggestions' | 'search' | 'analytics' | 'metadata' | 'automation';
  metrics: {
    processedItems: number;
    accuracyScore: number;
    timeSaved: string;
    automationRules: number;
    predictionsActive: number;
    lastUpdate: string;
  };
  alerts: {
    type: 'success' | 'warning' | 'info' | 'error';
    message: string;
    timestamp: string;
  }[];
}

// Main AI Dashboard Component
const AIDashboard: React.FC = () => {
  const [state, setState] = useState<AIDashboardState>({
    activeSection: 'overview',
    metrics: {
      processedItems: 2847,
      accuracyScore: 92,
      timeSaved: '12.5 hours',
      automationRules: 8,
      predictionsActive: 3,
      lastUpdate: new Date().toISOString()
    },
    alerts: []
  });

  const [expandedSections, setExpandedSections] = useState<Record<string, boolean>>({
    overview: true,
    suggestions: true,
    analytics: false,
    metadata: false,
    automation: false
  });

  // The processed-items counter used to be bumped by a random number every 30 seconds.
  // Nothing measures it, so it no longer changes on its own (CAT-001).

  // Handle section navigation
  const handleSectionChange = useCallback((section: AIDashboardState['activeSection']) => {
    setState(prev => ({ ...prev, activeSection: section }));
  }, []);

  // Handle suggestion acceptance
  const handleSuggestionAccept = useCallback((suggestion: AISuggestion) => {
    setState(prev => ({
      ...prev,
      alerts: [{
        type: 'success',
        message: `Accepted suggestion: ${suggestion.title}`,
        timestamp: new Date().toISOString()
      }]
    }));

    // Simulate implementing the suggestion
    setTimeout(() => {
      setState(prev => ({
        ...prev,
        alerts: prev.alerts.slice(1) // Remove the alert after a delay
      }));
    }, 3000);
  }, []);

  // Handle natural language search
  const handleNaturalSearch = useCallback((query: AISearchQuery) => {
    setState(prev => ({
      ...prev,
      alerts: [{
        type: 'info',
        message: `Processed search: "${query.query}" with intent: ${query.intent}`,
        timestamp: new Date().toISOString()
      }]
    }));

    setTimeout(() => {
      setState(prev => ({
        ...prev,
        alerts: prev.alerts.slice(1)
      }));
    }, 3000);
  }, []);

  // Handle categorization completion
  const handleCategorizationComplete = useCallback((result: AICategorizationResult) => {
    setState(prev => ({
      ...prev,
      alerts: [{
        type: 'success',
        message: `Content categorized as: ${result.category}`,
        timestamp: new Date().toISOString()
      }]
    }));

    setTimeout(() => {
      setState(prev => ({
        ...prev,
        alerts: prev.alerts.slice(1)
      }));
    }, 3000);
  }, []);

  // Handle user behavior analytics actions
  const handleActionImplement = useCallback((action: string) => {
    setState(prev => ({
      ...prev,
      alerts: [{
        type: 'success',
        message: `Action implemented: ${action}`,
        timestamp: new Date().toISOString()
      }]
    }));

    setTimeout(() => {
      setState(prev => ({
        ...prev,
        alerts: prev.alerts.slice(1)
      }));
    }, 3000);
  }, []);

  // Handle prediction actions
  const handlePredictionAction = useCallback((predictionId: string, actionId: string) => {
    setState(prev => ({
      ...prev,
      alerts: [{
        type: 'success',
        message: `Prediction action executed: ${actionId}`,
        timestamp: new Date().toISOString()
      }]
    }));

    setTimeout(() => {
      setState(prev => ({
        ...prev,
        alerts: prev.alerts.slice(1)
      }));
    }, 3000);
  }, []);

  // Handle organization suggestions
  const handleSuggestionApply = useCallback((suggestionId: string) => {
    setState(prev => ({
      ...prev,
      alerts: [{
        type: 'success',
        message: `Organization suggestion applied: ${suggestionId}`,
        timestamp: new Date().toISOString()
      }]
    }));

    setTimeout(() => {
      setState(prev => ({
        ...prev,
        alerts: prev.alerts.slice(1)
      }));
    }, 3000);
  }, []);

  // Handle metadata extraction
  const handleMetadataExtracted = useCallback((_metadata: ExtractedMetadata) => {
    setState(prev => ({
      ...prev,
      metrics: {
        ...prev.metrics,
        processedItems: prev.metrics.processedItems + 1
      }
    }));
  }, []);

  // Handle automation rule toggles
  const handleRuleToggle = useCallback((ruleId: string, enabled: boolean) => {
    setState(prev => ({
      ...prev,
      metrics: {
        ...prev.metrics,
        automationRules: prev.metrics.automationRules + (enabled ? 1 : -1)
      }
    }));
  }, []);

  // Handle rule execution
  const handleRuleExecute = useCallback((ruleId: string) => {
    setState(prev => ({
      ...prev,
      alerts: [{
        type: 'success',
        message: `Automation rule executed: ${ruleId}`,
        timestamp: new Date().toISOString()
      }]
    }));

    setTimeout(() => {
      setState(prev => ({
        ...prev,
        alerts: prev.alerts.slice(1)
      }));
    }, 3000);
  }, []);

  // Handle quality improvements
  const handleQualityImprovement = useCallback((improvement: string) => {
    setState(prev => ({
      ...prev,
      alerts: [{
        type: 'success',
        message: `Quality improvement applied: ${improvement}`,
        timestamp: new Date().toISOString()
      }]
    }));

    setTimeout(() => {
      setState(prev => ({
        ...prev,
        alerts: prev.alerts.slice(1)
      }));
    }, 3000);
  }, []);

  // Toggle section expansion
  const toggleSectionExpansion = useCallback((section: string) => {
    setExpandedSections(prev => ({
      ...prev,
      [section]: !prev[section]
    }));
  }, []);

  // Render navigation tabs
  const renderNavigationTabs = () => (
    <div className="border-b border-gray-200 mb-6">
      <nav className="-mb-px flex space-x-6 sm:space-x-8 overflow-x-auto">
        {[
          { id: 'overview', label: 'Overview', icon: Activity },
          { id: 'suggestions', label: 'AI Suggestions', icon: Sparkles },
          { id: 'search', label: 'Natural Search', icon: Search },
          { id: 'analytics', label: 'Analytics', icon: TrendingUp },
          { id: 'metadata', label: 'Metadata', icon: FileText },
          { id: 'automation', label: 'Automation', icon: Settings }
        ].map(tab => (
          <button
            key={tab.id}
            onClick={() => handleSectionChange(tab.id as AIDashboardState['activeSection'])}
            className={`flex flex-shrink-0 whitespace-nowrap items-center space-x-2 py-4 px-1 border-b-2 font-medium text-sm transition-colors ${
              state.activeSection === tab.id
                ? 'border-indigo-500 text-indigo-600'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
            }`}
          >
            <tab.icon className="w-4 h-4" />
            <span>{tab.label}</span>
          </button>
        ))}
      </nav>
    </div>
  );

  // Render overview section
  const renderOverview = () => (
    <div className="space-y-6">
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-4">
          <div className="flex items-center space-x-3">
            <div className="p-2 bg-indigo-100 rounded-lg">
              <Activity className="w-6 h-6 text-indigo-600" />
            </div>
            <div>
              <p className="text-sm font-medium text-gray-900">Processed Items</p>
              <p className="text-2xl font-bold text-gray-900">Unavailable</p>
            </div>
          </div>
        </div>
        
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-4">
          <div className="flex items-center space-x-3">
            <div className="p-2 bg-green-100 rounded-lg">
              <Brain className="w-6 h-6 text-green-600" />
            </div>
            <div>
              <p className="text-sm font-medium text-gray-900">AI Accuracy</p>
              <p className="text-2xl font-bold text-gray-900">Unavailable</p>
            </div>
          </div>
        </div>
        
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-4">
          <div className="flex items-center space-x-3">
            <div className="p-2 bg-yellow-100 rounded-lg">
              <Zap className="w-6 h-6 text-yellow-600" />
            </div>
            <div>
              <p className="text-sm font-medium text-gray-900">Time Saved</p>
              <p className="text-2xl font-bold text-gray-900">Unavailable</p>
            </div>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-4">
          <div className="flex items-center justify-between mb-4">
            <h3 className="font-semibold text-gray-900">AI Performance</h3>
            <button
              onClick={() => toggleSectionExpansion('overview-perf')}
              className="p-1 hover:bg-gray-100 rounded-full transition-colors"
            >
              {expandedSections['overview-perf'] ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
            </button>
          </div>
          {expandedSections['overview-perf'] && (
            <div className="space-y-3">
              <div className="flex justify-between items-center">
                <span className="text-sm text-gray-600">Automation Rules</span>
                <span className="text-sm font-medium">Unavailable</span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-sm text-gray-600">Active Predictions</span>
                <span className="text-sm font-medium">Unavailable</span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-sm text-gray-600">Last Update</span>
                <span className="text-sm text-gray-600">
                  Unavailable
                </span>
              </div>
            </div>
          )}
        </div>

        <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-4">
          <div className="flex items-center justify-between mb-4">
            <h3 className="font-semibold text-gray-900">Quick Actions</h3>
            <Info className="w-4 h-4 text-gray-400" />
          </div>
          <div className="grid grid-cols-2 gap-2">
            <button
              onClick={() => handleSectionChange('suggestions')}
              className="px-3 py-2 bg-indigo-600 text-white text-sm rounded-lg hover:bg-indigo-700 transition-colors"
            >
              Get Suggestions
            </button>
            <button
              onClick={() => handleSectionChange('search')}
              className="px-3 py-2 bg-green-600 text-white text-sm rounded-lg hover:bg-green-700 transition-colors"
            >
              AI Search
            </button>
            <button
              onClick={() => handleSectionChange('analytics')}
              className="px-3 py-2 bg-purple-600 text-white text-sm rounded-lg hover:bg-purple-700 transition-colors"
            >
              View Analytics
            </button>
            <button
              onClick={() => handleSectionChange('automation')}
              className="px-3 py-2 bg-yellow-600 text-white text-sm rounded-lg hover:bg-yellow-700 transition-colors"
            >
              Manage Rules
            </button>
          </div>
        </div>
      </div>
    </div>
  );

  // Render content based on active section
  const renderContent = () => {
    switch (state.activeSection) {
      case 'overview':
        return renderOverview();
      
      case 'suggestions':
        return (
          <div className="space-y-6">
            <div className="flex items-center justify-between">
              <h2 className="text-xl font-semibold text-gray-900">AI-Powered Suggestions</h2>
              <button
                onClick={() => toggleSectionExpansion('suggestions')}
                className="p-2 hover:bg-gray-100 rounded-full transition-colors"
              >
                {expandedSections['suggestions'] ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
              </button>
            </div>
            {expandedSections['suggestions'] && (
              <AICollectionSuggestions
                onSuggestionAccept={handleSuggestionAccept}
                maxSuggestions={3}
              />
            )}
          </div>
        );
      
      case 'search':
        return (
          <div className="space-y-6">
            <div className="flex items-center justify-between">
              <h2 className="text-xl font-semibold text-gray-900">Natural Language Search</h2>
            </div>
            <AINaturalSearch onSearch={handleNaturalSearch} />
            
            <div className="mt-8">
              <h3 className="text-lg font-medium text-gray-900 mb-4">Content Categorization</h3>
              <AIContentCategorizer
                item={{
                  title: 'Sample Content Item',
                  description: 'This is a sample content item for demonstration'
                }}
                onCategorizationComplete={handleCategorizationComplete}
              />
            </div>
          </div>
        );
      
      case 'analytics':
        return (
          <div className="space-y-6">
            <div className="flex items-center justify-between">
              <h2 className="text-xl font-semibold text-gray-900">AI Analytics</h2>
            </div>
            
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
              <AIUserBehaviorAnalytics
                userId="demo-user"
                onActionImplement={handleActionImplement}
              />
              <AIPredictions
                onPredictionAction={handlePredictionAction}
              />
            </div>
            
            <div>
              <AISmartOrganization
                collections={[]}
                onSuggestionApply={handleSuggestionApply}
              />
            </div>
          </div>
        );
      
      case 'metadata':
        return (
          <div className="space-y-6">
            <div className="flex items-center justify-between">
              <h2 className="text-xl font-semibold text-gray-900">AI Metadata Services</h2>
            </div>
            
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
              <AIMetadataExtractor
                content={{
                  title: 'Sample Content',
                  description: 'Sample description for metadata extraction',
                  fileType: 'video/mp4',
                  size: 1024000
                }}
                onMetadataExtracted={handleMetadataExtracted}
              />
              <AIContentQualityAnalyzer
                content={{
                  title: 'Sample Content',
                  description: 'Sample description'
                }}
                onQualityImprovement={handleQualityImprovement}
              />
            </div>
          </div>
        );
      
      case 'automation':
        return (
          <div className="space-y-6">
            <div className="flex items-center justify-between">
              <h2 className="text-xl font-semibold text-gray-900">AI Automation</h2>
              <div className="flex items-center space-x-2">
                <span className="text-sm text-gray-600">Automation unavailable</span>
                <button
                  onClick={() => toggleSectionExpansion('automation')}
                  className="p-2 hover:bg-gray-100 rounded-full transition-colors"
                >
                  <RefreshCw className="w-4 h-4" />
                </button>
              </div>
            </div>
            
            <AIAutomationRules
              onRuleToggle={handleRuleToggle}
              onRuleExecute={handleRuleExecute}
            />
          </div>
        );
      
      default:
        return null;
    }
  };

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900 p-6">
      <div className="max-w-7xl mx-auto">
        <div className="mb-8">
          <div className="flex items-center space-x-3">
            <Brain className="w-8 h-8 text-indigo-600" />
            <h1 className="text-3xl font-bold text-gray-900 dark:text-white">AI Dashboard</h1>
            <Sparkles className="w-6 h-6 text-yellow-500" />
          </div>
          <p className="text-gray-600 dark:text-gray-400 mt-2">
            Advanced AI-powered features for intelligent content management and automation
          </p>
        </div>

        {/* Alerts */}
        {state.alerts.length > 0 && (
          <div className="mb-6 space-y-2">
            {state.alerts.map((alert, index) => (
              <div
                key={index}
                className={`p-3 rounded-lg border ${
                  alert.type === 'success' ? 'bg-green-50 border-green-200 text-green-800' :
                  alert.type === 'warning' ? 'bg-yellow-50 border-yellow-200 text-yellow-800' :
                  alert.type === 'error' ? 'bg-red-50 border-red-200 text-red-800' :
                  'bg-blue-50 border-blue-200 text-blue-800'
                }`}
              >
                <div className="flex items-center space-x-2">
                  <Info className="w-4 h-4" />
                  <span className="text-sm">{alert.message}</span>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Navigation */}
        {renderNavigationTabs()}

        {/* Content */}
        <Suspense fallback={<div className="flex items-center justify-center min-h-[300px]"><div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600" /></div>}>
          {renderContent()}
        </Suspense>
      </div>
    </div>
  );
};

export default AIDashboard;