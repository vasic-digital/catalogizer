package com.catalogizer.android.data.playback

/** State of the reproduction-history dialog (AFAV-7b). */
sealed class HistoryUiState {
    data object Loading : HistoryUiState()
    data class Loaded(val progress: UiPlaybackProgress?, val sessions: List<UiPlaybackSession>) : HistoryUiState()
    data class Error(val message: String, val httpCode: Int? = null) : HistoryUiState()
}

suspend fun loadHistoryUiState(repository: PlaybackRepository, mediaItemId: Long, limit: Int): HistoryUiState {
    val progress = repository.loadProgress(mediaItemId)
    val history = repository.loadHistory(mediaItemId, limit)
    return when {
        progress is PlaybackLoadResult.Failure -> HistoryUiState.Error(progress.message, progress.httpCode)
        history is PlaybackLoadResult.Failure -> HistoryUiState.Error(history.message, history.httpCode)
        else -> HistoryUiState.Loaded(
            (progress as PlaybackLoadResult.Success).value,
            (history as PlaybackLoadResult.Success).value,
        )
    }
}
