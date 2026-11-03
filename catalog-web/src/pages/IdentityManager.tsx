import React from 'react'
import { motion } from 'framer-motion'
import { IdentityManager } from '@/components/identity/IdentityManager'
import { DiscoveredShares } from '@/components/identity/DiscoveredShares'
import { KeyRound, Network } from 'lucide-react'

/**
 * IdentityManager page — tabbed interface with Identity Manager +
 * Discovered Shares views.
 *
 * Wraps the two components in the standard page layout pattern
 * (max-w-7xl, mx-auto, padding, framer-motion fade-in).
 * Routes are registered in App.tsx under /identities.
 */

const IdentityManagerPage: React.FC = () => {
  const [tab, setTab] = React.useState<'identities' | 'shares'>('identities')

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5 }}
      >
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-gray-900 dark:text-white mb-2">
            Identity &amp; Share Discovery
          </h1>
          <p className="text-gray-600 dark:text-gray-400">
            Manage network identities and browse discovered shares
          </p>
        </div>

        <div className="inline-flex h-10 max-w-full overflow-x-auto items-center justify-start sm:justify-center rounded-md bg-gray-100 dark:bg-gray-800 p-1 text-gray-600 dark:text-gray-400 mb-6">
          <button
            onClick={() => setTab('identities')}
            className={`inline-flex flex-shrink-0 items-center justify-center whitespace-nowrap rounded-sm px-3 py-1.5 text-sm font-medium transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 ${
              tab === 'identities'
                ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm'
                : 'hover:bg-gray-200 dark:hover:bg-gray-700 hover:text-gray-900 dark:hover:text-white'
            }`}
          >
            <KeyRound className="h-4 w-4 mr-2" />
            Identity Manager
          </button>
          <button
            onClick={() => setTab('shares')}
            className={`inline-flex flex-shrink-0 items-center justify-center whitespace-nowrap rounded-sm px-3 py-1.5 text-sm font-medium transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 ${
              tab === 'shares'
                ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm'
                : 'hover:bg-gray-200 dark:hover:bg-gray-700 hover:text-gray-900 dark:hover:text-white'
            }`}
          >
            <Network className="h-4 w-4 mr-2" />
            Discovered Shares
          </button>
        </div>

        {tab === 'identities' && <IdentityManager />}
        {tab === 'shares' && <DiscoveredShares />}
      </motion.div>
    </div>
  )
}

export default IdentityManagerPage
