package com.catalogizer.android.data.remote

import com.catalogizer.android.data.models.*
import io.mockk.*
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.JsonObject
import org.junit.After
import org.junit.Assert.*
import org.junit.Before
import org.junit.Test
import retrofit2.Response

@OptIn(ExperimentalCoroutinesApi::class)
class CatalogizerApiTest {

    private val mockApi = mockk<CatalogizerApi>()

    @Before
    fun setup() {
        clearAllMocks()
    }

    @After
    fun tearDown() {
        clearAllMocks()
    }

    // --- Authentication endpoints ---

    @Test
    fun `login endpoint accepts LoginRequest and returns LoginResponse`() = runTest {
        val user = createTestUser()
        val loginResponse = LoginResponse(user = user, token = "tok", refreshToken = "ref", expiresIn = 3600)
        coEvery { mockApi.login(any()) } returns Response.success(loginResponse)

        val result = mockApi.login(LoginRequest("admin", "pass"))

        assertTrue(result.isSuccessful)
        assertEquals("tok", result.body()?.token)
        coVerify { mockApi.login(LoginRequest("admin", "pass")) }
    }

    @Test
    fun `register endpoint accepts RegisterRequest and returns User`() = runTest {
        val user = createTestUser(username = "newuser")
        coEvery { mockApi.register(any()) } returns Response.success(user)

        val request = RegisterRequest("newuser", "new@test.com", "pass", "New", "User")
        val result = mockApi.register(request)

        assertTrue(result.isSuccessful)
        assertEquals("newuser", result.body()?.username)
    }

    @Test
    fun `logout endpoint returns Unit`() = runTest {
        coEvery { mockApi.logout() } returns Response.success(Unit)

        val result = mockApi.logout()

        assertTrue(result.isSuccessful)
    }

    @Test
    fun `getProfile returns User`() = runTest {
        val user = createTestUser()
        coEvery { mockApi.getProfile() } returns Response.success(user)

        val result = mockApi.getProfile()

        assertTrue(result.isSuccessful)
        assertEquals("testuser", result.body()?.username)
    }

    @Test
    fun `updateProfile accepts UpdateProfileRequest and returns User`() = runTest {
        val updatedUser = createTestUser(firstName = "Updated")
        coEvery { mockApi.updateProfile(any()) } returns Response.success(updatedUser)

        val request = UpdateProfileRequest(firstName = "Updated")
        val result = mockApi.updateProfile(request)

        assertTrue(result.isSuccessful)
        assertEquals("Updated", result.body()?.firstName)
    }

    @Test
    fun `changePassword accepts ChangePasswordRequest`() = runTest {
        coEvery { mockApi.changePassword(any()) } returns Response.success(Unit)

        val request = ChangePasswordRequest("old", "new")
        val result = mockApi.changePassword(request)

        assertTrue(result.isSuccessful)
    }

    @Test
    fun `getAuthStatus returns AuthStatus`() = runTest {
        val status = AuthStatus(authenticated = true, user = createTestUser())
        coEvery { mockApi.getAuthStatus() } returns Response.success(status)

        val result = mockApi.getAuthStatus()

        assertTrue(result.isSuccessful)
        assertTrue(result.body()!!.authenticated)
    }

    // --- Media endpoints ---

    @Test
    fun `searchMedia accepts query parameters and returns MediaSearchResponse`() = runTest {
        val searchResponse = MediaSearchResponse(
            items = listOf(createTestMediaItem()),
            total = 1,
            limit = 20,
            offset = 0
        )
        coEvery {
            mockApi.searchMedia(
                query = any(), mediaType = any(), yearMin = any(), yearMax = any(),
                ratingMin = any(), quality = any(), sortBy = any(), sortOrder = any(),
                limit = any(), offset = any()
            )
        } returns Response.success(searchResponse)

        val result = mockApi.searchMedia(query = "test", mediaType = "movie", limit = 10, offset = 0)

        assertTrue(result.isSuccessful)
        assertEquals(1, result.body()?.items?.size)
    }

    @Test
    fun `getMediaById returns single MediaItem`() = runTest {
        val item = createTestMediaItem()
        coEvery { mockApi.getMediaById(1L) } returns Response.success(item)

        val result = mockApi.getMediaById(1L)

        assertTrue(result.isSuccessful)
        assertEquals(1L, result.body()?.id)
    }

    @Test
    fun `getRecentMedia returns list with default limit`() = runTest {
        val items = listOf(createTestMediaItem())
        coEvery { mockApi.getRecentMedia(any()) } returns Response.success(items)

        val result = mockApi.getRecentMedia()

        assertTrue(result.isSuccessful)
        coVerify { mockApi.getRecentMedia(10) }
    }

    @Test
    fun `getPopularMedia returns list with custom limit`() = runTest {
        coEvery { mockApi.getPopularMedia(any()) } returns Response.success(emptyList())

        mockApi.getPopularMedia(limit = 5)

        coVerify { mockApi.getPopularMedia(5) }
    }

    @Test
    fun `deleteMedia accepts mediaId`() = runTest {
        coEvery { mockApi.deleteMedia(42L) } returns Response.success(Unit)

        val result = mockApi.deleteMedia(42L)

        assertTrue(result.isSuccessful)
        coVerify { mockApi.deleteMedia(42L) }
    }

    @Test
    fun `updateMedia accepts id and MediaItem body`() = runTest {
        val item = createTestMediaItem()
        coEvery { mockApi.updateMedia(1L, any()) } returns Response.success(item)

        val result = mockApi.updateMedia(1L, item)

        assertTrue(result.isSuccessful)
    }

    // --- User interaction endpoints ---

    @Test
    fun `updateWatchProgress accepts progress data`() = runTest {
        coEvery { mockApi.updateWatchProgress(1L, any()) } returns Response.success(Unit)

        val progressData = mapOf<String, Any>("progress" to 0.75, "timestamp" to 1000L)
        val result = mockApi.updateWatchProgress(1L, progressData)

        assertTrue(result.isSuccessful)
    }

    @Test
    fun `setFavoriteStatus accepts favorite data`() = runTest {
        coEvery { mockApi.setFavoriteStatus(1L, any()) } returns Response.success(Unit)

        val favoriteData = mapOf<String, Any>("favorite" to true)
        val result = mockApi.setFavoriteStatus(1L, favoriteData)

        assertTrue(result.isSuccessful)
    }

    @Test
    fun `rateMedia accepts rating data`() = runTest {
        coEvery { mockApi.rateMedia(1L, any()) } returns Response.success(Unit)

        val ratingData = mapOf<String, Any>("rating" to 8.5)
        val result = mockApi.rateMedia(1L, ratingData)

        assertTrue(result.isSuccessful)
    }

    // --- Favorites and watchlist endpoints ---

    @Test
    fun `getFavorites returns paginated response`() = runTest {
        val response = MediaSearchResponse(items = emptyList(), total = 0, limit = 20, offset = 0)
        coEvery { mockApi.getFavorites(any(), any()) } returns Response.success(response)

        val result = mockApi.getFavorites(limit = 20, offset = 0)

        assertTrue(result.isSuccessful)
        assertEquals(0, result.body()?.total)
    }

    @Test
    fun `addToFavorites accepts mediaId`() = runTest {
        coEvery { mockApi.addToFavorites(5L) } returns Response.success(Unit)

        val result = mockApi.addToFavorites(5L)

        assertTrue(result.isSuccessful)
        coVerify { mockApi.addToFavorites(5L) }
    }

    @Test
    fun `removeFromFavorites accepts mediaId`() = runTest {
        coEvery { mockApi.removeFromFavorites(5L) } returns Response.success(Unit)

        val result = mockApi.removeFromFavorites(5L)

        assertTrue(result.isSuccessful)
    }

    @Test
    fun `getWatchlist returns paginated response`() = runTest {
        val response = MediaSearchResponse(items = emptyList(), total = 0, limit = 20, offset = 0)
        coEvery { mockApi.getWatchlist(any(), any()) } returns Response.success(response)

        val result = mockApi.getWatchlist()

        assertTrue(result.isSuccessful)
    }

    // --- Streaming endpoints ---

    @Test
    fun `getStreamUrl returns URL map`() = runTest {
        val urlMap = mapOf("url" to "http://server/stream/1")
        coEvery { mockApi.getStreamUrl(1L) } returns Response.success(urlMap)

        val result = mockApi.getStreamUrl(1L)

        assertTrue(result.isSuccessful)
        assertEquals("http://server/stream/1", result.body()?.get("url"))
    }

    @Test
    fun `getDownloadUrl returns URL map`() = runTest {
        val urlMap = mapOf("url" to "http://server/download/1")
        coEvery { mockApi.getDownloadUrl(1L) } returns Response.success(urlMap)

        val result = mockApi.getDownloadUrl(1L)

        assertTrue(result.isSuccessful)
        assertEquals("http://server/download/1", result.body()?.get("url"))
    }

    // --- System endpoints ---

    @Test
    fun `getSystemHealth returns health map`() = runTest {
        val healthMap = mapOf<String, Any>("status" to "healthy")
        coEvery { mockApi.getSystemHealth() } returns Response.success(healthMap)

        val result = mockApi.getSystemHealth()

        assertTrue(result.isSuccessful)
        assertEquals("healthy", result.body()?.get("status"))
    }

    @Test
    fun `getSystemStatus returns status map`() = runTest {
        val statusMap = mapOf<String, Any>("version" to "2.3.0")
        coEvery { mockApi.getSystemStatus() } returns Response.success(statusMap)

        val result = mockApi.getSystemStatus()

        assertTrue(result.isSuccessful)
    }

    // --- Playback entity endpoints ---

    @Test
    fun `getEntityProgress accepts mediaItemId`() = runTest {
        val jsonObj = mockk<JsonObject>(relaxed = true)
        coEvery { mockApi.getEntityProgress(42L) } returns Response.success(jsonObj)

        val result = mockApi.getEntityProgress(42L)

        assertTrue(result.isSuccessful)
        coVerify { mockApi.getEntityProgress(42L) }
    }

    @Test
    fun `getEntityHistory accepts mediaItemId and limit`() = runTest {
        val jsonObj = mockk<JsonObject>(relaxed = true)
        coEvery { mockApi.getEntityHistory(42L, 25) } returns Response.success(jsonObj)

        val result = mockApi.getEntityHistory(42L, 25)

        assertTrue(result.isSuccessful)
        coVerify { mockApi.getEntityHistory(42L, 25) }
    }

    // --- Error response handling ---

    @Test
    fun `API returns 401 unauthorized`() = runTest {
        val errorBody = okhttp3.ResponseBody.create(null, "Unauthorized")
        coEvery { mockApi.getProfile() } returns Response.error(401, errorBody)

        val result = mockApi.getProfile()

        assertFalse(result.isSuccessful)
        assertEquals(401, result.code())
    }

    @Test
    fun `API returns 404 not found`() = runTest {
        val errorBody = okhttp3.ResponseBody.create(null, "Not found")
        coEvery { mockApi.getMediaById(999L) } returns Response.error(404, errorBody)

        val result = mockApi.getMediaById(999L)

        assertFalse(result.isSuccessful)
        assertEquals(404, result.code())
    }

    @Test
    fun `API returns 500 server error`() = runTest {
        val errorBody = okhttp3.ResponseBody.create(null, "Internal server error")
        coEvery { mockApi.getMediaStats() } returns Response.error(500, errorBody)

        val result = mockApi.getMediaStats()

        assertFalse(result.isSuccessful)
        assertEquals(500, result.code())
    }

    // --- toApiResult extension ---

    @Test
    fun `toApiResult converts successful response`() = runTest {
        val response = Response.success("data")

        val result = response.toApiResult()

        assertTrue(result.isSuccess)
        assertEquals("data", result.data)
        assertNull(result.error)
    }

    @Test
    fun `toApiResult converts error response`() = runTest {
        val errorBody = okhttp3.ResponseBody.create(null, "Bad request")
        val response = Response.error<String>(400, errorBody)

        val result = response.toApiResult()

        assertFalse(result.isSuccess)
        assertNull(result.data)
        assertNotNull(result.error)
    }

    @Test
    fun `toApiResult returns error for successful response with null body`() = runTest {
        val response = Response.success<String>(null)

        val result = response.toApiResult()

        assertFalse(result.isSuccess)
        assertEquals("Empty response", result.error)
    }

    // --- Helpers ---

    private fun createTestUser(
        username: String = "testuser",
        firstName: String = "Test"
    ): User = User(
        id = 1L,
        username = username,
        email = "test@example.com",
        firstName = firstName,
        lastName = "User",
        role = "user",
        isActive = true,
        createdAt = "2025-01-01T00:00:00Z",
        updatedAt = "2025-01-01T00:00:00Z"
    )

    private fun createTestMediaItem(): MediaItem = MediaItem(
        id = 1L,
        title = "Test Movie",
        mediaType = "movie",
        year = 2024,
        description = "Test description",
        directoryPath = "/test/media",
        createdAt = "2025-01-01T00:00:00Z",
        updatedAt = "2025-01-01T00:00:00Z"
    )
}
