package com.catalogizer.androidtv.data.playback

import com.catalogizer.androidtv.MainDispatcherRule
import com.catalogizer.androidtv.data.remote.CatalogizerApi
import io.mockk.clearAllMocks
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.mockk
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import retrofit2.Response

@OptIn(ExperimentalCoroutinesApi::class)
class PlaybackTrackerTest {

    @get:Rule
    val mainDispatcherRule = MainDispatcherRule()

    private lateinit var api: CatalogizerApi

    @Before
    fun setup() {
        api = mockk(relaxed = true)
    }

    @After
    fun tearDown() {
        clearAllMocks()
    }

    // ------------------------------------------------------------------
    // start — success cases
    // ------------------------------------------------------------------

    @Test
    fun `start returns session id on success`() = runTest {
        val responseBody = buildJsonObject {
            put("session_id", JsonPrimitive(123L))
        }
        coEvery { api.startPlaybackSession(any()) } returns Response.success(responseBody)
        val tracker = PlaybackTracker(api, this)

        val id = tracker.start(mediaItemId = 1L, fileId = 10L)

        assertEquals(123L, id)
        assertTrue(tracker.isActive())
    }

    @Test
    fun `start sends correct body with all parameters`() = runTest {
        val responseBody = buildJsonObject {
            put("session_id", JsonPrimitive(1L))
        }
        coEvery { api.startPlaybackSession(any()) } returns Response.success(responseBody)
        val tracker = PlaybackTracker(api, this)

        tracker.start(
            mediaItemId = 42L,
            fileId = 7L,
            positionUnit = "pages",
            startPosition = 100L
        )

        coVerify {
            api.startPlaybackSession(match { body ->
                body.media_item_id == 42L &&
                body.file_id == 7L &&
                body.position_unit == "pages" &&
                body.start_position == 100L
            })
        }
    }

    @Test
    fun `start omits file_id when null`() = runTest {
        val responseBody = buildJsonObject {
            put("session_id", JsonPrimitive(1L))
        }
        coEvery { api.startPlaybackSession(any()) } returns Response.success(responseBody)
        val tracker = PlaybackTracker(api, this)

        tracker.start(mediaItemId = 42L, fileId = null)

        coVerify {
            api.startPlaybackSession(match { body ->
                body.file_id == null
            })
        }
    }

    @Test
    fun `start uses default positionUnit and startPosition`() = runTest {
        val responseBody = buildJsonObject {
            put("session_id", JsonPrimitive(1L))
        }
        coEvery { api.startPlaybackSession(any()) } returns Response.success(responseBody)
        val tracker = PlaybackTracker(api, this)

        tracker.start(mediaItemId = 1L, fileId = null)

        coVerify {
            api.startPlaybackSession(match { body ->
                body.position_unit == "seconds" &&
                body.start_position == 0L
            })
        }
    }

    // ------------------------------------------------------------------
    // start — failure cases
    // ------------------------------------------------------------------

    @Test
    fun `start returns 0 on HTTP error`() = runTest {
        coEvery { api.startPlaybackSession(any()) } returns Response.error(
            500, "Server error".toResponseBody(null)
        )
        val tracker = PlaybackTracker(api, this)

        val id = tracker.start(mediaItemId = 1L, fileId = null)

        assertEquals(0L, id)
        assertFalse(tracker.isActive())
    }

    @Test
    fun `start returns 0 on network exception`() = runTest {
        coEvery { api.startPlaybackSession(any()) } throws RuntimeException("No route to host")
        val tracker = PlaybackTracker(api, this)

        val id = tracker.start(mediaItemId = 1L, fileId = null)

        assertEquals(0L, id)
        assertFalse(tracker.isActive())
    }

    @Test
    fun `start returns 0 when response body is null`() = runTest {
        coEvery { api.startPlaybackSession(any()) } returns Response.success(null)
        val tracker = PlaybackTracker(api, this)

        val id = tracker.start(mediaItemId = 1L, fileId = null)

        assertEquals(0L, id)
    }

    @Test
    fun `start returns 0 when session_id key is missing`() = runTest {
        val responseBody = buildJsonObject {
            put("other_key", JsonPrimitive("value"))
        }
        coEvery { api.startPlaybackSession(any()) } returns Response.success(responseBody)
        val tracker = PlaybackTracker(api, this)

        val id = tracker.start(mediaItemId = 1L, fileId = null)

        assertEquals(0L, id)
    }

    // ------------------------------------------------------------------
    // isActive
    // ------------------------------------------------------------------

    @Test
    fun `isActive is false before start`() {
        val tracker = PlaybackTracker(api)
        assertFalse(tracker.isActive())
    }

    @Test
    fun `isActive is true after successful start`() = runTest {
        val responseBody = buildJsonObject {
            put("session_id", JsonPrimitive(5L))
        }
        coEvery { api.startPlaybackSession(any()) } returns Response.success(responseBody)
        val tracker = PlaybackTracker(api, this)

        tracker.start(mediaItemId = 1L, fileId = null)

        assertTrue(tracker.isActive())
    }

    @Test
    fun `isActive is false after failed start`() = runTest {
        coEvery { api.startPlaybackSession(any()) } throws RuntimeException("fail")
        val tracker = PlaybackTracker(api, this)

        tracker.start(mediaItemId = 1L, fileId = null)

        assertFalse(tracker.isActive())
    }

    // ------------------------------------------------------------------
    // end — success cases
    // ------------------------------------------------------------------

    @Test
    fun `end sends correct body and clears session`() = runTest {
        val responseBody = buildJsonObject {
            put("session_id", JsonPrimitive(10L))
        }
        coEvery { api.startPlaybackSession(any()) } returns Response.success(responseBody)
        coEvery { api.endPlaybackSession(any()) } returns Response.success(Unit)
        val tracker = PlaybackTracker(api, this)

        tracker.start(mediaItemId = 1L, fileId = null)
        assertTrue(tracker.isActive())

        tracker.end(endPosition = 3600L, totalAmount = 3600L, completed = true)

        assertFalse(tracker.isActive())
        coVerify {
            api.endPlaybackSession(match { body ->
                body.session_id == 10L &&
                body.end_position == 3600L &&
                body.total_amount == 3600L &&
                body.completed == true
            })
        }
    }

    @Test
    fun `end is no-op when no session is active`() = runTest {
        val tracker = PlaybackTracker(api, this)
        assertFalse(tracker.isActive())

        tracker.end(endPosition = 100L, totalAmount = 100L, completed = false)

        coVerify(exactly = 0) { api.endPlaybackSession(any()) }
    }

    @Test
    fun `end can be called multiple times safely`() = runTest {
        val responseBody = buildJsonObject {
            put("session_id", JsonPrimitive(10L))
        }
        coEvery { api.startPlaybackSession(any()) } returns Response.success(responseBody)
        coEvery { api.endPlaybackSession(any()) } returns Response.success(Unit)
        val tracker = PlaybackTracker(api, this)

        tracker.start(mediaItemId = 1L, fileId = null)
        tracker.end(endPosition = 100L, totalAmount = 100L, completed = true)
        tracker.end(endPosition = 200L, totalAmount = 200L, completed = false)

        coVerify(exactly = 1) { api.endPlaybackSession(any()) }
    }

    // ------------------------------------------------------------------
    // end — error cases
    // ------------------------------------------------------------------

    @Test
    fun `end clears session even when API call throws`() = runTest {
        val responseBody = buildJsonObject {
            put("session_id", JsonPrimitive(10L))
        }
        coEvery { api.startPlaybackSession(any()) } returns Response.success(responseBody)
        coEvery { api.endPlaybackSession(any()) } throws RuntimeException("Network error")
        val tracker = PlaybackTracker(api, this)

        tracker.start(mediaItemId = 1L, fileId = null)
        assertTrue(tracker.isActive())

        tracker.end(endPosition = 100L, totalAmount = 100L, completed = false)

        assertFalse(tracker.isActive())
    }

    @Test
    fun `end clears session on HTTP error`() = runTest {
        val responseBody = buildJsonObject {
            put("session_id", JsonPrimitive(10L))
        }
        coEvery { api.startPlaybackSession(any()) } returns Response.success(responseBody)
        coEvery { api.endPlaybackSession(any()) } returns Response.error(
            500, "error".toResponseBody(null)
        )
        val tracker = PlaybackTracker(api, this)

        tracker.start(mediaItemId = 1L, fileId = null)
        tracker.end(endPosition = 100L, totalAmount = 100L, completed = false)

        assertFalse(tracker.isActive())
    }

    // ------------------------------------------------------------------
    // end — stops progress ticker
    // ------------------------------------------------------------------

    @Test
    fun `end stops progress ticker`() = runTest {
        val responseBody = buildJsonObject {
            put("session_id", JsonPrimitive(10L))
        }
        coEvery { api.startPlaybackSession(any()) } returns Response.success(responseBody)
        coEvery { api.progressPlaybackSession(any()) } returns Response.success(Unit)
        val tracker = PlaybackTracker(api, this)

        tracker.start(mediaItemId = 1L, fileId = null)
        tracker.startProgressTicker(
            intervalMs = 1000L,
            getCurrentPosition = { 500L },
            getTotalAmount = { 500L }
        )

        tracker.end(endPosition = 500L, totalAmount = 500L, completed = true)

        // After end, advancing time should NOT produce progress calls
        advanceTimeBy(5000L)
        coVerify(exactly = 0) { api.progressPlaybackSession(any()) }
    }

    // ------------------------------------------------------------------
    // startProgressTicker
    // ------------------------------------------------------------------

    @Test
    fun `startProgressTicker sends progress after interval`() = runTest {
        val responseBody = buildJsonObject {
            put("session_id", JsonPrimitive(10L))
        }
        coEvery { api.startPlaybackSession(any()) } returns Response.success(responseBody)
        coEvery { api.progressPlaybackSession(any()) } returns Response.success(Unit)
        val tracker = PlaybackTracker(api, this)

        tracker.start(mediaItemId = 1L, fileId = null)
        tracker.startProgressTicker(
            intervalMs = 1000L,
            getCurrentPosition = { 500L },
            getTotalAmount = { 500L }
        )

        advanceTimeBy(1100L)

        coVerify(atLeast = 1) {
            api.progressPlaybackSession(match { body ->
                body.session_id == 10L &&
                body.end_position == 500L &&
                body.total_amount == 500L
            })
        }
        tracker.stopProgressTicker()
    }

    @Test
    fun `startProgressTicker sends multiple ticks over time`() = runTest {
        val responseBody = buildJsonObject {
            put("session_id", JsonPrimitive(10L))
        }
        coEvery { api.startPlaybackSession(any()) } returns Response.success(responseBody)
        coEvery { api.progressPlaybackSession(any()) } returns Response.success(Unit)
        val tracker = PlaybackTracker(api, this)

        tracker.start(mediaItemId = 1L, fileId = null)
        tracker.startProgressTicker(
            intervalMs = 1000L,
            getCurrentPosition = { 500L },
            getTotalAmount = { 500L }
        )

        advanceTimeBy(3100L)

        coVerify(atLeast = 3) { api.progressPlaybackSession(any()) }
        tracker.stopProgressTicker()
    }

    @Test
    fun `startProgressTicker reads live position from getter`() = runTest {
        val responseBody = buildJsonObject {
            put("session_id", JsonPrimitive(10L))
        }
        coEvery { api.startPlaybackSession(any()) } returns Response.success(responseBody)
        coEvery { api.progressPlaybackSession(any()) } returns Response.success(Unit)
        val tracker = PlaybackTracker(api, this)

        var position = 100L
        tracker.start(mediaItemId = 1L, fileId = null)
        tracker.startProgressTicker(
            intervalMs = 1000L,
            getCurrentPosition = { position },
            getTotalAmount = { position }
        )

        advanceTimeBy(1100L)
        position = 200L
        advanceTimeBy(1100L)

        coVerify {
            api.progressPlaybackSession(match { it.end_position == 100L })
        }
        coVerify {
            api.progressPlaybackSession(match { it.end_position == 200L })
        }
        tracker.stopProgressTicker()
    }

    @Test
    fun `startProgressTicker survives API exception and retries next tick`() = runTest {
        val responseBody = buildJsonObject {
            put("session_id", JsonPrimitive(10L))
        }
        coEvery { api.startPlaybackSession(any()) } returns Response.success(responseBody)
        // First call throws, second succeeds
        coEvery { api.progressPlaybackSession(any()) } throws RuntimeException("Network")
        coEvery { api.progressPlaybackSession(any()) } returns Response.success(Unit)
        val tracker = PlaybackTracker(api, this)

        tracker.start(mediaItemId = 1L, fileId = null)
        tracker.startProgressTicker(
            intervalMs = 1000L,
            getCurrentPosition = { 100L },
            getTotalAmount = { 100L }
        )

        advanceTimeBy(2100L)

        coVerify(atLeast = 2) { api.progressPlaybackSession(any()) }
        tracker.stopProgressTicker()
    }

    @Test
    fun `startProgressTicker replaces previous ticker`() = runTest {
        val responseBody = buildJsonObject {
            put("session_id", JsonPrimitive(10L))
        }
        coEvery { api.startPlaybackSession(any()) } returns Response.success(responseBody)
        coEvery { api.progressPlaybackSession(any()) } returns Response.success(Unit)
        val tracker = PlaybackTracker(api, this)

        tracker.start(mediaItemId = 1L, fileId = null)

        // Start first ticker
        tracker.startProgressTicker(
            intervalMs = 1000L,
            getCurrentPosition = { 100L },
            getTotalAmount = { 100L }
        )

        // Replace with second ticker
        tracker.startProgressTicker(
            intervalMs = 2000L,
            getCurrentPosition = { 200L },
            getTotalAmount = { 200L }
        )

        advanceTimeBy(2100L)

        coVerify {
            api.progressPlaybackSession(match { it.end_position == 200L })
        }
        tracker.stopProgressTicker()
    }

    // ------------------------------------------------------------------
    // stopProgressTicker
    // ------------------------------------------------------------------

    @Test
    fun `stopProgressTicker stops sending progress`() = runTest {
        val responseBody = buildJsonObject {
            put("session_id", JsonPrimitive(10L))
        }
        coEvery { api.startPlaybackSession(any()) } returns Response.success(responseBody)
        coEvery { api.progressPlaybackSession(any()) } returns Response.success(Unit)
        val tracker = PlaybackTracker(api, this)

        tracker.start(mediaItemId = 1L, fileId = null)
        tracker.startProgressTicker(
            intervalMs = 1000L,
            getCurrentPosition = { 100L },
            getTotalAmount = { 100L }
        )

        advanceTimeBy(1100L)
        tracker.stopProgressTicker()
        advanceTimeBy(5000L)

        coVerify(atMost = 1) { api.progressPlaybackSession(any()) }
    }

    @Test
    fun `stopProgressTicker is safe to call when no ticker is running`() {
        val tracker = PlaybackTracker(api)
        tracker.stopProgressTicker()
    }

    @Test
    fun `stopProgressTicker is safe to call multiple times`() {
        val tracker = PlaybackTracker(api)
        tracker.stopProgressTicker()
        tracker.stopProgressTicker()
        tracker.stopProgressTicker()
    }

    // ------------------------------------------------------------------
    // Full lifecycle: start -> tick -> end
    // ------------------------------------------------------------------

    @Test
    fun `full lifecycle start tick end works correctly`() = runTest {
        val responseBody = buildJsonObject {
            put("session_id", JsonPrimitive(42L))
        }
        coEvery { api.startPlaybackSession(any()) } returns Response.success(responseBody)
        coEvery { api.progressPlaybackSession(any()) } returns Response.success(Unit)
        coEvery { api.endPlaybackSession(any()) } returns Response.success(Unit)
        val tracker = PlaybackTracker(api, this)

        // Start
        val id = tracker.start(mediaItemId = 5L, fileId = 10L, positionUnit = "seconds", startPosition = 0L)
        assertEquals(42L, id)
        assertTrue(tracker.isActive())

        // Tick
        tracker.startProgressTicker(
            intervalMs = 1000L,
            getCurrentPosition = { 1500L },
            getTotalAmount = { 1500L }
        )
        advanceTimeBy(1100L)

        // End
        tracker.end(endPosition = 3000L, totalAmount = 3000L, completed = true)
        assertFalse(tracker.isActive())

        coVerify(exactly = 1) { api.startPlaybackSession(any()) }
        coVerify(atLeast = 1) { api.progressPlaybackSession(any()) }
        coVerify(exactly = 1) { api.endPlaybackSession(any()) }
    }
}
