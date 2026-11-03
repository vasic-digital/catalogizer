package services

import (
	"database/sql"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/models"
	"catalogizer/repository"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func setupPlaylistServiceDB(t *testing.T) *database.DB {
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

	return database.WrapDB(sqlDB, database.DialectSQLite)
}

func newPlaylistService(t *testing.T) *PlaylistService {
	t.Helper()
	db := setupPlaylistServiceDB(t)
	repo := repository.NewPlaylistRepository(db)
	return NewPlaylistService(repo)
}

// seedPlaylist creates a playlist for userID=1 and returns its ID.
func seedPlaylist(t *testing.T, svc *PlaylistService, name string) int {
	t.Helper()
	pl, err := svc.CreatePlaylist(1, &models.CreatePlaylistRequest{Name: name})
	require.NoError(t, err)
	return pl.ID
}

// ---------------------------------------------------------------------------
// NewPlaylistService
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// CreatePlaylist
// ---------------------------------------------------------------------------

func TestPlaylistService_CreatePlaylist(t *testing.T) {
	tests := []struct {
		name    string
		userID  int
		req     *models.CreatePlaylistRequest
		nilRepo bool
		wantErr bool
		errMsg  string
	}{
		{
			name:   "success",
			userID: 1,
			req:    &models.CreatePlaylistRequest{Name: "My Playlist", Description: "desc"},
		},
		{
			name:    "empty name",
			userID:  1,
			req:     &models.CreatePlaylistRequest{Name: ""},
			wantErr: true,
			errMsg:  "playlist name is required",
		},
		{
			name:    "nil repo",
			userID:  1,
			req:     &models.CreatePlaylistRequest{Name: "Foo"},
			nilRepo: true,
			wantErr: true,
			errMsg:  "playlist repository not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var svc *PlaylistService
			if tt.nilRepo {
				svc = NewPlaylistService(nil)
			} else {
				svc = newPlaylistService(t)
			}

			pl, err := svc.CreatePlaylist(tt.userID, tt.req)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, pl)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "My Playlist", pl.Name)
			assert.Equal(t, "desc", pl.Description)
			assert.Equal(t, tt.userID, pl.UserID)
			assert.Greater(t, pl.ID, 0)
			assert.False(t, pl.CreatedAt.IsZero())
		})
	}
}

// ---------------------------------------------------------------------------
// GetPlaylist
// ---------------------------------------------------------------------------

func TestPlaylistService_GetPlaylist(t *testing.T) {
	svc := newPlaylistService(t)
	plID := seedPlaylist(t, svc, "Get Test")

	tests := []struct {
		name       string
		userID     int
		playlistID int
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "owner can access",
			userID:     1,
			playlistID: plID,
		},
		{
			name:       "non-owner private denied",
			userID:     2,
			playlistID: plID,
			wantErr:    true,
			errMsg:     "playlist not found",
		},
		{
			name:       "not found",
			userID:     1,
			playlistID: 9999,
			wantErr:    true,
			errMsg:     "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.GetPlaylist(tt.userID, tt.playlistID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "Get Test", got.Name)
		})
	}
}

func TestPlaylistService_GetPlaylist_NilRepo(t *testing.T) {
	svc := NewPlaylistService(nil)
	_, err := svc.GetPlaylist(1, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "playlist repository not configured")
}

// ---------------------------------------------------------------------------
// GetUserPlaylists
// ---------------------------------------------------------------------------

func TestPlaylistService_GetUserPlaylists(t *testing.T) {
	svc := newPlaylistService(t)

	for i := 0; i < 3; i++ {
		seedPlaylist(t, svc, "List Test")
	}

	playlists, err := svc.GetUserPlaylists(1, 50, 0)
	require.NoError(t, err)
	assert.Len(t, playlists, 3)

	empty, err := svc.GetUserPlaylists(2, 50, 0)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestPlaylistService_GetUserPlaylists_NilRepo(t *testing.T) {
	svc := NewPlaylistService(nil)
	_, err := svc.GetUserPlaylists(1, 50, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "playlist repository not configured")
}

// ---------------------------------------------------------------------------
// UpdatePlaylist
// ---------------------------------------------------------------------------

func TestPlaylistService_UpdatePlaylist(t *testing.T) {
	svc := newPlaylistService(t)
	plID := seedPlaylist(t, svc, "Update Test")

	newName := "Updated"
	newDesc := "Updated desc"

	tests := []struct {
		name       string
		userID     int
		playlistID int
		req        *models.UpdatePlaylistRequest
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "owner updates name",
			userID:     1,
			playlistID: plID,
			req:        &models.UpdatePlaylistRequest{Name: &newName, Description: &newDesc},
		},
		{
			name:       "non-owner denied",
			userID:     2,
			playlistID: plID,
			wantErr:    true,
			errMsg:     "playlist not found",
		},
		{
			name:       "not found",
			userID:     1,
			playlistID: 9999,
			req:        &models.UpdatePlaylistRequest{Name: &newName},
			wantErr:    true,
			errMsg:     "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.UpdatePlaylist(tt.userID, tt.playlistID, tt.req)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "Updated", got.Name)
			assert.Equal(t, "Updated desc", got.Description)
		})
	}
}

func TestPlaylistService_UpdatePlaylist_PartialUpdate(t *testing.T) {
	svc := newPlaylistService(t)
	plID := seedPlaylist(t, svc, "Partial Update")

	isPublic := true
	got, err := svc.UpdatePlaylist(1, plID, &models.UpdatePlaylistRequest{IsPublic: &isPublic})
	require.NoError(t, err)
	assert.True(t, got.IsPublic)
	assert.Equal(t, "Partial Update", got.Name) // name unchanged
}

func TestPlaylistService_UpdatePlaylist_NilRepo(t *testing.T) {
	svc := NewPlaylistService(nil)
	name := "x"
	_, err := svc.UpdatePlaylist(1, 1, &models.UpdatePlaylistRequest{Name: &name})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "playlist repository not configured")
}

// ---------------------------------------------------------------------------
// DeletePlaylist
// ---------------------------------------------------------------------------

func TestPlaylistService_DeletePlaylist(t *testing.T) {
	svc := newPlaylistService(t)
	plID := seedPlaylist(t, svc, "Delete Test")

	tests := []struct {
		name       string
		userID     int
		playlistID int
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "non-owner denied",
			userID:     2,
			playlistID: plID,
			wantErr:    true,
			errMsg:     "playlist not found",
		},
		{
			name:       "not found",
			userID:     1,
			playlistID: 9999,
			wantErr:    true,
			errMsg:     "not found",
		},
		{
			name:       "owner deletes",
			userID:     1,
			playlistID: plID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.DeletePlaylist(tt.userID, tt.playlistID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestPlaylistService_DeletePlaylist_NilRepo(t *testing.T) {
	svc := NewPlaylistService(nil)
	err := svc.DeletePlaylist(1, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "playlist repository not configured")
}

// ---------------------------------------------------------------------------
// AddItem
// ---------------------------------------------------------------------------

func TestPlaylistService_AddItem(t *testing.T) {
	svc := newPlaylistService(t)
	plID := seedPlaylist(t, svc, "Add Item Test")

	tests := []struct {
		name       string
		userID     int
		playlistID int
		req        *models.AddPlaylistItemRequest
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "success",
			userID:     1,
			playlistID: plID,
			req:        &models.AddPlaylistItemRequest{EntityID: 42, EntityType: "movie"},
		},
		{
			name:       "non-owner denied",
			userID:     2,
			playlistID: plID,
			wantErr:    true,
			errMsg:     "playlist not found",
		},
		{
			name:       "playlist not found",
			userID:     1,
			playlistID: 9999,
			req:        &models.AddPlaylistItemRequest{EntityID: 1, EntityType: "song"},
			wantErr:    true,
			errMsg:     "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := svc.AddItem(tt.userID, tt.playlistID, tt.req)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			assert.Greater(t, item.ID, 0)
			assert.Equal(t, 42, item.EntityID)
			assert.Equal(t, "movie", item.EntityType)
			assert.False(t, item.AddedAt.IsZero())
		})
	}
}

func TestPlaylistService_AddItem_NilRepo(t *testing.T) {
	svc := NewPlaylistService(nil)
	_, err := svc.AddItem(1, 1, &models.AddPlaylistItemRequest{EntityID: 1, EntityType: "movie"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "playlist repository not configured")
}

// ---------------------------------------------------------------------------
// RemoveItem
// ---------------------------------------------------------------------------

func TestPlaylistService_RemoveItem(t *testing.T) {
	svc := newPlaylistService(t)
	plID := seedPlaylist(t, svc, "Remove Item Test")

	item, err := svc.AddItem(1, plID, &models.AddPlaylistItemRequest{EntityID: 10, EntityType: "song"})
	require.NoError(t, err)

	tests := []struct {
		name       string
		userID     int
		playlistID int
		itemID     int
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "non-owner denied",
			userID:     2,
			playlistID: plID,
			itemID:     item.ID,
			wantErr:    true,
			errMsg:     "playlist not found",
		},
		{
			name:       "playlist not found",
			userID:     1,
			playlistID: 9999,
			itemID:     item.ID,
			wantErr:    true,
			errMsg:     "not found",
		},
		{
			name:       "item not found",
			userID:     1,
			playlistID: plID,
			itemID:     9999,
			wantErr:    true,
			errMsg:     "not found",
		},
		{
			name:       "success",
			userID:     1,
			playlistID: plID,
			itemID:     item.ID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.RemoveItem(tt.userID, tt.playlistID, tt.itemID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestPlaylistService_RemoveItem_WrongPlaylist(t *testing.T) {
	svc := newPlaylistService(t)

	plID1 := seedPlaylist(t, svc, "Playlist A")
	plID2 := seedPlaylist(t, svc, "Playlist B")

	// Add item to playlist A
	item, err := svc.AddItem(1, plID1, &models.AddPlaylistItemRequest{EntityID: 1, EntityType: "movie"})
	require.NoError(t, err)

	// Try to remove from playlist B -- should fail
	err = svc.RemoveItem(1, plID2, item.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong")
}

func TestPlaylistService_RemoveItem_NilRepo(t *testing.T) {
	svc := NewPlaylistService(nil)
	err := svc.RemoveItem(1, 1, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "playlist repository not configured")
}

// ---------------------------------------------------------------------------
// GetPlaylist loads items
// ---------------------------------------------------------------------------

func TestPlaylistService_GetPlaylist_LoadsItems(t *testing.T) {
	svc := newPlaylistService(t)
	plID := seedPlaylist(t, svc, "Items Loaded")

	_, err := svc.AddItem(1, plID, &models.AddPlaylistItemRequest{EntityID: 1, EntityType: "movie"})
	require.NoError(t, err)
	_, err = svc.AddItem(1, plID, &models.AddPlaylistItemRequest{EntityID: 2, EntityType: "song"})
	require.NoError(t, err)

	pl, err := svc.GetPlaylist(1, plID)
	require.NoError(t, err)
	assert.Len(t, pl.Items, 2)
}

// ---------------------------------------------------------------------------
// Public playlist access by non-owner
// ---------------------------------------------------------------------------

func TestPlaylistService_GetPlaylist_PublicAccessByNonOwner(t *testing.T) {
	svc := newPlaylistService(t)
	plID := seedPlaylist(t, svc, "Public Playlist")

	// Make it public
	isPublic := true
	_, err := svc.UpdatePlaylist(1, plID, &models.UpdatePlaylistRequest{IsPublic: &isPublic})
	require.NoError(t, err)

	// Non-owner should be able to access public playlist
	pl, err := svc.GetPlaylist(2, plID)
	require.NoError(t, err)
	assert.Equal(t, "Public Playlist", pl.Name)
}

// ---------------------------------------------------------------------------
// CreatePlaylist sets timestamps
// ---------------------------------------------------------------------------

func TestPlaylistService_CreatePlaylist_SetsTimestamps(t *testing.T) {
	svc := newPlaylistService(t)
	before := time.Now().Add(-time.Second)

	pl, err := svc.CreatePlaylist(1, &models.CreatePlaylistRequest{Name: "Timestamped"})
	require.NoError(t, err)
	assert.True(t, pl.CreatedAt.After(before))
	assert.True(t, pl.UpdatedAt.After(before))
}
