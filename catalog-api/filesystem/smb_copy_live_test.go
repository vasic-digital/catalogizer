//go:build live_smb

package filesystem

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// CAT-SMB-COPY-1 live tests against a REAL SMB server (11.4.289; SMB_LIVE_* env as in smb_subdir_live_test.go). The class: SmbClient.CopyFile used
// to create the destination (which truncates it) before one byte of the source was read, so a copy onto the same file emptied the source.

func smbCopyClient(t *testing.T) (*SmbClient, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	c := NewSmbClient(liveCfg(t, envOr("SMB_LIVE_COPYSHARE", "testshare")))
	require.NoError(t, c.Connect(ctx))
	t.Cleanup(func() { _ = c.Disconnect(context.Background()) })
	return c, ctx
}

func smbPut(t *testing.T, c *SmbClient, p, body string) {
	t.Helper()
	f, err := c.share.Create(c.resolvePath(p))
	require.NoError(t, err)
	_, err = io.WriteString(f, body)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	t.Cleanup(func() { _ = c.share.Remove(c.resolvePath(p)) })
}

func smbGet(t *testing.T, c *SmbClient, p string) string {
	t.Helper()
	b, err := c.share.ReadFile(c.resolvePath(p))
	require.NoError(t, err, "read %q", p)
	return string(b)
}

func TestLiveSMBCopy_SamePathRefusedSourceIntact(t *testing.T) {
	c, ctx := smbCopyClient(t)
	smbPut(t, c, "copy1_same.txt", "precious source bytes")
	err := c.CopyFile(ctx, "copy1_same.txt", "copy1_same.txt")
	require.Error(t, err, "a copy onto itself must be refused")
	require.True(t, errors.Is(err, ErrCopySameFile), "got %v", err)
	require.Equal(t, "precious source bytes", smbGet(t, c, "copy1_same.txt"), "the source must be untouched")
}

func TestLiveSMBCopy_CaseAliasKeepsSourceBytes(t *testing.T) {
	c, ctx := smbCopyClient(t)
	smbPut(t, c, "copy1_alias.txt", "alias source bytes")
	// the server folds case, so the two spellings are one file: whatever the outcome, the content must survive
	_ = c.CopyFile(ctx, "copy1_alias.txt", "COPY1_ALIAS.TXT")
	require.Equal(t, "alias source bytes", smbGet(t, c, "copy1_alias.txt"))
}

func TestLiveSMBCopy_ReplacesExistingDestinationNoTempLeft(t *testing.T) {
	c, ctx := smbCopyClient(t)
	smbPut(t, c, "copy1_src.txt", "new content")
	smbPut(t, c, "copy1_dst.txt", "old content that is longer than the new one")
	require.NoError(t, c.CopyFile(ctx, "copy1_src.txt", "copy1_dst.txt"))
	require.Equal(t, "new content", smbGet(t, c, "copy1_dst.txt"))
	require.Equal(t, "new content", smbGet(t, c, "copy1_src.txt"))
	l, err := c.ListDirectory(ctx, "")
	require.NoError(t, err)
	for _, f := range l {
		require.False(t, strings.HasSuffix(f.Name, ".tmp"), "temp file left behind: %s", f.Name)
	}
}

func TestLiveSMBCopy_MissingSourceLeavesDestinationAlone(t *testing.T) {
	c, ctx := smbCopyClient(t)
	smbPut(t, c, "copy1_keep.txt", "keep me")
	require.Error(t, c.CopyFile(ctx, "copy1_nonexistent.txt", "copy1_keep.txt"))
	require.Equal(t, "keep me", smbGet(t, c, "copy1_keep.txt"))
}

func TestLiveSMBCopy_DestinationDirectoryRefused(t *testing.T) {
	c, ctx := smbCopyClient(t)
	smbPut(t, c, "copy1_f.txt", "x")
	require.NoError(t, c.share.Mkdir("copy1_dir", 0o755))
	t.Cleanup(func() { _ = c.share.Remove("copy1_dir") })
	err := c.CopyFile(ctx, "copy1_f.txt", "copy1_dir")
	require.True(t, errors.Is(err, ErrCopyDestIsDirectory), "got %v", err)
}

func TestLiveSMBCopy_SourceDirectoryRefused(t *testing.T) {
	c, ctx := smbCopyClient(t)
	require.NoError(t, c.share.Mkdir("copy1_sdir", 0o755))
	t.Cleanup(func() { _ = c.share.Remove("copy1_sdir") })
	err := c.CopyFile(ctx, "copy1_sdir", "copy1_out.txt")
	require.True(t, errors.Is(err, ErrCopySourceNotRegular), "got %v", err)
	_, serr := c.share.Stat("copy1_out.txt")
	require.Error(t, serr, "no destination may be created")
}

func TestLiveSMBCopy_CancelledContextLeavesDestination(t *testing.T) {
	c, _ := smbCopyClient(t)
	smbPut(t, c, "copy1_c_src.txt", strings.Repeat("a", 3<<20))
	smbPut(t, c, "copy1_c_dst.txt", "untouched")
	cctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.Error(t, c.CopyFile(cctx, "copy1_c_src.txt", "copy1_c_dst.txt"))
	require.Equal(t, "untouched", smbGet(t, c, "copy1_c_dst.txt"))
}

// A cancel that arrives while the data is moving must fail the copy, leave the pre-existing destination untouched and leave no temp file.
func TestLiveSMBCopy_CancelMidTransferKeepsDestinationNoTemp(t *testing.T) {
	c, _ := smbCopyClient(t)
	chunk := strings.Repeat("b", 1<<20)
	f, err := c.share.Create(c.resolvePath("copy1_big.bin"))
	require.NoError(t, err)
	for i := 0; i < 400; i++ {
		_, err = io.WriteString(f, chunk)
		require.NoError(t, err)
	}
	require.NoError(t, f.Close())
	t.Cleanup(func() { _ = c.share.Remove(c.resolvePath("copy1_big.bin")) })
	smbPut(t, c, "copy1_big_dst.txt", "untouched")
	cctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(30*time.Millisecond, cancel)
	start := time.Now()
	err = c.CopyFile(cctx, "copy1_big.bin", "copy1_big_dst.txt")
	require.Error(t, err, "a copy cancelled mid-transfer must fail (took %v)", time.Since(start))
	require.Equal(t, "untouched", smbGet(t, c, "copy1_big_dst.txt"))
	l, lerr := c.ListDirectory(context.Background(), "")
	require.NoError(t, lerr)
	for _, e := range l {
		require.False(t, strings.HasSuffix(e.Name, ".tmp"), "temp file left behind: %s", e.Name)
	}
}

// SMBCOPYR-m2: a differently spelled path to the same file is the same file.
func TestLiveSMBCopy_SamePathUncleanedSpellingRefused(t *testing.T) {
	c, ctx := smbCopyClient(t)
	smbPut(t, c, "copy1_clean.txt", "clean source bytes")
	err := c.CopyFile(ctx, "copy1_clean.txt", "./copy1_clean.txt")
	require.True(t, errors.Is(err, ErrCopySameFile), "got %v", err)
	require.Equal(t, "clean source bytes", smbGet(t, c, "copy1_clean.txt"))
}

// SMBCOPYR-m1: a read-only destination is refused and keeps its bytes (go-smb2 Remove would clear the attribute and delete it).
func TestLiveSMBCopy_ReadOnlyDestinationRefused(t *testing.T) {
	c, ctx := smbCopyClient(t)
	smbPut(t, c, "copy1_ro_src.txt", "new")
	smbPut(t, c, "copy1_ro_dst.txt", "protected")
	require.NoError(t, c.share.Chmod(c.resolvePath("copy1_ro_dst.txt"), 0o444))
	t.Cleanup(func() { _ = c.share.Chmod(c.resolvePath("copy1_ro_dst.txt"), 0o644) })
	err := c.CopyFile(ctx, "copy1_ro_src.txt", "copy1_ro_dst.txt")
	require.True(t, errors.Is(err, ErrCopyDestReadOnly), "got %v", err)
	require.Equal(t, "protected", smbGet(t, c, "copy1_ro_dst.txt"))
}
