package com.catalogizer.androidtv.ui.player

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * CLFU-R-3, activity wiring. HONEST LIMIT: this is a source-shape pin, not a behavioural test of the activity (the
 * activity needs libVLC, which a unit test cannot load). The behaviour (release before network, snapshot, NonCancellable)
 * is tested in PlayerShutdownTest; this pin only keeps onDestroy routed through PlayerShutdown and keeps a second,
 * network-ordered player release from coming back.
 */
class VLCPlayerActivityDestroyWiringTest {
    private val src = java.io.File("src/main/java/com/catalogizer/androidtv/ui/player/VLCPlayerActivity.kt").readText()
    private val onDestroy: String = src.substringAfter("override fun onDestroy()").substringBefore("override fun onKeyDown")

    @Test
    fun `onDestroy goes through PlayerShutdown and releases the player exactly once, inside it`() {
        assertTrue(onDestroy.contains("PlayerShutdown.run("))
        assertEquals(1, Regex("""vlcPlayer\.release\(\)""").findAll(onDestroy).count())
        assertTrue("release must be the release= lambda", onDestroy.contains("release = { vlcPlayer.release() }"))
    }

    @Test
    fun `the shutdown scope runs the read and the release inside onDestroy (Main immediate), not later`() {
        assertTrue(onDestroy.contains("Dispatchers.Main.immediate"))
        assertTrue(!onDestroy.contains("Dispatchers.Main)"))
    }

    @Test
    fun `the whole file has a single vlcPlayer release call`() {
        assertEquals(1, Regex("""vlcPlayer\.release\(\)""").findAll(src).count())
    }
}
