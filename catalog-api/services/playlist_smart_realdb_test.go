package services

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"catalogizer/config"
	"catalogizer/database"
	"catalogizer/models"
	"catalogizer/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func realPlaylistService(t *testing.T) (*PlaylistService, *database.DB) {
	t.Helper()
	dir := t.TempDir()
	db, err := database.NewConnection(&config.DatabaseConfig{Type: "sqlite", Path: filepath.Join(dir, "p.db"),
		EncryptionKeyFile: filepath.Join(dir, "k", "db.key"), MaxOpenConnections: 2})
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, db.RunMigrations(context.Background()))
	for _, u := range []string{"u1", "u2"} {
		_, err = db.Exec(`INSERT INTO users (username, email, password_hash, salt, role_id, is_active) VALUES (?,?,?,?,2,1)`, u, u+"@x.invalid", "h", "s")
		require.NoError(t, err)
	}
	return NewPlaylistService(repository.NewPlaylistRepository(db)), db
}

func smartReq(name string, public bool) *models.CreatePlaylistRequest {
	t := true
	return &models.CreatePlaylistRequest{Name: name, IsSmart: &t, IsPublic: &public,
		SmartRules: []json.RawMessage{json.RawMessage(`{"field":"year","operator":"greater_than","value":0}`)}}
}

// F9: a playlist the caller may not even read is "not found" for every verb; a visible (public) playlist
// the caller may not change is "forbidden".
func TestPlaylistService_PrivateIsNotFoundPublicIsForbidden(t *testing.T) {
	s, _ := realPlaylistService(t)
	priv, err := s.CreatePlaylist(1, &models.CreatePlaylistRequest{Name: "mine"})
	require.NoError(t, err)
	yes := true
	pub, err := s.CreatePlaylist(1, &models.CreatePlaylistRequest{Name: "shared", IsPublic: &yes})
	require.NoError(t, err)
	n := "x"
	for name, call := range map[string]func(id int) error{
		"update": func(id int) error {
			_, err := s.UpdatePlaylist(2, id, &models.UpdatePlaylistRequest{Name: &n})
			return err
		},
		"delete": func(id int) error { return s.DeletePlaylist(2, id) },
		"add": func(id int) error {
			_, err := s.AddItem(2, id, &models.AddPlaylistItemRequest{EntityID: 1, EntityType: "movie"})
			return err
		},
		"remove": func(id int) error { return s.RemoveItem(2, id, 1) },
	} {
		err := call(priv.ID)
		assert.True(t, errors.Is(err, ErrPlaylistNotFound), name+" on a private foreign playlist must look like not-found, got %v", err)
		assert.False(t, errors.Is(err, ErrPlaylistForbidden), name)
		err = call(pub.ID)
		assert.True(t, errors.Is(err, ErrPlaylistForbidden), name+" on a public foreign playlist is forbidden, got %v", err)
	}
	_, err = s.GetPlaylist(2, priv.ID)
	assert.True(t, errors.Is(err, ErrPlaylistNotFound), "GET agrees with PUT")
}

// F4: the number of smart playlists per user is capped.
func TestPlaylistService_SmartCapPerUser(t *testing.T) {
	s, _ := realPlaylistService(t)
	for i := 0; i < MaxSmartPlaylistsPerUser; i++ {
		_, err := s.CreatePlaylist(1, smartReq("s", false))
		require.NoError(t, err)
	}
	_, err := s.CreatePlaylist(1, smartReq("one too many", false))
	assert.True(t, errors.Is(err, ErrSmartLimit), "got %v", err)
	_, err = s.CreatePlaylist(2, smartReq("other user unaffected", false))
	assert.NoError(t, err)
}

// F4: the request context reaches the rule evaluation (a cancelled request aborts the query).
func TestPlaylistService_RequestContextPropagates(t *testing.T) {
	s, _ := realPlaylistService(t)
	p, err := s.CreatePlaylist(1, smartReq("s", false))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = s.GetPlaylistPageCtx(ctx, 1, p.ID, 10, 0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "got %v", err)
}

// F11: smart playlist members carry their title.
func TestPlaylistService_SmartItemsCarryTitle(t *testing.T) {
	s, db := realPlaylistService(t)
	var movie int64
	require.NoError(t, db.QueryRow(`SELECT id FROM media_types WHERE name='movie'`).Scan(&movie))
	_, err := db.Exec(`INSERT INTO media_items (media_type_id,title,year,status,first_detected) VALUES (?,?,?,'detected',datetime('now'))`, movie, "Alpha", 2019)
	require.NoError(t, err)
	p, err := s.CreatePlaylist(1, smartReq("s", false))
	require.NoError(t, err)
	got, err := s.GetPlaylistPage(1, p.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	assert.Equal(t, "Alpha", got.Items[0].Title)
}
