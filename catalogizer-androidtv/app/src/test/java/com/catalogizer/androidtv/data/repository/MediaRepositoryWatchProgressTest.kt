package com.catalogizer.androidtv.data.repository

import android.content.Context
import com.catalogizer.androidtv.MainDispatcherRule
import com.catalogizer.androidtv.data.remote.CatalogizerApi
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.mockk
import io.mockk.slot
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import retrofit2.Response

/**
 * WPROG3-FU-1..3: the TV client must send position + duration (seconds), must not claim "saved" for a
 * 200 persisted:false answer, and must retry a 409 concurrent_update (server contract of PUT /media/:id/progress).
 */
@OptIn(ExperimentalCoroutinesApi::class)
class MediaRepositoryWatchProgressTest {
    @get:Rule
    val mainDispatcherRule = MainDispatcherRule()

    private lateinit var api: CatalogizerApi
    private lateinit var repository: MediaRepository

    @Before
    fun setup() {
        api = mockk()
        repository = MediaRepository(mockk<Context>(relaxed = true), api)
    }

    private fun ok(persisted: Boolean?, reason: String? = null): Response<JsonObject> =
        Response.success(buildJsonObject {
            put("success", JsonPrimitive(true))
            if (persisted != null) put("persisted", JsonPrimitive(persisted))
            if (reason != null) put("reason", JsonPrimitive(reason))
        })

    private fun conflict(): Response<JsonObject> = Response.error(
        409,
        """{"success":false,"persisted":false,"reason":"concurrent_update"}"""
            .toResponseBody("application/json".toMediaType())
    )

    @Test
    fun `body carries progress position and duration in seconds (WPROG3-FU-1)`() {
        val body = MediaRepository.buildWatchProgressBody(progress = 0.5, positionMs = 90_000L, durationMs = 180_000L)
        assertEquals(0.5, body["progress"]!!, 0.0)
        assertEquals(90.0, body["position"]!!, 0.0)
        assertEquals(180.0, body["duration"]!!, 0.0)
    }

    @Test
    fun `body omits position and duration when the duration is unknown`() {
        val body = MediaRepository.buildWatchProgressBody(progress = 1.0, positionMs = null, durationMs = null)
        assertEquals(setOf("progress"), body.keys)
    }

    @Test
    fun `saveWatchProgress sends the duration to the server`() = runTest {
        val sent = slot<Map<String, Double>>()
        coEvery { api.updateWatchProgress(7L, capture(sent)) } returns ok(true)

        val out = repository.saveWatchProgress(7L, 0.5, positionMs = 90_000L, durationMs = 180_000L)

        assertEquals(WatchProgressOutcome.Saved, out)
        assertEquals(180.0, sent.captured["duration"]!!, 0.0)
        assertEquals(90.0, sent.captured["position"]!!, 0.0)
    }

    @Test
    fun `persisted false is NOT reported as saved and carries the reason (WPROG3-FU-3)`() = runTest {
        coEvery { api.updateWatchProgress(7L, any()) } returns ok(false, "unknown_duration_no_row")

        val out = repository.saveWatchProgress(7L, 0.5, positionMs = null, durationMs = null)

        assertEquals(WatchProgressOutcome.NotStored("unknown_duration_no_row"), out)
    }

    @Test
    fun `a 200 answer without the persisted flag is not claimed as saved`() = runTest {
        coEvery { api.updateWatchProgress(7L, any()) } returns ok(null)
        assertTrue(repository.saveWatchProgress(7L, 0.5) is WatchProgressOutcome.NotStored)
    }

    @Test
    fun `409 concurrent_update is retried and the eventual success is reported`() = runTest {
        coEvery { api.updateWatchProgress(7L, any()) } returnsMany listOf(conflict(), ok(true))

        val out = repository.saveWatchProgress(7L, 0.5, positionMs = 1000L, durationMs = 2000L)

        assertEquals(WatchProgressOutcome.Saved, out)
        coVerify(exactly = 2) { api.updateWatchProgress(7L, any()) }
    }

    @Test
    fun `persistent 409 is bounded to 3 attempts and reported as not stored`() = runTest {
        coEvery { api.updateWatchProgress(7L, any()) } returns conflict()

        val out = repository.saveWatchProgress(7L, 0.5, positionMs = 1000L, durationMs = 2000L)

        assertEquals(WatchProgressOutcome.NotStored("concurrent_update"), out)
        coVerify(exactly = 3) { api.updateWatchProgress(7L, any()) }
    }

    @Test
    fun `other HTTP errors still throw`() = runTest {
        coEvery { api.updateWatchProgress(7L, any()) } returns
            Response.error(404, "nope".toResponseBody(null))
        try {
            repository.saveWatchProgress(7L, 0.5)
            fail("expected exception")
        } catch (e: Exception) {
            assertTrue(e.message!!.contains("Failed to update watch progress"))
        }
    }

    @Test
    fun `legacy updateWatchProgress throws NotStored instead of claiming success`() = runTest {
        coEvery { api.updateWatchProgress(7L, any()) } returns ok(false, "unknown_duration_no_row")
        try {
            repository.updateWatchProgress(7L, 1.0)
            fail("expected WatchProgressNotStoredException")
        } catch (e: WatchProgressNotStoredException) {
            assertEquals("unknown_duration_no_row", e.reason)
        }
    }
}
