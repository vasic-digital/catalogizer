import React from 'react';
import { Zap, Target, Lightbulb } from 'lucide-react';

/**
 * AI analytics panels.
 *
 * The server has no behaviour-analytics, prediction or organisation-suggestion
 * engine. The earlier version of this file answered from a client-side mock
 * service (timers plus canned patterns, forecasts and suggestions) and
 * presented them as real analysis. These panels now say plainly that the
 * feature is unavailable and show no invented data (CAT-001, §11.4.289).
 * The component props are kept so existing pages compile unchanged.
 */

interface UnavailablePanelProps {
  title: string;
  testId: string;
  icon: React.ReactNode;
  reason: string;
}

const UnavailablePanel: React.FC<UnavailablePanelProps> = ({ title, testId, icon, reason }) => (
  <div className="bg-white rounded-lg shadow p-6" data-testid={testId}>
    <div className="flex items-center gap-2 mb-3">
      {icon}
      <h3 className="text-lg font-semibold text-gray-900">{title}</h3>
    </div>
    <p className="font-medium text-gray-700">Unavailable</p>
    <p className="text-sm text-gray-500">{reason}</p>
  </div>
);

interface AIUserBehaviorAnalyticsProps {
  userId: string;
  onActionImplement: (action: string) => void;
}

export const AIUserBehaviorAnalytics: React.FC<AIUserBehaviorAnalyticsProps> = () => (
  <UnavailablePanel
    title="User Behavior Analytics"
    testId="ai-behavior-unavailable"
    icon={<Target className="w-5 h-5 text-gray-400" />}
    reason="This server does not record or analyse user behaviour, so no patterns can be shown."
  />
);

interface AIPredictionsProps {
  onPredictionAction: (predictionId: string, actionId: string) => void;
}

export const AIPredictions: React.FC<AIPredictionsProps> = () => (
  <UnavailablePanel
    title="AI Predictions"
    testId="ai-predictions-unavailable"
    icon={<Zap className="w-5 h-5 text-gray-400" />}
    reason="This server has no prediction engine, so no forecasts can be shown."
  />
);

interface AISmartOrganizationProps {
  collections: unknown[];
  onSuggestionApply: (suggestionId: string) => void;
}

export const AISmartOrganization: React.FC<AISmartOrganizationProps> = () => (
  <UnavailablePanel
    title="Smart Organization"
    testId="ai-organization-unavailable"
    icon={<Lightbulb className="w-5 h-5 text-gray-400" />}
    reason="This server has no organisation-suggestion engine, so no suggestions can be shown."
  />
);
