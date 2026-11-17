package com.catalogizer.android.data.local

import android.content.Context
import androidx.room.Room
import com.catalogizer.android.data.sync.SyncOperationType
import io.mockk.every
import io.mockk.mockk
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import org.junit.Assert.*
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(manifest = Config.NONE)
class CatalogizerDatabaseTest {

    private val json = Json { ignoreUnknownKeys = true }

    // --- Converters ---

    private lateinit var converters: Converters

    @Before
    fun setup() {
        converters = Converters()
    }

    // -- String list converters --

    @Test
    fun `fromStringList converts list to JSON string`() {
        val list = listOf("action", "comedy", "drama")
        val result = converters.fromStringList(list)

        assertNotNull(result)
        assertTrue(result!!.contains("action"))
        assertTrue(result.contains("comedy"))
        assertTrue(result.contains("drama"))
    }

    @Test
    fun `fromStringList returns null for null input`() {
        val result = converters.fromStringList(null)
        assertNull(result)
    }

    @Test
    fun `toStringList converts JSON string to list`() {
        val jsonStr = """["action","comedy","drama"]"""
        val result = converters.toStringList(jsonStr)

        assertNotNull(result)
        assertEquals(3, result!!.size)
        assertEquals("action", result[0])
        assertEquals("comedy", result[1])
        assertEquals("drama", result[2])
    }

    @Test
    fun `toStringList returns null for null input`() {
        val result = converters.toStringList(null)
        assertNull(result)
    }

    @Test
    fun `String list round-trip preserves data`() {
        val original = listOf("one", "two", "three")
        val serialized = converters.fromStringList(original)
        val deserialized = converters.toStringList(serialized)

        assertEquals(original, deserialized)
    }

    @Test
    fun `fromStringList handles empty list`() {
        val result = converters.fromStringList(emptyList())
        assertNotNull(result)
        val back = converters.toStringList(result)
        assertNotNull(back)
        assertTrue(back!!.isEmpty())
    }

    // -- String map converters --

    @Test
    fun `fromStringMap converts map to JSON string`() {
        val map = mapOf("key1" to "value1", "key2" to "value2")
        val result = converters.fromStringMap(map)

        assertNotNull(result)
        assertTrue(result!!.contains("key1"))
        assertTrue(result.contains("value1"))
    }

    @Test
    fun `fromStringMap returns null for null input`() {
        val result = converters.fromStringMap(null)
        assertNull(result)
    }

    @Test
    fun `toStringMap converts JSON string to map`() {
        val jsonStr = """{"key1":"value1","key2":"value2"}"""
        val result = converters.toStringMap(jsonStr)

        assertNotNull(result)
        assertEquals(2, result!!.size)
        assertEquals("value1", result["key1"])
        assertEquals("value2", result["key2"])
    }

    @Test
    fun `toStringMap returns null for null input`() {
        val result = converters.toStringMap(null)
        assertNull(result)
    }

    @Test
    fun `String map round-trip preserves data`() {
        val original = mapOf("a" to "1", "b" to "2")
        val serialized = converters.fromStringMap(original)
        val deserialized = converters.toStringMap(serialized)

        assertEquals(original, deserialized)
    }

    @Test
    fun `fromStringMap handles empty map`() {
        val result = converters.fromStringMap(emptyMap())
        assertNotNull(result)
        val back = converters.toStringMap(result)
        assertNotNull(back)
        assertTrue(back!!.isEmpty())
    }

    // -- SyncOperationType converters --

    @Test
    fun `fromSyncOperationType converts enum to string`() {
        assertEquals("UPDATE_PROGRESS", converters.fromSyncOperationType(SyncOperationType.UPDATE_PROGRESS))
        assertEquals("TOGGLE_FAVORITE", converters.fromSyncOperationType(SyncOperationType.TOGGLE_FAVORITE))
        assertEquals("UPLOAD_RATING", converters.fromSyncOperationType(SyncOperationType.UPLOAD_RATING))
        assertEquals("UPDATE_METADATA", converters.fromSyncOperationType(SyncOperationType.UPDATE_METADATA))
        assertEquals("DELETE_MEDIA", converters.fromSyncOperationType(SyncOperationType.DELETE_MEDIA))
    }

    @Test
    fun `fromSyncOperationType returns null for null input`() {
        val result = converters.fromSyncOperationType(null)
        assertNull(result)
    }

    @Test
    fun `toSyncOperationType converts string to enum`() {
        assertEquals(SyncOperationType.UPDATE_PROGRESS, converters.toSyncOperationType("UPDATE_PROGRESS"))
        assertEquals(SyncOperationType.TOGGLE_FAVORITE, converters.toSyncOperationType("TOGGLE_FAVORITE"))
        assertEquals(SyncOperationType.UPLOAD_RATING, converters.toSyncOperationType("UPLOAD_RATING"))
        assertEquals(SyncOperationType.UPDATE_METADATA, converters.toSyncOperationType("UPDATE_METADATA"))
        assertEquals(SyncOperationType.DELETE_MEDIA, converters.toSyncOperationType("DELETE_MEDIA"))
    }

    @Test
    fun `toSyncOperationType returns null for null input`() {
        val result = converters.toSyncOperationType(null)
        assertNull(result)
    }

    @Test
    fun `toSyncOperationType throws for invalid string`() {
        try {
            converters.toSyncOperationType("INVALID_TYPE")
            fail("Expected IllegalArgumentException")
        } catch (e: IllegalArgumentException) {
            assertTrue(e.message!!.contains("INVALID_TYPE"))
        }
    }

    @Test
    fun `SyncOperationType round-trip preserves all values`() {
        for (type in SyncOperationType.values()) {
            val serialized = converters.fromSyncOperationType(type)
            val deserialized = converters.toSyncOperationType(serialized)
            assertEquals(type, deserialized)
        }
    }

    // --- Migration ---

    @Test
    fun `ALL_MIGRATIONS contains MIGRATION_1_2`() {
        val migrations = CatalogizerDatabase.ALL_MIGRATIONS
        assertEquals(1, migrations.size)
        assertEquals(1, migrations[0].startVersion)
        assertEquals(2, migrations[0].endVersion)
    }

    @Test
    fun `MIGRATION_1_2 creates user_ratings, keeps existing data, and matches the exported v2 schema`() {
        val ctx = RuntimeEnvironment.getApplication()
        val helper = androidx.sqlite.db.framework.FrameworkSQLiteOpenHelperFactory().create(
            androidx.sqlite.db.SupportSQLiteOpenHelper.Configuration.builder(ctx)
                .name(null)
                .callback(object : androidx.sqlite.db.SupportSQLiteOpenHelper.Callback(1) {
                    override fun onCreate(db: androidx.sqlite.db.SupportSQLiteDatabase) {
                        db.execSQL("CREATE TABLE `favorites` (`media_id` INTEGER NOT NULL, PRIMARY KEY(`media_id`))")
                        db.execSQL("INSERT INTO favorites (media_id) VALUES (5)")
                    }
                    override fun onUpgrade(db: androidx.sqlite.db.SupportSQLiteDatabase, o: Int, n: Int) {}
                })
                .build()
        )
        val db = helper.writableDatabase

        CatalogizerDatabase.MIGRATION_1_2.migrate(db)

        db.execSQL("INSERT INTO user_ratings (media_id, rating, updated_at) VALUES (1, 9, 1)")
        db.query("SELECT rating FROM user_ratings WHERE media_id = 1").use {
            assertTrue(it.moveToFirst()); assertEquals(9, it.getInt(0))
        }
        db.query("SELECT media_id FROM favorites").use {
            assertTrue(it.moveToFirst()); assertEquals(5L, it.getLong(0))
        }
        helper.close()

        // the SQL the migration runs must be the SQL Room expects for v2 (exported schema)
        val schema = java.io.File(
            "schemas/com.catalogizer.android.data.local.CatalogizerDatabase/2.json"
        ).readText()
        val expected = CatalogizerDatabase.CREATE_USER_RATINGS_SQL.replace("`user_ratings`", "`\${TABLE_NAME}`")
        assertTrue("v2 schema must contain the migration SQL", schema.contains(expected))
    }

    // --- Singleton pattern ---

    @Test
    fun `getDatabase returns same instance for same context`() {
        val context = RuntimeEnvironment.getApplication()
        val db1 = CatalogizerDatabase.getDatabase(context)
        val db2 = CatalogizerDatabase.getDatabase(context)

        assertSame(db1, db2)

        CatalogizerDatabase.closeDatabase()
    }

    @Test
    fun `closeDatabase sets instance to null`() {
        val context = RuntimeEnvironment.getApplication()
        val db1 = CatalogizerDatabase.getDatabase(context)
        assertNotNull(db1)

        CatalogizerDatabase.closeDatabase()

        // Getting database again should create a new instance
        val db2 = CatalogizerDatabase.getDatabase(context)
        assertNotNull(db2)
        assertNotSame(db1, db2)

        CatalogizerDatabase.closeDatabase()
    }

    @Test
    fun `closeDatabase is safe to call when no database exists`() {
        // Should not throw
        CatalogizerDatabase.closeDatabase()
        CatalogizerDatabase.closeDatabase()
    }

    @Test
    fun `database exposes all required DAOs`() {
        val context = RuntimeEnvironment.getApplication()
        val db = CatalogizerDatabase.getDatabase(context)

        assertNotNull(db.mediaDao())
        assertNotNull(db.syncOperationDao())
        assertNotNull(db.watchProgressDao())
        assertNotNull(db.favoriteDao())
        assertNotNull(db.searchHistoryDao())
        assertNotNull(db.downloadDao())

        CatalogizerDatabase.closeDatabase()
    }
}
