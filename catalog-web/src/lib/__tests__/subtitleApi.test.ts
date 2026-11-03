import { subtitleApi, toWireSearchParams } from '../subtitleApi'
import apiDefault from '../api'

vi.mock('../api', async () => {
  const mockApi = {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  }
  return {
    __esModule: true,
    default: mockApi,
    api: mockApi,
  }
})

const mockApi = vi.mocked(apiDefault)

describe('subtitleApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('searchSubtitles', () => {
    it('calls GET /subtitles/search with search params', async () => {
      const mockResults = {
        results: [{ id: 'sub1', title: 'English Subtitles', language: 'en' }],
        total: 1,
      }
      mockApi.get.mockResolvedValue({ data: mockResults })

      const params = { query: 'test movie', language: 'en' }
      const result = await subtitleApi.searchSubtitles(params)

      expect(mockApi.get).toHaveBeenCalledWith('/subtitles/search', { params: { query: 'test movie', languages: 'en' } })
      expect(result).toMatchObject(mockResults)
    })
  })

  describe('toWireSearchParams', () => {
    it('joins providers with commas and renames language to languages', () => {
      expect(toWireSearchParams({ media_path: '/x', language: 'en', providers: ['subdb', 'subscene'] })).toEqual({
        media_path: '/x',
        languages: 'en',
        providers: 'subdb,subscene',
      })
    })
    it('omits empty providers / language', () => {
      expect(toWireSearchParams({ media_path: '/x', language: '', providers: [] })).toEqual({ media_path: '/x' })
    })
  })

  describe('downloadSubtitle', () => {
    it('calls POST /subtitles/download with request data', async () => {
      const downloadResponse = {
        success: true,
        subtitle_path: '/subtitles/test.srt',
        language: 'en',
      }
      mockApi.post.mockResolvedValue({ data: downloadResponse })

      const request = { id: 'sub1', media_path: '/media/test.mkv' }
      const result = await subtitleApi.downloadSubtitle(request)

      expect(mockApi.post).toHaveBeenCalledWith('/subtitles/download', request)
      expect(result).toEqual(downloadResponse)
    })
  })

  describe('getMediaSubtitles', () => {
    it('calls GET /subtitles/media/:mediaId', async () => {
      const mediaInfo = {
        media_id: 42,
        subtitles: [
          { id: 'sub1', language: 'en', language_name: 'English', format: 'srt' },
        ],
      }
      mockApi.get.mockResolvedValue({ data: mediaInfo })

      const result = await subtitleApi.getMediaSubtitles(42)

      expect(mockApi.get).toHaveBeenCalledWith('/subtitles/media/42')
      expect(result).toMatchObject(mediaInfo)
    })
  })

  describe('verifySync', () => {
    it('calls GET /subtitles/:id/verify-sync/:mediaId', async () => {
      const syncResult = { synced: true, offset_ms: 0, confidence: 0.95 }
      mockApi.get.mockResolvedValue({ data: syncResult })

      const result = await subtitleApi.verifySync('sub1', 42)

      expect(mockApi.get).toHaveBeenCalledWith('/subtitles/sub1/verify-sync/42', {
        params: undefined,
      })
      expect(result).toEqual(syncResult)
    })

    it('passes additional params for sync verification', async () => {
      const syncResult = { synced: true, offset_ms: 100, confidence: 0.88 }
      mockApi.get.mockResolvedValue({ data: syncResult })

      const additionalParams = { sample_duration: 60 }
      await subtitleApi.verifySync('sub1', 42, additionalParams as any)

      expect(mockApi.get).toHaveBeenCalledWith('/subtitles/sub1/verify-sync/42', {
        params: additionalParams,
      })
    })
  })

  describe('translateSubtitle', () => {
    it('calls POST /subtitles/translate with translation request', async () => {
      const translationResponse = {
        translated_text: 'Translated content',
        source_language: 'en',
        target_language: 'es',
      }
      mockApi.post.mockResolvedValue({ data: translationResponse })

      const request = {
        subtitle_id: 'sub1',
        source_language: 'en',
        target_language: 'es',
      }
      const result = await subtitleApi.translateSubtitle(request as any)

      expect(mockApi.post).toHaveBeenCalledWith('/subtitles/translate', request)
      expect(result).toEqual(translationResponse)
    })
  })

  describe('getSupportedLanguages', () => {
    it('calls GET /subtitles/languages', async () => {
      const languages = [
        { code: 'en', name: 'English', native_name: 'English' },
        { code: 'es', name: 'Spanish', native_name: 'Espanol' },
      ]
      mockApi.get.mockResolvedValue({ data: languages })

      const result = await subtitleApi.getSupportedLanguages()

      expect(mockApi.get).toHaveBeenCalledWith('/subtitles/languages')
      expect(result).toEqual(languages)
    })
  })

  describe('getSupportedProviders', () => {
    it('calls GET /subtitles/providers', async () => {
      const providers = [
        { name: 'opensubtitles', display_name: 'OpenSubtitles', enabled: true },
      ]
      mockApi.get.mockResolvedValue({ data: providers })

      const result = await subtitleApi.getSupportedProviders()

      expect(mockApi.get).toHaveBeenCalledWith('/subtitles/providers')
      expect(result).toMatchObject(providers)
    })
  })

  describe('deleteSubtitle', () => {
    it('calls DELETE /subtitles/:id', async () => {
      mockApi.delete.mockResolvedValue({ data: { success: true } })

      const result = await subtitleApi.deleteSubtitle('sub1')

      expect(mockApi.delete).toHaveBeenCalledWith('/subtitles/sub1')
      expect(result).toEqual({ success: true })
    })
  })

  describe('updateSubtitle', () => {
    it('calls PUT /subtitles/:id with update data', async () => {
      const updated = { id: 'sub1', sync_offset: 200, language: 'en' }
      mockApi.put.mockResolvedValue({ data: updated })

      const result = await subtitleApi.updateSubtitle('sub1', { sync_offset: 200 } as any)

      expect(mockApi.put).toHaveBeenCalledWith('/subtitles/sub1', { sync_offset: 200 })
      expect(result).toEqual(updated)
    })
  })

  describe('uploadSubtitle', () => {
    it('calls POST /subtitles/upload with form data', async () => {
      const uploadResponse = { success: true, subtitle_path: '/subs/test.srt' }
      mockApi.post.mockResolvedValue({ data: uploadResponse })

      const file = new File(['subtitle content'], 'test.srt', { type: 'text/plain' })
      const result = await subtitleApi.uploadSubtitle(42, file, 'en', 'srt')

      expect(mockApi.post).toHaveBeenCalledWith(
        '/subtitles/upload',
        expect.any(FormData),
        { headers: { 'Content-Type': 'multipart/form-data' } }
      )
      expect(result).toEqual(uploadResponse)
    })

    it('creates FormData with correct fields', async () => {
      mockApi.post.mockResolvedValue({ data: { success: true } })

      const file = new File(['content'], 'test.srt', { type: 'text/plain' })
      await subtitleApi.uploadSubtitle(42, file, 'en', 'srt')

      const formData = mockApi.post.mock.calls[0][1] as FormData
      expect(formData.get('media_id')).toBe('42')
      expect(formData.get('language')).toBe('en')
      expect(formData.get('format')).toBe('srt')
      expect(formData.get('file')).toBeTruthy()
    })

    it('omits format when not provided', async () => {
      mockApi.post.mockResolvedValue({ data: { success: true } })

      const file = new File(['content'], 'test.srt', { type: 'text/plain' })
      await subtitleApi.uploadSubtitle(42, file, 'en')

      const formData = mockApi.post.mock.calls[0][1] as FormData
      expect(formData.get('format')).toBeNull()
    })
  })
})
