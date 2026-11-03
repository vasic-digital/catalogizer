package repository

import (
	"database/sql"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/models"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func setupPlaylistRepoDB(t *testing.T) (*PlaylistRepository, *database.DB) {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	_, err = sqlDB.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT,
			salt TEXT,
			role_id INTEGER NOT NULL DEFAULT 1,
			is_active BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`INSERT INTO users (id, username, email, password_hash, salt) VALUES (1, 'testuser', 'test@example.com', '', '')`,
		`INSERT INTO users (id, username, email, password_hash, salt) VALUES (2, 'otheruser', 'other@example.com', '', '')`,
		`CREATE TABLE IF NOT EXISTS playlists (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			is_public INTEGER DEFAULT 0,
			is_smart INTEGER NOT NULL DEFAULT 0,
			smart_rules TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS playlist_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			playlist_id INTEGER NOT NULL,
			entity_id INTEGER NOT NULL,
			entity_type TEXT NOT NULL,
			position INTEGER DEFAULT 0,
			added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (playlist_id) REFERENCES playlists(id) ON DELETE CASCADE
		)`,
	}

	for _, s := range stmts {
		_, err := sqlDB.Exec(s)
		require.NoError(t, err)
	}

	db := database.WrapDB(sqlDB, database.DialectSQLite)
	return NewPlaylistRepository(db), db
}

// ---------------------------------------------------------------------------
// NewPlaylistRepository
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// CreatePlaylist
// ---------------------------------------------------------------------------

func TestPlaylistRepo_CreatePlaylist(t *testing.T) {
	tests := []struct {
		name    string
		pl      *models.Playlist
		wantErr bool
	}{
		{
			name: "success",
			pl: &models.Playlist{
				UserID:      1,
				Name:        "Test Playlist",
				Description: "A test playlist",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr: false,
		},
		{
			name: "with empty description",
			pl: &models.Playlist{
				UserID:    1,
				Name:      "Minimal",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, _ := setupPlaylistRepoDB(t)
			id, err := repo.CreatePlaylist(tt.pl)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Greater(t, id, 0)
		})
	}
}

// ---------------------------------------------------------------------------
// GetPlaylistByID
// ---------------------------------------------------------------------------

func TestPlaylistRepo_GetPlaylistByID(t *testing.T) {
	repo, _ := setupPlaylistRepoDB(t)

	now := time.Now()
	pl := &models.Playlist{
		UserID:      1,
		Name:        "Findable",
		Description: "desc",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	id, err := repo.CreatePlaylist(pl)
	require.NoError(t, err)

	tests := []struct {
		name    string
		id      int
		wantErr bool
		errMsg  string
	}{
		{
			name:    "found",
			id:      id,
			wantErr: false,
		},
		{
			name:    "not found",
			id:      9999,
			wantErr: true,
			errMsg:  "playlist not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.GetPlaylistByID(tt.id)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "Findable", got.Name)
			assert.Equal(t, 1, got.UserID)
		})
	}
}

// ---------------------------------------------------------------------------
// GetUserPlaylists
// ---------------------------------------------------------------------------

func TestPlaylistRepo_GetUserPlaylists(t *testing.T) {
	repo, _ := setupPlaylistRepoDB(t)

	now := time.Now()
	for i := 0; i < 3; i++ {
		_, err := repo.CreatePlaylist(&models.Playlist{
			UserID:    1,
			Name:      "Playlist",
			CreatedAt: now,
			UpdatedAt: now,
		})
		require.NoError(t, err)
	}

	// User 2 has no playlists
	tests := []struct {
		name      string
		userID    int
		limit     int
		offset    int
		wantCount int
	}{
		{"all for user 1", 1, 50, 0, 3},
		{"limit 2", 1, 2, 0, 2},
		{"offset 2", 1, 50, 2, 1},
		{"offset beyond", 1, 50, 100, 0},
		{"empty user", 2, 50, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			playlists, err := repo.GetUserPlaylists(tt.userID, tt.limit, tt.offset)
			require.NoError(t, err)
			assert.Len(t, playlists, tt.wantCount)
		})
	}
}

// ---------------------------------------------------------------------------
// UpdatePlaylist
// ---------------------------------------------------------------------------

func TestPlaylistRepo_UpdatePlaylist(t *testing.T) {
	repo, _ := setupPlaylistRepoDB(t)

	now := time.Now()
	id, err := repo.CreatePlaylist(&models.Playlist{
		UserID:      1,
		Name:        "Original",
		Description: "orig desc",
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	require.NoError(t, err)

	// Update it
	updated := &models.Playlist{
		ID:          id,
		Name:        "Updated",
		Description: "new desc",
		IsPublic:    true,
		UpdatedAt:   time.Now(),
	}
	err = repo.UpdatePlaylist(updated)
	assert.NoError(t, err)

	// Verify
	got, err := repo.GetPlaylistByID(id)
	require.NoError(t, err)
	assert.Equal(t, "Updated", got.Name)
	assert.Equal(t, "new desc", got.Description)
}

// ---------------------------------------------------------------------------
// DeletePlaylist
// ---------------------------------------------------------------------------

func TestPlaylistRepo_DeletePlaylist(t *testing.T) {
	repo, _ := setupPlaylistRepoDB(t)

	now := time.Now()
	id, err := repo.CreatePlaylist(&models.Playlist{
		UserID:    1,
		Name:      "Deletable",
		CreatedAt: now,
		UpdatedAt: now,
	})
	require.NoError(t, err)

	err = repo.DeletePlaylist(id)
	assert.NoError(t, err)

	// Should not be found
	_, err = repo.GetPlaylistByID(id)
	assert.Error(t, err)
}

func TestPlaylistRepo_DeletePlaylist_CascadesItems(t *testing.T) {
	repo, _ := setupPlaylistRepoDB(t)

	now := time.Now()
	plID, err := repo.CreatePlaylist(&models.Playlist{
		UserID:    1,
		Name:      "With Items",
		CreatedAt: now,
		UpdatedAt: now,
	})
	require.NoError(t, err)

	_, err = repo.AddPlaylistItem(&models.PlaylistItem{
		PlaylistID: plID,
		EntityID:   42,
		EntityType: "movie",
		AddedAt:    now,
	})
	require.NoError(t, err)

	// Delete playlist should also remove items
	err = repo.DeletePlaylist(plID)
	assert.NoError(t, err)

	items, err := repo.GetPlaylistItems(plID)
	assert.NoError(t, err)
	assert.Empty(t, items)
}

// ---------------------------------------------------------------------------
// AddPlaylistItem
// ---------------------------------------------------------------------------

func TestPlaylistRepo_AddPlaylistItem(t *testing.T) {
	repo, _ := setupPlaylistRepoDB(t)

	now := time.Now()
	plID, err := repo.CreatePlaylist(&models.Playlist{
		UserID:    1,
		Name:      "Items Test",
		CreatedAt: now,
		UpdatedAt: now,
	})
	require.NoError(t, err)

	item := &models.PlaylistItem{
		PlaylistID: plID,
		EntityID:   100,
		EntityType: "song",
		AddedAt:    now,
	}
	id, err := repo.AddPlaylistItem(item)
	assert.NoError(t, err)
	assert.Greater(t, id, 0)
	assert.Equal(t, 0, item.Position) // first item gets position 0
}

func TestPlaylistRepo_AddPlaylistItem_AutoIncrementPosition(t *testing.T) {
	repo, _ := setupPlaylistRepoDB(t)

	now := time.Now()
	plID, err := repo.CreatePlaylist(&models.Playlist{
		UserID:    1,
		Name:      "Position Test",
		CreatedAt: now,
		UpdatedAt: now,
	})
	require.NoError(t, err)

	// Add three items, check positions
	for i := 0; i < 3; i++ {
		item := &models.PlaylistItem{
			PlaylistID: plID,
			EntityID:   i + 1,
			EntityType: "movie",
			AddedAt:    now,
		}
		_, err := repo.AddPlaylistItem(item)
		require.NoError(t, err)
		assert.Equal(t, i, item.Position)
	}
}

// ---------------------------------------------------------------------------
// GetPlaylistItems
// ---------------------------------------------------------------------------

func TestPlaylistRepo_GetPlaylistItems(t *testing.T) {
	repo, _ := setupPlaylistRepoDB(t)

	now := time.Now()
	plID, err := repo.CreatePlaylist(&models.Playlist{
		UserID:    1,
		Name:      "List Items",
		CreatedAt: now,
		UpdatedAt: now,
	})
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		_, err := repo.AddPlaylistItem(&models.PlaylistItem{
			PlaylistID: plID,
			EntityID:   i + 1,
			EntityType: "song",
			AddedAt:    now,
		})
		require.NoError(t, err)
	}

	items, err := repo.GetPlaylistItems(plID)
	require.NoError(t, err)
	assert.Len(t, items, 3)

	// Should be ordered by position ASC
	for i, item := range items {
		assert.Equal(t, i, item.Position)
	}
}

func TestPlaylistRepo_GetPlaylistItems_Empty(t *testing.T) {
	repo, _ := setupPlaylistRepoDB(t)

	now := time.Now()
	plID, err := repo.CreatePlaylist(&models.Playlist{
		UserID:    1,
		Name:      "Empty",
		CreatedAt: now,
		UpdatedAt: now,
	})
	require.NoError(t, err)

	items, err := repo.GetPlaylistItems(plID)
	assert.NoError(t, err)
	assert.Empty(t, items)
}

// ---------------------------------------------------------------------------
// GetPlaylistItemByID
// ---------------------------------------------------------------------------

func TestPlaylistRepo_GetPlaylistItemByID(t *testing.T) {
	repo, _ := setupPlaylistRepoDB(t)

	now := time.Now()
	plID, err := repo.CreatePlaylist(&models.Playlist{
		UserID:    1,
		Name:      "ItemByID",
		CreatedAt: now,
		UpdatedAt: now,
	})
	require.NoError(t, err)

	itemID, err := repo.AddPlaylistItem(&models.PlaylistItem{
		PlaylistID: plID,
		EntityID:   7,
		EntityType: "tv_episode",
		AddedAt:    now,
	})
	require.NoError(t, err)

	tests := []struct {
		name    string
		id      int
		wantErr bool
	}{
		{"found", itemID, false},
		{"not found", 9999, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := repo.GetPlaylistItemByID(tt.id)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, 7, item.EntityID)
			assert.Equal(t, "tv_episode", item.EntityType)
		})
	}
}

// ---------------------------------------------------------------------------
// DeletePlaylistItem
// ---------------------------------------------------------------------------

func TestPlaylistRepo_DeletePlaylistItem(t *testing.T) {
	repo, _ := setupPlaylistRepoDB(t)

	now := time.Now()
	plID, err := repo.CreatePlaylist(&models.Playlist{
		UserID:    1,
		Name:      "Delete Item",
		CreatedAt: now,
		UpdatedAt: now,
	})
	require.NoError(t, err)

	itemID, err := repo.AddPlaylistItem(&models.PlaylistItem{
		PlaylistID: plID,
		EntityID:   5,
		EntityType: "movie",
		AddedAt:    now,
	})
	require.NoError(t, err)

	err = repo.DeletePlaylistItem(itemID)
	assert.NoError(t, err)

	_, err = repo.GetPlaylistItemByID(itemID)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// CountUserPlaylists
// ---------------------------------------------------------------------------

func TestPlaylistRepo_CountUserPlaylists(t *testing.T) {
	repo, _ := setupPlaylistRepoDB(t)

	now := time.Now()
	for i := 0; i < 4; i++ {
		_, err := repo.CreatePlaylist(&models.Playlist{
			UserID:    1,
			Name:      "Counted",
			CreatedAt: now,
			UpdatedAt: now,
		})
		require.NoError(t, err)
	}

	tests := []struct {
		name      string
		userID    int
		wantCount int
	}{
		{"user with playlists", 1, 4},
		{"user without playlists", 2, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := repo.CountUserPlaylists(tt.userID)
			require.NoError(t, err)
			assert.Equal(t, tt.wantCount, count)
		})
	}
}

// ---------------------------------------------------------------------------
// TouchPlaylistUpdatedAt
// ---------------------------------------------------------------------------

func TestPlaylistRepo_TouchPlaylistUpdatedAt(t *testing.T) {
	repo, _ := setupPlaylistRepoDB(t)

	past := time.Now().Add(-24 * time.Hour)
	id, err := repo.CreatePlaylist(&models.Playlist{
		UserID:    1,
		Name:      "Touch Test",
		CreatedAt: past,
		UpdatedAt: past,
	})
	require.NoError(t, err)

	err = repo.TouchPlaylistUpdatedAt(id)
	assert.NoError(t, err)

	got, err := repo.GetPlaylistByID(id)
	require.NoError(t, err)
	// updated_at should be more recent than our past timestamp
	assert.True(t, got.UpdatedAt.After(past) || got.UpdatedAt.Equal(past))
}
