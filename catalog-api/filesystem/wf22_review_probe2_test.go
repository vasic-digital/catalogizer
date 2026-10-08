package filesystem

// WF22 review probe F5: PA-01 now forwards the WebDAV root Path, but NewWebDAVClient REPLACES the URL's own path with it.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewProbe_F5_WebDAVPathReplacesURLPath(t *testing.T) {
	c0, err := NewWebDAVClient(&WebDAVConfig{URL: "https://nas.example/remote.php/dav/files/alice"})
	require.NoError(t, err)
	require.Equal(t, "https://nas.example/remote.php/dav/files/alice/Movies/x.mkv", c0.resolveURL("/Movies/x.mkv"), "control: without Path the URL path is kept")
	c, err := NewWebDAVClient(&WebDAVConfig{URL: "https://nas.example/remote.php/dav/files/alice", Path: "/Movies"})
	require.NoError(t, err)
	got := c.resolveURL("/x.mkv")
	t.Logf("F5 url=.../remote.php/dav/files/alice path=/Movies -> %s", got)
	assert.Equal(t, "https://nas.example/remote.php/dav/files/alice/Movies/x.mkv", got, "the root path must be joined under the URL path, not replace it")
}
