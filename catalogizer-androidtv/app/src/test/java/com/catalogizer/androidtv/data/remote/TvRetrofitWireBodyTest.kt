package com.catalogizer.androidtv.data.remote

import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import retrofit2.Retrofit

/**
 * ANDROID-RETROFIT-RESPONSE-ANY-1 wire tests, request side: the body of each POST is asserted on the wire against the
 * field names the catalog-api handlers bind (playback_handler.go start/progress/end requests, FavoritesHandler.AddFavorite
 * {entity_id, entity_type}, models.CreatePlaylistRequest, models.AddPlaylistItemRequest {entity_id, entity_type}).
 * On the original interface these methods threw in Retrofit's service-method loader and could not be called at all.
 */
class TvRetrofitWireBodyTest {
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

    private fun json(code: Int, body: String) = server.enqueue(MockResponse().setResponseCode(code).setHeader("Content-Type", "application/json").setBody(body))
    private fun bodyOf(req: okhttp3.mockwebserver.RecordedRequest) = Json.parseToJsonElement(req.body.readUtf8()).jsonObject

    @Test
    fun `startPlaybackSession sends the handler field names and parses session_id`() = runBlocking {
        json(200, """{"session_id":55}""")
        val r = api.startPlaybackSession(StartPlaybackRequest(media_item_id = 42L, file_id = 7L, position_unit = "seconds", start_position = 0L))
        assertEquals(55L, r.body()!!["session_id"]!!.jsonPrimitive.content.toLong())
        val req = server.takeRequest()
        assertEquals("/api/v1/playback/sessions/start", req.path)
        val b = bodyOf(req)
        assertEquals("42", b["media_item_id"]!!.jsonPrimitive.content)
        assertEquals("7", b["file_id"]!!.jsonPrimitive.content)
        // position_unit is required by the server (oneof seconds pages events): it must be on the wire even though it has no default
        assertEquals("seconds", b["position_unit"]!!.jsonPrimitive.content)
        assertEquals("0", b["start_position"]!!.jsonPrimitive.content)
    }

    @Test
    fun `startPlaybackSession omits file_id when absent`() = runBlocking {
        json(200, """{"session_id":1}""")
        api.startPlaybackSession(StartPlaybackRequest(media_item_id = 42L, position_unit = "pages", start_position = 3L))
        assertFalse(bodyOf(server.takeRequest()).containsKey("file_id"))
    }

    @Test
    fun `progress and end sessions send session_id positions and completed false on the wire`() = runBlocking {
        json(200, """{"ok":true}""")
        json(200, """{"ok":true}""")
        assertTrue(api.progressPlaybackSession(ProgressPlaybackRequest(10L, 500L, 900L)).isSuccessful)
        assertTrue(api.endPlaybackSession(EndPlaybackRequest(10L, 900L, 900L, completed = false)).isSuccessful)
        val p = bodyOf(server.takeRequest())
        assertEquals("10", p["session_id"]!!.jsonPrimitive.content)
        assertEquals("500", p["end_position"]!!.jsonPrimitive.content)
        assertEquals("900", p["total_amount"]!!.jsonPrimitive.content)
        val e = bodyOf(server.takeRequest())
        // completed=false must still be sent (no default => not dropped by encodeDefaults=false)
        assertEquals("false", e["completed"]!!.jsonPrimitive.content)
    }

    @Test
    fun `addFavorite sends entity_id and entity_type`() = runBlocking {
        json(200, """{"status":"added"}""")
        val r = api.addFavorite(FavoriteRequest("movie", 123L))
        assertEquals("added", r.body()!!["status"])
        val req = server.takeRequest()
        assertEquals("/api/v1/favorites", req.path)
        val b = bodyOf(req)
        assertEquals("123", b["entity_id"]!!.jsonPrimitive.content)
        assertEquals("movie", b["entity_type"]!!.jsonPrimitive.content)
    }

    @Test
    fun `createPlaylist and addPlaylistItem send the handler bodies and parse the object responses`() = runBlocking {
        json(201, """{"id":9,"name":"n","is_smart":false}""")
        json(201, """{"id":1,"playlist_id":9,"entity_id":5,"entity_type":"movie"}""")
        val created = api.createPlaylist(CreatePlaylistRequest("n"))
        assertEquals("9", created.body()!!["id"]!!.jsonPrimitive.content)
        val added = api.addPlaylistItem(9L, PlaylistItemRequest(entity_id = 5L, entity_type = "movie"))
        assertEquals("5", added.body()!!["entity_id"]!!.jsonPrimitive.content)
        val c = server.takeRequest()
        assertEquals("/api/v1/playlists", c.path)
        val cb = bodyOf(c)
        assertEquals("n", cb["name"]!!.jsonPrimitive.content)
        assertFalse("description is omitted when null", cb.containsKey("description"))
        val a = server.takeRequest()
        assertEquals("/api/v1/playlists/9/items", a.path)
        val ab = bodyOf(a)
        assertEquals("5", ab["entity_id"]!!.jsonPrimitive.content)
        assertEquals("movie", ab["entity_type"]!!.jsonPrimitive.content)
        assertFalse("server binds entity_id, not media_id", ab.containsKey("media_id"))
    }
}
