package com.catalogizer.android.data.playback

import com.catalogizer.android.data.remote.CatalogizerApi
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeout
import kotlin.coroutines.cancellation.CancellationException
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import android.util.Log
import io.mockk.coEvery
import io.mockk.every
import io.mockk.mockk
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

    // ---- AFAV7B-R1-F2/F3 additions ----
    private fun ok(body: String) = MockResponse().setResponseCode(200).setHeader("Content-Type", "application/json").setBody(body)

    @Test fun `progress 200 plus history 500 is an Error state`() = runBlocking {
        server.enqueue(ok("""{"progress":null}"""))
        server.enqueue(MockResponse().setResponseCode(500).setBody("{}"))
        val st = loadHistoryUiState(repo, 5L, 10)
        assertTrue("got $st", st is HistoryUiState.Error)
        assertEquals(500, (st as HistoryUiState.Error).httpCode)
    }
    @Test fun `progress 401 error state carries the code for the login hint`() = runBlocking {
        server.enqueue(MockResponse().setResponseCode(401).setBody("{}"))
        server.enqueue(ok("""{"sessions":[]}"""))
        val st = loadHistoryUiState(repo, 5L, 10)
        assertEquals(HistoryUiState.Error("HTTP 401", 401), st)
    }
    @Test fun `loadProgress network exception is a Failure with null code`() = runBlocking {
        server.shutdown()
        val r = repo.loadProgress(5L)
        assertTrue("got $r", r is PlaybackLoadResult.Failure)
        assertNull((r as PlaybackLoadResult.Failure).httpCode)
    }
    @Test fun `malformed JSON is a Failure not empty`() = runBlocking {
        server.enqueue(ok("""{"sessions": [ not json"""))
        assertTrue(repo.loadHistory(5L) is PlaybackLoadResult.Failure)
        server.enqueue(ok("""{"progress": {"""))
        assertTrue(repo.loadProgress(5L) is PlaybackLoadResult.Failure)
    }
    @Test fun `204 no content is Success empty and Success null`() = runBlocking {
        server.enqueue(MockResponse().setResponseCode(204))
        assertEquals(PlaybackLoadResult.Success(emptyList<UiPlaybackSession>()), repo.loadHistory(5L))
        server.enqueue(MockResponse().setResponseCode(204))
        assertEquals(PlaybackLoadResult.Success(null), repo.loadProgress(5L))
    }
    @Test fun `cancelling mid-load leaves no Failure result and propagates cancellation`() = runBlocking {
        server.enqueue(ok("""{"sessions":[]}""").setBodyDelay(30, java.util.concurrent.TimeUnit.SECONDS))
        var result: PlaybackLoadResult<List<UiPlaybackSession>>? = null
        val job = launch(Dispatchers.IO) { result = repo.loadHistory(5L) }
        delay(1500)
        job.cancel()
        withTimeout(10_000) { job.join() }
        assertTrue("job must end cancelled", job.isCancelled)
        assertNull("cancellation must not become a result, got $result", result)
    }
    @Test fun `cancellation raised by the api is rethrown by loadProgress and loadHistory`() = runBlocking {
        val failing = mockk<CatalogizerApi>()
        coEvery { failing.getEntityProgress(any()) } throws CancellationException("cancelled")
        coEvery { failing.getEntityHistory(any(), any()) } throws CancellationException("cancelled")
        val r = PlaybackRepository(failing)
        assertThrows(CancellationException::class.java) { runBlocking { r.loadProgress(5L) } }
        assertThrows(CancellationException::class.java) { runBlocking { r.loadHistory(5L) } }
        assertThrows(CancellationException::class.java) { runBlocking { loadHistoryUiState(r, 5L, 10) } }
        Unit
    }
}
