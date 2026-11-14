package com.catalogizer.android

import android.content.Context
import android.util.Log
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.PreferenceDataStoreFactory
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStoreFile
import androidx.room.Room
import com.catalogizer.android.data.local.CatalogizerDatabase
import com.catalogizer.android.data.remote.CatalogizerApi
import com.catalogizer.android.data.repository.AuthRepository
import com.catalogizer.android.data.repository.MediaRepository
import com.catalogizer.android.data.repository.OfflineRepository
import com.catalogizer.android.data.repository.WebSocketRepository
import com.catalogizer.android.data.sync.SyncManager
import com.catalogizer.android.ui.viewmodel.AuthViewModel
import com.catalogizer.android.ui.viewmodel.HomeViewModel
import com.catalogizer.android.ui.viewmodel.MainViewModel
import com.catalogizer.android.ui.viewmodel.SearchViewModel
import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.logging.HttpLoggingInterceptor
import retrofit2.Retrofit
import java.net.HttpURLConnection
import java.net.URL
import java.util.concurrent.TimeUnit

/**
 * Manual dependency injection container providing singleton access to repositories,
 * API clients, and ViewModel factories. Manages server URL configuration, Retrofit
 * client lifecycle, and async initialization from persisted [DataStore] settings.
 */
class DependencyContainer(private val context: Context) {

    private val json = Json {
        ignoreUnknownKeys = true
        coerceInputValues = true
        isLenient = true
    }

    // DataStore
    val dataStore: DataStore<Preferences> by lazy {
        PreferenceDataStoreFactory.create {
            context.preferencesDataStoreFile("catalogizer_prefs")
        }
    }

    // Database
    private val database: CatalogizerDatabase by lazy {
        Room.databaseBuilder(
            context.applicationContext,
            CatalogizerDatabase::class.java,
            "catalogizer_database"
        )
            .addMigrations(*CatalogizerDatabase.ALL_MIGRATIONS)
            .build()
    }

    // Current active base URL — read from settings or discovered at runtime
    private var currentBaseUrl: String = BuildConfig.API_BASE_URL.ifBlank { "" }

    private var okHttpClient: OkHttpClient? = null

    private fun buildOkHttpClient(): OkHttpClient {
        val logging = HttpLoggingInterceptor().apply {
            level = if (BuildConfig.DEBUG) HttpLoggingInterceptor.Level.BODY
                    else HttpLoggingInterceptor.Level.NONE
        }
        return OkHttpClient.Builder()
            .addInterceptor(logging)
            .connectTimeout(30, TimeUnit.SECONDS)
            .readTimeout(30, TimeUnit.SECONDS)
            .writeTimeout(30, TimeUnit.SECONDS)
            .build().also { okHttpClient = it }
    }

    private fun buildApi(baseUrl: String): CatalogizerApi {
        // Retrofit requires a non-empty base URL. Use a placeholder when unconfigured;
        // all calls will fail with a connection error, which is the expected behavior
        // until the user configures a real server URL.
        val effectiveUrl = baseUrl.ifBlank { "http://localhost:8080" }
        val sanitized = effectiveUrl.trimEnd('/')
        // Append /api/v1 so Retrofit paths like "auth/login" resolve
        // to /api/v1/auth/login on the backend.
        val apiBase = if (sanitized.endsWith("/api/v1")) "$sanitized/"
                      else "$sanitized/api/v1/"
        val contentType = "application/json".toMediaType()
        return Retrofit.Builder()
            .baseUrl(apiBase)
            .client(buildOkHttpClient())
            .addConverterFactory(json.asConverterFactory(contentType))
            .build()
            .create(CatalogizerApi::class.java)
    }

    private var _api: CatalogizerApi? = null

    val api: CatalogizerApi
        get() {
            if (_api == null) {
                _api = buildApi(currentBaseUrl)
            }
            return _api!!
        }

    // Repositories
    val authRepository: AuthRepository by lazy {
        AuthRepository(api, dataStore).also { repo ->
            repo.onLogout = { syncManager.cancelSync() }
        }
    }

    val mediaRepository: MediaRepository by lazy {
        MediaRepository(api, database.mediaDao())
    }

    val playbackRepository: com.catalogizer.android.data.playback.PlaybackRepository
        get() = com.catalogizer.android.data.playback.PlaybackRepository(api)

    // Sync Manager
    val syncManager: SyncManager by lazy {
        SyncManager(database, api, authRepository, mediaRepository, context)
    }

    // Offline Repository
    val offlineRepository: OfflineRepository by lazy {
        OfflineRepository(database, syncManager, context)
    }

    // WebSocket Repository
    val webSocketRepository: WebSocketRepository by lazy {
        WebSocketRepository(
            baseUrl = currentBaseUrl,
            tokenProvider = {
                runBlocking { authRepository.authToken.first() }
            },
            ticketMinter = com.catalogizer.android.data.repository.ApiWsTicketMinter(api)
        )
    }

    // ViewModels
    fun createAuthViewModel(): AuthViewModel {
        return AuthViewModel(authRepository)
    }

    fun createMainViewModel(): MainViewModel {
        return MainViewModel(mediaRepository)
    }

    fun createHomeViewModel(): HomeViewModel {
        return HomeViewModel(mediaRepository, playbackRepository)
    }

    fun createSearchViewModel(): SearchViewModel {
        return SearchViewModel(mediaRepository)
    }

    /**
     * Change the API base URL at runtime. Recreates the Retrofit client.
     */
    fun switchServer(newBaseUrl: String) {
        currentBaseUrl = newBaseUrl.trimEnd('/')
        _api = buildApi(currentBaseUrl)
    }

    /**
     * Get the currently configured API base URL.
     */
    fun getServerUrl(): String = currentBaseUrl

    /** Shared OkHttpClient used by the Nexus cover-quality debug probe. */
    fun httpClient(): OkHttpClient = okHttpClient ?: buildOkHttpClient()

    /**
     * Persist the server URL to DataStore so it survives app restarts.
     */
    suspend fun saveServerUrl(url: String) {
        dataStore.edit { preferences ->
            preferences[SERVER_URL_PREF_KEY] = url
        }
    }

    /**
     * Load the persisted server URL from DataStore.
     */
    suspend fun loadServerUrl(): String? {
        return dataStore.data.map { preferences ->
            preferences[SERVER_URL_PREF_KEY]
        }.first()
    }

    /**
     * Probe a URL to check if it's a Catalogizer API server.
     */
    suspend fun probeServer(baseUrl: String): Boolean = withContext(Dispatchers.IO) {
        try {
            val url = URL("${baseUrl.trimEnd('/')}/health")
            val conn = url.openConnection() as HttpURLConnection
            conn.connectTimeout = 2000
            conn.readTimeout = 2000
            conn.requestMethod = "GET"
            val result = conn.responseCode == 200
            conn.disconnect()
            result
        } catch (_: Exception) {
            false
        }
    }

    /**
     * Shut down OkHttpClient resources (dispatcher, connection pool, cache).
     * Called from Application.onTerminate() as a safety net.
     */
    fun shutdown() {
        try {
            webSocketRepository.disconnect()
        } catch (e: Exception) { 
            Log.w(TAG, "Error disconnecting WebSocket during shutdown", e)
        }
        try {
            okHttpClient?.let { client ->
                client.dispatcher.executorService.shutdown()
                client.connectionPool.evictAll()
                client.cache?.close()
            }
        } catch (e: Exception) { 
            Log.w(TAG, "Error shutting down OkHttp client", e)
        }
    }

    /**
     * Initialize the container: load saved server URL, create API client.
     * Call from Application.onCreate().
     *
     * Priority:
     * 1. Persisted server URL from DataStore (previous user selection)
     * 2. Probe localhost:8080 (works when ADB reverse is active)
     * 3. Leave empty — LoginScreen will prompt for manual entry
     */
    suspend fun initializeAsync() {
        try {
            val savedUrl = loadServerUrl()
            if (!savedUrl.isNullOrBlank()) {
                currentBaseUrl = savedUrl
            } else {
                // No saved URL — try localhost (ADB reverse proxy)
                val localhostUrl = "http://localhost:8080"
                if (probeServer(localhostUrl)) {
                    currentBaseUrl = localhostUrl
                    saveServerUrl(localhostUrl)
                }
            }
        } catch (_: Exception) {
            // Leave empty — LoginScreen handles unconfigured state
        }
        // Trigger API creation with the loaded URL (if any)
        _api = null
        if (currentBaseUrl.isNotBlank()) {
            api
        }
    }

    companion object {
        private const val TAG = "DependencyContainer"
        private val SERVER_URL_PREF_KEY = stringPreferencesKey("server_url")

        @Volatile
        private var instance: DependencyContainer? = null

        fun getInstance(context: Context): DependencyContainer {
            return instance ?: synchronized(this) {
                instance ?: DependencyContainer(context.applicationContext).also { instance = it }
            }
        }
    }
}