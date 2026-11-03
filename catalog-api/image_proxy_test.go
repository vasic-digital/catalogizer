package main

import (
	root_config "catalogizer/config"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateImageProxyURL(t *testing.T) {
	ok := []string{"https://image.tmdb.org/t/p/w500/a.jpg", "https://IMAGE.TMDB.ORG/a", "https://img.omdbapi.com/?i=x", "https://images.igdb.com:443/a.jpg"}
	for _, s := range ok {
		_, err := validateImageProxyURL(s)
		assert.NoError(t, err, s)
	}
	bad := []string{"", "http://image.tmdb.org/a", "https://image.tmdb.org.evil.example/", "https://x.image.tmdb.org/", "http://127.0.0.1/x?q=image.tmdb.org",
		"https://image.tmdb.org@127.0.0.1/", "https://user:pw@image.tmdb.org/", "https://image.tmdb.org:444/", "//image.tmdb.org/a", "https:///image.tmdb.org/a"}
	for _, s := range bad {
		_, err := validateImageProxyURL(s)
		assert.Error(t, err, s)
	}
}

func TestImageRedirectPolicy(t *testing.T) {
	cl := newImageProxyClient("https://image.tmdb.org/a", root_config.ProxyConfig{})
	next := func(raw string) error {
		u, _ := url.Parse(raw)
		return cl.CheckRedirect(&http.Request{URL: u}, nil)
	}
	assert.NoError(t, next("https://image.tmdb.org/b"), "redirect within the allow-list")
	assert.ErrorIs(t, next("http://127.0.0.1/internal"), http.ErrUseLastResponse)
	assert.ErrorIs(t, next("https://evil.example/x"), http.ErrUseLastResponse)
	assert.ErrorIs(t, next("http://image.tmdb.org/b"), http.ErrUseLastResponse, "downgrade to http")
	assert.ErrorIs(t, cl.CheckRedirect(&http.Request{URL: &url.URL{Scheme: "https", Host: "image.tmdb.org"}}, make([]*http.Request, 4)), http.ErrUseLastResponse, "too many hops")
}
