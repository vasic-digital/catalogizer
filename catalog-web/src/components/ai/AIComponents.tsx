import React from 'react';

// Types for AI-powered suggestions
interface AISuggestion {
  id: string;
  title: string;
  description: string;
  type: 'collection' | 'smart-search' | 'content-categorization' | 'tag-suggestion';
  confidence: number;
  reasoning: string;
  metadata: {
    relatedItems: number;
    estimatedTimeSaved: string;
    category?: string;
    tags?: string[];
  };
}

interface AICategorizationResult {
  id: string;
  category: string;
  subcategory: string;
  confidence: number;
  reasoning: string;
  suggestedTags: string[];
  metadata: {
    contentType: string;
    quality: 'high' | 'medium' | 'low';
    completion: number;
  };
}

interface AISearchQuery {
  query: string;
  intent: 'browse' | 'search' | 'compare' | 'organize';
  entities: string[];
  filters: Record<string, unknown>;
  naturalLanguage: boolean;
}

/**
 * The server has no AI engine behind these panels. The earlier version used a
 * client-side mock service (timers, Math.random scores and canned text) and
 * presented the output as real analysis. They now say plainly that the feature
 * is unavailable and show no invented data (CAT-001, constitution 11.4.289).
 * Props are kept so existing pages compile unchanged.
 */

interface UnavailablePanelProps {
  title: string;
  testId: string;
  reason: string;
}

const UnavailablePanel: React.FC<UnavailablePanelProps> = ({ title, testId, reason }) => (
  <div className="bg-white rounded-lg shadow p-6" data-testid={testId}>
    <h3 className="text-lg font-semibold text-gray-900 mb-2">{title}</h3>
    <p className="font-medium text-gray-700">Unavailable</p>
    <p className="text-sm text-gray-500">{reason}</p>
  </div>
);

interface AICollectionSuggestionsProps {
  onSuggestionAccept: (suggestion: AISuggestion) => void;
  maxSuggestions?: number;
}
interface AINaturalSearchProps {
  onSearch: (query: AISearchQuery) => void;
  placeholder?: string;
}
interface AIContentCategorizerProps {
  item: {
    title: string;
    description?: string;
    metadata?: unknown;
  };
  onCategorizationComplete: (result: AICategorizationResult) => void;
}

export const AICollectionSuggestions: React.FC<AICollectionSuggestionsProps> = () => (
  <UnavailablePanel title="AI Collection Suggestions" testId="ai-suggestions-unavailable" reason="This server has no suggestion engine, so no collection suggestions can be shown." />
);

export const AINaturalSearch: React.FC<AINaturalSearchProps> = () => (
  <UnavailablePanel title="Natural Language Search" testId="ai-nl-search-unavailable" reason="This server has no natural-language query engine; use the regular search." />
);

export const AIContentCategorizer: React.FC<AIContentCategorizerProps> = () => (
  <UnavailablePanel title="Content Categorizer" testId="ai-categorizer-unavailable" reason="This server has no categorisation engine, so no categories can be proposed." />
);

export type { AISuggestion, AICategorizationResult, AISearchQuery };
