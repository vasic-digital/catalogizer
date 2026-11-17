package com.catalogizer.android.data.local

import androidx.room.Room
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/**
 * CLFU-R-2: a REAL version-1 database file (built from the exported 1.json schema, with a media row and a queued
 * UPLOAD_RATING) is opened by Room at version 2 with ALL_MIGRATIONS; Room itself validates the result against the
 * v2 schema. Adopted from the reviewer's probe (ReviewerV1UpgradeProbeTest).
 */
@RunWith(RobolectricTestRunner::class)
@Config(manifest = Config.NONE)
class CatalogizerDatabaseUpgradeTest {
    private fun createV1(name: String) {
        val ctx = ApplicationProvider.getApplicationContext<android.content.Context>()
        ctx.deleteDatabase(name)
        val v1 = Json.parseToJsonElement(
            java.io.File("schemas/com.catalogizer.android.data.local.CatalogizerDatabase/1.json").readText()
        ).jsonObject["database"]!!.jsonObject
        val path = ctx.getDatabasePath(name).also { it.parentFile!!.mkdirs() }
        val raw = android.database.sqlite.SQLiteDatabase.openOrCreateDatabase(path, null)
        for (e in v1["entities"]!!.jsonArray) {
            val o = e.jsonObject
            val t = o["tableName"]!!.jsonPrimitive.content
            raw.execSQL(o["createSql"]!!.jsonPrimitive.content.replace("\${TABLE_NAME}", t))
            o["indices"]?.jsonArray?.forEach { i ->
                raw.execSQL(i.jsonObject["createSql"]!!.jsonPrimitive.content.replace("\${TABLE_NAME}", t))
            }
        }
        v1["setupQueries"]!!.jsonArray.forEach { raw.execSQL(it.jsonPrimitive.content) }
        raw.execSQL(
            "INSERT INTO media_items (id,title,media_type,rating,directory_path,created_at,updated_at,is_favorite,watch_progress,is_downloaded) " +
                "VALUES (1,'T','movie',6.5,'/d','2024','2024',0,0.0,0)"
        )
        raw.execSQL(
            "INSERT INTO sync_operations (id,type,mediaId,data,timestamp,retryCount,maxRetries) " +
                "VALUES (8,'UPLOAD_RATING',1,'{\"mediaId\":1,\"rating\":4.5}',1,0,3)"
        )
        raw.version = 1
        raw.close()
    }

    @Test
    fun `a real v1 database upgrades to v2 and keeps media and queued sync rows`() = runBlocking {
        val ctx = ApplicationProvider.getApplicationContext<android.content.Context>()
        val name = "clfu_v1.db"
        createV1(name)

        val db = Room.databaseBuilder(ctx, CatalogizerDatabase::class.java, name)
            .addMigrations(*CatalogizerDatabase.ALL_MIGRATIONS).allowMainThreadQueries().build()
        try {
            assertEquals(6.5, db.mediaDao().getById(1L)!!.rating!!, 0.0)
            assertEquals(1, db.syncOperationDao().getPendingOperationsCount())
            assertNull(db.userRatingDao().get(1L))
            db.userRatingDao().upsert(UserRating(1L, 7))
            assertEquals(7, db.userRatingDao().get(1L)!!.rating)
            // the community rating of the media row is untouched by the user rating
            assertEquals(6.5, db.mediaDao().getById(1L)!!.rating!!, 0.0)
            assertEquals(2, db.openHelper.readableDatabase.version)
        } finally {
            db.close()
            ctx.deleteDatabase(name)
        }
    }

    @Test
    fun `rating the same item twice on the upgraded database replaces the first rating`() = runBlocking {
        val ctx = ApplicationProvider.getApplicationContext<android.content.Context>()
        val name = "clfu_v1_rerate.db"
        createV1(name)
        val db = Room.databaseBuilder(ctx, CatalogizerDatabase::class.java, name)
            .addMigrations(*CatalogizerDatabase.ALL_MIGRATIONS).allowMainThreadQueries().build()
        try {
            db.userRatingDao().upsert(UserRating(1L, 7))
            db.userRatingDao().upsert(UserRating(1L, 8))
            assertEquals(8, db.userRatingDao().get(1L)!!.rating)
        } finally {
            db.close()
            ctx.deleteDatabase(name)
        }
    }
}
