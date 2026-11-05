/**
 * Round 2 class closure: api modules run over wire shapes as catalog-api emits
 * them (transport-level fake only). Each case crashed or silently misrendered
 * a page when the module returned `res.data` untouched.
 */
import { installRealApi } from '@/test/realApiTransport'
import { subtitleApi } from '../subtitleApi'
import { recommendationsApi } from '../recommendationsApi'
import { collectionsApi } from '../collectionsApi'
import { smbApi } from '../smbApi'

let restore: () => void
afterEach(() => restore?.())

describe('subtitles: {success, languages|providers, count}', () => {
  it('languages are returned as an array', async () => {
    restore = installRealApi({
      '/subtitles/languages': { success: true, count: 1, languages: [{ code: 'en', name: 'English', native_name: 'English' }] },
    })
    const langs = await subtitleApi.getSupportedLanguages()
    expect(Array.isArray(langs)).toBe(true)
    expect(langs.map((l) => l.code)).toEqual(['en'])
  })

  it('providers are returned as an array', async () => {
    restore = installRealApi({
      '/subtitles/providers': { success: true, count: 1, providers: [{ provider: 'opensubtitles', name: 'OpenSubtitles', description: 'd', supported: true }] },
    })
    const p = await subtitleApi.getSupportedProviders()
    // page keys/toggles by `name`, labels with `display_name`, disables when !enabled
    expect(p).toMatchObject([{ name: 'opensubtitles', display_name: 'OpenSubtitles', enabled: true }])
  })

  it('null lists become []', async () => {
    restore = installRealApi({
      '/subtitles/languages': { success: true, count: 0, languages: null },
      '/subtitles/providers': { success: true, count: 0, providers: null },
    })
    expect(await subtitleApi.getSupportedLanguages()).toEqual([])
    expect(await subtitleApi.getSupportedProviders()).toEqual([])
  })
})

describe('recommendations: {local_items,external_items} / {items}', () => {
  it('getSimilar maps local_items to flat recommended items', async () => {
    restore = installRealApi({
      '/recommendations/similar/7': {
        local_items: [
          { media_id: '12', media_metadata: { id: 12, title: 'Alien', media_type: 'movie', year: 1979, rating: 8.4 }, similarity_score: 0.9, similarity_reasons: ['same genre', 'same era'] },
        ],
        external_items: [],
        total_found: 1,
      },
    })
    const items = await recommendationsApi.getSimilar(7)
    expect(items).toEqual([
      { id: 12, title: 'Alien', media_type: 'movie', year: 1979, rating: 8.4, similarity_score: 0.9, reason: 'same genre, same era' },
    ])
  })

  it('getSimilar tolerates null lists', async () => {
    restore = installRealApi({ '/recommendations/similar/7': { local_items: null, external_items: null } })
    expect(await recommendationsApi.getSimilar(7)).toEqual([])
  })

  it('getTrending / getPersonalized read `items`', async () => {
    restore = installRealApi({
      '/recommendations/trending': { items: [{ id: 3, title: 'T', media_type: 'movie' }], media_type: 'all', time_range: 'week' },
      '/recommendations/personalized/1': { user_id: 1, items: null },
    })
    const t = await recommendationsApi.getTrending(5)
    expect(t.map((x) => x.id)).toEqual([3])
    expect(await recommendationsApi.getPersonalized(1)).toEqual([])
  })
})

describe('collections.getCollection', () => {
  it('normalises a raw collection (nil slices, missing flags)', async () => {
    restore = installRealApi({
      '/collections/5': { id: 5, name: 'X', is_smart: false, items: null },
    })
    const c = await collectionsApi.getCollection('5')
    expect(c.id).toBe('5')
    expect(c.name).toBe('X')
    expect(c.is_smart).toBe(false)
  })
})

describe('smb: null / non-array 200', () => {
  it('discover and browse never hand a non-array to callers', async () => {
    restore = installRealApi({
      '/smb/discover': null,
      '/smb/browse': { entries: null },
    })
    expect(await smbApi.discover()).toEqual([])
    expect(await smbApi.browse({ host: 'h', share: 's' })).toEqual([])
  })
})

describe('subtitles: omitted empty lists and field names (round 3)', () => {
  it('getMediaSubtitles: omitted `subtitles` becomes [] and media_id is mapped', async () => {
    restore = installRealApi({ '/subtitles/media/7': { success: true, media_item_id: 7 } })
    const info = await subtitleApi.getMediaSubtitles(7)
    expect(info.subtitles).toEqual([])
    expect(info.media_id).toBe(7)
  })

  it('getMediaSubtitles keeps real tracks', async () => {
    restore = installRealApi({ '/subtitles/media/7': { success: true, media_item_id: 7, subtitles: [{ id: 's1', language: 'en' }] } })
    const info = await subtitleApi.getMediaSubtitles(7)
    expect(info.subtitles.map((s) => s.id)).toEqual(['s1'])
  })

  it('searchSubtitles: omitted `results` becomes [] and `count` maps to total', async () => {
    restore = installRealApi({ '/subtitles/search': { success: true, count: 0 } })
    const r = await subtitleApi.searchSubtitles({ media_path: '/x' } as never)
    expect(r.results).toEqual([])
    expect(r.total).toBe(0)
  })

  it('searchSubtitles: count wins when present, results kept', async () => {
    restore = installRealApi({ '/subtitles/search': { success: true, count: 2, results: [{ id: 'a' }, { id: 'b' }] } })
    const r = await subtitleApi.searchSubtitles({ media_path: '/x' } as never)
    expect(r.results).toHaveLength(2)
    expect(r.total).toBe(2)
  })

  it('searchSubtitles: total is the backend count even when it exceeds the page', async () => {
    restore = installRealApi({ '/subtitles/search': { success: true, count: 5, results: [{ id: 'a' }, { id: 'b' }] } })
    const r = await subtitleApi.searchSubtitles({ media_path: '/x' } as never)
    expect(r.total).toBe(5)
  })

  it('search results: language_code/language map to language/language_name', async () => {
    restore = installRealApi({
      '/subtitles/search': { success: true, count: 1, results: [{ id: 'o1', provider: 'opensubtitles', language: 'English', language_code: 'en', title: 'T', download_url: 'u' }] },
    })
    const r = await subtitleApi.searchSubtitles({ media_path: '/x' } as never)
    expect(r.results[0]).toMatchObject({ id: 'o1', language: 'en', language_name: 'English', hearing_impaired: false })
  })

  it('media tracks: language_code/language/source map to language/language_name/provider', async () => {
    restore = installRealApi({
      '/subtitles/media/7': { success: true, media_item_id: 7, subtitles: [{ id: 't1', language: 'English', language_code: 'en', source: 'downloaded', format: 'srt', encoding: 'utf-8', sync_offset: 0 }] },
    })
    const info = await subtitleApi.getMediaSubtitles(7)
    expect(info.subtitles[0]).toMatchObject({ id: 't1', language: 'en', language_name: 'English', provider: 'downloaded', format: 'srt' })
  })
})
