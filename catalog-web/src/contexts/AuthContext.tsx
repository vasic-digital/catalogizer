import React, { createContext, useContext, useEffect, useState, ReactNode } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { authApi } from '@/lib/api'
import type { User, LoginRequest, RegisterRequest, ChangePasswordRequest, UpdateProfileRequest } from '@/types/auth'
// Network error detection for typed error narrowing in mutations.
// Note: @vasic-digital/catalogizer-api-client uses Node.js EventEmitter,
// so we use a duck-type check instead of instanceof for browser compat.
const isNetworkError = (err: unknown): boolean =>
  err instanceof Error && (err.name === 'NetworkError' || err.message.includes('Network'))
// The shared AuthContextType defines the canonical contract for auth contexts.
// The internal interface below extends the same shape; once react-query versions
// are aligned, AuthProvider can be replaced with the shared package's version.
import type { AuthContextType as _SharedAuthContextType } from '@vasic-digital/auth-context'
import toast from 'react-hot-toast'

interface AuthContextType {
  user: User | null
  isAuthenticated: boolean
  isLoading: boolean
  permissions: string[]
  isAdmin: boolean
  login: (data: LoginRequest) => Promise<unknown>
  register: (data: RegisterRequest) => Promise<unknown>
  logout: () => Promise<void>
  updateProfile: (data: UpdateProfileRequest) => Promise<unknown>
  changePassword: (data: ChangePasswordRequest) => Promise<void>
  hasPermission: (permission: string) => boolean
  canAccess: (resource: string, action: string) => boolean
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

/**
 * useAuth returns the authentication context including user state, login/logout
 * actions, and permission checks. Throws if used outside AuthProvider.
 *
 * @returns Auth state and actions
 */
export const useAuth = () => {
  const context = useContext(AuthContext)
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}

interface AuthProviderProps {
  children: ReactNode
}

/**
 * AuthProvider manages authentication state, token refresh, and permission
 * resolution, exposing login/register/logout actions to the component tree.
 */
export const AuthProvider: React.FC<AuthProviderProps> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null)
  const [permissions, setPermissions] = useState<string[]>([])
  const queryClient = useQueryClient()

  const { data: authStatus, isLoading } = useQuery({
    queryKey: ['auth-status'],
    queryFn: authApi.getAuthStatus,
    retry: (failureCount, error: unknown) => {
      const err = error as { response?: { status?: number } }
      if (err?.response?.status === 401) return false
      return failureCount < 2
    },
    staleTime: 1000 * 60 * 5,
  })

  const { data: permissionsData } = useQuery({
    queryKey: ['permissions'],
    queryFn: authApi.getPermissions,
    enabled: !!user,
    staleTime: 1000 * 60 * 10,
  })

  useEffect(() => {
    if (authStatus?.authenticated && authStatus.user) {
      setUser(authStatus.user)
      setPermissions(permissionsData?.permissions || authStatus.permissions || [])
    } else {
      setUser(null)
      setPermissions([])
    }
  }, [authStatus, permissionsData])

  const loginMutation = useMutation({
    mutationFn: authApi.login,
    onSuccess: (data) => {
      localStorage.setItem('auth_token', data.session_token)
      localStorage.setItem('user', JSON.stringify(data.user))
      setUser(data.user)
      queryClient.invalidateQueries({ queryKey: ['auth-status'] })
      queryClient.invalidateQueries({ queryKey: ['permissions'] })
      toast.success('Successfully logged in!')
    },
    onError: (error: unknown) => {
      // Use shared NetworkError for typed error narrowing
      if (isNetworkError(error)) {
        toast.error('Network error: please check your connection')
        return
      }
      const err = error as { response?: { data?: { error?: string } } }
      const message = err?.response?.data?.error || 'Login failed'
      toast.error(message)
    },
  })

  const registerMutation = useMutation({
    mutationFn: authApi.register,
    onSuccess: () => {
      toast.success('Registration successful! Please log in.')
    },
    onError: (error: unknown) => {
      if (isNetworkError(error)) {
        toast.error('Network error: please check your connection')
        return
      }
      const err = error as { response?: { data?: { error?: string } } }
      const message = err?.response?.data?.error || 'Registration failed'
      toast.error(message)
    },
  })

  const logoutMutation = useMutation({
    mutationFn: authApi.logout,
    onSuccess: () => {
      localStorage.removeItem('auth_token')
      localStorage.removeItem('user')
      setUser(null)
      setPermissions([])
      queryClient.clear()
      toast.success('Successfully logged out!')
    },
    onError: () => {
      localStorage.removeItem('auth_token')
      localStorage.removeItem('user')
      setUser(null)
      setPermissions([])
      queryClient.clear()
    },
  })

  const updateProfileMutation = useMutation({
    mutationFn: authApi.updateProfile,
    onSuccess: (updatedUser) => {
      setUser(updatedUser)
      queryClient.invalidateQueries({ queryKey: ['auth-status'] })
      toast.success('Profile updated successfully!')
    },
    onError: (error: unknown) => {
      const err = error as { response?: { data?: { error?: string } } }
      const message = err?.response?.data?.error || 'Profile update failed'
      toast.error(message)
    },
  })

  const changePasswordMutation = useMutation({
    mutationFn: authApi.changePassword,
    onSuccess: () => {
      toast.success('Password changed successfully!')
    },
    onError: (error: unknown) => {
      const err = error as { response?: { data?: { error?: string } } }
      const message = err?.response?.data?.error || 'Password change failed'
      toast.error(message)
    },
  })

  const isUserAdmin = user?.role?.name === 'Admin' || user?.role_id === 1

  const hasPermission = (permission: string): boolean => {
    if (isUserAdmin) return true
    return permissions.includes(permission)
  }

  const canAccess = (resource: string, action: string): boolean => {
    const permission = `${action}:${resource}`
    return hasPermission(permission) || hasPermission('admin:system')
  }

  const value: AuthContextType = {
    user,
    isAuthenticated: !!user,
    // Stay in the loading state until the effect above has copied the fetched
    // user into local state; otherwise ProtectedRoute sees one render with
    // isLoading=false && user=null and redirects a valid session to /login.
    isLoading: isLoading || (!!authStatus?.authenticated && !!authStatus.user && !user),
    permissions,
    isAdmin: isUserAdmin,
    login: loginMutation.mutateAsync,
    register: registerMutation.mutateAsync,
    logout: logoutMutation.mutateAsync,
    updateProfile: updateProfileMutation.mutateAsync,
    changePassword: changePasswordMutation.mutateAsync,
    hasPermission,
    canAccess,
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export default AuthContext