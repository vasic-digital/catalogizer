//go:build s3live

// s3_live_test.go - CAT-S3-1 LIVE evidence (constitution §11.4.289). These tests talk to a REAL S3-compatible server (Garage, started by
// docs/qa/stack3/s3/up.sh). There is no httptest double here. They are skipped (honest SKIP, never a pass) unless S3_LIVE_ENDPOINT,
// S3_LIVE_BUCKET, S3_LIVE_ACCESS_KEY and S3_LIVE_SECRET_KEY are set. Credentials come only from the environment and are never logged.
package filesystem

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"testing"
	"time"

	"catalogizer/internal/netguard"

	"github.com/stretchr/testify/require"
)

func liveClient(t *testing.T, prefix string) *S3Client {
	t.Helper()
	ep, bucket, ak, sk := os.Getenv("S3_LIVE_ENDPOINT"), os.Getenv("S3_LIVE_BUCKET"), os.Getenv("S3_LIVE_ACCESS_KEY"), os.Getenv("S3_LIVE_SECRET_KEY")
	if ep == "" || bucket == "" || ak == "" || sk == "" {
		t.Skip("SKIP(credentials_absent/hardware_not_present): S3_LIVE_ENDPOINT/BUCKET/ACCESS_KEY/SECRET_KEY not set; no real S3 server targeted")
	}
	if cidr := os.Getenv("S3_LIVE_ALLOW_CIDR"); cidr != "" {
		t.Setenv(netguard.AllowCIDRsEnv, cidr)
	}
	region := os.Getenv("S3_LIVE_REGION")
	c, err := NewS3Client(&S3Config{Endpoint: ep, Region: region, Bucket: bucket, Prefix: prefix, AccessKey: ak, SecretKey: sk, PathStyle: true})
	require.NoError(t, err)
	require.NoError(t, c.Connect(context.Background()))
	t.Cleanup(func() { _ = c.Disconnect(context.Background()) })
	if prefix != "" {
		// sweep everything this run wrote under its unique prefix (a root-level client, so DeleteDirectory is not the refused root)
		root, err := NewS3Client(&S3Config{Endpoint: ep, Region: region, Bucket: bucket, AccessKey: ak, SecretKey: sk, PathStyle: true})
		require.NoError(t, err)
		require.NoError(t, root.Connect(context.Background()))
		t.Cleanup(func() {
			_ = root.DeleteDirectory(context.Background(), "/"+prefix)
			_ = root.Disconnect(context.Background())
		})
	}
	return c
}

func runPrefix(t *testing.T) string {
	return fmt.Sprintf("live-%d/%s", time.Now().UnixNano(), t.Name())
}

func putRead(t *testing.T, c *S3Client, p string, body []byte) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, c.WriteFile(ctx, p, bytes.NewReader(body)), p)
	rc, err := c.ReadFile(ctx, p)
	require.NoError(t, err, p)
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	require.Equal(t, body, got, p)
}

// TestS3Live_SpecialKeysCRUD: write/read/stat/list/copy/rename/delete on keys with + & = : @ , $ space % and unicode.
func TestS3Live_SpecialKeysCRUD(t *testing.T) {
	c := liveClient(t, runPrefix(t))
	ctx := context.Background()
	names := []string{"plain.txt", "a+b.txt", "a&b.txt", "a=b.txt", "a:b.txt", "a@b.txt", "a,b.txt", "a$b.txt", "a b.txt", "a%20b.txt", "a%2Bb.txt", "čšž ü.txt", "x;y.txt", "q?r.txt", "h#i.txt"}
	for _, n := range names {
		putRead(t, c, "/dir/"+n, []byte("body:"+n))
	}
	list, err := c.ListDirectory(ctx, "/dir")
	require.NoError(t, err)
	var got []string
	for _, f := range list {
		require.False(t, f.IsDir)
		got = append(got, f.Name)
	}
	want := append([]string(nil), names...)
	sort.Strings(got)
	sort.Strings(want)
	require.Equal(t, want, got)

	for _, n := range names {
		fi, err := c.GetFileInfo(ctx, "/dir/"+n)
		require.NoError(t, err, n)
		require.Equal(t, int64(len("body:"+n)), fi.Size, n)

		// server-side copy: exercises escapeCopySource against the real server's decoder
		require.NoError(t, c.CopyFile(ctx, "/dir/"+n, "/copy/"+n), "copy "+n)
		rc, err := c.ReadFile(ctx, "/copy/"+n)
		require.NoError(t, err, n)
		b, _ := io.ReadAll(rc)
		_ = rc.Close()
		require.Equal(t, "body:"+n, string(b), "copy content "+n)

		// rename = copy + delete
		require.NoError(t, c.CopyFile(ctx, "/copy/"+n, "/renamed/"+n), n)
		require.NoError(t, c.DeleteFile(ctx, "/copy/"+n), n)
		ok, err := c.FileExists(ctx, "/copy/"+n)
		require.NoError(t, err)
		require.False(t, ok, "deleted source must be gone: "+n)
		ok, err = c.FileExists(ctx, "/renamed/"+n)
		require.NoError(t, err)
		require.True(t, ok, n)
		require.NoError(t, c.DeleteFile(ctx, "/renamed/"+n), n)
		require.NoError(t, c.DeleteFile(ctx, "/dir/"+n), n)
	}
	// deleting a missing object is an error, not a silent pass
	require.Error(t, c.DeleteFile(ctx, "/dir/plain.txt"))
	left, err := c.ListDirectory(ctx, "/dir")
	require.NoError(t, err)
	require.Empty(t, left)
}

// TestS3Live_DeleteDirectoryOver1000: >1000 objects forces pagination of LIST (1000/page) and more than one DeleteObjects batch against
// the real server; this also proves the SDK's DeleteObjects checksum header is accepted by it.
func TestS3Live_DeleteDirectoryOver1000(t *testing.T) {
	c := liveClient(t, runPrefix(t))
	ctx := context.Background()
	const n = 2105
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("/big/k%05d+&=:@,.bin", i)
		require.NoError(t, c.WriteFile(ctx, name, bytes.NewReader([]byte{byte(i)})), name)
	}
	// siblings whose names merely START with "big": a prefix delete without the trailing "/" would take them too
	require.NoError(t, c.WriteFile(ctx, "/big2/keep", bytes.NewReader([]byte("keep"))))
	require.NoError(t, c.WriteFile(ctx, "/bigger.bin", bytes.NewReader([]byte("keep"))))
	list, err := c.ListDirectory(ctx, "/big")
	require.NoError(t, err)
	require.Len(t, list, n, "paginated listing must return every object")
	require.NoError(t, c.DeleteDirectory(ctx, "/big"))
	for _, sib := range []string{"/big2/keep", "/bigger.bin"} {
		ok, err := c.FileExists(ctx, sib)
		require.NoError(t, err)
		require.True(t, ok, "sibling must survive DeleteDirectory(/big): "+sib)
	}
	left, err := c.ListDirectory(ctx, "/big")
	require.NoError(t, err)
	require.Empty(t, left, "every object must be gone after DeleteDirectory")
	ok, err := c.FileExists(ctx, "/big/k00000+&=:@,.bin")
	require.NoError(t, err)
	require.False(t, ok)
	// root delete refused
	require.ErrorIs(t, c.DeleteDirectory(ctx, "/"), ErrRefuseDeleteRoot)
}

// TestS3Live_DirectoriesAndConfinement: CreateDirectory marker, emulated dirs, '..' confinement to the prefix.
func TestS3Live_DirectoriesAndConfinement(t *testing.T) {
	pfx := runPrefix(t)
	c := liveClient(t, pfx)
	ctx := context.Background()
	require.NoError(t, c.CreateDirectory(ctx, "/d1"))
	putRead(t, c, "/d1/sub/f.txt", []byte("x"))
	list, err := c.ListDirectory(ctx, "/d1")
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.True(t, list[0].IsDir)
	require.Equal(t, "sub", list[0].Name)
	// a '..' path stays inside the prefix: it resolves to the same object, not to a sibling prefix
	putRead(t, c, "/../../escape.txt", []byte("e"))
	root, err := c.ListDirectory(ctx, "/")
	require.NoError(t, err)
	var names []string
	for _, f := range root {
		names = append(names, f.Name)
	}
	require.Contains(t, names, "escape.txt")
	require.NoError(t, c.DeleteDirectory(ctx, "/d1"))
	require.NoError(t, c.DeleteFile(ctx, "/escape.txt"))
}

// TestS3Live_SweepStaleRuns removes "live-*" run prefixes left by earlier or aborted runs. Only runs when S3_LIVE_SWEEP=1.
func TestS3Live_SweepStaleRuns(t *testing.T) {
	if os.Getenv("S3_LIVE_SWEEP") != "1" {
		t.Skip("SKIP: S3_LIVE_SWEEP!=1")
	}
	c := liveClient(t, "")
	ctx := context.Background()
	top, err := c.ListDirectory(ctx, "/")
	require.NoError(t, err)
	stale := 0
	defer func() { t.Logf("STALE_LIVE_PREFIXES_FOUND=%d", stale) }()
	for _, f := range top {
		if f.IsDir && len(f.Name) > 5 && f.Name[:5] == "live-" {
			stale++
			require.NoError(t, c.DeleteDirectory(ctx, "/"+f.Name), f.Name)
		}
	}
}
