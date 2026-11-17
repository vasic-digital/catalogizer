package com.catalogizer.android.data.playback

import com.catalogizer.android.data.remote.CatalogizerApi
import kotlinx.coroutines.runBlocking
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import android.util.Log
import io.mockk.every
import io.mockk.mockkStatic
import io.mockk.unmockkStatic
import org.junit.After
import org.junit.Assert.*
import org.junit.Before
import org.junit.Test
import retrofit2.Retrofit
import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import kotlinx.serialization.json.Json

/** AFAV-7b: strict load must expose non-2xx as Failure, distinct from a genuinely empty history. */
class PlaybackRepositoryErrorStateTest {
    private lateinit var server: MockWebServer
    private lateinit var repo: PlaybackRepository

    @Before fun setUp() {
        mockkStatic(Log::class)
        every { Log.w(any<String>(), any<String>()) } returns 0
        server = MockWebServer(); server.start()
        val api = Retrofit.Builder().baseUrl(server.url("/api/v1/"))
            .client(OkHttpClient())
            .addConverterFactory(Json { ignoreUnknownKeys = true }.asConverterFactory("application/json".toMediaType()))
            .build().create(CatalogizerApi::class.java)
        repo = PlaybackRepository(api)
    }
    @After fun tearDown() { unmockkStatic(Log::class); runCatching { server.shutdown() } }

    @Test fun `history 500 is a Failure not empty`() = runBlocking {
        server.enqueue(MockResponse().setResponseCode(500).setBody("{}"))
        val r = repo.loadHistory(5L)
        assertTrue("got $r", r is PlaybackLoadResult.Failure)
        assertEquals(500, (r as PlaybackLoadResult.Failure).httpCode)
    }
    @Test fun `history 401 is a Failure not empty`() = runBlocking {
        server.enqueue(MockResponse().setResponseCode(401).setBody("{}"))
        val r = repo.loadHistory(5L)
        assertTrue("got $r", r is PlaybackLoadResult.Failure)
        assertEquals(401, (r as PlaybackLoadResult.Failure).httpCode)
    }
    @Test fun `progress 500 and 401 are Failures`() = runBlocking {
        for (c in listOf(500, 401)) {
            server.enqueue(MockResponse().setResponseCode(c).setBody("{}"))
            val r = repo.loadProgress(5L)
            assertTrue("code $c got $r", r is PlaybackLoadResult.Failure)
        }
    }
    @Test fun `genuinely empty history is Success empty`() = runBlocking {
        server.enqueue(MockResponse().setResponseCode(200).setHeader("Content-Type", "application/json").setBody("""{"sessions":[]}"""))
        val r = repo.loadHistory(5L)
        assertEquals(PlaybackLoadResult.Success(emptyList<UiPlaybackSession>()), r)
    }
    @Test fun `no progress recorded is Success null`() = runBlocking {
        server.enqueue(MockResponse().setResponseCode(200).setHeader("Content-Type", "application/json").setBody("""{"progress":null}"""))
        assertEquals(PlaybackLoadResult.Success(null), repo.loadProgress(5L))
    }
    @Test fun `network exception is a Failure`() = runBlocking {
        server.shutdown()
        val r = repo.loadHistory(5L)
        assertTrue("got $r", r is PlaybackLoadResult.Failure)
        assertNull((r as PlaybackLoadResult.Failure).httpCode)
    }
    @Test fun `history UI state maps failure to Error and empty to Empty`() = runBlocking {
        server.enqueue(MockResponse().setResponseCode(500).setBody("{}"))
        server.enqueue(MockResponse().setResponseCode(500).setBody("{}"))
        assertTrue(loadHistoryUiState(repo, 5L, 10) is HistoryUiState.Error)
        server.enqueue(MockResponse().setResponseCode(200).setHeader("Content-Type", "application/json").setBody("""{"progress":null}"""))
        server.enqueue(MockResponse().setResponseCode(200).setHeader("Content-Type", "application/json").setBody("""{"sessions":[]}"""))
        assertTrue(loadHistoryUiState(repo, 5L, 10) is HistoryUiState.Loaded)
    }
}
