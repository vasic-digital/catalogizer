import React from 'react';
import { Zap } from 'lucide-react';

/**
 * Collection automation rules.
 *
 * The server has no automation engine or rule store (no endpoint creates,
 * stores, schedules or runs a rule). The earlier version of this component
 * showed hard-coded sample rules and "executed" them with a timer, which made
 * a rule that never ran look successful. It now says plainly that automation
 * is not available (CAT-001, constitution §11.4.289).
 */
const CollectionAutomation: React.FC = () => (
  <div className="space-y-4" data-testid="automation-unavailable">
    <div className="flex items-center justify-between">
      <div>
        <h3 className="text-lg font-semibold text-gray-900">Automation Rules</h3>
        <p className="text-sm text-gray-500">Create workflows to automatically manage your collections</p>
      </div>
      <button
        type="button"
        disabled
        className="inline-flex items-center gap-2 px-3 py-2 rounded bg-gray-200 text-gray-500 cursor-not-allowed"
        title="Collection automation is not available on this server"
      >
        <Zap className="w-4 h-4" />
        Create Rule
      </button>
    </div>
    <div className="rounded border border-gray-200 bg-gray-50 p-6 text-center">
      <p className="font-medium text-gray-700">Automation unavailable</p>
      <p className="text-sm text-gray-500">
        This server has no automation engine, so rules cannot be created, stored or run.
      </p>
    </div>
  </div>
);

export default CollectionAutomation;
