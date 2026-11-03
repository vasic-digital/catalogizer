import React from 'react'
import { Outlet } from 'react-router-dom'
import { Header } from './Header'

/**
 * Layout provides the top-level page structure with a sticky header and routed content area.
 */
export const Layout: React.FC = () => {
  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      <Header />
      <main className="flex-1">
        <Outlet />
      </main>
      <span className="fixed bottom-2 right-3 text-xs text-slate-500/70 dark:text-slate-400/60 select-none pointer-events-none">v1.1.0</span>
    </div>
  )
}

export default Layout