package com.catalogizer.androidtv.data.remote

import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import retrofit2.Retrofit

/**
 * ANDROID-RETROFIT-RESPONSE-ANY-1 wire tests, response side: the real JSON shapes the catalog-api handlers return
 * (gin.H objects: handlers/collection_handler.go, playlist_handler.go, stats.go, storage/scan handler, subtitle_handler.go,
 * service_handlers.go FavoritesHandler.ListFavorites) go through the same Retrofit + kotlinx converter the app uses.
 * Written to compile against both the old (`Response<Map<String, Any>>`) and the fixed interface: results are only read
 * through `toString()` so the RED run on the original shows the converter failure, not a compile error.
 */
class TvRetrofitWireResponseTest {
    private lateinit var server: MockWebServer
    private lateinit var api: CatalogizerApi

    @Before
    fun setup() {
        server = MockWebServer().also { it.start() }
        api = Retrofit.Builder()
            .baseUrl(server.url("/"))
            .addConverterFactory(Json { ignoreUnknownKeys = true }.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(CatalogizerApi::class.java)
    }

    @After
    fun tearDown() = server.shutdown()

    private fun json(body: String) = server.enqueue(MockResponse().setResponseCode(200).setHeader("Content-Type", "application/json").setBody(body))

    @Test
    fun `listFavorites parses the favorites envelope`() = runBlocking {
        json("""{"favorites":[{"entity_id":3,"entity_type":"movie"}],"total":1,"limit":50,"offset":0}""")
        val r = api.listFavorites()
        assertTrue(r.isSuccessful)
        assertTrue(r.body().toString().contains("\"entity_id\":3"))
        assertEquals("/api/v1/favorites", server.takeRequest().path)
    }

    @Test
    fun `getCollections and getCollection parse object bodies`() = runBlocking {
        json("""{"collections":[{"id":1,"name":"c"}],"total":1}""")
        json("""{"id":1,"name":"c","is_smart":false}""")
        assertTrue(api.getCollections().body().toString().contains("\"name\":\"c\""))
        assertTrue(api.getCollection(1L).body().toString().contains("\"is_smart\":false"))
        assertEquals("/api/v1/collections", server.takeRequest().path)
        assertEquals("/api/v1/collections/1", server.takeRequest().path)
    }

    @Test
    fun `getPlaylists getScans getOverallStats parse object bodies`() = runBlocking {
        json("""{"playlists":[],"count":0,"limit":50,"offset":0}""")
        json("""{"scans":[{"job_id":"j1","status":"done"}]}""")
        json("""{"total_files":10,"by_type":{"video":4}}""")
        assertTrue(api.getPlaylists().body().toString().contains("\"count\":0"))
        assertTrue(api.getScans().body().toString().contains("\"job_id\":\"j1\""))
        assertTrue(api.getOverallStats().body().toString().contains("\"total_files\":10"))
        assertEquals("/api/v1/playlists", server.takeRequest().path)
        assertEquals("/api/v1/scans", server.takeRequest().path)
        assertEquals("/api/v1/stats/overall", server.takeRequest().path)
    }

    @Test
    fun `getSubtitles and getSubtitleLanguages parse object bodies`() = runBlocking {
        json("""{"media_id":7,"subtitles":[],"total":0}""")
        json("""{"languages":[{"code":"en"}],"total":1}""")
        assertTrue(api.getSubtitles(7L).body().toString().contains("\"media_id\":7"))
        assertTrue(api.getSubtitleLanguages().body().toString().contains("\"code\":\"en\""))
        assertEquals("/api/v1/subtitles/media/7", server.takeRequest().path)
        assertEquals("/api/v1/subtitles/languages", server.takeRequest().path)
    }

    @Test
    fun `parsed body is not null`() = runBlocking {
        json("""{"a":1}""")
        assertNotNull(api.getOverallStats().body())
    }
}
