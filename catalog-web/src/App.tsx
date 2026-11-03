import React, { Suspense, useState } from 'react'
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider } from '@/contexts/AuthContext'
import { WebSocketProvider } from '@/contexts/WebSocketContext'
import { Layout } from '@/components/layout/Layout'
import { ProtectedRoute } from '@/components/auth/ProtectedRoute'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { PageErrorBoundary } from '@/components/PageErrorBoundary'
import { SplashScreen } from '@/components/SplashScreen'
import { PerformanceOverlay } from '@/components/collections/PerformanceOptimizer'

// Lazy-loaded page components for code splitting
const LoginForm = React.lazy(() => import('@/components/auth/LoginForm').then(m => ({ default: m.LoginForm })))
const RegisterForm = React.lazy(() => import('@/components/auth/RegisterForm').then(m => ({ default: m.RegisterForm })))
const ForgotPassword = React.lazy(() => import('@/components/auth/ForgotPassword').then(m => ({ default: m.ForgotPassword })))
const Dashboard = React.lazy(() => import('@/pages/Dashboard').then(m => ({ default: m.Dashboard })))
const MediaBrowser = React.lazy(() => import('@/pages/MediaBrowser').then(m => ({ default: m.MediaBrowser })))
const Analytics = React.lazy(() => import('@/pages/Analytics').then(m => ({ default: m.Analytics })))
const SubtitleManager = React.lazy(() => import('@/pages/SubtitleManager').then(m => ({ default: m.SubtitleManager })))
const Collections = React.lazy(() => import('@/pages/Collections').then(m => ({ default: m.Collections })))
const ConversionTools = React.lazy(() => import('@/pages/ConversionTools').then(m => ({ default: m.ConversionTools })))
const Admin = React.lazy(() => import('@/pages/Admin').then(m => ({ default: m.Admin })))
const FavoritesPage = React.lazy(() => import('@/pages/Favorites'))
const PlaylistsPage = React.lazy(() => import('@/pages/Playlists').then(m => ({ default: m.PlaylistsPage })))
const AIDashboard = React.lazy(() => import('@/pages/AIDashboard'))
const EntityBrowser = React.lazy(() => import('@/pages/EntityBrowser').then(m => ({ default: m.EntityBrowser })))
const EntityDetail = React.lazy(() => import('@/pages/EntityDetail').then(m => ({ default: m.EntityDetail })))
const SettingsPage = React.lazy(() => import('@/pages/Settings').then(m => ({ default: m.Settings })))
const IdentityManagerPage = React.lazy(() => import('@/pages/IdentityManager'))

const PageLoader: React.FC = () => (
  <div className="p-6 space-y-4 animate-pulse min-h-[400px]">
    <div className="h-8 bg-gray-200 dark:bg-gray-700 rounded w-1/3" />
    <div className="h-4 bg-gray-200 dark:bg-gray-700 rounded w-2/3" />
    <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mt-6">
      <div className="h-32 bg-gray-200 dark:bg-gray-700 rounded" />
      <div className="h-32 bg-gray-200 dark:bg-gray-700 rounded" />
      <div className="h-32 bg-gray-200 dark:bg-gray-700 rounded" />
    </div>
    <div className="h-4 bg-gray-200 dark:bg-gray-700 rounded w-1/2 mt-4" />
    <div className="h-4 bg-gray-200 dark:bg-gray-700 rounded w-3/4" />
  </div>
)

function App() {
  const [splashComplete, setSplashComplete] = useState(false)

  if (!splashComplete) {
    return <SplashScreen onComplete={() => setSplashComplete(true)} />
  }

  return (
    <ErrorBoundary>
      <AuthProvider>
        <WebSocketProvider>
          <Router future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
            <Suspense fallback={<PageLoader />}>
              <Routes>
              {/* Public routes */}
              <Route path="/login" element={<LoginForm />} />
              <Route path="/register" element={<RegisterForm />} />
              <Route path="/forgot-password" element={<ForgotPassword />} />

              {/* Protected routes */}
              <Route path="/" element={<Layout />}>
                <Route index element={<Navigate to="/dashboard" replace />} />
                <Route
                  path="/dashboard"
                  element={
                    <ProtectedRoute>
                      <PageErrorBoundary pageName="Dashboard">
                        <Dashboard />
                      </PageErrorBoundary>
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/media"
                  element={
                    <ProtectedRoute requiredPermission="read:media">
                      <PageErrorBoundary pageName="Media Browser">
                        <MediaBrowser />
                      </PageErrorBoundary>
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/analytics"
                  element={
                    <ProtectedRoute requiredPermission="view:analysis">
                      <PageErrorBoundary pageName="Analytics">
                        <Analytics />
                      </PageErrorBoundary>
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/subtitles"
                  element={
                    <ProtectedRoute requiredPermission="manage:subtitles">
                      <PageErrorBoundary pageName="Subtitle Manager">
                        <SubtitleManager />
                      </PageErrorBoundary>
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/collections"
                  element={
                    <ProtectedRoute requiredPermission="read:collections">
                      <PageErrorBoundary pageName="Collections">
                        <Collections />
                      </PageErrorBoundary>
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/favorites"
                  element={
                    <ProtectedRoute>
                      <PageErrorBoundary pageName="Favorites">
                        <FavoritesPage />
                      </PageErrorBoundary>
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/playlists"
                  element={
                    <ProtectedRoute>
                      <PageErrorBoundary pageName="Playlists">
                        <PlaylistsPage />
                      </PageErrorBoundary>
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/conversion"
                  element={
                    <ProtectedRoute requiredPermission="convert:media">
                      <PageErrorBoundary pageName="Conversion Tools">
                        <ConversionTools />
                      </PageErrorBoundary>
                    </ProtectedRoute>
                  }
                />
                 <Route
                   path="/admin"
                   element={
                     <ProtectedRoute requireAdmin>
                       <PageErrorBoundary pageName="Admin">
                         <Admin />
                       </PageErrorBoundary>
                     </ProtectedRoute>
                   }
                 />
                <Route
                  path="/browse"
                  element={
                    <ProtectedRoute requiredPermission="read:media">
                      <PageErrorBoundary pageName="Entity Browser">
                        <EntityBrowser />
                      </PageErrorBoundary>
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/entity/:id"
                  element={
                    <ProtectedRoute requiredPermission="read:media">
                      <PageErrorBoundary pageName="Entity Detail">
                        <EntityDetail />
                      </PageErrorBoundary>
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/settings"
                  element={
                    <ProtectedRoute>
                      <PageErrorBoundary pageName="Settings">
                        <SettingsPage />
                      </PageErrorBoundary>
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/ai"
                  element={
                    <ProtectedRoute>
                      <PageErrorBoundary pageName="AI Dashboard">
                        <AIDashboard />
                      </PageErrorBoundary>
                    </ProtectedRoute>
                  }
                />
                <Route
                  path="/identities"
                  element={
                    <ProtectedRoute requiredPermission="read:media">
                      <PageErrorBoundary pageName="Identity &amp; Share Discovery">
                        <IdentityManagerPage />
                      </PageErrorBoundary>
                    </ProtectedRoute>
                  }
                />
              </Route>

              {/* Catch all route */}
              <Route path="*" element={<Navigate to="/dashboard" replace />} />
              </Routes>
            </Suspense>
          </Router>
        </WebSocketProvider>
        {import.meta.env.DEV && <PerformanceOverlay />}
      </AuthProvider>
    </ErrorBoundary>
  )
}

export default App
