package com.catalogizer.android.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.catalogizer.android.data.playback.HistoryUiState
import com.catalogizer.android.data.playback.PlaybackFormatter
import com.catalogizer.android.data.playback.loadHistoryUiState
import com.catalogizer.android.data.playback.PlaybackRepository
import com.catalogizer.android.data.playback.UiPlaybackProgress
import com.catalogizer.android.data.playback.UiPlaybackSession
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.TimeZone

/**
 * Phone variant of the reproduction history dialog. Uses Material 3
 * AlertDialog so it respects the system theme and navigation gestures.
 * Loads fresh progress + sessions on mount via [PlaybackRepository].
 */
@Composable
fun HistoryDialog(
    mediaItemId: Long,
    mediaTitle: String,
    repository: PlaybackRepository,
    onDismiss: () -> Unit,
) {
    var state by remember { mutableStateOf<HistoryUiState>(HistoryUiState.Loading) }

    LaunchedEffect(mediaItemId) {
        state = loadHistoryUiState(repository, mediaItemId, limit = 100)
    }

    AlertDialog(
        onDismissRequest = onDismiss,
        confirmButton = {
            TextButton(onClick = onDismiss) { Text("Close") }
        },
        title = {
            Column {
                Text("Reproduction History", fontWeight = FontWeight.Bold)
                Text(
                    mediaTitle,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        },
        text = {
            Column(
                verticalArrangement = Arrangement.spacedBy(12.dp),
                modifier = Modifier.heightIn(max = 480.dp),
            ) {
                when (val st = state) {
                    is HistoryUiState.Loading -> Text(
                        "Loading…",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    is HistoryUiState.Error -> Text(
                        "Could not load history (${st.message}). Close and try again.",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.error,
                    )
                    is HistoryUiState.Loaded -> {
                        HistorySummary(st.progress)
                        HistorySessionList(st.sessions, true)
                    }
                }
            }
        },
    )
}

@Composable
private fun HistorySummary(progress: UiPlaybackProgress?) {
    val summaryRows = if (progress == null) {
        listOf("Status" to "Never reproduced")
    } else {
        buildList {
            add("Total duration" to PlaybackFormatter.formatAmount(
                progress.durationTotal ?: 0L, progress.positionUnit))
            add("Current position" to PlaybackFormatter.formatProgress(
                progress.lastPosition, progress.durationTotal, progress.positionUnit))
            add("Last session" to PlaybackFormatter.formatAmount(
                progress.lastSessionAmount, progress.positionUnit))
            add("Times played" to PlaybackFormatter.formatReproductionCount(
                progress.totalReproductions))
            add("Total reproduced" to PlaybackFormatter.formatAmount(
                progress.aggregateAmount, progress.positionUnit))
        }
    }
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(12.dp))
            .background(MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.5f))
            .padding(12.dp),
        verticalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        summaryRows.forEach { (label, value) ->
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
            ) {
                Text(label, style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
                Text(value, style = MaterialTheme.typography.bodySmall,
                    fontWeight = FontWeight.SemiBold)
            }
        }
    }
}

@Composable
private fun HistorySessionList(
    sessions: List<UiPlaybackSession>,
    loaded: Boolean,
) {
    when {
        !loaded -> Text(
            "Loading…",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        sessions.isEmpty() -> Text(
            "No sessions yet. Press Play to record your first.",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        else -> {
            Text(
                "Sessions",
                style = MaterialTheme.typography.titleSmall,
                fontWeight = FontWeight.Bold,
            )
            LazyColumn(
                verticalArrangement = Arrangement.spacedBy(6.dp),
                contentPadding = PaddingValues(bottom = 4.dp),
            ) {
                items(sessions, key = { it.id }) { session -> SessionRow(session) }
            }
        }
    }
}

@Composable
private fun SessionRow(session: UiPlaybackSession) {
    val date = formatInstant(session.startedAtMs)
    val amount = PlaybackFormatter.formatAmount(session.totalAmount, session.positionUnit)
    val status = if (session.completed) "Completed" else "In progress"
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(8.dp))
            .background(MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.35f))
            .padding(horizontal = 10.dp, vertical = 6.dp),
    ) {
        Text(date, style = MaterialTheme.typography.bodySmall,
            modifier = Modifier.width(130.dp))
        Text(amount, style = MaterialTheme.typography.bodySmall,
            modifier = Modifier.width(80.dp))
        Text(
            status,
            style = MaterialTheme.typography.bodySmall,
            color = if (session.completed) MaterialTheme.colorScheme.primary
                    else MaterialTheme.colorScheme.tertiary,
        )
    }
}

private fun formatInstant(ms: Long): String {
    if (ms <= 0L) return "-"
    val fmt = SimpleDateFormat("yyyy-MM-dd HH:mm", Locale.getDefault())
    fmt.timeZone = TimeZone.getDefault()
    return fmt.format(Date(ms))
}
