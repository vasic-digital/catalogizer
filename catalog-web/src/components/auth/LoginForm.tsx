import React, { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '@/contexts/AuthContext'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { User, Lock, Eye, EyeOff, AlertCircle } from 'lucide-react'
import { motion } from 'framer-motion'

/**
 * LoginForm renders the user authentication form with username and password fields.
 */
export const LoginForm: React.FC = () => {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  // Article XI §11.5: a persistent inline error message is shown
  // alongside the toast so users have a discoverable, non-transient
  // explanation when login fails. The toast alone disappears in ~4 s;
  // a user who looks down at the keyboard while submitting (or a
  // Playwright suite that captures a few seconds post-click) loses
  // the diagnostic. Caught by the 2026-04-29 anti-bluff sweep —
  // wrong-password previously left the form in a state visually
  // indistinguishable from the initial render.
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const { login } = useAuth()
  const navigate = useNavigate()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!username.trim() || !password.trim()) return

    setErrorMessage(null)
    setIsLoading(true)
    try {
      const deviceInfo = {
        device_type: 'desktop',
        platform: 'web',
        platform_version: navigator.userAgent,
        app_version: '1.0.0',
        device_name: 'Web Browser'
      }

      await login({
        username: username.trim(),
        password,
        device_info: deviceInfo,
        remember_me: false
      })
      navigate('/dashboard')
    } catch (error) {
      // Capture a human-readable error for inline display in addition
      // to the toast that AuthContext.loginMutation.onError fires.
      const err = error as { response?: { data?: { error?: string } }; message?: string }
      const apiMessage = err?.response?.data?.error
      const fallbackMessage = err?.message
      setErrorMessage(
        apiMessage ||
        fallbackMessage ||
        'Login failed. Please check your username and password.'
      )
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-blue-50 to-indigo-100 dark:from-gray-900 dark:to-gray-800 px-4">
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5 }}
        className="w-full max-w-md"
      >
        <Card className="shadow-2xl border-0 bg-white/80 backdrop-blur-md dark:bg-gray-800/80">
          <CardHeader className="space-y-1">
            <div className="flex justify-center mb-4">
              <div className="w-12 h-12 bg-gradient-to-r from-blue-600 to-purple-600 rounded-full flex items-center justify-center">
                <span className="text-white font-bold text-xl">C</span>
              </div>
            </div>
            <CardTitle className="text-2xl text-center">Welcome back</CardTitle>
            <CardDescription className="text-center">
              Enter your credentials to access Catalogizer
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4" noValidate>
              {errorMessage && (
                <div
                  role="alert"
                  data-testid="login-error"
                  className="flex items-start gap-2 rounded-md border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-700 dark:bg-red-950 dark:text-red-200"
                >
                  <AlertCircle className="mt-0.5 h-4 w-4 flex-shrink-0" aria-hidden />
                  <span>{errorMessage}</span>
                </div>
              )}
              <Input
                label="Username"
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="Enter your username"
                icon={<User className="h-4 w-4" />}
                required
              />

              <div className="space-y-2">
                <label htmlFor="login-password" className="text-sm font-medium text-gray-700 dark:text-gray-200">
                  Password
                </label>
                <div className="relative">
                  <Lock className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-500 dark:text-gray-400" />
                  <Input
                    id="login-password"
                    type={showPassword ? 'text' : 'password'}
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder="Enter your password"
                    className="pl-10 pr-12"
                    required
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute right-0 top-1/2 transform -translate-y-1/2 flex h-11 w-11 items-center justify-center text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2"
                    aria-label={showPassword ? 'Hide password' : 'Show password'}
                  >
                    {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </button>
                </div>
              </div>

              <div className="flex items-center justify-between">
                <div className="flex items-center">
                  <input
                    id="remember-me"
                    name="remember-me"
                    type="checkbox"
                    className="h-4 w-4 text-blue-600 focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 border-gray-300 rounded"
                  />
                  <label htmlFor="remember-me" className="ml-2 flex items-center [@media(pointer:coarse)]:min-h-[44px] [@media(pointer:coarse)]:min-w-[44px] text-sm text-gray-900 dark:text-gray-300">
                    Remember me
                  </label>
                </div>
                <Link
                  to="/forgot-password"
                  className="inline-flex items-center [@media(pointer:coarse)]:min-h-[44px] [@media(pointer:coarse)]:min-w-[44px] text-sm text-blue-600 hover:text-blue-500 hover:underline dark:text-blue-400 dark:hover:text-blue-300"
                >
                  Forgot password?
                </Link>
              </div>

              <Button
                type="submit"
                className="w-full"
                loading={isLoading}
                disabled={!username.trim() || !password.trim()}
              >
                Sign in
              </Button>
            </form>

            <div className="mt-6">
              <div className="relative">
                <div className="absolute inset-0 flex items-center">
                  <div className="w-full border-t border-gray-300 dark:border-gray-600" />
                </div>
                <div className="relative flex justify-center text-sm">
                  <span className="px-2 bg-white text-gray-600 dark:bg-gray-800 dark:text-gray-300">
                    Don&apos;t have an account?
                  </span>
                </div>
              </div>
              <div className="mt-6">
                <Link to="/register" className="block">
                  <Button variant="outline" className="w-full">
                    Create new account
                  </Button>
                </Link>
              </div>
            </div>
          </CardContent>
        </Card>
      </motion.div>
      <div className="mt-8 text-center text-xs text-gray-500 dark:text-gray-400">
        <p>Made with &#10084; by Vasic Digital</p>
        <p className="mt-1">&copy; {new Date().getFullYear()} Vasic Digital. All rights reserved.</p>
      </div>
    </div>
  )
}