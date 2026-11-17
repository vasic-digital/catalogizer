package com.catalogizer.android.data.remote

import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.*
import org.junit.Before
import org.junit.Test
import retrofit2.Response
import retrofit2.Retrofit
import com.catalogizer.android.data.models.FavoriteRequest
import com.catalogizer.android.data.models.WatchProgressRequest
import com.catalogizer.android.data.models.RatingRequest

/**
 * AFAV-6 / FAVC-F2: the former cases of this class stubbed `CatalogizerApi` itself with MockK and asserted
 * the stub's own answer, so they proved nothing about the code under test; they were deleted. What remains
 * is (1) a wire test that drives the REAL Retrofit interface (built like production, with the kotlinx
 * converter) against a MockWebServer and asserts the HTTP request the server would receive, and (2) the
 * `toApiResult` extension tests, which never used the stub.
 *
 * NOT RUN by the author (no Gradle at the time): the wire test is UNCONFIRMED until the Android unit tests run.
 */
@OptIn(ExperimentalCoroutinesApi::class)
class CatalogizerApiTest {

    private lateinit var mockWebServer: MockWebServer
    private lateinit var api: CatalogizerApi
    private lateinit var retrofit: Retrofit

    @Before
    fun setup() {
        mockWebServer = MockWebServer()
        mockWebServer.start()
        val json = Json {
            ignoreUnknownKeys = true
            coerceInputValues = true
            isLenient = true
        }
        retrofit = Retrofit.Builder()
            .baseUrl(mockWebServer.url("/"))
            .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
            .build()
        api = retrofit.create(CatalogizerApi::class.java)
    }

    @After
    fun tearDown() {
        mockWebServer.shutdown()
    }

    // --- Wire test: favorite write ---

    @Test
    fun `setFavoriteStatus sends PUT media-id-favorite with the favorite flag as JSON`() = runBlocking {
        mockWebServer.enqueue(MockResponse().setResponseCode(200).setBody("""{"success":true,"message":"ok"}"""))

        val result = api.setFavoriteStatus(1L, FavoriteRequest(true))

        assertTrue(result.isSuccessful)
        val recorded = mockWebServer.takeRequest()
        assertEquals("PUT", recorded.method)
        assertEquals("/media/1/favorite", recorded.path)
        assertTrue(recorded.getHeader("Content-Type").orEmpty().startsWith("application/json"))
        assertEquals("""{"favorite":true}""", recorded.body.readUtf8())
    }

    @Test
    fun `setFavoriteStatus surfaces a 400 from the server as an unsuccessful response`() = runBlocking {
        mockWebServer.enqueue(MockResponse().setResponseCode(400).setBody("""{"error":"bad"}"""))

        val result = api.setFavoriteStatus(1L, FavoriteRequest(false))

        assertFalse(result.isSuccessful)
        assertEquals(400, result.code())
    }

    // --- AFAV-7: entity progress/history must resolve under the production base URL (<server>/api/v1/) ---

    private fun productionLikeApi(): CatalogizerApi {
        val json = Json { ignoreUnknownKeys = true }
        return Retrofit.Builder()
            .baseUrl(mockWebServer.url("/api/v1/")) // same shape as DependencyContainer.buildApi
            .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(CatalogizerApi::class.java)
    }

    @Test
    fun `getEntityProgress requests api-v1-entities-id-progress exactly once under the production base`() = runBlocking {
        mockWebServer.enqueue(MockResponse().setResponseCode(200).setBody("""{"progress":null}"""))

        val result = productionLikeApi().getEntityProgress(5L)

        assertTrue(result.isSuccessful)
        val recorded = mockWebServer.takeRequest()
        assertEquals("GET", recorded.method)
        assertEquals("/api/v1/entities/5/progress", recorded.path)
    }

    @Test
    fun `getEntityHistory requests api-v1-entities-id-history with the limit under the production base`() = runBlocking {
        mockWebServer.enqueue(MockResponse().setResponseCode(200).setBody("""{"sessions":[]}"""))

        val result = productionLikeApi().getEntityHistory(5L, 20)

        assertTrue(result.isSuccessful)
        val recorded = mockWebServer.takeRequest()
        assertEquals("GET", recorded.method)
        assertEquals("/api/v1/entities/5/history?limit=20", recorded.path)
    }

    // --- Wire tests: other typed @Body endpoints (ANDROID-RETROFIT-MAP-1) ---

    @Test
    fun `updateWatchProgress sends PUT media-id-progress with progress as JSON`() = runBlocking {
        mockWebServer.enqueue(MockResponse().setResponseCode(200).setBody("{}"))

        val result = api.updateWatchProgress(7L, WatchProgressRequest(progress = 0.5))

        assertTrue(result.isSuccessful)
        val recorded = mockWebServer.takeRequest()
        assertEquals("PUT", recorded.method)
        assertEquals("/media/7/progress", recorded.path)
        assertEquals("""{"progress":0.5}""", recorded.body.readUtf8())
    }

    @Test
    fun `updateWatchProgress sends duration and reads the persisted flag`() = runBlocking {
        mockWebServer.enqueue(MockResponse().setResponseCode(200).setBody("""{"success":true,"persisted":false,"reason":"unknown_duration_no_row"}"""))

        val result = api.updateWatchProgress(7L, WatchProgressRequest(progress = 0.5, position = 30L, duration = 60.0))

        assertEquals(false, result.body()!!["persisted"]!!.toString().toBoolean())
        assertEquals("""{"progress":0.5,"position":30,"duration":60.0}""", mockWebServer.takeRequest().body.readUtf8())
    }

    @Test
    fun `updateUserWatchProgress sends progress timestamp and position`() = runBlocking {
        mockWebServer.enqueue(MockResponse().setResponseCode(200).setBody("{}"))

        val result = api.updateUserWatchProgress(3L, WatchProgressRequest(0.25, "1700", 42L))

        assertTrue(result.isSuccessful)
        val recorded = mockWebServer.takeRequest()
        assertEquals("/user/progress/3", recorded.path)
        assertEquals("""{"progress":0.25,"timestamp":"1700","position":42}""", recorded.body.readUtf8())
    }

    @Test
    fun `rateMedia sends PUT media-id-rating with rating as JSON`() = runBlocking {
        mockWebServer.enqueue(MockResponse().setResponseCode(200).setBody("{}"))

        val result = api.rateMedia(9L, RatingRequest(4.5))

        assertTrue(result.isSuccessful)
        val recorded = mockWebServer.takeRequest()
        assertEquals("/media/9/rating", recorded.path)
        assertEquals("""{"rating":4.5}""", recorded.body.readUtf8())
    }

    // --- Guard: no method of the interface may be un-callable (ANDROID-RETROFIT-MAP-1) ---

    @Test
    fun `every CatalogizerApi method is accepted by Retrofit (no wildcard, no unserialisable Any)`() {
        // Retrofit parses a method (parameter types, response type, converters) when it is first used; a
        // `Map<String, Any>` @Body throws "Parameter type must not include a type variable or wildcard", and a
        // `Response<Map<String, Any>>` makes the kotlinx converter throw "Serializer for class 'Any' is not found".
        // Both only surface when the method is called on a device, so each method is loaded here explicitly.
        val bad = unparseableMethods(retrofit, CatalogizerApi::class.java)
        assertEquals("methods Retrofit cannot parse: $bad", emptyMap<String, String>(), bad)
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
}
