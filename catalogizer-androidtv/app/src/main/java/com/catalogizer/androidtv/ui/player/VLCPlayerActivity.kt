package com.catalogizer.androidtv.ui.player

import android.content.Intent
import android.os.Bundle
import android.util.Log
import android.view.KeyEvent
import android.view.View
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Pause
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Slider
import androidx.compose.material3.SliderDefaults
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import androidx.tv.material3.*
import com.catalogizer.androidtv.player.VLCPlayer
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.flow.first
import kotlinx.serialization.json.jsonPrimitive
import org.videolan.libvlc.util.VLCVideoLayout

// Progress tracking constants
private const val PROGRESS_SAVE_INTERVAL_MS = 5000L // Save every 5 seconds
private const val MIN_PROGRESS_TO_SAVE = 0.05 // Only save if > 5% watched
private const val MAX_PROGRESS_TO_SAVE = 0.95 // Don't save if > 95% (completed)

/**
 * VLC-based video player activity for Android TV.
 * Provides full-screen video playback with TV-optimized controls.
 */
class VLCPlayerActivity : ComponentActivity() {

    companion object {
        const val EXTRA_MEDIA_ID = "media_id"
        const val EXTRA_STREAM_URL = "stream_url"
        const val EXTRA_MEDIA_TITLE = "media_title"
        private const val TAG = "VLCPlayerActivity"
        private const val CONTROLS_AUTO_HIDE_DELAY = 5000L // 5 seconds
    }

    private lateinit var vlcPlayer: VLCPlayer
    private var mediaId: Long = -1
    private var streamUrl: String? = null
    private var mediaTitle: String = ""
    private var lastSavedProgress: Double = 0.0
    private var progressTrackingJob: kotlinx.coroutines.Job? = null
    private var resumePositionMs: Long = 0

    /**
     * Records a single reproduction session against the backend
     * /api/v1/playback/sessions endpoints so the ProgressBadge
     * on every media card can show duration, current position,
     * last session amount and total reproductions.
     *
     * Lazily constructed in onCreate after the DependencyContainer
     * has a ready API client.
     */
    private lateinit var playbackTracker: com.catalogizer.androidtv.data.playback.PlaybackTracker

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        // Extract extras
        mediaId = intent.getLongExtra(EXTRA_MEDIA_ID, -1)
        streamUrl = intent.getStringExtra(EXTRA_STREAM_URL)
        mediaTitle = intent.getStringExtra(EXTRA_MEDIA_TITLE) ?: ""

        // Initialize VLC
        vlcPlayer = VLCPlayer(this)

        // Initialise the backend playback tracker so the card
        // ProgressBadge and HistoryDrawer reflect this session.
        playbackTracker = com.catalogizer.androidtv.data.playback.PlaybackTracker(
            com.catalogizer.androidtv.DependencyContainer.getInstance(this).api
        )

        setContent {
            VLCPlayerScreen(
                vlcPlayer = vlcPlayer,
                mediaTitle = mediaTitle,
                onBackPressed = { finish() }
            )
        }

        // Resolve stream URL if media_id provided
        if (streamUrl.isNullOrBlank() && mediaId > 0) {
            lifecycleScope.launch {
                resolveAndPlay(mediaId)
            }
        } else if (!streamUrl.isNullOrBlank()) {
            // Start playback with provided URL
            vlcPlayer.play(streamUrl!!)
            Log.d(TAG, "Starting playback: $streamUrl")
        } else {
            Toast.makeText(this, "No media to play", Toast.LENGTH_SHORT).show()
            finish()
        }
    }
    
    private suspend fun resolveAndPlay(mediaId: Long) {
        try {
            val container = com.catalogizer.androidtv.DependencyContainer.getInstance(this)
            val baseUrl = container.getServerUrl().trimEnd('/')
            
            // Fetch media details including watch progress
            try {
                val mediaFlow = container.mediaRepository.getMediaById(mediaId)
                val item = mediaFlow.first()
                if (item != null) {
                    if (item.title.isNotBlank()) {
                        mediaTitle = item.title
                    }
                    // Get resume position from watch progress
                    val durationMs = item.duration ?: 0
                    if (item.watchProgress > 0 && item.watchProgress < 0.95 && durationMs > 0) {
                        resumePositionMs = (item.watchProgress * durationMs).toLong()
                        Log.d(TAG, "Will resume from ${resumePositionMs}ms (progress: ${item.watchProgress})")
                    }
                }
            } catch (e: Exception) {
                Log.w(TAG, "Could not fetch media details: ${e.message}")
            }
            
            // Fetch stream URL from API
            val response = container.api.getEntityStream(mediaId)
            if (response.isSuccessful) {
                val body = response.body()
                val streamPath = body?.get("stream_url")?.jsonPrimitive?.content
                if (streamPath != null) {
                    val rawUrl = if (streamPath.startsWith("/")) "$baseUrl$streamPath" else streamPath
                    // libVLC's HTTP data source cannot inject
                    // custom headers (no Authorization: Bearer
                    // support in the 3.x line), so we append the
                    // JWT as ?access_token=... instead. The
                    // backend's JWTMiddleware.RequireAuth accepts
                    // the same token from either source — see
                    // catalog-api/middleware/auth.go.
                    val token = container.authRepository.authState.value.token.orEmpty()
                    val resolvedUrl = if (token.isBlank()) {
                        rawUrl
                    } else {
                        val sep = if (rawUrl.contains('?')) '&' else '?'
                        "$rawUrl${sep}access_token=$token"
                    }
                    streamUrl = resolvedUrl

                    // Start playback
                    vlcPlayer.play(resolvedUrl)

                    // Open a backend playback session so the
                    // ProgressBadge and HistoryDrawer reflect
                    // this reproduction. Session id is cached
                    // inside the tracker; onDestroy issues end().
                    playbackTracker.start(
                        mediaItemId = mediaId,
                        fileId = null,
                        positionUnit = "seconds",
                        startPosition = resumePositionMs / 1000L,
                    )
                    playbackTracker.startProgressTicker(
                        intervalMs = 15_000L,
                        getCurrentPosition = { vlcPlayer.currentPosition.value / 1000L },
                        getTotalAmount = { vlcPlayer.currentPosition.value / 1000L },
                    )

                    // Seek to resume position if available
                    if (resumePositionMs > 0) {
                        delay(1000) // Wait for player to be ready
                        vlcPlayer.seek(resumePositionMs.toFloat() / 1000 / 60) // Approximate seek
                        Log.d(TAG, "Resumed playback at ${resumePositionMs}ms")
                    }
                    
                    Log.d(TAG, "Starting playback for media $mediaId: $resolvedUrl")
                    
                    // Start progress tracking
                    startProgressTracking()
                } else {
                    showErrorAndFinish("No stream URL available")
                }
            } else {
                showErrorAndFinish("Stream unavailable (${response.code()})")
            }
        } catch (e: Exception) {
            Log.e(TAG, "Failed to resolve stream: ${e.message}", e)
            showErrorAndFinish("Failed to load stream: ${e.message}")
        }
    }
    
    private fun startProgressTracking() {
        progressTrackingJob = lifecycleScope.launch {
            while (true) {
                delay(PROGRESS_SAVE_INTERVAL_MS)
                saveWatchProgress()
            }
        }
    }
    
    private suspend fun saveWatchProgress() {
        if (mediaId <= 0) return
        
        val currentPosition = vlcPlayer.currentPosition.value
        val duration = vlcPlayer.duration.value
        
        if (duration <= 0) return
        
        val progress = currentPosition.toDouble() / duration.toDouble()
        
        // Only save if progress changed significantly and is in valid range
        if (progress in MIN_PROGRESS_TO_SAVE..MAX_PROGRESS_TO_SAVE && 
            kotlin.math.abs(progress - lastSavedProgress) > 0.01) {
            
            try {
                val container = com.catalogizer.androidtv.DependencyContainer.getInstance(this)
                val outcome = container.mediaRepository.saveWatchProgress(
                    mediaId, progress, positionMs = currentPosition, durationMs = duration,
                )
                when (outcome) {
                    is com.catalogizer.androidtv.data.repository.WatchProgressOutcome.Saved -> {
                        lastSavedProgress = progress
                        Log.d(TAG, "Saved watch progress: ${"%.2f".format(progress * 100)}%")
                    }
                    // not stored: keep lastSavedProgress so the next tick tries again (never claim saved)
                    is com.catalogizer.androidtv.data.repository.WatchProgressOutcome.NotStored ->
                        Log.w(TAG, "Watch progress NOT stored by the server: ${outcome.reason}")
                }
            } catch (e: Exception) {
                Log.w(TAG, "Failed to save watch progress: ${e.message}")
            }
        }
    }
    
    private fun showErrorAndFinish(message: String) {
        Toast.makeText(this, message, Toast.LENGTH_LONG).show()
        finish()
    }

    override fun onPause() {
        super.onPause()
        vlcPlayer.pause()
        // Save progress when pausing
        lifecycleScope.launch {
            saveWatchProgress()
        }
    }

    override fun onResume() {
        super.onResume()
        vlcPlayer.resume()
    }

    override fun onDestroy() {
        super.onDestroy()
        // Save final progress before destroying
        progressTrackingJob?.cancel()

        // Finalise the backend playback session so the rolled
        // up media_progress row reflects the last-known
        // position and increments total_reproductions.
        val finalPositionSec = vlcPlayer.currentPosition.value / 1000L
        val durationSec = vlcPlayer.duration.value / 1000L
        val completed = durationSec > 0 &&
            finalPositionSec >= (durationSec - 5L) // within 5s of end

        lifecycleScope.launch {
            if (::playbackTracker.isInitialized && playbackTracker.isActive()) {
                playbackTracker.end(
                    endPosition = finalPositionSec,
                    totalAmount = finalPositionSec,
                    completed = completed,
                )
            }
            saveWatchProgress()
            vlcPlayer.release()
        }
    }

    override fun onKeyDown(keyCode: Int, event: KeyEvent?): Boolean {
        // Handle TV remote buttons
        return when (keyCode) {
            KeyEvent.KEYCODE_MEDIA_PLAY_PAUSE -> {
                togglePlayPause()
                true
            }
            KeyEvent.KEYCODE_MEDIA_PLAY -> {
                vlcPlayer.resume()
                true
            }
            KeyEvent.KEYCODE_MEDIA_PAUSE -> {
                vlcPlayer.pause()
                true
            }
            KeyEvent.KEYCODE_MEDIA_STOP -> {
                vlcPlayer.stop()
                finish()
                true
            }
            KeyEvent.KEYCODE_MEDIA_FAST_FORWARD -> {
                vlcPlayer.seekForward(30000) // 30 seconds
                true
            }
            KeyEvent.KEYCODE_MEDIA_REWIND -> {
                vlcPlayer.seekBackward(30000) // 30 seconds
                true
            }
            KeyEvent.KEYCODE_MEDIA_NEXT -> {
                // Play next episode in series if available
                lifecycleScope.launch {
                    playNextEpisode()
                }
                true
            }
            KeyEvent.KEYCODE_MEDIA_PREVIOUS -> {
                // Play previous episode in series if available
                lifecycleScope.launch {
                    playPreviousEpisode()
                }
                true
            }
            KeyEvent.KEYCODE_DPAD_CENTER -> {
                // Show/hide controls
                true
            }
            KeyEvent.KEYCODE_BACK -> {
                finish()
                true
            }
            else -> super.onKeyDown(keyCode, event)
        }
    }

    /**
     * Resolve sibling episodes for the current media item by fetching the parent's
     * children from the API. Returns the sorted list of siblings and the index of
     * the current episode, or null if the current item has no parent or the request fails.
     */
    private suspend fun resolveSiblingEpisodes(): Pair<List<com.catalogizer.androidtv.data.models.MediaItem>, Int>? {
        try {
            val container = com.catalogizer.androidtv.DependencyContainer.getInstance(this)

            // Fetch the current entity to get its parentId
            val response = container.api.getEntityById(mediaId)
            if (!response.isSuccessful) {
                Log.w(TAG, "Failed to fetch current entity: ${response.code()}")
                return null
            }
            val currentItem = response.body() ?: return null
            val parentId = currentItem.parentId
            if (parentId == null) {
                Log.d(TAG, "Current media item has no parent — episode navigation unavailable")
                return null
            }

            // Fetch all siblings (episodes in the same season)
            val siblings = container.mediaRepository.getEntityChildren(parentId)
            if (siblings.isEmpty()) {
                Log.w(TAG, "No sibling episodes found for parent $parentId")
                return null
            }

            // Sort by episode number (fall back to ID for stable ordering)
            val sorted = siblings.sortedWith(compareBy({ it.episodeNumber ?: Int.MAX_VALUE }, { it.id }))

            // Find the current episode's position
            val currentIndex = sorted.indexOfFirst { it.id == mediaId }
            if (currentIndex == -1) {
                Log.w(TAG, "Current media $mediaId not found among siblings of parent $parentId")
                return null
            }

            return Pair(sorted, currentIndex)
        } catch (e: Exception) {
            Log.e(TAG, "Failed to resolve sibling episodes: ${e.message}", e)
            return null
        }
    }

    /**
     * Launch playback of a different episode by starting a new VLCPlayerActivity
     * with the target episode's ID. Finishes the current activity so the back
     * stack does not accumulate player instances.
     */
    private fun launchEpisode(episode: com.catalogizer.androidtv.data.models.MediaItem) {
        val intent = Intent(this, VLCPlayerActivity::class.java).apply {
            putExtra(EXTRA_MEDIA_ID, episode.id)
            putExtra(EXTRA_MEDIA_TITLE, episode.title)
        }
        startActivity(intent)
        finish()
    }

    /**
     * Play the next episode in the series if available.
     * Fetches the parent entity's children, finds the current position,
     * and launches the next sibling episode.
     */
    private suspend fun playNextEpisode() {
        val result = resolveSiblingEpisodes()
        if (result == null) {
            Toast.makeText(this, "Episode navigation unavailable", Toast.LENGTH_SHORT).show()
            return
        }
        val (siblings, currentIndex) = result
        val nextIndex = currentIndex + 1
        if (nextIndex >= siblings.size) {
            Toast.makeText(this, "No more episodes", Toast.LENGTH_SHORT).show()
            Log.d(TAG, "Already at the last episode")
            return
        }
        val nextEpisode = siblings[nextIndex]
        Log.d(TAG, "Playing next episode: ${nextEpisode.title} (id=${nextEpisode.id})")
        launchEpisode(nextEpisode)
    }

    /**
     * Play the previous episode in the series if available.
     * Fetches the parent entity's children, finds the current position,
     * and launches the previous sibling episode.
     */
    private suspend fun playPreviousEpisode() {
        val result = resolveSiblingEpisodes()
        if (result == null) {
            Toast.makeText(this, "Episode navigation unavailable", Toast.LENGTH_SHORT).show()
            return
        }
        val (siblings, currentIndex) = result
        val prevIndex = currentIndex - 1
        if (prevIndex < 0) {
            Toast.makeText(this, "No more episodes", Toast.LENGTH_SHORT).show()
            Log.d(TAG, "Already at the first episode")
            return
        }
        val prevEpisode = siblings[prevIndex]
        Log.d(TAG, "Playing previous episode: ${prevEpisode.title} (id=${prevEpisode.id})")
        launchEpisode(prevEpisode)
    }

    private fun togglePlayPause() {
        if (vlcPlayer.isPlaying.value) {
            vlcPlayer.pause()
        } else {
            vlcPlayer.resume()
        }
    }
}

@OptIn(ExperimentalTvMaterial3Api::class)
@Composable
fun VLCPlayerScreen(
    vlcPlayer: VLCPlayer,
    mediaTitle: String,
    onBackPressed: () -> Unit
) {
    val context = LocalContext.current
    
    // Collect VLC state
    val isPlaying by vlcPlayer.isPlaying.collectAsState()
    val currentPosition by vlcPlayer.currentPosition.collectAsState()
    val duration by vlcPlayer.duration.collectAsState()
    val playbackState by vlcPlayer.playbackState.collectAsState()
    val volume by vlcPlayer.volume.collectAsState()
    val audioTracks by vlcPlayer.audioTracks.collectAsState()
    val subtitleTracks by vlcPlayer.subtitleTracks.collectAsState()
    val currentAudioTrack by vlcPlayer.currentAudioTrack.collectAsState()
    val currentSubtitleTrack by vlcPlayer.currentSubtitleTrack.collectAsState()

    // UI State
    var showControls by remember { mutableStateOf(true) }
    var showAudioTrackMenu by remember { mutableStateOf(false) }
    var showSubtitleMenu by remember { mutableStateOf(false) }
    var showSettingsMenu by remember { mutableStateOf(false) }
    
    // Auto-hide controls
    LaunchedEffect(showControls) {
        if (showControls) {
            delay(5000) // 5 seconds
            showControls = false
        }
    }

    // Handle state changes
    LaunchedEffect(playbackState) {
        when (playbackState) {
            VLCPlayer.PlaybackState.ERROR -> {
                Toast.makeText(context, "Playback error occurred", Toast.LENGTH_LONG).show()
            }
            VLCPlayer.PlaybackState.COMPLETED -> {
                onBackPressed()
            }
            else -> {}
        }
    }

    Box(
        modifier = Modifier.fillMaxSize()
    ) {
        // Video Surface
        AndroidView(
            factory = { ctx ->
                VLCVideoLayout(ctx).apply {
                    vlcPlayer.attachView(this)
                }
            },
            modifier = Modifier.fillMaxSize()
        )

        // Loading Indicator
        if (playbackState == VLCPlayer.PlaybackState.BUFFERING) {
            CircularProgressIndicator(
                modifier = Modifier.align(Alignment.Center),
                color = Color.White
            )
        }

        // Controls Overlay
        if (showControls) {
            PlayerControlsOverlay(
                mediaTitle = mediaTitle,
                isPlaying = isPlaying,
                currentPosition = currentPosition,
                duration = duration,
                volume = volume,
                onPlayPauseClick = {
                    if (isPlaying) vlcPlayer.pause() else vlcPlayer.resume()
                },
                onSeek = { position -> vlcPlayer.seek(position) },
                onVolumeChange = { vlcPlayer.setVolume(it) },
                onAudioTrackClick = { showAudioTrackMenu = true },
                onSubtitleTrackClick = { showSubtitleMenu = true },
                onSettingsClick = { showSettingsMenu = true },
                onBackClick = onBackPressed,
                modifier = Modifier
                    .fillMaxWidth()
                    .align(Alignment.BottomCenter)
                    .background(Color.Black.copy(alpha = 0.7f))
                    .padding(16.dp)
            )
        }

        // Audio Track Selection Menu
        if (showAudioTrackMenu) {
            TrackSelectionMenu(
                title = "Select Audio Track",
                tracks = audioTracks,
                currentTrack = currentAudioTrack,
                onTrackSelected = { vlcPlayer.setAudioTrack(it.id) },
                onDismiss = { showAudioTrackMenu = false }
            )
        }

        // Subtitle Track Selection Menu
        if (showSubtitleMenu) {
            TrackSelectionMenu(
                title = "Select Subtitle Track",
                tracks = subtitleTracks,
                currentTrack = currentSubtitleTrack,
                onTrackSelected = { vlcPlayer.setSubtitleTrack(it.id) },
                onDismiss = { showSubtitleMenu = false }
            )
        }

        // Settings Menu
        if (showSettingsMenu) {
            PlayerSettingsMenu(
                vlcPlayer = vlcPlayer,
                onDismiss = { showSettingsMenu = false }
            )
        }

        // Tap to show controls
        if (!showControls) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .focusable()
                    .onFocusChanged { 
                        if (it.isFocused || it.hasFocus) {
                            showControls = true
                        }
                    }
            )
        }
    }
}

@OptIn(ExperimentalTvMaterial3Api::class)
@Composable
fun PlayerControlsOverlay(
    mediaTitle: String,
    isPlaying: Boolean,
    currentPosition: Long,
    duration: Long,
    volume: Int,
    onPlayPauseClick: () -> Unit,
    onSeek: (Float) -> Unit,
    onVolumeChange: (Int) -> Unit,
    onAudioTrackClick: () -> Unit,
    onSubtitleTrackClick: () -> Unit,
    onSettingsClick: () -> Unit,
    onBackClick: () -> Unit,
    modifier: Modifier = Modifier
) {
    Column(modifier = modifier) {
        // Title
        Text(
            text = mediaTitle,
            style = MaterialTheme.typography.headlineSmall,
            color = Color.White,
            modifier = Modifier.padding(bottom = 16.dp)
        )

        // Progress Bar
        if (duration > 0) {
            val progress = currentPosition.toFloat() / duration.toFloat()
            
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = formatTime(currentPosition),
                    color = Color.White,
                    modifier = Modifier.width(60.dp)
                )
                
                Slider(
                    value = progress,
                    onValueChange = onSeek,
                    modifier = Modifier.weight(1f).padding(horizontal = 8.dp)
                )
                
                Text(
                    text = formatTime(duration),
                    color = Color.White,
                    modifier = Modifier.width(60.dp)
                )
            }
        }

        // Control Buttons
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(top = 16.dp),
            horizontalArrangement = Arrangement.SpaceEvenly,
            verticalAlignment = Alignment.CenterVertically
        ) {
            // Back button
            IconButton(onClick = onBackClick) {
                Text("←", color = Color.White, style = MaterialTheme.typography.titleLarge)
            }

            // Rewind
            IconButton(onClick = { /* Seek backward */ }) {
                Text("⏪", color = Color.White, style = MaterialTheme.typography.titleLarge)
            }

            // Play/Pause
            IconButton(
                onClick = onPlayPauseClick,
                modifier = Modifier.size(64.dp)
            ) {
                Icon(
                    imageVector = if (isPlaying) Icons.Default.Pause else Icons.Default.PlayArrow,
                    contentDescription = if (isPlaying) "Pause" else "Play",
                    tint = Color.White,
                    modifier = Modifier.size(48.dp)
                )
            }

            // Fast Forward
            IconButton(onClick = { /* Seek forward */ }) {
                Text("⏩", color = Color.White, style = MaterialTheme.typography.titleLarge)
            }

            // Audio Track
            IconButton(onClick = onAudioTrackClick) {
                Text("🎵", style = MaterialTheme.typography.titleLarge)
            }

            // Subtitle Track
            IconButton(onClick = onSubtitleTrackClick) {
                Text("💬", style = MaterialTheme.typography.titleLarge)
            }

            // Settings
            IconButton(onClick = onSettingsClick) {
                Text("⚙️", style = MaterialTheme.typography.titleLarge)
            }
        }
    }
}

@OptIn(ExperimentalTvMaterial3Api::class)
@Composable
fun TrackSelectionMenu(
    title: String,
    tracks: List<VLCPlayer.Track>,
    currentTrack: VLCPlayer.Track?,
    onTrackSelected: (VLCPlayer.Track) -> Unit,
    onDismiss: () -> Unit
) {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black.copy(alpha = 0.7f))
            .clickable(onClick = onDismiss),
        contentAlignment = Alignment.Center
    ) {
        Surface(
            modifier = Modifier
                .widthIn(max = 400.dp)
                .padding(16.dp),
            shape = SurfaceDefaults.shape,
            colors = SurfaceDefaults.colors(
                containerColor = Color(0xFF1E1E1E)
            )
        ) {
            Column(modifier = Modifier.padding(16.dp)) {
                Text(
                    text = title,
                    style = MaterialTheme.typography.titleLarge,
                    color = Color(0xFFFF6600),
                    fontWeight = FontWeight.Bold,
                    modifier = Modifier.padding(bottom = 16.dp)
                )
                
                tracks.forEach { track ->
                    val isSelected = track.id == currentTrack?.id
                    
                    Surface(
                        onClick = {
                            onTrackSelected(track)
                            onDismiss()
                        },
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(vertical = 4.dp),
                        colors = ClickableSurfaceDefaults.colors(
                            containerColor = if (isSelected) Color(0xFFFF6600).copy(alpha = 0.2f) else Color.Transparent
                        )
                    ) {
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(12.dp),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Text(
                                text = track.name,
                                color = if (isSelected) Color(0xFFFF6600) else Color.White
                            )
                            if (isSelected) {
                                Text(
                                    text = "✓",
                                    color = Color(0xFFFF6600)
                                )
                            }
                        }
                    }
                }
                
                Spacer(modifier = Modifier.height(16.dp))
                
                Button(
                    onClick = onDismiss,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Text("Close")
                }
            }
        }
    }
}

@OptIn(ExperimentalTvMaterial3Api::class)
@Composable
fun PlayerSettingsMenu(
    vlcPlayer: VLCPlayer,
    onDismiss: () -> Unit
) {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black.copy(alpha = 0.7f))
            .clickable(onClick = onDismiss),
        contentAlignment = Alignment.Center
    ) {
        Surface(
            modifier = Modifier
                .widthIn(max = 400.dp)
                .padding(16.dp),
            shape = SurfaceDefaults.shape,
            colors = SurfaceDefaults.colors(
                containerColor = Color(0xFF1E1E1E)
            )
        ) {
            Column(modifier = Modifier.padding(16.dp)) {
                Text(
                    text = "Player Settings",
                    style = MaterialTheme.typography.titleLarge,
                    color = Color(0xFFFF6600),
                    fontWeight = FontWeight.Bold,
                    modifier = Modifier.padding(bottom = 16.dp)
                )
                
                // Playback Speed
                Text(
                    text = "Playback Speed",
                    style = MaterialTheme.typography.titleMedium,
                    color = Color.White,
                    modifier = Modifier.padding(bottom = 8.dp)
                )
                Row(
                    modifier = Modifier.padding(bottom = 16.dp),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    listOf(0.5f, 0.75f, 1.0f, 1.25f, 1.5f, 2.0f).forEach { speed ->
                        FilterChip(
                            selected = false,
                            onClick = { vlcPlayer.setSpeed(speed) }
                        ) {
                            Text("${speed}x")
                        }
                    }
                }
                
                // Aspect Ratio
                Text(
                    text = "Aspect Ratio",
                    style = MaterialTheme.typography.titleMedium,
                    color = Color.White,
                    modifier = Modifier.padding(bottom = 8.dp)
                )
                Row(
                    modifier = Modifier.padding(bottom = 16.dp),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    listOf("16:9", "4:3", "1:1", "2.21:1").forEach { ratio ->
                        FilterChip(
                            selected = false,
                            onClick = { vlcPlayer.setAspectRatio(ratio) }
                        ) {
                            Text(ratio)
                        }
                    }
                }
                
                Spacer(modifier = Modifier.height(16.dp))
                
                Button(
                    onClick = onDismiss,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Text("Close")
                }
            }
        }
    }
}

private fun formatTime(milliseconds: Long): String {
    val seconds = milliseconds / 1000
    val minutes = seconds / 60
    val hours = minutes / 60
    val remainingMinutes = minutes % 60
    val remainingSeconds = seconds % 60
    
    return if (hours > 0) {
        String.format("%d:%02d:%02d", hours, remainingMinutes, remainingSeconds)
    } else {
        String.format("%02d:%02d", remainingMinutes, remainingSeconds)
    }
}
