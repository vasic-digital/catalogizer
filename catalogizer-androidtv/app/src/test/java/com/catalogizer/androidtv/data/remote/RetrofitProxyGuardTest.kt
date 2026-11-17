package com.catalogizer.androidtv.data.remote

import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Before
import org.junit.Test
import retrofit2.Retrofit

/**
 * ANDROID-RETROFIT-MAP-1 guard for the TV client: every method must be accepted by a real Retrofit proxy
 * (a `Map<String, Any>` @Body or response throws at call time), and no new un-parseable method may appear beyond the
 * tracked ratchet list below.
 */
class RetrofitProxyGuardTest {
    private lateinit var server: MockWebServer
    private lateinit var retrofit: Retrofit

    @Before
    fun setup() {
        server = MockWebServer().also { it.start() }
        retrofit = Retrofit.Builder()
            .baseUrl(server.url("/"))
            .addConverterFactory(Json { ignoreUnknownKeys = true }.asConverterFactory("application/json".toMediaType()))
            .build()
    }

    @After
    fun tearDown() = server.shutdown()

    /**
     * Ratchet: methods known to be un-callable today, each with its reason. Fixed in ANDROID-RETROFIT-RESPONSE-ANY-1 (typed
     * request models / JsonObject responses): addFavorite, addPlaylistItem, createPlaylist, endPlaybackSession, getCollection,
     * getCollections, getOverallStats, getPlaylists, getScans, getSubtitleLanguages, getSubtitles, listFavorites,
     * progressPlaybackSession, startPlaybackSession. The remaining one is pinned, NOT deleted (§11.4.122):
     * `recognizeMedia` has NO server route (docs/qa/client-route-drift-20261117/REPORT.md, AFAV-11), so there is no real
     * request/response shape to type; it keeps `Map<String, Any>` and stays un-parseable until AFAV-11 is decided
     * (route added, or the client method removed by an operator decision). The test fails if ANY other method is
     * un-parseable, and also if the listed one becomes parseable (shrink the list).
     */
    private val knownBroken = setOf(
        "recognizeMedia"
    )

    @Test
    fun `no TV CatalogizerApi method is un-parseable except the tracked ones`() {
        val bad = unparseableMethods(retrofit, CatalogizerApi::class.java)
        assertEquals("un-parseable methods: $bad", knownBroken.toSortedSet(), bad.keys.toSortedSet())
    }

    /** AFAV-7 control: the TV base is `<server>/` and its paths carry `api/v1/`, so they must resolve exactly once. */
    @Test
    fun `TV entity progress and history resolve to api-v1 exactly once`() = kotlinx.coroutines.runBlocking {
        val api = retrofit.create(CatalogizerApi::class.java)
        server.enqueue(okhttp3.mockwebserver.MockResponse().setResponseCode(200).setBody("{}"))
        server.enqueue(okhttp3.mockwebserver.MockResponse().setResponseCode(200).setBody("{}"))
        api.getEntityProgress(5L)
        api.getEntityHistory(5L)
        assertEquals("/api/v1/entities/5/progress", server.takeRequest().path)
        assertEquals("/api/v1/entities/5/history?limit=50", server.takeRequest().path)
    }

    /** Names of methods of [apiClass] that this Retrofit instance cannot turn into a service method. */
    private fun unparseableMethods(retrofit: Retrofit, apiClass: Class<*>): Map<String, String> {
        val load = Retrofit::class.java.getDeclaredMethod("loadServiceMethod", java.lang.reflect.Method::class.java)
        load.isAccessible = true
        val bad = sortedMapOf<String, String>()
        for (m in apiClass.declaredMethods.filter { !it.isSynthetic }) {
            try {
                load.invoke(retrofit, m)
            } catch (e: java.lang.reflect.InvocationTargetException) {
                bad[m.name] = (e.targetException.cause ?: e.targetException).let { it.javaClass.simpleName + ": " + it.message?.lineSequence()?.firstOrNull() }
            }
        }
        return bad
    }
}
