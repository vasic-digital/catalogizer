import React, { useState } from 'react'
import { Link, useNavigate, useLocation } from 'react-router-dom'
import { useAuth } from '@/contexts/AuthContext'
import { Button } from '@/components/ui/Button'
import { ThemeToggle } from '@/components/ThemeToggle'
import { Menu, X, User, LogOut, Settings, Search, Folder, Heart, ListMusic, Library, XCircle } from 'lucide-react'
import { motion, AnimatePresence } from 'framer-motion'

const navLinkClass = (pathname: string, href: string) => {
  const isActive = pathname === href || (href !== '/' && pathname.startsWith(href))
  return isActive
    ? 'text-blue-600 dark:text-blue-400 font-semibold transition-colors rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-gray-900'
    : 'text-gray-700 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white transition-colors rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-gray-900'
}

const mobileNavLinkClass = (pathname: string, href: string) => {
  const isActive = pathname === href || (href !== '/' && pathname.startsWith(href))
  return isActive
    ? 'block px-3 py-2 text-blue-600 dark:text-blue-400 font-semibold bg-blue-50 dark:bg-blue-900/20 rounded-md transition-colors'
    : 'block px-3 py-2 text-gray-700 hover:text-gray-900 hover:bg-gray-100 rounded-md dark:text-gray-300 dark:hover:text-white dark:hover:bg-gray-800 transition-colors'
}

/**
 * Header renders the sticky top navigation bar with logo, navigation links,
 * search, and user account controls.
 */
export const Header: React.FC = () => {
  const { user, isAuthenticated, logout } = useAuth()
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false)
  const [searchValue, setSearchValue] = useState('')
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

          {/* Desktop Navigation */}
          {isAuthenticated && (
            <nav className="hidden md:flex items-center space-x-4 2xl:space-x-8 flex-1 min-w-0 overflow-x-auto whitespace-nowrap [scrollbar-width:thin]">
              <Link to="/dashboard" className={navLinkClass(location.pathname, '/dashboard')}>
                Dashboard
              </Link>
              <Link to="/media" className={navLinkClass(location.pathname, '/media')}>
                Media
              </Link>
              <Link to="/browse" className={navLinkClass(location.pathname, '/browse')}>
                <div className="flex items-center gap-1">
                  <Library className="h-4 w-4" />
                  Browse
                </div>
              </Link>
              <Link to="/favorites" className={navLinkClass(location.pathname, '/favorites')}>
                <div className="flex items-center gap-1">
                  <Heart className="h-4 w-4" />
                  Favorites
                </div>
              </Link>
              <Link to="/playlists" className={navLinkClass(location.pathname, '/playlists')}>
                <div className="flex items-center gap-1">
                  <ListMusic className="h-4 w-4" />
                  Playlists
                </div>
              </Link>
              <Link to="/analytics" className={navLinkClass(location.pathname, '/analytics')}>
                Analytics
              </Link>
              <Link to="/subtitles" className={navLinkClass(location.pathname, '/subtitles')}>
                Subtitles
              </Link>
              <Link to="/collections" className={navLinkClass(location.pathname, '/collections')}>
                <div className="flex items-center gap-1">
                  <Folder className="h-4 w-4" />
                  Collections
                </div>
              </Link>
              <Link to="/conversion" className={navLinkClass(location.pathname, '/conversion')}>
                Convert
              </Link>
              {user?.role?.name === 'Admin' && (
                <Link to="/admin" className={navLinkClass(location.pathname, '/admin')}>
                  Admin
                </Link>
              )}
            </nav>
          )}

          {/* Search Bar */}
          {isAuthenticated && (
            <div className="hidden md:flex items-center flex-none w-40 lg:w-44 xl:w-56 mx-3">
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
                  className="w-full pl-10 pr-10 py-2 bg-gray-100 border border-gray-300 rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:border-transparent dark:bg-gray-800 dark:border-gray-600 dark:text-white dark:placeholder:text-gray-500"
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

          {/* Desktop User Menu */}
          <div className="hidden md:flex items-center space-x-4">
            {isAuthenticated ? (
              <div className="flex items-center space-x-4">
                <span className="hidden 2xl:inline text-sm text-gray-700 dark:text-gray-300">
                  Welcome, {user?.first_name || user?.username}
                </span>
                <div className="flex items-center space-x-2">
                  <ThemeToggle />
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => navigate('/profile')}
                    className="h-8 w-8"
                  >
                    <User className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => navigate('/settings')}
                    className="h-8 w-8"
                  >
                    <Settings className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={handleLogout}
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

          {/* Mobile Menu Button */}
          <Button
            variant="ghost"
            size="icon"
            className="md:hidden"
            onClick={toggleMobileMenu}
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
                    className="w-full pl-10 pr-10 py-2 bg-gray-100 border border-gray-300 rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:border-transparent dark:bg-gray-800 dark:border-gray-600 dark:text-white dark:placeholder:text-gray-500"
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
                    <Link
                      to="/dashboard"
                      className={mobileNavLinkClass(location.pathname, '/dashboard')}
                      onClick={() => setIsMobileMenuOpen(false)}
                    >
                      Dashboard
                    </Link>
                    <Link
                      to="/media"
                      className={mobileNavLinkClass(location.pathname, '/media')}
                      onClick={() => setIsMobileMenuOpen(false)}
                    >
                      Media
                    </Link>
                    <Link
                      to="/browse"
                      className={mobileNavLinkClass(location.pathname, '/browse')}
                      onClick={() => setIsMobileMenuOpen(false)}
                    >
                      <div className="flex items-center gap-2">
                        <Library className="h-4 w-4" />
                        Browse
                      </div>
                    </Link>
                    <Link
                      to="/favorites"
                      className={mobileNavLinkClass(location.pathname, '/favorites')}
                      onClick={() => setIsMobileMenuOpen(false)}
                    >
                      <div className="flex items-center gap-2">
                        <Heart className="h-4 w-4" />
                        Favorites
                      </div>
                    </Link>
                    <Link
                      to="/playlists"
                      className={mobileNavLinkClass(location.pathname, '/playlists')}
                      onClick={() => setIsMobileMenuOpen(false)}
                    >
                      <div className="flex items-center gap-2">
                        <ListMusic className="h-4 w-4" />
                        Playlists
                      </div>
                    </Link>
                    <Link
                      to="/subtitles"
                      className={mobileNavLinkClass(location.pathname, '/subtitles')}
                      onClick={() => setIsMobileMenuOpen(false)}
                    >
                      Subtitles
                    </Link>
                    <Link
                      to="/collections"
                      className={mobileNavLinkClass(location.pathname, '/collections')}
                      onClick={() => setIsMobileMenuOpen(false)}
                    >
                      <div className="flex items-center gap-2">
                        <Folder className="h-4 w-4" />
                        Collections
                      </div>
                    </Link>
                    <Link
                      to="/conversion"
                      className={mobileNavLinkClass(location.pathname, '/conversion')}
                      onClick={() => setIsMobileMenuOpen(false)}
                    >
                      Convert
                    </Link>
                    {user?.role?.name === 'Admin' && (
                      <Link
                        to="/admin"
                        className={mobileNavLinkClass(location.pathname, '/admin')}
                        onClick={() => setIsMobileMenuOpen(false)}
                      >
                        Admin
                      </Link>
                    )}
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