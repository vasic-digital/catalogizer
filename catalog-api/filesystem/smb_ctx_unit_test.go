package filesystem

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unit-class (loopback listeners count connection attempts; they answer nothing). Live confirmation against a real SMB server is owed.

func countingListener(t *testing.T, hold bool) (port int, accepts *int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	var n int32
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			atomic.AddInt32(&n, 1)
			if hold {
				t.Cleanup(func() { _ = c.Close() })
			} else {
				_ = c.Close()
			}
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, &n
}

func TestSmbConnect_CancelledContextNoNetworkTraffic(t *testing.T) {
	port, accepts := countingListener(t, false)
	c := NewSmbClient(&SmbConfig{Host: "127.0.0.1", Port: port, Share: "s", Username: "u", Password: "p"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := c.Connect(ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "got %v", err)
	time.Sleep(100 * time.Millisecond)
	assert.EqualValues(t, 0, atomic.LoadInt32(accepts), "a cancelled context must not open a connection")
}

func TestSmbConnect_ReturnsPromptlyWhenContextEndsMidHandshake(t *testing.T) {
	port, accepts := countingListener(t, true) // accepts, never speaks SMB
	c := NewSmbClient(&SmbConfig{Host: "127.0.0.1", Port: port, Share: "s", Username: "u", Password: "p"})
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- c.Connect(ctx) }()
	select {
	case err := <-done:
		require.Error(t, err)
		assert.Less(t, time.Since(start), 5*time.Second)
		assert.EqualValues(t, 1, atomic.LoadInt32(accepts), "control: the handshake really reached the listener")
		assert.False(t, c.IsConnected())
	case <-time.After(20 * time.Second):
		t.Fatal("Connect ignored its context during the SMB handshake")
	}
}

func TestSmbOperationsFailFastOnCancelledContext(t *testing.T) {
	c := NewSmbClient(&SmbConfig{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.GetFileInfo(ctx, "x")
	assert.ErrorIs(t, err, context.Canceled)
	_, err = c.ReadFile(ctx, "x")
	assert.ErrorIs(t, err, context.Canceled)
	_, err = c.ListDirectory(ctx, "x")
	assert.ErrorIs(t, err, context.Canceled)
	_, err = c.FileExists(ctx, "x")
	assert.ErrorIs(t, err, context.Canceled)
	assert.ErrorIs(t, c.WriteFile(ctx, "x", nil), context.Canceled)
	assert.ErrorIs(t, c.CopyFile(ctx, "x", "y"), context.Canceled)
	// control: a live context on a disconnected client still says "not connected"
	_, err = c.GetFileInfo(context.Background(), "x")
	assert.EqualError(t, err, "not connected")
}

func TestSmbPath_LeadingSlashIsShareRelative(t *testing.T) { // ST-D3
	assert.Equal(t, "movies/a", smbPath("/movies/a"))
	assert.Equal(t, "movies/a", smbPath("//movies/a"))
	assert.Equal(t, ".", smbPath("/"))
	assert.Equal(t, "movies/a", smbPath("movies/a"), "relative paths are untouched")
	assert.Equal(t, "", smbPath(""), "empty stays as before")
	assert.Equal(t, "etc/passwd", smbPath("/../etc/passwd"), "confinement still applies")
}

func TestFTPCancelledContextNoNetworkTraffic(t *testing.T) {
	port, accepts := countingListener(t, false)
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "u", Password: "p"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.ErrorIs(t, c.Connect(ctx), context.Canceled)
	_, err := c.GetFileInfo(ctx, "/x")
	assert.ErrorIs(t, err, context.Canceled)
	_, err = c.ReadFile(ctx, "/x")
	assert.ErrorIs(t, err, context.Canceled)
	_, err = c.ListDirectory(ctx, "/x")
	assert.ErrorIs(t, err, context.Canceled)
	_, err = c.FileExists(ctx, "/x")
	assert.ErrorIs(t, err, context.Canceled)
	assert.ErrorIs(t, c.CopyFile(ctx, "/x", "/y"), context.Canceled)
	time.Sleep(100 * time.Millisecond)
	assert.EqualValues(t, 0, atomic.LoadInt32(accepts))
}

// ST-D1 on a connected client: a real FTP server, cancelled ctx -> error, and the session stays usable.
func TestLiveRealFTP_CancelledContextFailsFastSessionSurvives(t *testing.T) {
	c, sizes := realFTPFixture(t, 2)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.GetFileInfo(ctx, "/f00.bin")
	assert.ErrorIs(t, err, context.Canceled)
	_, err = c.ReadFile(ctx, "/f00.bin")
	assert.ErrorIs(t, err, context.Canceled)
	fi, err := c.GetFileInfo(context.Background(), "/f00.bin")
	require.NoError(t, err)
	assert.Equal(t, sizes["f00.bin"], fi.Size)
}

// S1: a backslash is a path separator for SMB, so `..` between backslashes is a traversal segment and is confined like `/..`.
func TestSmbPath_BackslashDotDotConfined(t *testing.T) {
	assert.Equal(t, "x", smbPath(`a\..\..\x`))
	assert.Equal(t, "x", smbPath(`\..\x`))
	assert.Equal(t, "x", smbPath(`..\x`))
	assert.Equal(t, `a\.\b`, smbPath(`a\.\b`), "a single dot segment is not a traversal")
	assert.Equal(t, `movies\Wait.. Live`, smbPath(`movies\Wait.. Live`), "a name containing .. is untouched")
	assert.Equal(t, `a\b`, smbPath(`a\b`))
}
