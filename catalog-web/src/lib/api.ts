import axios from 'axios'
import type {
  LoginRequest,
  LoginResponse,
  RegisterRequest,
  AuthStatus,
  ChangePasswordRequest,
  UpdateProfileRequest,
  User
} from '@/types/auth'

// In dev mode (Vite proxy), use relative URL so requests go through the proxy.
// In production or when VITE_API_BASE_URL is set, use absolute URL.
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || ''

export const api = axios.create({
  baseURL: `${API_BASE_URL}/api/v1`,
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// CSRF double-submit: the server mints a token on GET/HEAD (X-CSRF-Token response header + cookie) for the groups that enforce it
// (e.g. /admin/*) and rejects every mutating call that does not echo it back ("missing csrf header", 403). Keep the latest one.
let csrfToken: string | null = null
const SAFE_METHODS = new Set(['get', 'head', 'options'])
const rememberCsrf = (headers: unknown) => {
  const h = headers as { get?: (n: string) => unknown; [k: string]: unknown } | undefined
  const t = (typeof h?.get === 'function' ? h.get('x-csrf-token') : h?.['x-csrf-token']) as string | undefined
  if (t) csrfToken = t
}

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('auth_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  if (csrfToken && !SAFE_METHODS.has((config.method || 'get').toLowerCase())) {
    config.headers['X-CSRF-Token'] = csrfToken
  }
  return config
})

api.interceptors.response.use(
  (response) => {
    rememberCsrf(response.headers)
    return response
  },
  (error) => {
    rememberCsrf(error.response?.headers)
    // Article XI §11.5: when a 401 comes back FROM the login or
    // refresh endpoint itself, it means the credentials submitted
    // were wrong — NOT that an existing session expired. We must
    // let the caller handle that error inline (and surface it to
    // the user); redirecting to /login here erased the diagnostic
    // and made wrong-password indistinguishable from the initial
    // login render. Caught by the 2026-04-29 anti-bluff sweep.
    const reqUrl: string = error.config?.url || ''
    const isAuthEndpoint =
      reqUrl.includes('/auth/login') ||
      reqUrl.includes('/auth/register') ||
      reqUrl.includes('/auth/refresh')
    if (error.response?.status === 401 && !isAuthEndpoint) {
      localStorage.removeItem('auth_token')
      localStorage.removeItem('user')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

export const authApi = {
  login: (data: LoginRequest): Promise<LoginResponse> =>
    api.post('/auth/login', data).then((res) => res.data),

  register: (data: RegisterRequest): Promise<User> =>
    api.post('/auth/register', data).then((res) => res.data),

  logout: (): Promise<void> =>
    api.post('/auth/logout').then(() => {/* no content */}),

  getProfile: (): Promise<User> =>
    api.get('/auth/profile').then((res) => res.data),

  updateProfile: (data: UpdateProfileRequest): Promise<User> =>
    api.put('/auth/profile', data).then((res) => res.data),

  changePassword: (data: ChangePasswordRequest): Promise<void> =>
    api.post('/auth/change-password', data).then(() => {/* no content */}),

  getAuthStatus: (): Promise<AuthStatus> =>
    api.get('/auth/status').then((res) => res.data),

  getPermissions: (): Promise<{ role: string; permissions: string[]; is_admin: boolean }> =>
    api.get('/auth/permissions').then((res) => res.data),

  getInitStatus: (): Promise<{ initialized: boolean; has_admin: boolean; user_count: number }> =>
    api.get('/auth/init-status').then((res) => res.data),
}

export default api