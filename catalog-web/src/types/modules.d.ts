/**
 * Module declarations for @vasic-digital local packages.
 *
 * These packages are linked via `file:../submodules/*` in package.json
 * and are built separately. Their `dist/` directories may not exist during
 * catalog-web type-check, so we declare minimal ambient modules here.
 *
 * NOTE: When the submodules are built, their own `*.d.ts` files take
 * precedence over these ambient declarations.
 */

/* -------------------------------------------------------------------------- */
/* 1. Media types — shared base types                                         */
/* -------------------------------------------------------------------------- */

declare module '@vasic-digital/media-types' {
  export interface Role {
    id: number
    name: string
    permissions: string[]
  }

  export interface User {
    id: number
    username: string
    email: string
    first_name?: string
    last_name?: string
    role?: Role
    role_id?: number
    created_at: string
    updated_at: string
  }

  export interface DeviceInfo {
    id: number
    user_id: number
    name: string
    type: string
    last_seen: string
  }

  export interface LoginRequest {
    username: string
    password: string
    device_info?: {
      device_type: string
      platform: string
      platform_version: string
      app_version: string
      device_name: string
    }
    remember_me?: boolean
  }

  export interface LoginResponse {
    user: User
    session_token: string
    permissions: string[]
  }

  export interface RegisterRequest {
    username: string
    email: string
    password: string
    first_name?: string
    last_name?: string
  }

  export interface AuthStatus {
    authenticated: boolean
    user?: User
    permissions?: string[]
  }

  export interface ChangePasswordRequest {
    old_password: string
    new_password: string
  }

  export interface UpdateProfileRequest {
    username?: string
    email?: string
    first_name?: string
    last_name?: string
  }

  export interface MediaItem {
    id: number
    title: string
    original_title?: string
    year?: number
    description?: string
    genre?: string[]
    rating?: number
    duration?: number
    file_size?: number
    quality?: string
    media_type?: string
    cover_image?: string
    directory_path?: string
    storage_root_name?: string
    storage_root_protocol?: string
    external_metadata?: ExternalMetadata[]
    versions?: MediaVersion[]
    created_at?: string
    updated_at?: string
  }

  export interface ExternalMetadata {
    id?: number
    media_item_id?: number
    provider?: string
    provider_id?: string
    title?: string
    description?: string
    poster_url?: string
    backdrop_url?: string
    genres?: string[]
    cast?: string[]
    director?: string
    year?: number
    rating?: number
    runtime?: number
    created_at?: string
    updated_at?: string
  }

  export interface MediaVersion {
    id: number
    media_item_id?: number
    quality?: string
    resolution?: string
    codec?: string
    file_size?: number
    language?: string
    created_at?: string
  }

  export interface QualityInfo {
    codec?: string
    resolution?: string
    bitrate?: number
  }

  export interface MediaSearchRequest {
    query?: string
    media_type?: string
    year?: number
    year_min?: number
    year_max?: number
    genre?: string[]
    rating_min?: number
    rating_max?: number
    quality?: string[]
    sort_by?: string
    sort_order?: 'asc' | 'desc'
    limit?: number
    offset?: number
  }

  export interface MediaSearchResponse {
    items: MediaItem[]
    total: number
    limit: number
    offset: number
  }

  export interface MediaEntity {
    id: number
    media_type_id: number
    title: string
    original_title?: string
    year?: number
    description?: string
    genre?: string[]
    director?: string
    rating?: number
    runtime?: number
    language?: string
    status: string
    parent_id?: number
    season_number?: number
    episode_number?: number
    track_number?: number
    first_detected: string
    last_updated: string
    media_type: string
    file_count: number
    children_count: number
    external_metadata: EntityExternalMetadata[]
  }

  export interface MediaFile {
    id: number
    media_item_id: number
    file_path: string
    file_size: number
    checksum?: string
    created_at: string
    updated_at: string
  }

  export interface EntityExternalMetadata {
    id: number
    media_item_id: number
    provider: string
    provider_id: string
    external_id?: string
    rating?: number
    review_url?: string
    data: Record<string, unknown>
    created_at: string
    updated_at: string
  }

  export interface UserMetadata {
    user_rating?: number
    watched_status?: string
    favorite?: boolean
    personal_notes?: string
    tags?: string[]
  }
}

/* -------------------------------------------------------------------------- */
/* 2. Auth context — React provider + hook                                   */
/* -------------------------------------------------------------------------- */

declare module '@vasic-digital/auth-context' {
  import type { ReactNode } from 'react'

  export interface AuthContextType {
    user: unknown
    isAuthenticated: boolean
    isLoading: boolean
    permissions: string[]
    isAdmin: boolean
    login: (data: unknown) => Promise<unknown>
    register: (data: unknown) => Promise<unknown>
    logout: () => Promise<void>
    updateProfile: (data: unknown) => Promise<unknown>
    changePassword: (data: unknown) => Promise<void>
    hasPermission: (permission: string) => boolean
    canAccess: (resource: string, action: string) => boolean
  }

  export interface AuthProviderProps {
    children: ReactNode
  }

  export function AuthProvider(props: AuthProviderProps): JSX.Element
  export function useAuth(): AuthContextType
}

/* -------------------------------------------------------------------------- */
/* 3. WebSocket client                                                        */
/* -------------------------------------------------------------------------- */

declare module '@vasic-digital/websocket-client' {
  export type ConnectionState = 'connecting' | 'connected' | 'disconnecting' | 'disconnected'

  export interface WebSocketMessage {
    type: string
    payload: unknown
    timestamp?: string
  }

  export interface WebSocketClientOptions {
    url: string
    reconnect?: boolean
    reconnectAttempts?: number
    reconnectInterval?: number
    bufferWhileDisconnected?: boolean
  }

  export class WebSocketClient {
    constructor(options: WebSocketClientOptions)
    connect(): void
    disconnect(): void
    dispose(): void
    sendJSON(message: unknown): void
    getState(): ConnectionState
    on(event: string, callback: (data: WebSocketMessage) => void): void
    off(event: string, callback: (data: WebSocketMessage) => void): void
  }
}

/* -------------------------------------------------------------------------- */
/* 4. Catalogizer API client — type exports only                              */
/* -------------------------------------------------------------------------- */

declare module '@vasic-digital/catalogizer-api-client' {
  export interface ClientConfig {
    baseURL?: string
    timeout?: number
    headers?: Record<string, string>
  }

  export interface ApiResponse<T = unknown> {
    data: T
    status: number
    statusText: string
  }

  export class NetworkError extends Error {
    constructor(message: string)
  }
}

/* -------------------------------------------------------------------------- */
/* 5. Collection manager                                                      */
/* -------------------------------------------------------------------------- */

declare module '@vasic-digital/collection-manager' {
  import type { ReactNode } from 'react'

  export interface CollectionListProps {
    children?: ReactNode
  }
  export interface CollectionCardProps {
    children?: ReactNode
  }
  export interface CollectionFormProps {
    children?: ReactNode
  }
  export interface SmartRuleBuilderProps {
    children?: ReactNode
  }

  export function CollectionList(props: CollectionListProps): JSX.Element
  export function CollectionCard(props: CollectionCardProps): JSX.Element
  export function CollectionForm(props: CollectionFormProps): JSX.Element
  export function SmartRuleBuilder(props: SmartRuleBuilderProps): JSX.Element
}

/* -------------------------------------------------------------------------- */
/* 6. Dashboard analytics                                                     */
/* -------------------------------------------------------------------------- */

declare module '@vasic-digital/dashboard-analytics' {
  import type { ReactNode } from 'react'

  export interface StatsCardProps {
    children?: ReactNode
  }
  export interface EntityStatsGridProps {
    children?: ReactNode
  }
  export interface MediaDistributionBarProps {
    children?: ReactNode
  }
  export interface ActivityFeedProps {
    children?: ReactNode
  }
  export interface ActivityItem {
    id: string
    type: string
    message: string
    timestamp: string
  }

  export function StatsCard(props: StatsCardProps): JSX.Element
  export function EntityStatsGrid(props: EntityStatsGridProps): JSX.Element
  export function MediaDistributionBar(props: MediaDistributionBarProps): JSX.Element
  export function ActivityFeed(props: ActivityFeedProps): JSX.Element
}

/* -------------------------------------------------------------------------- */
/* 7. Media browser                                                           */
/* -------------------------------------------------------------------------- */

declare module '@vasic-digital/media-browser' {
  import type { ReactNode } from 'react'

  export interface EntityBrowserProps {
    children?: ReactNode
  }
  export interface EntityGridProps {
    children?: ReactNode
  }
  export interface EntityCardProps {
    children?: ReactNode
  }
  export interface TypeSelectorProps {
    children?: ReactNode
  }
  export interface PaginationProps {
    children?: ReactNode
  }

  export function EntityBrowser(props: EntityBrowserProps): JSX.Element
  export function EntityGrid(props: EntityGridProps): JSX.Element
  export function EntityCard(props: EntityCardProps): JSX.Element
  export function TypeSelector(props: TypeSelectorProps): JSX.Element
  export function Pagination(props: PaginationProps): JSX.Element
}

/* -------------------------------------------------------------------------- */
/* 8. Media player                                                            */
/* -------------------------------------------------------------------------- */

declare module '@vasic-digital/media-player' {
  import type { ReactNode } from 'react'

  export interface MediaPlayerProps {
    children?: ReactNode
  }
  export interface PlayerControlsProps {
    children?: ReactNode
  }
  export interface MediaPlayerState {
    isPlaying: boolean
    currentTime: number
    duration: number
    volume: number
  }
  export interface MediaPlayerControls {
    play(): void
    pause(): void
    seek(time: number): void
    setVolume(volume: number): void
  }

  export function MediaPlayer(props: MediaPlayerProps): JSX.Element
  export function PlayerControls(props: PlayerControlsProps): JSX.Element
  export function useMediaPlayer(): MediaPlayerState & MediaPlayerControls
}

/* -------------------------------------------------------------------------- */
/* 9. UI components                                                           */
/* -------------------------------------------------------------------------- */

declare module '@vasic-digital/ui-components' {
  import type { ReactNode } from 'react'

  export interface ButtonProps {
    children?: ReactNode
    onClick?: () => void
  }
  export interface CardProps {
    children?: ReactNode
  }

  export function Button(props: ButtonProps): JSX.Element
  export function Card(props: CardProps): JSX.Element

  /* Avatar */
  export type AvatarSize = 'xs' | 'sm' | 'md' | 'lg' | 'xl'
  export type PresenceStatus = 'online' | 'away' | 'busy' | 'offline'

  export interface AvatarProps {
    src?: string
    alt?: string
    size?: AvatarSize
    fallback?: string
    presence?: PresenceStatus
    className?: string
  }

  export function Avatar(props: AvatarProps): JSX.Element

  /* EmptyState */
  export interface EmptyStateProps {
    title?: string
    description?: string
    icon?: ReactNode
    action?: ReactNode
    className?: string
  }

  export function EmptyState(props: EmptyStateProps): JSX.Element

  /* LoadingSpinner */
  export type SpinnerSize = 'xs' | 'sm' | 'md' | 'lg' | 'xl'

  export interface LoadingSpinnerProps {
    size?: SpinnerSize
    className?: string
  }

  export function LoadingSpinner(props: LoadingSpinnerProps): JSX.Element
}
