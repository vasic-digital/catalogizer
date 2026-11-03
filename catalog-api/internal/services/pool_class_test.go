package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"catalogizer/internal/pooltest"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Pool-deadlock class (see internal/poolguard): no query through the pool while an outer
// rows cursor of the same call still holds a pooled connection. Each test uses a real
// SQLite file, the production migrations (or the service's own DDL where production has none)
// and a pool of pooltest.PoolSize connections against more concurrent callers.

func seedFiles(t *testing.T, exec func(string, ...any) error) {
	t.Helper()
	require.NoError(t, exec(`INSERT INTO storage_roots (name, protocol, path, enabled) VALUES ('r','local','/x',1)`))
	// 5 duplicate groups of 3 files each (same quick_hash + size) plus unique noise.
	for g := 0; g < 5; g++ {
		for k := 0; k < 3; k++ {
			require.NoError(t, exec(`INSERT INTO files (storage_root_id, path, name, extension, mime_type, file_type, size, is_directory, quick_hash, modified_at)
				VALUES (1, ?, ?, 'mp4', 'video/mp4', 'video', ?, 0, ?, '2026-01-01 00:00:00')`,
				fmt.Sprintf("/x/g%d/f%d.mp4", g, k), fmt.Sprintf("f%d.mp4", k), 100+g, fmt.Sprintf("hash%d", g)))
		}
	}
	for i := 0; i < 4; i++ {
		require.NoError(t, exec(`INSERT INTO files (storage_root_id, path, name, extension, mime_type, file_type, size, is_directory, quick_hash, modified_at)
			VALUES (1, ?, 'u.mp4', 'mp4', 'video/mp4', 'video', 7, 0, ?, '2026-01-01 00:00:00')`, fmt.Sprintf("/x/u%d.mp4", i), fmt.Sprintf("uniq%d", i)))
	}
}

func TestCatalogService_GetDuplicateGroups_ConcurrentCallsExceedingPoolDoNotDeadlock(t *testing.T) {
	db, sqlDB := pooltest.NewPooledDB(t)
	seedFiles(t, func(q string, a ...any) error { _, err := sqlDB.Exec(q, a...); return err })
	svc := NewCatalogService(nil, zap.NewNop())
	svc.SetDB(db)

	// Semantics reference (single caller): order and contents.
	want, err := svc.GetDuplicateGroups("", 2, 0)
	require.NoError(t, err)
	require.Len(t, want, 5)
	for _, g := range want {
		require.Equal(t, 3, g.Count)
		require.Len(t, g.Files, 3)
		require.Equal(t, g.Size*3, g.TotalSize)
		require.True(t, g.Files[0].Path < g.Files[1].Path && g.Files[1].Path < g.Files[2].Path, "files ordered by path")
	}
	// ORDER BY COUNT(*) DESC, size DESC -> sizes 104..100
	for i, g := range want {
		require.EqualValues(t, 104-i, g.Size)
	}

	pooltest.RunConcurrent(t, 12, 15*time.Second, func(int) error {
		got, e := svc.GetDuplicateGroups("r", 2, 0)
		if e != nil {
			return e
		}
		if len(got) != 5 {
			return fmt.Errorf("got %d groups, want 5", len(got))
		}
		for _, g := range got {
			if len(g.Files) != 3 {
				return fmt.Errorf("group has %d files, want 3", len(g.Files))
			}
		}
		return nil
	})
}

func TestAggregationService_GetTopLevelDirectories_ConcurrentCallsExceedingPoolDoNotDeadlock(t *testing.T) {
	db, sqlDB := pooltest.NewPooledDB(t)
	_, err := sqlDB.Exec(`INSERT INTO storage_roots (name, protocol, path, enabled) VALUES ('r','local','/x',1)`)
	require.NoError(t, err)
	// 5 top-level directories (ids 1..5 = d3,d1,d2,d0,d4; ORDER BY name must reorder them) with
	// 2 child files each, except d4 which has none and must be excluded.
	names := []string{"d3", "d1", "d2", "d0", "d4"}
	for _, n := range names {
		_, err = sqlDB.Exec(`INSERT INTO files (storage_root_id, path, name, is_directory, size, parent_id, modified_at)
			VALUES (1, ?, ?, 1, 0, NULL, '2026-01-01 00:00:00')`, "/x/"+n, n)
		require.NoError(t, err)
	}
	for i, n := range names[:4] {
		for k := 0; k < 2; k++ {
			_, err = sqlDB.Exec(`INSERT INTO files (storage_root_id, path, name, extension, is_directory, size, parent_id, modified_at)
				VALUES (1, ?, ?, 'mp4', 0, ?, ?, '2026-01-01 00:00:00')`, fmt.Sprintf("/x/%s/f%d.mp4", n, k), fmt.Sprintf("f%d.mp4", k), 10*(i+1)+k, i+1)
			require.NoError(t, err)
		}
	}
	svc := NewAggregationService(db, zap.NewNop(), nil, nil, nil, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	want, err := svc.getTopLevelDirectories(ctx, 1)
	require.NoError(t, err)
	require.Len(t, want, 4, "directory without files is excluded")
	require.Equal(t, []string{"d0", "d1", "d2", "d3"}, []string{want[0].name, want[1].name, want[2].name, want[3].name})
	require.Equal(t, 2, want[0].fileCount)
	require.EqualValues(t, 40+41, want[0].totalSize) // d0 is id 4 -> sizes 40,41
	require.Equal(t, map[string]int{"mp4": 2}, want[0].fileTypes)

	pooltest.RunConcurrent(t, 12, 15*time.Second, func(int) error {
		got, e := svc.getTopLevelDirectories(ctx, 1)
		if e != nil {
			return e
		}
		if len(got) != 4 {
			return fmt.Errorf("got %d dirs, want 4", len(got))
		}
		return nil
	})
}

func TestPlaylistService_GetUserPlaylists_ConcurrentCallsExceedingPoolDoNotDeadlock(t *testing.T) {
	// The production migrations create no schema for this service (it needs smart-playlist,
	// tags and collaborators tables that only exist in its own integration-test DDL), so the
	// test supplies that DDL on a real pooled SQLite file.
	db, sqlDB := pooltest.NewEmptyPooledDB(t)
	for _, ddl := range []string{
		`CREATE TABLE playlists (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, name TEXT NOT NULL,
			description TEXT DEFAULT '', is_public BOOLEAN DEFAULT FALSE, is_smart_playlist BOOLEAN DEFAULT FALSE,
			smart_criteria TEXT DEFAULT '', cover_art_url TEXT DEFAULT '', track_count INTEGER DEFAULT 0,
			total_duration INTEGER DEFAULT 0, play_count INTEGER DEFAULT 0, last_played DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE playlist_tags (id INTEGER PRIMARY KEY AUTOINCREMENT, playlist_id INTEGER NOT NULL, tag TEXT NOT NULL)`,
		`CREATE TABLE playlist_collaborators (id INTEGER PRIMARY KEY AUTOINCREMENT, playlist_id INTEGER NOT NULL, user_id INTEGER NOT NULL, added_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
	} {
		_, err := sqlDB.Exec(ddl)
		require.NoError(t, err)
	}
	for i := 0; i < 8; i++ {
		_, err := sqlDB.Exec(`INSERT INTO playlists (user_id, name, updated_at) VALUES (1, ?, ?)`, fmt.Sprintf("p%d", i), fmt.Sprintf("2026-01-%02d 00:00:00", i+1))
		require.NoError(t, err)
		_, err = sqlDB.Exec(`INSERT INTO playlist_tags (playlist_id, tag) VALUES (?, 'a'), (?, 'b')`, i+1, i+1)
		require.NoError(t, err)
		_, err = sqlDB.Exec(`INSERT INTO playlist_collaborators (playlist_id, user_id) VALUES (?, 7)`, i+1)
		require.NoError(t, err)
	}
	svc := NewPlaylistService(db, zap.NewNop())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	want, err := svc.GetUserPlaylists(ctx, 1, false)
	require.NoError(t, err)
	require.Len(t, want, 8)
	for i, p := range want { // ORDER BY updated_at DESC
		require.Equal(t, fmt.Sprintf("p%d", 7-i), p.Name)
		require.Equal(t, []string{"a", "b"}, p.Tags)
		require.Equal(t, []int64{7}, p.CollaboratorIDs)
	}

	pooltest.RunConcurrent(t, 12, 15*time.Second, func(int) error {
		got, e := svc.GetUserPlaylists(ctx, 1, false)
		if e != nil {
			return e
		}
		if len(got) != 8 {
			return fmt.Errorf("got %d playlists, want 8", len(got))
		}
		for _, p := range got {
			if len(p.Tags) != 2 || len(p.CollaboratorIDs) != 1 {
				return fmt.Errorf("playlist %q has %d tags / %d collaborators", p.Name, len(p.Tags), len(p.CollaboratorIDs))
			}
		}
		return nil
	})
}
