package services

import (
	"catalogizer/filesystem"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// FTPClient.ListDirectory reports in-flight/stale CopyFile temp files (the
// scanner is the layer that hides them). moveDirectory must therefore see and
// MOVE such an entry; otherwise the final directory remove fails with "not
// empty" on a real server. This pins the second caller of ListDirectory.
func TestFTPProtocolHandler_MoveDirectory_MovesStaleCopyTemp(t *testing.T) {
	tmpName := ".movie.mkv.ftpcopy-0123456789abcdef.part"
	require.True(t, filesystem.IsCopyTempName(tmpName))

	var moved []string
	var deletedDirs []string
	client := &mockFSClient{
		listDirectoryFn: func(ctx context.Context, path string) ([]*filesystem.FileInfo, error) {
			return []*filesystem.FileInfo{
				{Name: "movie.mkv"},
				{Name: tmpName},
			}, nil
		},
		copyFileFn: func(ctx context.Context, src, dst string) error {
			moved = append(moved, src)
			return nil
		},
		deleteDirectoryFn: func(ctx context.Context, path string) error {
			deletedDirs = append(deletedDirs, path)
			return nil
		},
	}
	h := NewFTPProtocolHandler(zap.NewNop())
	require.NoError(t, h.PerformMove(context.Background(), client, "/old", "/new", true))

	assert.Contains(t, moved, "/old/"+tmpName, "stale temp must be moved, not left behind")
	assert.Contains(t, moved, "/old/movie.mkv")
	assert.Equal(t, []string{"/old"}, deletedDirs)
}
