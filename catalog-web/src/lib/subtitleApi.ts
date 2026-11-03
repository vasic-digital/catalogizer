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

export const subtitleApi = {
  // Search for subtitles
  searchSubtitles: (params: SubtitleSearchRequest): Promise<SubtitleSearchResponse> =>
    api.get('/subtitles/search', { params }).then(res => res.data),

  // Download subtitle
  downloadSubtitle: (request: SubtitleDownloadRequest): Promise<SubtitleDownloadResponse> =>
    api.post('/subtitles/download', request).then(res => res.data),

  // Get subtitles for a specific media item
  getMediaSubtitles: (mediaId: number): Promise<SubtitleMediaInfo> =>
    api.get(`/subtitles/media/${mediaId}`).then(res => res.data),

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
    api.get('/subtitles/providers').then(res => asList<SupportedProvider>(res.data, 'providers')),

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