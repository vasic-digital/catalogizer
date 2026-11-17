package com.catalogizer.androidtv.ui.viewmodel

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.catalogizer.androidtv.data.models.MediaItem
import com.catalogizer.androidtv.data.models.MediaSearchRequest
import com.catalogizer.androidtv.data.models.MediaType
import com.catalogizer.androidtv.data.playback.PlaybackRepository
import com.catalogizer.androidtv.data.playback.UiPlaybackProgress
import com.catalogizer.androidtv.data.repository.MediaRepository
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.flow.*
import kotlinx.coroutines.launch

/**
 * UI state holder for the TV home screen containing all media rails,
 * recommendations, trending items, catalog statistics, and loading/error state.
 */
data class HomeUiState(
    // Default to loading=true so the first compose frame renders the
    // loading indicator instead of the "empty library" branch. Without
    // this, there's a one-frame window between initial composition and
    // LaunchedEffect(loadHomeData) where the screen shows nothing —
    // which HelixQA captured as the "black screen after login"
    // (TICKET-002-androidtv-black-screen-transition).
    val isLoading: Boolean = true,
    val error: String? = null,
    val continueWatching: List<MediaItem> = emptyList(),
    val recentMovies: List<MediaItem> = emptyList(),
    val recentTvShows: List<MediaItem> = emptyList(),
    val recentMusicAlbums: List<MediaItem> = emptyList(),
    val recentGames: List<MediaItem> = emptyList(),
    val recentBooks: List<MediaItem> = emptyList(),
    val recentComics: List<MediaItem> = emptyList(),
    val recentSoftware: List<MediaItem> = emptyList(),
    val recentConcerts: List<MediaItem> = emptyList(),
    val recommended: List<MediaItem> = emptyList(),
    val trending: List<MediaItem> = emptyList(),
    val topRatedMovies: List<MediaItem> = emptyList(),
    val topRatedTvShows: List<MediaItem> = emptyList(),
    val topRatedMusic: List<MediaItem> = emptyList(),
    val topRatedDocuments: List<MediaItem> = emptyList(),
    val featuredItem: MediaItem? = null,
    val totalEntities: Int = 0,
    val statsByType: Map<String, Int> = emptyMap(),
    val progressById: Map<Long, UiPlaybackProgress> = emptyMap()
)

/**
 * Provides home screen data for the TV app with parallel API calls for all content rails.
 * Loads recent items by type, top-rated lists, recommendations, trending items, and
 * catalog statistics. Exposes [uiState] as a [StateFlow] of [HomeUiState].
 */
class HomeViewModel(
    private val mediaRepository: MediaRepository,
    private val playbackRepository: PlaybackRepository? = null,
    private val tvChannelRepository: com.catalogizer.androidtv.data.tv.TvChannelRepository? = null,
    private val watchNextManager: com.catalogizer.androidtv.data.tv.WatchNextManager? = null
) : ViewModel() {

    private val _uiState = MutableStateFlow(HomeUiState())
    val uiState: StateFlow<HomeUiState> = _uiState.asStateFlow()

    fun loadHomeData() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }

            try {
                coroutineScope {
                    val continueWatchingDef = async { loadContinueWatching() }
                    val recentMoviesDef = async { loadRecentByType("movie") }
                    val recentTvShowsDef = async { loadRecentByType("tv_show") }
                    val recentMusicAlbumsDef = async { loadRecentByType("music_album") }
                    val recentGamesDef = async { loadRecentByType("game") }
                    val recentBooksDef = async { loadRecentByType("book") }
                    val recentComicsDef = async { loadRecentByType("comic") }
                    val recentSoftwareDef = async { loadRecentByType("software") }
                    val recentConcertsDef = async { loadRecentByType("concert") }
                    val topRatedMoviesDef = async { loadTopRatedByType("movie") }
                    val topRatedTvShowsDef = async { loadTopRatedByType("tv_show") }
                    val topRatedMusicDef = async { loadTopRatedByType("music_album") }
                    val topRatedDocumentsDef = async { loadDocuments() }
                    val trendingDef = async { loadTrending() }
                    val recommendedDef = async { loadRecommended() }
                    val statsDef = async { loadStats() }

                    val continueWatching = continueWatchingDef.await()
                    val recentMovies = recentMoviesDef.await()
                    val recentTvShows = recentTvShowsDef.await()
                    val recentMusicAlbums = recentMusicAlbumsDef.await()
                    val recentGames = recentGamesDef.await()
                    val recentBooks = recentBooksDef.await()
                    val recentComics = recentComicsDef.await()
                    val recentSoftware = recentSoftwareDef.await()
                    val recentConcerts = recentConcertsDef.await()
                    val topRatedMovies = topRatedMoviesDef.await()
                    val topRatedTvShows = topRatedTvShowsDef.await()
                    val topRatedMusic = topRatedMusicDef.await()
                    val topRatedDocuments = topRatedDocumentsDef.await()
                    val trending = trendingDef.await()
                    val recommended = recommendedDef.await()
                    val stats = statsDef.await()

                    val featuredItem = continueWatching.firstOrNull()
                        ?: recentMovies.firstOrNull()
                        ?: trending.firstOrNull()

                    _uiState.update {
                        it.copy(
                            isLoading = false,
                            continueWatching = continueWatching,
                            recentMovies = recentMovies,
                            recentTvShows = recentTvShows,
                            recentMusicAlbums = recentMusicAlbums,
                            recentGames = recentGames,
                            recentBooks = recentBooks,
                            recentComics = recentComics,
                            recentSoftware = recentSoftware,
                            recentConcerts = recentConcerts,
                            recommended = recommended,
                            trending = trending,
                            topRatedMovies = topRatedMovies,
                            topRatedTvShows = topRatedTvShows,
                            topRatedMusic = topRatedMusic,
                            topRatedDocuments = topRatedDocuments,
                            featuredItem = featuredItem,
                            totalEntities = stats.first,
                            statsByType = stats.second
                        )
                    }
                    // Refresh TV home screen channels (non-blocking)
                    launch {
                        try {
                            tvChannelRepository?.refreshAllChannels()
                            watchNextManager?.refreshWatchNext()
                        } catch (e: Exception) {
                            android.util.Log.w("HomeVM", "Channel refresh failed: ${e.message}")
                        }
                    }
                    // Load playback progress for every visible card (non-blocking)
                    launch { loadProgressForVisibleItems() }
                }
            } catch (e: Exception) {
                _uiState.update {
                    it.copy(
                        isLoading = false,
                        error = e.message ?: "Failed to load content"
                    )
                }
            }
        }
    }

    private suspend fun loadContinueWatching(): List<MediaItem> {
        return try {
            mediaRepository.searchMedia(
                MediaSearchRequest(
                    sortBy = "created",
                    sortOrder = "desc",
                    limit = 10
                )
            ).first().filter { it.hasWatchProgress && !it.isCompleted }
        } catch (e: Exception) {
            emptyList()
        }
    }

    private suspend fun loadRecentByType(type: String): List<MediaItem> {
        return try {
            mediaRepository.browseEntities(type, limit = 10, sortBy = "created", sortOrder = "desc").first()
        } catch (e: Exception) {
            emptyList()
        }
    }

    private suspend fun loadTopRatedByType(type: String): List<MediaItem> {
        return try {
            mediaRepository.browseEntities(type, limit = 10, sortBy = "rating", sortOrder = "desc").first()
        } catch (e: Exception) {
            emptyList()
        }
    }

    private suspend fun loadDocuments(): List<MediaItem> {
        return try {
            mediaRepository.searchMedia(
                MediaSearchRequest(
                    mediaType = MediaType.EBOOK.value,
                    sortBy = "rating",
                    sortOrder = "desc",
                    limit = 10
                )
            ).first()
        } catch (e: Exception) {
            emptyList()
        }
    }

    private suspend fun loadRecommended(): List<MediaItem> {
        return try {
            val playedItems = mediaRepository.searchMedia(
                MediaSearchRequest(
                    sortBy = "updated_at",
                    sortOrder = "desc",
                    limit = 50
                )
            ).first().filter { it.hasWatchProgress }

            if (playedItems.isEmpty()) return emptyList()

            val byType = playedItems.groupBy { it.mediaType }
            val seenIds = mutableSetOf<Long>()
            val result = mutableListOf<MediaItem>()

            // Process each media type in parallel for better performance
            val similarResults = coroutineScope {
                byType.map { (_, items) ->
                    async {
                        try {
                            val topItem = items.firstOrNull() ?: return@async emptyList<MediaItem>()
                            mediaRepository.getSimilarItems(topItem.id)
                        } catch (e: Exception) {
                            // Individual failure shouldn't break the whole recommendation section
                            emptyList<MediaItem>()
                        }
                    }
                }
            }.map { it.await() }.flatten()

            // Deduplicate and limit to 10 items
            for (item in similarResults) {
                if (seenIds.add(item.id) && result.size < 10) {
                    result.add(item)
                }
                if (result.size >= 10) break
            }

            result
        } catch (e: Exception) {
            emptyList()
        }
    }

    private suspend fun loadTrending(): List<MediaItem> {
        return try {
            mediaRepository.getTrendingItems(10)
        } catch (e: Exception) {
            emptyList()
        }
    }

    fun refreshContent() {
        loadHomeData()
    }

    fun markAsWatched(mediaId: Long) {
        viewModelScope.launch {
            try {
                mediaRepository.updateWatchProgress(mediaId, 1.0)
                loadHomeData()
            } catch (e: com.catalogizer.androidtv.data.repository.WatchProgressNotStoredException) {
                // the server did not store it (no row/duration yet): do not reload as if it had been marked
            } catch (e: Exception) {
                // Handle error
            }
        }
    }

    fun updateWatchProgress(mediaId: Long, progress: Double) {
        viewModelScope.launch {
            try {
                mediaRepository.updateWatchProgress(mediaId, progress)
            } catch (e: Exception) {
                // Handle error
            }
        }
    }

    private suspend fun loadProgressForVisibleItems() {
        val repo = playbackRepository ?: return
        val state = _uiState.value
        val allItems = buildList {
            addAll(state.continueWatching)
            addAll(state.recentMovies)
            addAll(state.recentTvShows)
            addAll(state.recentMusicAlbums)
            addAll(state.recentGames)
            addAll(state.recentBooks)
            addAll(state.recentComics)
            addAll(state.recentSoftware)
            addAll(state.recentConcerts)
            addAll(state.recommended)
            addAll(state.trending)
            addAll(state.topRatedMovies)
            addAll(state.topRatedTvShows)
            addAll(state.topRatedMusic)
            addAll(state.topRatedDocuments)
        }.distinctBy { it.id }

        if (allItems.isEmpty()) return

        try {
            coroutineScope {
                val results = allItems.map { item ->
                    async {
                        try {
                            item.id to repo.getProgress(item.id)
                        } catch (_: Throwable) {
                            item.id to null
                        }
                    }
                }.map { it.await() }

                val progressMap = results
                    .mapNotNull { (id, prog) -> prog?.let { id to it } }
                    .toMap()

                if (progressMap.isNotEmpty()) {
                    _uiState.update { it.copy(progressById = progressMap) }
                }
            }
        } catch (e: Exception) {
            android.util.Log.w("HomeVM", "Progress fetch failed: ${e.message}")
        }
    }

    private suspend fun loadStats(): Pair<Int, Map<String, Int>> {
        return try {
            val response = mediaRepository.getEntityStats()
            Pair(response.first, response.second)
        } catch (_: Exception) {
            Pair(0, emptyMap())
        }
    }

    fun toggleFavorite(mediaId: Long) {
        viewModelScope.launch {
            try {
                val mediaItem = mediaRepository.getMediaById(mediaId).first()
                mediaItem?.let {
                    val entityType = it.mediaType ?: "movie"
                    val isFav = mediaRepository.checkFavorite(
                        entityType, mediaId
                    )
                    mediaRepository.toggleFavorite(
                        entityType, mediaId, isFav
                    )
                    loadHomeData()
                }
            } catch (_: Exception) {
                // Handle error
            }
        }
    }
}
