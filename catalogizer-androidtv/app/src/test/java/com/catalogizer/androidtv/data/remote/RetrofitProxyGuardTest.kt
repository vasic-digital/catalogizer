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
 * (a `Map<String, Any>` @Body without @JvmSuppressWildcards throws at call time), and 
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
     * Ratchet: methods known to be un-callable today (tracked as ANDROID-RETROFIT-RESPONSE-ANY-1: they use
     * `Map<String, Any>` in a body or response position, which the kotlinx converter cannot serialise). The test
     * fails if ANY other method is un-parseable, and also if a listed method has become parseable (shrink the list).
     */
    private val knownBroken = setOf(
        "addFavorite",
        "addPlaylistItem",
        "createPlaylist",
        "endPlaybackSession",
        "getCollection",
        "getCollections",
        "getOverallStats",
        "getPlaylists",
        "getScans",
        "getSubtitleLanguages",
        "getSubtitles",
        "listFavorites",
        "progressPlaybackSession",
        "recognizeMedia",
        "startPlaybackSession"
    )

    @Test
    fun `no TV CatalogizerApi method is un-parseable except the tracked ones`() {
        val bad = unparseableMethods(retrofit, CatalogizerApi::class.java)
        assertEquals("un-parseable methods: $bad", knownBroken.toSortedSet(), bad.keys.toSortedSet())
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
