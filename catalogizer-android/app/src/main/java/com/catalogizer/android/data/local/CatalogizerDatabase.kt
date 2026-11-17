package com.catalogizer.android.data.local

import androidx.room.Database
import androidx.room.Room
import androidx.room.RoomDatabase
import androidx.room.TypeConverter
import androidx.room.TypeConverters
import androidx.room.migration.Migration
import androidx.sqlite.db.SupportSQLiteDatabase
import android.content.Context
import com.catalogizer.android.data.models.ExternalMetadata
import com.catalogizer.android.data.models.MediaItem
import com.catalogizer.android.data.models.MediaVersion
import com.catalogizer.android.data.sync.SyncOperation
import com.catalogizer.android.data.sync.SyncOperationType
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import kotlinx.serialization.decodeFromString

@Database(
    entities = [MediaItem::class, SearchHistory::class, DownloadItem::class, SyncOperation::class, WatchProgress::class, Favorite::class, UserRating::class],
    version = 2,
    exportSchema = true
)
@TypeConverters(Converters::class)
/**
 * Room database providing access to all local DAOs for media items, search history,
 * downloads, sync operations, watch progress, and favorites.
 */
abstract class CatalogizerDatabase : RoomDatabase() {
    abstract fun mediaDao(): MediaDao
    abstract fun searchHistoryDao(): SearchHistoryDao
    abstract fun downloadDao(): DownloadDao
    abstract fun syncOperationDao(): SyncOperationDao
    abstract fun watchProgressDao(): WatchProgressDao
    abstract fun favoriteDao(): FavoriteDao
    abstract fun userRatingDao(): UserRatingDao

    companion object {
        @Volatile
        private var INSTANCE: CatalogizerDatabase? = null

        /** v2: the per-user rating table (RATING1-R-F3). Must equal the createSql in schemas/.../2.json. */
        const val CREATE_USER_RATINGS_SQL =
            "CREATE TABLE IF NOT EXISTS `user_ratings` (`media_id` INTEGER NOT NULL, `rating` INTEGER NOT NULL, " +
                "`updated_at` INTEGER NOT NULL, PRIMARY KEY(`media_id`))"

        val MIGRATION_1_2: Migration = object : Migration(1, 2) {
            override fun migrate(database: SupportSQLiteDatabase) {
                database.execSQL(CREATE_USER_RATINGS_SQL)
            }
        }

        val ALL_MIGRATIONS: Array<Migration> = arrayOf(
            MIGRATION_1_2
        )

        fun getDatabase(context: Context): CatalogizerDatabase {
            return INSTANCE ?: synchronized(this) {
                val instance = Room.databaseBuilder(
                    context.applicationContext,
                    CatalogizerDatabase::class.java,
                    "catalogizer_database"
                )
                    .addMigrations(*ALL_MIGRATIONS)
                    .build()
                INSTANCE = instance
                instance
            }
        }

        fun closeDatabase() {
            synchronized(this) {
                INSTANCE?.run {
                    if (isOpen) {
                        close()
                    }
                }
                INSTANCE = null
            }
        }
    }
}

/**
 * Room [TypeConverter] collection for serializing complex types (lists, maps,
 * enums) to and from JSON strings for SQLite storage.
 */
class Converters {
    private val json = Json { ignoreUnknownKeys = true }

    @TypeConverter
    fun fromStringList(value: List<String>?): String? {
        return value?.let { json.encodeToString(it) }
    }

    @TypeConverter
    fun toStringList(value: String?): List<String>? {
        return value?.let { json.decodeFromString(it) }
    }

    @TypeConverter
    fun fromExternalMetadataList(value: List<ExternalMetadata>?): String? {
        return value?.let { json.encodeToString(it) }
    }

    @TypeConverter
    fun toExternalMetadataList(value: String?): List<ExternalMetadata>? {
        return value?.let { json.decodeFromString(it) }
    }

    @TypeConverter
    fun fromMediaVersionList(value: List<MediaVersion>?): String? {
        return value?.let { json.encodeToString(it) }
    }

    @TypeConverter
    fun toMediaVersionList(value: String?): List<MediaVersion>? {
        return value?.let { json.decodeFromString(it) }
    }

    @TypeConverter
    fun fromStringMap(value: Map<String, String>?): String? {
        return value?.let { json.encodeToString(it) }
    }

    @TypeConverter
    fun toStringMap(value: String?): Map<String, String>? {
        return value?.let { json.decodeFromString(it) }
    }

    @TypeConverter
    fun fromSyncOperationType(value: SyncOperationType?): String? {
        return value?.name
    }

    @TypeConverter
    fun toSyncOperationType(value: String?): SyncOperationType? {
        return value?.let { SyncOperationType.valueOf(it) }
    }
}