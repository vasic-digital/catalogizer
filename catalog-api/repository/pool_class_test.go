package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"catalogizer/internal/pooltest"
	"catalogizer/models"

	"github.com/stretchr/testify/require"
)

// Pool-deadlock class (see internal/poolguard): a query must not be issued through the
// pool while an outer rows cursor of the same call still holds a pooled connection.

// TestDuplicateEntityRepository_GetDuplicateGroups_ConcurrentCallsExceedingPoolDoNotDeadlock
// is RED on the code that ran a per-group id query inside `for rows.Next()`.
func TestDuplicateEntityRepository_GetDuplicateGroups_ConcurrentCallsExceedingPoolDoNotDeadlock(t *testing.T) {
	db, sqlDB := pooltest.NewPooledDB(t)
	var t1, t2 int
	require.NoError(t, sqlDB.QueryRow(`SELECT id FROM media_types ORDER BY id LIMIT 1`).Scan(&t1))
	require.NoError(t, sqlDB.QueryRow(`SELECT id FROM media_types ORDER BY id LIMIT 1 OFFSET 1`).Scan(&t2))
	var err error
	// 6 duplicate groups (3 entities each, one group without a year) plus unique noise.
	for g := 0; g < 6; g++ {
		for k := 0; k < 3; k++ {
			year := any(2000 + g)
			if g == 5 {
				year = nil
			}
			_, err = sqlDB.Exec(`INSERT INTO media_items (media_type_id, title, year) VALUES (?, ?, ?)`, []int{t1, t2}[g%2], fmt.Sprintf("Dup %d", g), year)
			require.NoError(t, err)
		}
	}
	for i := 0; i < 5; i++ {
		_, err = sqlDB.Exec(`INSERT INTO media_items (media_type_id, title, year) VALUES (?, ?, 1999)`, t1, fmt.Sprintf("Unique %d", i))
		require.NoError(t, err)
	}

	repo := NewDuplicateEntityRepository(db)

	// Semantics: the result of one (sequential) call is the reference for all concurrent ones.
	ctx := context.Background()
	want, wantTotal, err := repo.GetDuplicateGroups(ctx, 50, 0)
	require.NoError(t, err)
	require.Len(t, want, 6)
	require.EqualValues(t, 6, wantTotal)
	for _, g := range want {
		require.Len(t, g.EntityIDs, 3, "group %q", g.Title)
	}

	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pooltest.RunConcurrent(t, 12, 15*time.Second, func(int) error {
		got, total, e := repo.GetDuplicateGroups(cctx, 50, 0)
		if e != nil {
			return e
		}
		if total != wantTotal || len(got) != len(want) {
			return fmt.Errorf("got %d groups total %d, want %d / %d", len(got), total, len(want), wantTotal)
		}
		return nil
	})
}

// TestDuplicateEntityRepository_GetDuplicateGroups_ResultUnchanged pins the exact
// result (group order and entity ids) so the cursor-buffering fix cannot change semantics.
func TestDuplicateEntityRepository_GetDuplicateGroups_ResultUnchanged(t *testing.T) {
	db, sqlDB := pooltest.NewPooledDB(t)
	var mt int
	require.NoError(t, sqlDB.QueryRow(`SELECT id FROM media_types ORDER BY id LIMIT 1`).Scan(&mt))
	var err error
	// ids 1,2,3 = "A"/2001 ; 4,5 = "B"/NULL ; 6 = unique
	for _, r := range []struct {
		title string
		year  any
	}{{"A", 2001}, {"A", 2001}, {"A", 2001}, {"B", nil}, {"B", nil}, {"C", 1990}} {
		_, err = sqlDB.Exec(`INSERT INTO media_items (media_type_id, title, year) VALUES (?, ?, ?)`, mt, r.title, r.year)
		require.NoError(t, err)
	}
	got, total, err := NewDuplicateEntityRepository(db).GetDuplicateGroups(context.Background(), 10, 0)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, got, 2)
	require.Equal(t, "A", got[0].Title)
	require.Equal(t, 3, got[0].Count)
	require.ElementsMatch(t, []int64{1, 2, 3}, got[0].EntityIDs)
	require.Equal(t, "B", got[1].Title)
	require.Nil(t, got[1].Year)
	require.ElementsMatch(t, []int64{4, 5}, got[1].EntityIDs)
}

// TestFileRepository_GetDirectoryContents_ConcurrentCallsExceedingPoolDoNotDeadlock is a
// class-member regression guard: GetDirectoryContents iterates a cursor and must stay free
// of nested pooled queries (a reviewer mutation re-introducing one survived the suite).
func TestFileRepository_GetDirectoryContents_ConcurrentCallsExceedingPoolDoNotDeadlock(t *testing.T) {
	db, sqlDB := pooltest.NewPooledDB(t)
	_, err := sqlDB.Exec(`INSERT INTO storage_roots (name, protocol, path, enabled) VALUES ('r','local','/x',1)`)
	require.NoError(t, err)
	for i := 0; i < 40; i++ {
		_, err = sqlDB.Exec(`INSERT INTO files (storage_root_id, path, name, extension, mime_type, file_type, size, is_directory, modified_at)
			VALUES (1, ?, ?, 'mp4', 'video/mp4', 'video', 10, 0, '2026-01-01 00:00:00')`, fmt.Sprintf("/x/f%03d.mp4", i), fmt.Sprintf("f%03d.mp4", i))
		require.NoError(t, err)
	}
	repo := NewFileRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pooltest.RunConcurrent(t, 12, 15*time.Second, func(int) error {
		res, e := repo.GetDirectoryContents(ctx, "r", "/x", models.PaginationOptions{Page: 1, Limit: 25}, models.SortOptions{})
		if e != nil {
			return e
		}
		if len(res.Files) != 25 || res.TotalCount != 40 {
			return fmt.Errorf("got %d files / total %d, want 25 / 40", len(res.Files), res.TotalCount)
		}
		return nil
	})
}
