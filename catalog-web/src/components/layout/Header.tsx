import { ConnectionStatus } from '@/components/ui/ConnectionStatus'
import React, { useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useLocation } from 'react-router-dom'
import { useAuth } from '@/contexts/AuthContext'
import { Button } from '@/components/ui/Button'
import { ThemeToggle } from '@/components/ThemeToggle'
import { Menu, X, User, LogOut, Settings, Search, Folder, Heart, ListMusic, Library, XCircle, MoreHorizontal } from 'lucide-react'
import { motion, AnimatePresence } from 'framer-motion'

const navLinkClass = (pathname: string, href: string) => {
  const isActive = pathname === href || (href !== '/' && pathname.startsWith(href))
  return isActive
    ? 'touch-target flex-shrink-0 text-blue-600 dark:text-blue-400 font-semibold transition-colors rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-gray-900'
    : 'touch-target flex-shrink-0 text-gray-700 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white transition-colors rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-gray-900'
}

const mobileNavLinkClass = (pathname: string, href: string) => {
  const isActive = pathname === href || (href !== '/' && pathname.startsWith(href))
  return isActive
    ? 'block px-3 py-2 text-blue-600 dark:text-blue-400 font-semibold bg-blue-50 dark:bg-blue-900/20 rounded-md transition-colors'
    : 'block px-3 py-2 text-gray-700 hover:text-gray-900 hover:bg-gray-100 rounded-md dark:text-gray-300 dark:hover:text-white dark:hover:bg-gray-800 transition-colors'
}


interface NavItem {
  to: string
  label: string
  icon?: React.ComponentType<{ className?: string }>
  /** First breakpoint at which the item is shown inline; lower widths reach it through the "More" menu. */
  tier: 0 | 1 | 2 | 3
  adminOnly?: boolean
}

/**
 * Single source of truth for the primary navigation, in priority order. The desktop bar, the "More"
 * overflow menu and the mobile drawer all derive from it, so a destination can never exist in one and
 * be missing from another. Tier 0 is always inline from `md`; tier 1 from `lg`, 2 from `xl`, 3 from `2xl`.
 */
export const NAV_ITEMS: NavItem[] = [
  { to: '/dashboard', label: 'Dashboard', tier: 0 },
  { to: '/media', label: 'Media', tier: 0 },
  { to: '/browse', label: 'Browse', icon: Library, tier: 1 },
  { to: '/favorites', label: 'Favorites', icon: Heart, tier: 1 },
  { to: '/playlists', label: 'Playlists', icon: ListMusic, tier: 2 },
  { to: '/analytics', label: 'Analytics', tier: 2 },
  { to: '/subtitles', label: 'Subtitles', tier: 3 },
  { to: '/collections', label: 'Collections', icon: Folder, tier: 3 },
  { to: '/conversion', label: 'Convert', tier: 3 },
  { to: '/admin', label: 'Admin', tier: 3, adminOnly: true },
]

/** Visibility of an item inside the bar (hidden until its tier breakpoint) and inside the More menu (hidden once inline). */
const INLINE_BY_TIER = ['', 'hidden lg:block', 'hidden xl:block', 'hidden 2xl:block']
const MENU_HIDDEN_FROM_TIER = ['', 'lg:hidden', 'xl:hidden', '2xl:hidden']

const moreItemClass = (pathname: string, href: string, tier: number) => {
  const isActive = pathname === href || (href !== '/' && pathname.startsWith(href))
  return `flex items-center gap-2 px-3 min-h-[44px] text-sm rounded-md transition-colors ${MENU_HIDDEN_FROM_TIER[tier]} ${
    isActive
      ? 'text-blue-600 dark:text-blue-400 font-semibold bg-blue-50 dark:bg-blue-900/20'
      : 'text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800'
  }`
}

/**
 * Header renders the sticky top navigation bar with logo, navigation links,
 * search, and user account controls.
 */
export const Header: React.FC = () => {
  const { user, isAuthenticated, logout } = useAuth()
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false)
  const [searchValue, setSearchValue] = useState('')
  const [isMoreOpen, setIsMoreOpen] = useState(false)
  const moreRef = useRef<HTMLDivElement>(null)
  const moreButtonRef = useRef<HTMLButtonElement>(null)
  const navigate = useNavigate()
  const location = useLocation()

  const handleLogout = async () => {
    try {
      await logout()
      navigate('/login')
    } catch (error) {
      console.error("Error:", error);
      // Logout failure is non-critical — navigation already happened or will be retried
    }
  }


  // Close the overflow menu on Escape, on a click outside it and whenever the route changes.
  useEffect(() => {
    if (!isMoreOpen) return
    // Escape closes the disclosure and puts focus back on its trigger, so keyboard users never lose their place (WCAG 2.4.3).
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') { setIsMoreOpen(false); moreButtonRef.current?.focus() }
    }
    const onDown = (e: MouseEvent) => { if (moreRef.current && !moreRef.current.contains(e.target as Node)) setIsMoreOpen(false) }
    document.addEventListener('keydown', onKey)
    document.addEventListener('mousedown', onDown)
    return () => { document.removeEventListener('keydown', onKey); document.removeEventListener('mousedown', onDown) }
  }, [isMoreOpen])
  useEffect(() => { setIsMoreOpen(false) }, [location.pathname])

  const isAdmin = user?.role?.name === 'Admin'
  const navItems = NAV_ITEMS.filter((i) => !i.adminOnly || isAdmin)

  const toggleMobileMenu = () => setIsMobileMenuOpen(!isMobileMenuOpen)

  return (
    <header className="sticky top-0 z-50 bg-white/80 backdrop-blur-md border-b border-gray-200 dark:bg-gray-900/80 dark:border-gray-700">
      <div className="max-w-7xl 2xl:max-w-screen-2xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex justify-between items-center h-16">
          {/* Logo */}
          <Link
            to="/"
            className="flex flex-shrink-0 items-center space-x-2 [@media(pointer:coarse)]:min-h-[44px] text-xl font-bold text-gray-900 dark:text-white"
          >
            <div className="w-8 h-8 bg-gradient-to-r from-blue-600 to-purple-600 rounded-lg flex items-center justify-center">
              <span className="text-white font-bold text-sm">C</span>
            </div>
            <span>Catalogizer</span>
          </Link>

          {/* Desktop Navigation: priority items inline, the rest behind "More" below 2xl */}
          {isAuthenticated && (
            <div className="hidden md:flex items-center flex-1 min-w-0">
              <nav className="flex items-center space-x-4 min-w-0 overflow-x-auto whitespace-nowrap px-1 py-1 [scrollbar-width:thin]">
                {navItems.map(({ to, label, icon: Icon, tier }) => (
                  <Link key={to} to={to} className={`${INLINE_BY_TIER[tier]} ${navLinkClass(location.pathname, to)}`.trim()}>
                    {Icon ? (
                      <div className="flex items-center gap-1">
                        <Icon className="h-4 w-4" />
                        {label}
                      </div>
                    ) : (
                      label
                    )}
                  </Link>
                ))}
              </nav>
              <div ref={moreRef} className="relative flex-none ml-2 2xl:hidden">
                <button
                  type="button"
                  ref={moreButtonRef}
                  aria-expanded={isMoreOpen}
                  aria-controls="header-more-list"
                  onClick={() => setIsMoreOpen((o) => !o)}
                  className="inline-flex items-center gap-1 px-2 min-h-[44px] rounded-md text-gray-700 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
                >
                  <MoreHorizontal className="h-5 w-5" aria-hidden="true" />
                  <span>More</span>
                </button>
                {isMoreOpen && (
                  <ul
                    id="header-more-list"
                    aria-label="More navigation"
                    className="absolute right-0 top-full mt-2 w-56 p-1 z-50 list-none m-0 bg-white border border-gray-200 rounded-lg shadow-lg dark:bg-gray-900 dark:border-gray-700"
                  >
                    {navItems.filter((i) => i.tier > 0).map(({ to, label, icon: Icon, tier }) => (
                      <li key={to} className={MENU_HIDDEN_FROM_TIER[tier]}>
                        <Link
                          to={to}
                          className={moreItemClass(location.pathname, to, tier)}
                          onClick={() => { setIsMoreOpen(false); moreButtonRef.current?.focus() }}
                        >
                          {Icon && <Icon className="h-4 w-4" />}
                          {label}
                        </Link>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </div>
          )}

          {/* Search Bar */}
          {isAuthenticated && (
            <div className="hidden lg:flex items-center flex-none w-44 xl:w-56 mx-3">
              <div className="relative w-full">
                <label htmlFor="header-search" className="sr-only">Search media</label>
                <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-500 dark:text-gray-400" />
                <input
                  id="header-search"
                  type="text"
                  value={searchValue}
                  onChange={(e) => setSearchValue(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && searchValue.trim()) {
                      navigate(`/browse?search=${encodeURIComponent(searchValue.trim())}`)
                    }
                  }}
                  placeholder="Search movies, shows, music..."
                  className="w-full pl-10 pr-14 py-2 bg-gray-100 border border-gray-300 rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:border-transparent dark:bg-gray-800 dark:border-gray-600 dark:text-white dark:placeholder:text-gray-500"
                />
                {searchValue && (
                  <button
                    type="button"
                    onClick={() => setSearchValue('')}
                    className="absolute right-3 top-1/2 transform -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                    aria-label="Clear search"
                  >
                    <XCircle className="h-4 w-4" />
                  </button>
                )}
              </div>
            </div>
          )}

          {/* Compact search entry for tablet widths (full box shows from lg). No search input exists at md, so it opens Browse, which has its own search field. */}
          {isAuthenticated && (
            <Link
              to="/browse"
              aria-label="Search"
              className="hidden md:inline-flex lg:hidden touch-target items-center justify-center rounded-md text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white mx-1"
            >
              <Search className="h-5 w-5" />
            </Link>
          )}

          {/* Desktop User Menu */}
          <div className="hidden md:flex items-center space-x-4">
            {isAuthenticated ? (
              <div className="flex items-center space-x-4">
                <span className="hidden min-[1700px]:inline text-sm text-gray-700 dark:text-gray-300">
                  Welcome, {user?.first_name || user?.username}
                </span>
                <div className="flex items-center space-x-2">
                  <ThemeToggle />
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => navigate('/profile')}
                    aria-label="Profile"
                    className="h-8 w-8"
                  >
                    <User className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => navigate('/settings')}
                    aria-label="Settings"
                    className="h-8 w-8"
                  >
                    <Settings className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={handleLogout}
                    aria-label="Log out"
                    className="h-8 w-8"
                  >
                    <LogOut className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            ) : (
              <div className="flex items-center space-x-4">
                <ThemeToggle />
                <Button variant="ghost" onClick={() => navigate('/login')}>
                  Login
                </Button>
                <Button onClick={() => navigate('/register')}>
                  Sign Up
                </Button>
              </div>
            )}
          </div>

          {/* Connection status: docked in the header (never an overlay), shown only while not connected */}
          <div className="flex-none ml-2 md:ml-3">
            <ConnectionStatus />
          </div>

          {/* Mobile Menu Button */}
          <Button
            variant="ghost"
            size="icon"
            className="md:hidden"
            onClick={toggleMobileMenu}
            aria-label={isMobileMenuOpen ? 'Close menu' : 'Open menu'}
            aria-expanded={isMobileMenuOpen}
          >
            {isMobileMenuOpen ? <X className="h-6 w-6" /> : <Menu className="h-6 w-6" />}
          </Button>
        </div>
      </div>

      {/* Mobile Menu */}
      <AnimatePresence>
        {isMobileMenuOpen && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
            className="md:hidden bg-white border-t border-gray-200 dark:bg-gray-900 dark:border-gray-700"
          >
            <div className="px-4 py-4 space-y-4">
              {/* Mobile Search */}
              {isAuthenticated && (
                <div className="relative">
                  <label htmlFor="mobile-search" className="sr-only">Search media</label>
                  <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-500 dark:text-gray-400" />
                  <input
                    id="mobile-search"
                    type="text"
                    value={searchValue}
                    onChange={(e) => setSearchValue(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && searchValue.trim()) {
                        navigate(`/browse?search=${encodeURIComponent(searchValue.trim())}`)
                        setIsMobileMenuOpen(false)
                      }
                    }}
                    placeholder="Search movies, shows, music..."
                    className="w-full pl-10 pr-14 py-2 bg-gray-100 border border-gray-300 rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:border-transparent dark:bg-gray-800 dark:border-gray-600 dark:text-white dark:placeholder:text-gray-500"
                  />
                  {searchValue && (
                    <button
                      type="button"
                      onClick={() => setSearchValue('')}
                      className="absolute right-3 top-1/2 transform -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                      aria-label="Clear search"
                    >
                      <XCircle className="h-4 w-4" />
                    </button>
                  )}
                </div>
              )}

              {/* Mobile Navigation */}
              {isAuthenticated ? (
                <>
                  <div className="space-y-1">
                    {navItems.map(({ to, label, icon: Icon }) => (
                      <Link
                        key={to}
                        to={to}
                        className={mobileNavLinkClass(location.pathname, to)}
                        onClick={() => setIsMobileMenuOpen(false)}
                      >
                        {Icon ? (
                          <div className="flex items-center gap-2">
                            <Icon className="h-4 w-4" />
                            {label}
                          </div>
                        ) : (
                          label
                        )}
                      </Link>
                    ))}
                  </div>

                  <div className="border-t border-gray-200 dark:border-gray-700 pt-4">
                    <div className="flex items-center justify-between mb-3">
                      <span className="text-sm text-gray-700 dark:text-gray-300">
                        {user?.first_name || user?.username}
                      </span>
                      <ThemeToggle />
                    </div>
                    <div className="space-y-2">
                      <Link
                        to="/profile"
                        className="block px-3 py-2 text-gray-700 hover:text-gray-900 hover:bg-gray-100 rounded-md dark:text-gray-300 dark:hover:text-white dark:hover:bg-gray-800 transition-colors"
                        onClick={() => setIsMobileMenuOpen(false)}
                      >
                        Profile
                      </Link>
                      <Link
                        to="/settings"
                        className="block px-3 py-2 text-gray-700 hover:text-gray-900 hover:bg-gray-100 rounded-md dark:text-gray-300 dark:hover:text-white dark:hover:bg-gray-800 transition-colors"
                        onClick={() => setIsMobileMenuOpen(false)}
                      >
                        Settings
                      </Link>
                      <button
                        onClick={() => {
                          handleLogout()
                          setIsMobileMenuOpen(false)
                        }}
                        className="block w-full text-left px-3 py-2 text-red-600 hover:text-red-700 hover:bg-red-50 rounded-md dark:text-red-400 dark:hover:text-red-300 dark:hover:bg-red-900/20 transition-colors"
                      >
                        Logout
                      </button>
                    </div>
                  </div>
                </>
              ) : (
                <div className="space-y-2">
                  <div className="flex items-center justify-between px-3 pb-1">
                    <span className="text-sm text-gray-700 dark:text-gray-300">Theme</span>
                    <ThemeToggle />
                  </div>
                  <Link
                    to="/login"
                    className="block px-3 py-2 text-center bg-gray-100 text-gray-900 rounded-md hover:bg-gray-200 transition-colors dark:bg-gray-800 dark:text-white dark:hover:bg-gray-700"
                    onClick={() => setIsMobileMenuOpen(false)}
                  >
                    Login
                  </Link>
                  <Link
                    to="/register"
                    className="block px-3 py-2 text-center bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors"
                    onClick={() => setIsMobileMenuOpen(false)}
                  >
                    Sign Up
                  </Link>
                </div>
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </header>
  )
}