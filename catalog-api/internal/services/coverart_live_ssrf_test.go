//go:build live_coverart

// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Live cover-art fetch checks (CAT-COVERART-LIVE-1). They drive the REAL
// CoverArtService.downloadImage (product code path: GuardProviderURL, the
// netguard client, the redirect re-check, the production 32 MiB cap) over real
// TCP sockets against local fixture-input servers on 127.0.0.1 / 127.0.0.2.
// The servers are test servers the system under test talks to, not mocks of it.
// The cap is NOT lowered here: the default constant is asserted and used.
package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"catalogizer/internal/netguard"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const liveCap = 32 << 20

// repeatHandler streams n zero bytes without holding them in memory.
func repeatHandler(n int64, hits *int32) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(hits, 1)
		w.Header().Set("Content-Type", "image/jpeg")
		buf := make([]byte, 64*1024)
		for n > 0 {
			c := int64(len(buf))
			if c > n {
				c = n
			}
			if _, err := w.Write(buf[:c]); err != nil {
				return
			}
			n -= c
		}
	})
}

func liveService(t *testing.T, allow string) *CoverArtService {
	t.Helper()
	AllowPrivateSSRFForTests(t) // string guard only; the dial-time netguard stays real
	t.Setenv(netguard.AllowCIDRsEnv, allow)
	s := NewCoverArtService(nil, zap.NewNop())
	s.httpClient = netguard.Client(60*time.Second, 3, netguard.HTTPOrHTTPS)
	return s
}

func TestLiveCoverArt_CapIsProductionDefault(t *testing.T) {
	require.Equal(t, int64(liveCap), coverArtMaxBytes, "the cap under test must be the unmodified 32 MiB production default")
}

func TestLiveCoverArt_F2_Cap(t *testing.T) {
	s := liveService(t, "127.0.0.1/32")

	var hOver, hExact, hNormal int32
	over := httptest.NewServer(repeatHandler(liveCap+1, &hOver))
	defer over.Close()
	exact := httptest.NewServer(repeatHandler(liveCap, &hExact))
	defer exact.Close()
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 2048)...)
	normal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hNormal, 1)
		w.Header().Set("Content-Type", "image/png")
		w.Write(png)
	}))
	defer normal.Close()

	// normal image accepted, byte-identical
	b, err := s.downloadImage(context.Background(), normal.URL)
	require.NoError(t, err)
	assert.Equal(t, png, b)
	assert.EqualValues(t, 1, atomic.LoadInt32(&hNormal), "control: the request really reached the server")

	// exactly at the cap accepted
	b, err = s.downloadImage(context.Background(), exact.URL)
	require.NoError(t, err, "a body of exactly 32 MiB must be accepted")
	assert.Len(t, b, liveCap)
	assert.EqualValues(t, 1, atomic.LoadInt32(&hExact))

	// one byte over rejected with the documented error, no body returned
	b, err = s.downloadImage(context.Background(), over.URL)
	require.Error(t, err, "a body of 32 MiB + 1 must be rejected")
	assert.Nil(t, b)
	assert.Contains(t, err.Error(), "cover_art: download exceeds 33554432 bytes")
	assert.EqualValues(t, 1, atomic.LoadInt32(&hOver), "control: the oversize server was really contacted")
}

func TestLiveCoverArt_F4_Redirects(t *testing.T) {
	// only 127.0.0.1 is allow-listed (the fixture); 127.0.0.2 is loopback but NOT allow-listed
	s := liveService(t, "127.0.0.1/32")

	ln, err := net.Listen("tcp", "127.0.0.2:0")
	if err != nil {
		t.Skipf("cannot bind 127.0.0.2: %v", err)
	}
	var hInternal, hOK int32
	internal := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hInternal, 1)
		io.WriteString(w, "INTERNAL-SECRET")
	}))
	internal.Listener = ln
	internal.Start()
	defer internal.Close()

	okTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hOK, 1)
		io.WriteString(w, "ALLOWED-IMAGE")
	}))
	defer okTarget.Close()

	redir := func(to string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, to, http.StatusFound)
		}))
	}

	// control needle: a redirect to an allowed target IS followed (the check can pass)
	good := redir(okTarget.URL + "/x")
	defer good.Close()
	b, err := s.downloadImage(context.Background(), good.URL)
	require.NoError(t, err)
	assert.Equal(t, "ALLOWED-IMAGE", string(b))
	assert.EqualValues(t, 1, atomic.LoadInt32(&hOK), "control: allowed redirect target was contacted")

	// counted [::1] listener (when IPv6 loopback exists): must see 0 hits
	var hV6 int32
	v6target := "http://[::1]:1/x"
	if l6, e6 := net.Listen("tcp6", "[::1]:0"); e6 == nil {
		v6 := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&hV6, 1)
			io.WriteString(w, "V6-SECRET")
		}))
		v6.Listener = l6
		v6.Start()
		defer v6.Close()
		v6target = v6.URL + "/x"
	} else {
		t.Logf("no IPv6 loopback (%v): ::1 case uses a closed port, error class only", e6)
	}

	// bad cases: refused by the netguard dial layer (error class asserted), target never contacted
	for name, to := range map[string]string{
		"loopback-not-allowlisted": internal.URL + "/x",
		"link-local-metadata":      "http://169.254.169.254/latest/meta-data/",
		"private-rfc1918":          "http://10.0.0.5/x",
		"ipv6-loopback":            v6target,
	} {
		t.Run(name, func(t *testing.T) {
			rs := redir(to)
			defer rs.Close()
			b, err := s.downloadImage(context.Background(), rs.URL)
			require.Error(t, err)
			assert.Empty(t, b)
			assert.True(t, errors.Is(err, netguard.ErrBlocked), "must be refused by the guard, got: %v", err)
		})
	}
	assert.Zero(t, atomic.LoadInt32(&hInternal), "the non-allow-listed internal server must never be contacted")
	assert.Zero(t, atomic.LoadInt32(&hV6), "the [::1] server must never be contacted")
}

// Redirect chains: 3 hops are followed (control), 5 hops are not (limit enforced).
func TestLiveCoverArt_F4_RedirectChainLimit(t *testing.T) {
	s := liveService(t, "127.0.0.1/32")
	var hFinal int32
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hFinal, 1)
		io.WriteString(w, "END")
	}))
	defer final.Close()
	chain := func(hops int) string {
		var mux http.ServeMux
		srv := httptest.NewServer(&mux)
		t.Cleanup(srv.Close)
		for i := 0; i < hops; i++ {
			next := fmt.Sprintf("%s/h%d", srv.URL, i+1)
			if i == hops-1 {
				next = final.URL
			}
			to := next
			mux.HandleFunc(fmt.Sprintf("/h%d", i), func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, to, http.StatusFound) })
		}
		return srv.URL + "/h0"
	}
	b, err := s.downloadImage(context.Background(), chain(3))
	require.NoError(t, err, "control: a 3-hop chain is followed")
	assert.Equal(t, "END", string(b))
	assert.EqualValues(t, 1, atomic.LoadInt32(&hFinal))
	b, err = s.downloadImage(context.Background(), chain(5))
	require.Error(t, err, "a 5-hop chain exceeds the limit of 3")
	assert.Empty(t, b)
	assert.EqualValues(t, 1, atomic.LoadInt32(&hFinal), "the end of the over-long chain must not be reached")
}

// Production configuration: the default strict client refuses a loopback origin outright.
func TestLiveCoverArt_F4_StrictProductionClientRefusesLoopback(t *testing.T) {
	restore := SetTestAllowPrivateNetworks(false)
	defer restore()
	// even an operator allow-list that admits loopback must NOT re-admit it for the production client
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8")
	var hits int32
	srv := httptest.NewServer(repeatHandler(1024, &hits))
	defer srv.Close()
	s := NewCoverArtService(nil, zap.NewNop()) // untouched production client
	b, err := s.downloadImage(context.Background(), srv.URL)
	require.Error(t, err)
	assert.Empty(t, b)
	assert.True(t, errors.Is(err, ErrSSRFBlocked), "string guard must refuse, got: %v", err)
	assert.Zero(t, atomic.LoadInt32(&hits), "loopback origin must not be contacted by the production client")
}

// Dial layer of the untouched production client (StrictClient): the string guard is relaxed so
// it cannot be what refuses; the operator allow-list admits loopback and must not matter.
func TestLiveCoverArt_F4_StrictDialLayerOfProductionClient(t *testing.T) {
	restore := SetTestAllowPrivateNetworks(true) // string guard off
	defer restore()
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8")
	var hits int32
	srv := httptest.NewServer(repeatHandler(1024, &hits))
	defer srv.Close()
	s := NewCoverArtService(nil, zap.NewNop()) // production client, not replaced
	b, err := s.downloadImage(context.Background(), srv.URL)
	require.Error(t, err)
	assert.Empty(t, b)
	assert.True(t, errors.Is(err, netguard.ErrBlocked), "dial layer must refuse with ErrBlocked, got: %v", err)
	assert.Zero(t, atomic.LoadInt32(&hits))
}
