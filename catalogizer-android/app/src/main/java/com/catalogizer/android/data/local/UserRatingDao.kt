package com.catalogizer.android.data.local

import androidx.room.*

/**
 * The signed-in user's own rating of a media item, integer 0..10 (server contract CAT-RATING-1). Stored apart
 * from media_items.rating, which holds the community / external rating and is replaced by every server refresh
 * (RATING1-R-F3).
 */
@Entity(tableName = "user_ratings")
data class UserRating(
    @PrimaryKey
    @ColumnInfo(name = "media_id")
    val mediaId: Long,
    @ColumnInfo(name = "rating")
    val rating: Int,
    @ColumnInfo(name = "updated_at")
    val updatedAt: Long = System.currentTimeMillis()
) {
    init {
        require(rating in 0..10) { "rating must be an integer 0..10, got $rating" }
    }
}

@Dao
interface UserRatingDao {
    @Query("SELECT * FROM user_ratings WHERE media_id = :mediaId")
    suspend fun get(mediaId: Long): UserRating?

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsert(rating: UserRating)

    @Query("DELETE FROM user_ratings WHERE media_id = :mediaId")
    suspend fun delete(mediaId: Long)
}
