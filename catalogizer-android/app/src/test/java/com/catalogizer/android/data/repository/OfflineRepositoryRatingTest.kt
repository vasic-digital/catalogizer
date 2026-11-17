package com.catalogizer.android.data.repository

import androidx.room.Room
import androidx.test.core.app.ApplicationProvider
import com.catalogizer.android.data.local.CatalogizerDatabase
import com.catalogizer.android.data.models.MediaItem
import com.catalogizer.android.data.sync.SyncManager
import io.mockk.coVerify
import io.mockk.mockk
import kotlinx.coroutines.runBlocking
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/**
 * RATING1-R-F3: the user's own 0..10 rating goes to the per-user user_ratings table; the community
 * media_items.rating column is not touched. Real Room (in-memory) through the real OfflineRepository.
 */
@RunWith(RobolectricTestRunner::class)
@Config(manifest = Config.NONE)
class OfflineRepositoryRatingTest {
    private lateinit var db: CatalogizerDatabase
    private val sync = mockk<SyncManager>(relaxed = true)
    private lateinit var repo: OfflineRepository

    @Before
    fun setup() {
        val ctx = ApplicationProvider.getApplicationContext<android.content.Context>()
        db = Room.inMemoryDatabaseBuilder(ctx, CatalogizerDatabase::class.java).allowMainThreadQueries().build()
        repo = OfflineRepository(db, sync, ctx)
    }

    @After
    fun tearDown() = db.close()

    private fun item(rating: Double?) = MediaItem(
        id = 1L, title = "T", mediaType = "movie", directoryPath = "/d",
        createdAt = "2024-01-01T00:00:00Z", updatedAt = "2024-01-01T00:00:00Z", rating = rating,
    )

    @Test
    fun `rateMediaOffline stores the user rating and leaves the community rating alone`() = runBlocking {
        db.mediaDao().insertOrUpdate(item(rating = 6.2))

        repo.rateMediaOffline(1L, 9)

        assertEquals(9, repo.getUserRating(1L))
        assertEquals(6.2, db.mediaDao().getById(1L)!!.rating!!, 0.0)
        coVerify { sync.queueRatingUpdate(1L, 9) }
    }

    @Test
    fun `a server refresh that replaces the media row keeps the user rating`() = runBlocking {
        db.mediaDao().insertOrUpdate(item(rating = 6.2))
        repo.rateMediaOffline(1L, 3)

        db.mediaDao().insertOrUpdate(item(rating = 7.7))

        assertEquals(3, repo.getUserRating(1L))
        assertEquals(7.7, db.mediaDao().getById(1L)!!.rating!!, 0.0)
    }

    @Test
    fun `rating 0 is a real rating and an unrated item is null`() = runBlocking {
        assertNull(repo.getUserRating(2L))
        repo.rateMediaOffline(2L, 0)
        assertEquals(0, repo.getUserRating(2L))
    }

    @Test
    fun `a rating outside 0 to 10 is refused and nothing is stored or queued`() = runBlocking {
        val e = runCatching { repo.rateMediaOffline(1L, 11) }.exceptionOrNull()
        assertTrue(e is IllegalArgumentException)
        assertNull(repo.getUserRating(1L))
        coVerify(exactly = 0) { sync.queueRatingUpdate(any(), any()) }
    }

    @Test
    fun `rating the same item twice replaces the first rating (P2 REPLACE not ABORT)`() = runBlocking {
        repo.rateMediaOffline(1L, 4)
        repo.rateMediaOffline(1L, 9)
        assertEquals(9, repo.getUserRating(1L))
        assertEquals(1, db.openHelper.readableDatabase.query("SELECT COUNT(*) FROM user_ratings WHERE media_id = 1").use {
            it.moveToFirst(); it.getInt(0)
        })
    }

    @Test
    fun `UserRatingDao delete removes the rating`() = runBlocking {
        repo.rateMediaOffline(1L, 6)
        db.userRatingDao().delete(1L)
        assertNull(repo.getUserRating(1L))
        db.userRatingDao().delete(1L) // deleting a missing row is a no-op
        assertNull(db.userRatingDao().get(1L))
    }
}
