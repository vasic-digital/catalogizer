import React, { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { cn } from '@/lib/utils'

interface Breadcrumb {
  label: string
  href?: string
}

interface PageHeaderProps {
  title: string
  subtitle?: string
  icon?: ReactNode
  actions?: ReactNode
  breadcrumbs?: Breadcrumb[]
  className?: string
}

/**
 * PageHeader renders a page title with optional subtitle, icon, breadcrumbs, and action buttons.
 */
export const PageHeader: React.FC<PageHeaderProps> = ({
  title,
  subtitle,
  icon,
  actions,
  breadcrumbs,
  className
}) => {
  return (
    <div className={cn(
      'mb-6',
      className
    )}>
      {/* Breadcrumbs */}
      {breadcrumbs && breadcrumbs.length > 0 && (
        <nav className="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400 mb-4">
          {breadcrumbs.map((crumb, index) => (
            <React.Fragment key={index}>
              {index > 0 && <span>/</span>}
              {crumb.href ? (
                <Link
                  to={crumb.href}
                  className="hover:text-gray-900 hover:underline dark:hover:text-white transition-colors"
                >
                  {crumb.label}
                </Link>
              ) : (
                <span className="text-gray-900 dark:text-white font-medium">
                  {crumb.label}
                </span>
              )}
            </React.Fragment>
          ))}
        </nav>
      )}

      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between bg-white dark:bg-gray-800 rounded-lg p-6 shadow-sm border border-gray-200 dark:border-gray-700">
        <div className="flex min-w-0 items-center gap-3">
          {icon && (
            <div className="p-2 bg-blue-100 dark:bg-blue-900/20 rounded-lg">
              {icon}
            </div>
          )}
          <div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
              {title}
            </h1>
            {subtitle && (
              <p className="text-gray-600 dark:text-gray-400 mt-1">
                {subtitle}
              </p>
            )}
          </div>
        </div>
        
        {actions && (
          <div className="flex flex-wrap items-center gap-2 mt-4 sm:mt-0">
            {actions}
          </div>
        )}
      </div>
    </div>
  )
}