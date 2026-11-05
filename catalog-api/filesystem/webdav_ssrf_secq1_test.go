package filesystem

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"catalogizer/internal/netguard"

	"github.com/stretchr/testify/require"
)

// SECQ-1/W5: the WebDAV storage-root client must not connect to loopback / link-local / metadata addresses
// unless the operator allow-list names them (the package TestMain allow-lists loopback for the in-process fixtures;
// these tests clear it to prove the guard).

func TestSECQ1_WebDAVConnectRefusesLoopbackByDefault(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusMultiStatus)
	}))
	defer srv.Close()

	// positive control: with the operator allow-list the same client reaches the server
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.1/32")
	c, err := NewWebDAVClient(&WebDAVConfig{URL: srv.URL})
	require.NoError(t, err)
	require.NoError(t, c.Connect(context.Background()))
	require.Equal(t, int32(1), hits.Load())

	t.Setenv(netguard.AllowCIDRsEnv, "")
	c2, err := NewWebDAVClient(&WebDAVConfig{URL: srv.URL})
	require.NoError(t, err)
	err = c2.Connect(context.Background())
	require.Error(t, err)
	require.True(t, errors.Is(err, netguard.ErrBlocked), "got %v", err)
	require.Equal(t, int32(1), hits.Load(), "the refused client must not have reached the server")
}

func TestSECQ1_WebDAVRedirectToMetadataRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.1/32") // the operator admits the NAS stand-in, never metadata
	c, err := NewWebDAVClient(&WebDAVConfig{URL: srv.URL})
	require.NoError(t, err)
	err = c.Connect(context.Background())
	require.Error(t, err)
	require.True(t, errors.Is(err, netguard.ErrBlocked), "got %v", err)
}

func TestSECQ1_WebDAVLiteralMetadataURLRefused(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	for _, u := range []string{"http://169.254.169.254/", "http://[::ffff:169.254.169.254]/", "http://[fe80::1]/"} {
		c, err := NewWebDAVClient(&WebDAVConfig{URL: u})
		require.NoError(t, err)
		err = c.Connect(context.Background())
		require.Error(t, err, u)
		require.True(t, errors.Is(err, netguard.ErrBlocked), "%s: %v", u, err)
	}
}
