@file:OptIn(ExperimentalTvMaterial3Api::class)
package com.catalogizer.androidtv.ui.screens.media

import androidx.compose.foundation.background
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.MenuBook as AutoMirroredMenuBook
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.filled.FavoriteBorder
import androidx.compose.material.icons.filled.History
import androidx.compose.material.icons.filled.Image
import androidx.compose.material.icons.filled.MenuBook
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Star
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon as M3Icon
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.tv.material3.*

import com.catalogizer.androidtv.DependencyContainer
import com.catalogizer.androidtv.data.models.MediaItem
import com.catalogizer.androidtv.ui.components.CoverImage
import com.catalogizer.androidtv.ui.components.HeroPoster
import com.catalogizer.androidtv.ui.components.HistoryDialog
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

/**
 * Container entity types — a tv_show holds tv_seasons, a tv_season holds
 * tv_episodes, a music_album holds songs. For these, the detail screen MUST
 * expose the children so the user can drill down and pick a specific title /
 * episode / track to play (§11.4.143 — choose a specific title). Leaf types
 * (movie, tv_episode, song, …) are directly playable and keep the Play button.
 *
 * Top-level + pure so it is unit-testable without Compose / Robolectric,
 * mirroring the existing helper-function test pattern in MediaDetailScreenTest.
 */
internal fun isContainerType(mediaType: String?): Boolean = when (mediaType) {
    "tv_show", "tv_season", "music_album" -> true
    else -> false
}

/**
 * Section heading for a container's children row. Names the child kind so the
 * user understands what the row contains (Seasons for a show, Episodes for a
 * season, Tracks for an album).
 */
internal fun childSectionTitle(parentMediaType: String?): String = when (parentMediaType) {
    "tv_show" -> "Seasons"
    "tv_season" -> "Episodes"
    "music_album" -> "Tracks"
    else -> "Episodes"
}

/**
 * The leaf-entity action a directly-playable item dispatches to when the user
 * presses the primary button. Container types (see [isContainerType]) never
 * reach here — they show a children row instead of a primary action.
 *
 * EXTENSION POINT (mergeable, §11.4.124): a comic / book reader adds its own
 * kind here (e.g. `COMIC`) + a branch in [leafActionKindFor], and the call site
 * in [MediaDetailScreen] adds one `when` arm wired to a new `onNavigateTo…`
 * callback — without disturbing the existing IMAGE / VIDEO routing. Keep
 * [LeafActionKind.VIDEO] the exhaustive default so any unknown leaf type still
 * gets the existing (working) video route rather than a dead end.
 *
 * COMIC was added per the above plan: a `comic` media_type (or a comic-archive
 * path) opens the dedicated [ComicReaderScreen] instead of the player.
 *
 * BOOK was added the same way: a `book` media_type (or a `.pdf` path) opens the
 * dedicated [BookReaderScreen] (catalog-api renders each PDF page server-side via
 * poppler) instead of the player.
 */
internal enum class LeafActionKind { VIDEO, IMAGE, COMIC, BOOK }

/** Image file extensions catalog-api recognises as still images. */
private val IMAGE_EXTENSIONS =
    setOf("jpg", "jpeg", "png", "webp", "gif", "bmp", "heic", "heif", "tif", "tiff")

/**
 * Comic-archive extensions the catalog-api comic-pages endpoint can page through.
 * Only `.cbz` is fully served today; `.cbr` is dispatched here too so the reader
 * can show the honest "not yet supported" message (HTTP 501) rather than the
 * video player silently failing on a RAR archive (§11.4.1).
 */
private val COMIC_EXTENSIONS = setOf("cbz", "cbr")

/**
 * Book extensions the catalog-api pdf-pages endpoint can render through. Only
 * `.pdf` is served today — `pdf_pages_handler.go` rejects any non-`.pdf` file with
 * HTTP 400 — so the set is intentionally `.pdf`-only (§11.4.6 — matched to the
 * backend, not guessed). Other formats keep the existing VIDEO fallback.
 */
private val BOOK_EXTENSIONS = setOf("pdf")

/**
 * Classify a leaf entity for primary-action dispatch. The authoritative signal
 * is the backend `media_type` (catalog-api seeds `comic` + `image` + `book`); when
 * that is absent/unknown a file-extension fallback on the entity path keeps a
 * comic / book out of the video player and an image out of the player. Pure +
 * internal for unit testing without a device (§11.4.6 — classification proven,
 * not guessed).
 *
 * Order is load-bearing: the `comic` media_type wins first, then `image`, then
 * `book`, then the extension fallbacks (comic, image, book) — so the pre-existing
 * IMAGE / VIDEO / COMIC behaviour is unchanged and only the new BOOK case is added.
 */
internal fun leafActionKindFor(mediaType: String?, path: String? = null): LeafActionKind {
    if (mediaType == "comic") return LeafActionKind.COMIC
    if (mediaType == "image") return LeafActionKind.IMAGE
    if (mediaType == "book") return LeafActionKind.BOOK
    val ext = path?.substringAfterLast('.', "")?.lowercase()
    if (!ext.isNullOrEmpty() && ext in COMIC_EXTENSIONS) return LeafActionKind.COMIC
    if (!ext.isNullOrEmpty() && ext in IMAGE_EXTENSIONS) return LeafActionKind.IMAGE
    if (!ext.isNullOrEmpty() && ext in BOOK_EXTENSIONS) return LeafActionKind.BOOK
    return LeafActionKind.VIDEO
}

/**
 * Enhanced media detail screen with improved UI/UX.
 * Features hero poster, metadata badges, action buttons with clear CTAs,
 * synopsis, and file info with proper spacing and visual feedback.
 *
 * For container entities (tv_show / tv_season / music_album) it additionally
 * fetches `GET /api/v1/entities/{id}/children` and renders the children as a
 * focusable D-pad row, each navigating into its own detail screen so the user
 * can drill seasons → episodes (or album → tracks) and pick a specific title
 * to play. Without this row a TV show exposes only Play / Back and the user can
 * never choose an episode (DEFECT-F, §11.4.143).
 */
@Composable
fun MediaDetailScreen(
    mediaId: Long,
    onNavigateBack: () -> Unit,
    onNavigateToPlayer: (Long) -> Unit,
    onNavigateToMediaDetail: (Long) -> Unit = onNavigateToPlayer,
    // Image leaf types dispatch here instead of the video player. Defaults to
    // the player callback so existing callers/tests stay backwards-compatible
    // (an image still routes somewhere rather than crashing) until the host
    // wires the dedicated image-viewer route.
    onNavigateToImageViewer: (Long) -> Unit = onNavigateToPlayer,
    // Comic leaf types (media_type == "comic" / .cbz / .cbr) dispatch here to the
    // dedicated comic reader. Defaults to the player callback for back-compat so
    // existing callers/tests keep compiling and a comic still routes somewhere
    // rather than crashing until the host wires the comic-reader route.
    onNavigateToComicReader: (Long) -> Unit = onNavigateToPlayer,
    // Book leaf types (media_type == "book" / .pdf) dispatch here to the dedicated
    // PDF book reader. Defaults to the player callback for back-compat so existing
    // callers/tests keep compiling and a book still routes somewhere rather than
    // crashing until the host wires the book-reader route.
    onNavigateToBookReader: (Long) -> Unit = onNavigateToPlayer
) {
    val container = DependencyContainer.getInstance(androidx.compose.ui.platform.LocalContext.current)
    var mediaItem by remember { mutableStateOf<MediaItem?>(null) }
    var isLoading by remember { mutableStateOf(true) }
    var error by remember { mutableStateOf<String?>(null) }
    var retryCount by remember { mutableStateOf(0) }
    var isFavorite by remember { mutableStateOf(false) }
    // Children of a container entity (seasons of a show, episodes of a season,
    // tracks of an album). Empty for leaf types or until the fetch completes.
    var children by remember { mutableStateOf<List<MediaItem>>(emptyList()) }
    var childrenLoading by remember { mutableStateOf(false) }
    var showHistory by remember { mutableStateOf(false) }
    // Tracks previous showHistory value so we can detect the
    // true→false transition (dialog dismissed) and restore focus.
    // Without this, LaunchedEffect(showHistory) would fire on the
    // initial composition and try to focus before the button exists.
    var wasHistoryOpen by remember { mutableStateOf(false) }
    val coroutineScope = rememberCoroutineScope()
    val scrollState = rememberScrollState()
    val playButtonFocus = remember { FocusRequester() }
    // FocusRequester for the History button — paired with the
    // wasHistoryOpen flag to restore focus after HistoryDialog closes
    // (HELIX-154 fix — RULE-TV-002 / master plan §4.4 focus hygiene).
    val historyButtonFocus = remember { FocusRequester() }
    // Focus target for the first child tile so a container entity (which hides
    // the Play button) still lands D-pad focus on an actionable element.
    val firstChildFocus = remember { FocusRequester() }
    val config = LocalConfiguration.current
    val isCompact = config.screenWidthDp < 600

    // Fetch entity details from API with retry
    LaunchedEffect(mediaId, retryCount) {
        isLoading = true
        var lastError: String? = null
        for (attempt in 0..2) {
            try {
                val result = container.mediaRepository.getMediaById(mediaId).first()
                if (result != null) {
                    mediaItem = result
                    error = null
                    val entityType = result.mediaType ?: "movie"
                    isFavorite = container.mediaRepository.checkFavorite(entityType, mediaId)
                    lastError = null
                    break
                } else {
                    lastError = "Media item not found (id=$mediaId). The server may still be processing this entry."
                }
            } catch (e: java.net.ConnectException) {
                lastError = "Cannot connect to server. Check that the server is running and reachable."
            } catch (e: java.net.SocketTimeoutException) {
                lastError = "Server request timed out. The server may be busy or unreachable."
            } catch (e: Exception) {
                lastError = "Failed to load media: ${e.message}"
            }
            if (attempt < 2 && lastError != null) {
                kotlinx.coroutines.delay((attempt + 1) * 1000L)
            }
        }
        error = lastError
        isLoading = false
    }

    // Fetch children for container entities so the user can drill into
    // seasons → episodes (or album → tracks) and pick a specific title.
    // Keyed on the loaded entity's type+id so it re-runs when the item loads
    // (or changes via retry). Leaf entities clear the children list.
    LaunchedEffect(mediaItem?.id, mediaItem?.mediaType) {
        val item = mediaItem
        if (item != null && isContainerType(item.mediaType)) {
            childrenLoading = true
            try {
                children = container.mediaRepository.getEntityChildren(item.id)
            } catch (e: Exception) {
                children = emptyList()
                android.util.Log.w("MediaDetail", "getEntityChildren failed", e)
            } finally {
                childrenLoading = false
            }
        } else {
            children = emptyList()
            childrenLoading = false
        }
    }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFF0A0A0A))
    ) {
        when {
            isLoading -> {
                Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    CircularProgressIndicator(color = Color.White)
                }
            }
            error != null -> {
                Column(
                    Modifier.fillMaxSize(),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.Center
                ) {
                    Text(
                        "Unable to Load Media",
                        style = MaterialTheme.typography.headlineMedium,
                        color = Color.White,
                        fontWeight = FontWeight.Bold
                    )
                    Spacer(Modifier.height(8.dp))
                    Text(
                        "Please check your network connection and verify the server is reachable.",
                        style = MaterialTheme.typography.bodyMedium,
                        color = Color.White.copy(alpha = 0.6f)
                    )
                    Spacer(Modifier.height(12.dp))
                    Text(
                        error ?: "An unknown error occurred",
                        style = MaterialTheme.typography.bodyLarge,
                        color = Color.White.copy(alpha = 0.85f)
                    )
                    Spacer(Modifier.height(32.dp))
                    Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                        Button(onClick = {
                            error = null
                            isLoading = true
                            retryCount++
                        }) {
                            Text("Retry")
                        }
                        // Simplified back button with full text visibility
                        Button(onClick = onNavigateBack) {
                            Text("Back")
                        }
                    }
                }
            }
            mediaItem != null -> {
                // HELIX-152 fix — capture the checked value into a local
                // so a concurrent recomposition that clears mediaItem
                // (retry, process restore from Recents, navigation
                // tear-down) can't NPE at the !! below. Previously this
                // was `val item = mediaItem!!` which could throw if the
                // branch body re-read mediaItem after the when-head
                // already checked it.
                val item = mediaItem ?: return@Box
                // Container entities (show / season / album) are not directly
                // playable — they expose their children instead of Play Now.
                val isContainer = isContainerType(item.mediaType)
                // Leaf-action dispatch: an "image" entity (or image-extension
                // path) opens the image viewer; a "comic" entity (or .cbz/.cbr
                // path) opens the comic reader; everything else keeps the video
                // player route. Container entities never use this.
                val leafKind = leafActionKindFor(item.mediaType, item.smbPath)
                val isImageLeaf = leafKind == LeafActionKind.IMAGE
                val isComicLeaf = leafKind == LeafActionKind.COMIC
                val isBookLeaf = leafKind == LeafActionKind.BOOK
                val coverUrl = item.thumbnailUrl?.let { url ->
                    when {
                        url.startsWith("/") ->
                            container.getServerUrl().trimEnd('/') + url
                        url.contains("image.tmdb.org") -> {
                            val encoded = java.net.URLEncoder.encode(url, "UTF-8")
                            container.getServerUrl().trimEnd('/') +
                                "/api/v1/image-proxy?url=$encoded"
                        }
                        else -> url
                    }
                }

                Column(
                    modifier = Modifier
                        .fillMaxSize()
                        .verticalScroll(scrollState)
                ) {
                    // Hero poster section
                    Box(
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(if (isCompact) 280.dp else 400.dp),
                        contentAlignment = Alignment.TopCenter
                    ) {
                        // Bounded + branded hero (§11.4.162 / §11.4.170). HeroPoster
                        // fills this 280/400 dp Box with ContentScale.Crop when a
                        // cover is present, and renders the branded navy-gradient +
                        // logo + title fallback when the cover is missing or fails —
                        // never the old unbounded FillWidth gray placeholder bar.
                        HeroPoster(
                            coverUrl = coverUrl,
                            title = item.title,
                            mediaType = item.mediaType,
                            modifier = Modifier.fillMaxSize()
                        )
                        // Gradient overlay for text readability
                        Box(
                            Modifier.fillMaxSize().background(
                                Brush.verticalGradient(
                                    listOf(Color.Transparent, Color(0xCC0A0A0A), Color(0xFF0A0A0A)),
                                    startY = 100f
                                )
                            )
                        )
                        // Back button with icon and label for clarity
                        Button(
                            onClick = onNavigateBack,
                            modifier = Modifier.padding(16.dp).align(Alignment.TopStart),
                            scale = ButtonDefaults.scale(focusedScale = 1.08f),
                            glow = ButtonDefaults.glow(focusedGlow = Glow(elevation = 6.dp, elevationColor = MaterialTheme.colorScheme.primary))
                        ) {
                            Box(
                                modifier = Modifier.fillMaxSize(),
                                contentAlignment = Alignment.Center
                            ) {
                                Row(
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                                ) {
                                    M3Icon(Icons.Default.ArrowBack, "Back", Modifier.size(20.dp))
                                    Text("Back")
                                }
                            }
                        }
                    }

                    // Metadata content with improved spacing
                    Column(
                        modifier = Modifier.padding(
                            horizontal = if (isCompact) 24.dp else 48.dp,
                            vertical = 24.dp
                        )
                    ) {
                        // Title
                        Text(
                            text = item.title,
                            style = if (isCompact) MaterialTheme.typography.headlineSmall
                                    else MaterialTheme.typography.headlineMedium,
                            color = Color.White,
                            fontWeight = FontWeight.Bold,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis
                        )

                        Spacer(Modifier.height(12.dp))

                        // Year + Rating + Type badges with improved styling
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(16.dp)
                        ) {
                            item.year?.let {
                                Text(
                                    it.toString(), 
                                    color = Color.White.copy(0.8f), 
                                    style = MaterialTheme.typography.bodyLarge
                                )
                            }
                            item.rating?.let { r ->
                                if (r > 0) {
                                    Row(verticalAlignment = Alignment.CenterVertically) {
                                        M3Icon(
                                            Icons.Default.Star, 
                                            "Rating", 
                                            Modifier.size(18.dp), 
                                            tint = Color(0xFFFFD700)
                                        )
                                        Spacer(Modifier.width(6.dp))
                                        Text(
                                            String.format("%.1f", r), 
                                            color = Color(0xFFFFD700),
                                            style = MaterialTheme.typography.bodyLarge, 
                                            fontWeight = FontWeight.Bold
                                        )
                                    }
                                }
                            }
                            Box(
                                Modifier.background(Color.White.copy(0.15f), RoundedCornerShape(6.dp))
                                    .padding(horizontal = 12.dp, vertical = 4.dp)
                            ) {
                                val typeLabel = when(item.mediaType) {
                                    "movie" -> "Movie"
                                    "tv_show" -> "TV Show"
                                    "tv_episode" -> "Episode"
                                    "music_album" -> "Album"
                                    "song" -> "Song"
                                    "game" -> "Game"
                                    "book" -> "Book"
                                    else -> (item.mediaType ?: "unknown").replace("_", " ")
                                        .replaceFirstChar { it.uppercase() }
                                }
                                Text(
                                    typeLabel,
                                    color = Color.White.copy(0.9f), 
                                    style = MaterialTheme.typography.labelMedium
                                )
                            }
                            item.quality?.let { q ->
                                Box(
                                    Modifier.background(Color(0xFF1B5E20).copy(0.8f), RoundedCornerShape(6.dp))
                                        .padding(horizontal = 12.dp, vertical = 4.dp)
                                ) {
                                    Text(q.uppercase(), color = Color.White, style = MaterialTheme.typography.labelMedium)
                                }
                            }
                        }

                        Spacer(Modifier.height(24.dp))

                        // Action buttons with improved spacing and clear CTAs
                        Row(
                            horizontalArrangement = Arrangement.spacedBy(16.dp),
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            // Primary Play button — only for directly-playable
                            // leaf types. Container entities (show/season/album)
                            // can't be played directly; they show their children
                            // row below so the user picks a specific title.
                            if (!isContainer) {
                                Button(
                                    onClick = {
                                        when (leafKind) {
                                            LeafActionKind.IMAGE -> onNavigateToImageViewer(mediaId)
                                            LeafActionKind.COMIC -> onNavigateToComicReader(mediaId)
                                            LeafActionKind.BOOK -> onNavigateToBookReader(mediaId)
                                            LeafActionKind.VIDEO -> onNavigateToPlayer(mediaId)
                                        }
                                    },
                                    modifier = Modifier
                                        .height(52.dp)
                                        .focusRequester(playButtonFocus)
                                        .focusable(),
                                    scale = ButtonDefaults.scale(focusedScale = 1.08f),
                                    glow = ButtonDefaults.glow(focusedGlow = Glow(elevation = 8.dp, elevationColor = MaterialTheme.colorScheme.primary)),
                                    border = ButtonDefaults.border(
                                        focusedBorder = Border(
                                            androidx.compose.foundation.BorderStroke(2.dp, MaterialTheme.colorScheme.primary),
                                            shape = RoundedCornerShape(8.dp)
                                        )
                                    )
                                ) {
                                    Box(
                                        modifier = Modifier.fillMaxSize(),
                                        contentAlignment = Alignment.Center
                                    ) {
                                        Row(
                                            verticalAlignment = Alignment.CenterVertically,
                                            horizontalArrangement = Arrangement.spacedBy(10.dp)
                                        ) {
                                            M3Icon(
                                                when {
                                                    isComicLeaf -> Icons.Default.MenuBook
                                                    isImageLeaf -> Icons.Default.Image
                                                    else -> Icons.Default.PlayArrow
                                                },
                                                when {
                                                    isComicLeaf -> "Read"
                                                    isImageLeaf -> "View"
                                                    else -> "Play"
                                                },
                                                Modifier.size(28.dp)
                                            )
                                            Text(
                                                when {
                                                    isComicLeaf -> "Read Comic"
                                                    isImageLeaf -> "View Image"
                                                    else -> "Play Now"
                                                },
                                                style = MaterialTheme.typography.titleMedium,
                                                fontWeight = FontWeight.SemiBold
                                            )
                                        }
                                    }
                                }
                            }

                            // Favorite button with clear state indication
                            Button(
                                onClick = {
                                    val oldState = isFavorite
                                    isFavorite = !oldState
                                    coroutineScope.launch {
                                        try {
                                            val entityType = item.mediaType ?: "movie"
                                            container.mediaRepository.toggleFavorite(entityType, mediaId, oldState)
                                        } catch (e: Exception) {
                                            isFavorite = oldState
                                            android.util.Log.e("MediaDetail", "Favorite toggle failed", e)
                                        }
                                    }
                                },
                                modifier = Modifier.height(52.dp),
                                scale = ButtonDefaults.scale(focusedScale = 1.08f),
                                glow = ButtonDefaults.glow(focusedGlow = Glow(elevation = 8.dp, elevationColor = MaterialTheme.colorScheme.primary)),
                                border = ButtonDefaults.border(
                                    focusedBorder = Border(
                                        androidx.compose.foundation.BorderStroke(2.dp, MaterialTheme.colorScheme.primary),
                                        shape = RoundedCornerShape(8.dp)
                                    )
                                )
                            ) {
                                Box(
                                    modifier = Modifier.fillMaxSize(),
                                    contentAlignment = Alignment.Center
                                ) {
                                    Row(
                                        verticalAlignment = Alignment.CenterVertically,
                                        horizontalArrangement = Arrangement.spacedBy(10.dp)
                                    ) {
                                        M3Icon(
                                            if (isFavorite) Icons.Default.Favorite else Icons.Default.FavoriteBorder,
                                            contentDescription = if (isFavorite) "Remove from Favorites" else "Add to Favorites",
                                            modifier = Modifier.size(24.dp),
                                            tint = if (isFavorite) Color(0xFFFF6B6B) else Color.White
                                        )
                                        Text(
                                            if (isFavorite) "Favorited" else "Favorite",
                                            style = MaterialTheme.typography.titleMedium
                                        )
                                    }
                                }
                            }

                            // History button — opens full reproduction
                            // history dialog showing aggregate totals + all
                            // session rows. Owns its own FocusRequester so
                            // that after HistoryDialog dismisses (via BACK
                            // or KEYCODE_ENTER on a focusable dismiss area)
                            // focus can be restored here rather than
                            // evaporating into the void (HELIX-154 fix —
                            // "Focus Lost After Dialog Dismissal").
                            Button(
                                onClick = { showHistory = true },
                                modifier = Modifier
                                    .height(52.dp)
                                    .focusRequester(historyButtonFocus),
                                scale = ButtonDefaults.scale(focusedScale = 1.08f),
                                glow = ButtonDefaults.glow(focusedGlow = Glow(elevation = 8.dp, elevationColor = MaterialTheme.colorScheme.primary)),
                                border = ButtonDefaults.border(
                                    focusedBorder = Border(
                                        androidx.compose.foundation.BorderStroke(2.dp, MaterialTheme.colorScheme.primary),
                                        shape = RoundedCornerShape(8.dp)
                                    )
                                )
                            ) {
                                Box(
                                    modifier = Modifier.fillMaxSize(),
                                    contentAlignment = Alignment.Center
                                ) {
                                    Row(
                                        verticalAlignment = Alignment.CenterVertically,
                                        horizontalArrangement = Arrangement.spacedBy(10.dp)
                                    ) {
                                        M3Icon(
                                            Icons.Default.History,
                                            contentDescription = "View reproduction history",
                                            modifier = Modifier.size(24.dp)
                                        )
                                        Text(
                                            "History",
                                            style = MaterialTheme.typography.titleMedium
                                        )
                                    }
                                }
                            }
                        }

                        // Children row for container entities — lets the user
                        // drill into seasons → episodes (or album → tracks) and
                        // pick a specific title to play (DEFECT-F, §11.4.143).
                        if (isContainer) {
                            Spacer(Modifier.height(32.dp))
                            Text(
                                childSectionTitle(item.mediaType),
                                style = MaterialTheme.typography.titleLarge,
                                color = Color.White,
                                fontWeight = FontWeight.Bold
                            )
                            Spacer(Modifier.height(12.dp))
                            when {
                                childrenLoading -> {
                                    Box(
                                        Modifier.fillMaxWidth().height(180.dp),
                                        contentAlignment = Alignment.CenterStart
                                    ) {
                                        CircularProgressIndicator(color = Color.White)
                                    }
                                }
                                children.isEmpty() -> {
                                    Text(
                                        "No ${childSectionTitle(item.mediaType).lowercase()} available.",
                                        style = MaterialTheme.typography.bodyMedium,
                                        color = Color.White.copy(0.6f)
                                    )
                                }
                                else -> {
                                    androidx.tv.foundation.lazy.list.TvLazyRow(
                                        horizontalArrangement = Arrangement.spacedBy(16.dp)
                                    ) {
                                        items(children.size) { index ->
                                            val child = children[index]
                                            ChildTile(
                                                child = child,
                                                serverUrl = container.getServerUrl(),
                                                onClick = { onNavigateToMediaDetail(child.id) },
                                                modifier = if (index == 0) {
                                                    Modifier.focusRequester(firstChildFocus)
                                                } else {
                                                    Modifier
                                                }
                                            )
                                        }
                                    }
                                }
                            }
                        }

                        Spacer(Modifier.height(32.dp))

                        // Synopsis section
                        item.description?.let { desc ->
                            if (desc.isNotBlank()) {
                                Text(
                                    "Synopsis",
                                    style = MaterialTheme.typography.titleLarge,
                                    color = Color.White,
                                    fontWeight = FontWeight.Bold
                                )
                                Spacer(Modifier.height(12.dp))
                                Text(
                                    desc,
                                    style = MaterialTheme.typography.bodyLarge,
                                    color = Color.White.copy(0.85f),
                                    lineHeight = MaterialTheme.typography.bodyLarge.lineHeight * 1.2f
                                )
                                Spacer(Modifier.height(24.dp))
                            }
                        }

                        // File info section
                        item.fileSize?.let { size ->
                            if (size > 0) {
                                val s = when {
                                    size > 1073741824 -> String.format("%.1f GB", size.toFloat() / 1073741824)
                                    size > 1048576 -> String.format("%.1f MB", size.toFloat() / 1048576)
                                    else -> String.format("%.1f KB", size.toFloat() / 1024)
                                }
                                Row(
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                                ) {
                                    Text(
                                        "File Size:",
                                        color = Color.White.copy(0.6f),
                                        style = MaterialTheme.typography.bodyMedium
                                    )
                                    Text(
                                        s,
                                        color = Color.White.copy(0.9f),
                                        style = MaterialTheme.typography.bodyMedium,
                                        fontWeight = FontWeight.Medium
                                    )
                                }
                            }
                        }
                        Spacer(Modifier.height(48.dp))
                    }
                }
            }
        }
    }

    // Auto-focus when content loads. For a container entity (no Play button)
    // land focus on the first child tile once children are present; otherwise
    // focus the Play button. Keyed on children.size so a container re-requests
    // focus after its children arrive.
    LaunchedEffect(mediaItem, children.size) {
        val item = mediaItem ?: return@LaunchedEffect
        kotlinx.coroutines.delay(300)
        if (isContainerType(item.mediaType) && children.isNotEmpty()) {
            try { firstChildFocus.requestFocus() } catch (_: Exception) {}
        } else if (!isContainerType(item.mediaType)) {
            try { playButtonFocus.requestFocus() } catch (_: Exception) {}
        }
    }

    // HELIX-154 fix — after HistoryDialog dismisses, restore focus to
    // the trigger button (History). Detect the true→false transition
    // via wasHistoryOpen so this LaunchedEffect doesn't fire during
    // the initial composition. A small delay lets the dialog teardown
    // settle before the focus request lands.
    LaunchedEffect(showHistory) {
        if (!showHistory && wasHistoryOpen) {
            wasHistoryOpen = false
            kotlinx.coroutines.delay(100)
            try { historyButtonFocus.requestFocus() } catch (_: Exception) {}
        } else if (showHistory) {
            wasHistoryOpen = true
        }
    }

    if (showHistory) {
        mediaItem?.let { item ->
            HistoryDialog(
                mediaItemId = mediaId,
                mediaTitle = item.title,
                repository = container.playbackRepository,
                onDismiss = { showHistory = false }
            )
        }
    }
}

/**
 * Resolve a child entity's cover URL the same way the hero poster is resolved:
 * relative paths get the server prefix, TMDB urls route through the image proxy,
 * everything else is used as-is. Pure + internal so it is unit-testable.
 */
internal fun resolveChildCoverUrl(rawUrl: String?, serverUrl: String): String? = rawUrl?.let { url ->
    when {
        url.startsWith("/") -> serverUrl.trimEnd('/') + url
        url.contains("image.tmdb.org") -> {
            val encoded = java.net.URLEncoder.encode(url, "UTF-8")
            serverUrl.trimEnd('/') + "/api/v1/image-proxy?url=$encoded"
        }
        else -> url
    }
}

/**
 * Subtitle for a child tile — surfaces the season/episode/track ordinal so a
 * row of children is self-describing (e.g. "Season 1", "Episode 3", "Track 5").
 * Falls back to a humanised media-type label. Pure + internal for unit testing.
 */
internal fun childTileSubtitle(child: MediaItem): String? = when {
    child.seasonNumber != null && child.mediaType == "tv_season" -> "Season ${child.seasonNumber}"
    child.episodeNumber != null -> "Episode ${child.episodeNumber}"
    child.trackNumber != null -> "Track ${child.trackNumber}"
    child.seasonNumber != null -> "Season ${child.seasonNumber}"
    else -> child.mediaType?.replace("_", " ")?.replaceFirstChar { it.uppercase() }
}

/**
 * Focusable D-pad tile for a single child entity (season / episode / track).
 * Self-contained (does not depend on MediaCard) so it does not couple this
 * screen to a component under concurrent review.
 */
@Composable
private fun ChildTile(
    child: MediaItem,
    serverUrl: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier
) {
    val coverUrl = resolveChildCoverUrl(child.thumbnailUrl, serverUrl)
    Card(
        onClick = onClick,
        modifier = modifier.width(140.dp),
        scale = CardDefaults.scale(focusedScale = 1.1f),
        glow = CardDefaults.glow(
            focusedGlow = Glow(elevation = 8.dp, elevationColor = MaterialTheme.colorScheme.primary)
        ),
        border = CardDefaults.border(
            focusedBorder = Border(
                androidx.compose.foundation.BorderStroke(2.dp, MaterialTheme.colorScheme.primary),
                shape = RoundedCornerShape(8.dp)
            )
        )
    ) {
        Column {
            CoverImage(
                url = coverUrl,
                contentDescription = child.title,
                modifier = Modifier
                    .width(140.dp)
                    .height(200.dp),
                contentScale = ContentScale.Crop,
                mediaType = child.mediaType
            )
            Column(modifier = Modifier.padding(8.dp)) {
                Text(
                    text = child.title,
                    style = MaterialTheme.typography.bodyMedium,
                    color = Color.White,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis
                )
                childTileSubtitle(child)?.let { subtitle ->
                    Spacer(Modifier.height(2.dp))
                    Text(
                        text = subtitle,
                        style = MaterialTheme.typography.labelSmall,
                        color = Color.White.copy(0.7f),
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }
        }
    }
}
