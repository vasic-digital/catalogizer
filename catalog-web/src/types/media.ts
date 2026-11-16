// Re-export shared base types from submodule
export type {
  MediaItem,
  ExternalMetadata,
  MediaVersion,
  QualityInfo,
  MediaSearchRequest,
  MediaSearchResponse,
  MediaEntity,
  MediaFile,
  EntityExternalMetadata,
} from '@vasic-digital/media-types'

// Frontend-specific type aliases and extensions
export type { UserMetadata as EntityUserMetadataBase } from '@vasic-digital/media-types'

export const MEDIA_TYPES = [
  'movie',
  'tv_show',
  'tv_season',
  'tv_episode',
  'music_artist',
  'music_album',
  'song',
  'game',
  'software',
  'book',
  'comic',
] as const

export type MediaType = typeof MEDIA_TYPES[number]

// Frontend-specific entity detail (extends base MediaEntity with view-layer fields)
export interface MediaEntityDetail {
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
  external_metadata: import('@vasic-digital/media-types').EntityExternalMetadata[]
}

export interface EntityUserMetadata {
  user_rating?: number
  watched_status?: string
  favorite?: boolean
  personal_notes?: string
  tags?: string[]
}

export interface MediaTypeInfo {
  id: number
  name: string
  description: string
  count: number
}

export interface EntityListResponse {
  items: import('@vasic-digital/media-types').MediaEntity[]
  total: number
  limit: number
  offset: number
}

export interface EntityStatsResponse {
  total_entities: number
  by_type: Record<string, number>
}

export interface EntityFile {
  id: number
  media_item_id: number
  file_id: number
  quality_info?: string
  language?: string
  is_primary: boolean
  created_at: string
}

export const QUALITY_LEVELS = [
  'cam',
  'ts',
  'dvdrip',
  'brrip',
  '720p',
  '1080p',
  '4k',
  'hdr',
  'dolby_vision',
] as const

export type QualityLevel = typeof QUALITY_LEVELS[number]

export interface StorageRoot {
  id: number
  name: string
  protocol: string
  enabled: boolean
  max_depth: number
  enable_duplicate_detection: boolean
  enable_metadata_extraction: boolean
  created_at: string
  updated_at: string
  last_scan_at?: string
}

/**
 * Body of POST /storage/roots. A destination on a private network (or one that cannot be resolved now) is refused with 400
 * private_network_confirmation_required unless `confirm_private_network` is true (every networked protocol); a create form must
 * send it only after the administrator ticked the confirmation.
 */
export type CreateStorageRootInput = Omit<StorageRoot, 'id' | 'created_at' | 'updated_at'> & {
  host?: string
  port?: number
  path?: string
  url?: string
  username?: string
  password?: string
  domain?: string
  mount_point?: string
  options?: string
  allow_empty?: boolean
  confirm_private_network?: boolean
}

export const SUPPORTED_PROTOCOLS = [
  'smb',
  'ftp',
  'nfs',
  'webdav',
  'local',
] as const

export type StorageProtocol = typeof SUPPORTED_PROTOCOLS[number]
