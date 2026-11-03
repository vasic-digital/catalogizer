import api from './api'
import { asList } from './apiShapes'
import type {
  SubtitleSearchRequest,
  SubtitleSearchResponse,
  SubtitleDownloadRequest,
  SubtitleDownloadResponse,
  SubtitleTrack,
  SubtitleSyncVerificationRequest,
  SubtitleSyncVerificationResponse,
  SubtitleTranslationRequest,
  SubtitleTranslationResponse,
  SupportedLanguage,
  SupportedProvider,
  SubtitleMediaInfo
} from '@/types/subtitles'

type Raw = Record<string, unknown>
const str = (v: unknown, d = '') => (typeof v === 'string' ? v : d)

/** wire search result {language:'English', language_code:'en', ...} -> page shape {language:'en', language_name:'English'} */
function toSearchResult(r: Raw): SubtitleSearchResponse['results'][number] {
  return {
    hearing_impaired: false,
    foreign_parts_only: false,
    machine_translated: false,
    download_url: '',
    title: '',
    provider: '',
    ...r,
    id: str(r.id),
    language: str(r.language_code, str(r.language)),
    language_name: str(r.language_name, str(r.language)),
  } as SubtitleSearchResponse['results'][number]
}

/** wire track {language:'English', language_code:'en', source, path, verified_sync} -> page SubtitleTrack */
function toTrack(r: Raw): SubtitleTrack {
  return {
    media_id: 0,
    file_path: str(r.path),
    file_size: 0,
    updated_at: str(r.created_at),
    hearing_impaired: false,
    foreign_parts_only: false,
    machine_translated: false,
    format: '',
    encoding: '',
    created_at: '',
    ...r,
    id: str(r.id),
    language: str(r.language_code, str(r.language)),
    language_name: str(r.language_name, str(r.language)),
    provider: str(r.provider, str(r.source)),
  } as unknown as SubtitleTrack
}

/** wire provider {provider:'subdb', name:'SubDB', supported:true} -> page {name:'subdb', display_name:'SubDB', enabled:true} */
function toProvider(r: Raw): SupportedProvider {
  return {
    features: [],
    ...r,
    name: str(r.provider, str(r.name)),
    display_name: str(r.display_name, str(r.name, str(r.provider))),
    enabled: typeof r.enabled === 'boolean' ? r.enabled : r.supported !== false,
  } as SupportedProvider
}

/**
 * The backend reads `languages` and `providers` as comma-separated strings;
 * axios would send an array as `providers[]=a&providers[]=b`, which the handler
 * ignores (every provider is then searched).
 */
export function toWireSearchParams(params: SubtitleSearchRequest): Record<string, unknown> {
  const { language, providers, ...rest } = params
  return {
    ...rest,
    ...(language ? { languages: language } : {}),
    ...(providers && providers.length > 0 ? { providers: providers.join(',') } : {}),
  }
}

export const subtitleApi = {
  // Search for subtitles
  searchSubtitles: (params: SubtitleSearchRequest): Promise<SubtitleSearchResponse> =>
    api.get('/subtitles/search', { params: toWireSearchParams(params) }).then(res => {
      // wire: {success, results?, count}; an empty list may be omitted by older backends
      const results = asList<Raw>(res.data, 'results').map(toSearchResult)
      const d = (res.data ?? {}) as Partial<SubtitleSearchResponse> & { count?: number }
      return {
        limit: 0,
        offset: 0,
        query_time_ms: 0,
        ...d,
        results,
        total: typeof d.total === 'number' ? d.total : typeof d.count === 'number' ? d.count : results.length,
      }
    }),

  // Download subtitle
  downloadSubtitle: (request: SubtitleDownloadRequest): Promise<SubtitleDownloadResponse> =>
    api.post('/subtitles/download', request).then(res => res.data),

  // Get subtitles for a specific media item
  getMediaSubtitles: (mediaId: number): Promise<SubtitleMediaInfo> =>
    api.get(`/subtitles/media/${mediaId}`).then(res => {
      // wire: {success, media_item_id, subtitles?}; an empty list may be omitted by older backends
      const d = (res.data ?? {}) as Partial<SubtitleMediaInfo> & { media_item_id?: number }
      return {
        title: '',
        media_type: '',
        directory_path: '',
        ...d,
        media_id: typeof d.media_id === 'number' ? d.media_id : (d.media_item_id ?? mediaId),
        subtitles: asList<Raw>(res.data, 'subtitles').map(toTrack),
      }
    }),

  // Verify subtitle synchronization
  verifySync: (subtitleId: string, mediaId: number, request?: Partial<SubtitleSyncVerificationRequest>): Promise<SubtitleSyncVerificationResponse> =>
    api.get(`/subtitles/${subtitleId}/verify-sync/${mediaId}`, { params: request }).then(res => res.data),

  // Translate subtitle text
  translateSubtitle: (request: SubtitleTranslationRequest): Promise<SubtitleTranslationResponse> =>
    api.post('/subtitles/translate', request).then(res => res.data),

  // Get supported languages
  getSupportedLanguages: (): Promise<SupportedLanguage[]> =>
    api.get('/subtitles/languages').then(res => asList<SupportedLanguage>(res.data, 'languages')),

  // Get supported providers
  getSupportedProviders: (): Promise<SupportedProvider[]> =>
    api.get('/subtitles/providers').then(res => asList<Raw>(res.data, 'providers').map(toProvider)),

  // Delete a subtitle track
  deleteSubtitle: (subtitleId: string): Promise<{ success: boolean }> =>
    api.delete(`/subtitles/${subtitleId}`).then(res => res.data),

  // Update subtitle metadata (sync offset, etc.)
  updateSubtitle: (subtitleId: string, data: Partial<SubtitleTrack>): Promise<SubtitleTrack> =>
    api.put(`/subtitles/${subtitleId}`, data).then(res => res.data),

  // Upload custom subtitle file
  uploadSubtitle: (mediaId: number, file: File, language: string, format?: string): Promise<SubtitleDownloadResponse> => {
    const formData = new FormData()
    formData.append('file', file)
    formData.append('media_id', mediaId.toString())
    formData.append('language', language)
    if (format) {
      formData.append('format', format)
    }

    return api.post('/subtitles/upload', formData, {
      headers: {
        'Content-Type': 'multipart/form-data',
      },
    }).then(res => res.data)
  },
}