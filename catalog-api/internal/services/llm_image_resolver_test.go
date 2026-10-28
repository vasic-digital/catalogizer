package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"digital.vasic.assets/pkg/resolver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLLMResolver_DisabledByDefault(t *testing.T) {
	t.Setenv("CATALOGIZER_LLM_IMAGE_SEARCH_ENABLED", "")
	t.Setenv("CATALOGIZER_LLM_IMAGE_SEARCH_ENDPOINT", "")
	r := NewLLMImageSearchResolver(90)
	req := &resolver.ResolveRequest{EntityType: "movie", EntityID: "1"}
	assert.False(t, r.CanResolve(context.Background(), req))
	_, err := r.Resolve(context.Background(), req)
	assert.True(t, errors.Is(err, ErrLLMResolverDisabled))
}

func TestLLMResolver_EnabledOnlyWhenBothFlagsSet(t *testing.T) {
	t.Setenv("CATALOGIZER_LLM_IMAGE_SEARCH_ENABLED", "true")
	t.Setenv("CATALOGIZER_LLM_IMAGE_SEARCH_ENDPOINT", "")
	r := NewLLMImageSearchResolver(90)
	if r.enabled {
		t.Fatal("resolver must not enable without endpoint")
	}

	t.Setenv("CATALOGIZER_LLM_IMAGE_SEARCH_ENABLED", "")
	t.Setenv("CATALOGIZER_LLM_IMAGE_SEARCH_ENDPOINT", "http://example.com")
	r = NewLLMImageSearchResolver(90)
	if r.enabled {
		t.Fatal("resolver must not enable without ENABLED flag")
	}
}

func TestLLMResolver_HappyPath(t *testing.T) {
	withStrictSSRFGuard(t)
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("IMAGEBYTES"))
	}))
	defer imgSrv.Close()

	orchestratorSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"url":"%s"}`, imgSrv.URL)
	}))
	defer orchestratorSrv.Close()

	t.Setenv("CATALOGIZER_LLM_IMAGE_SEARCH_ENABLED", "true")
	t.Setenv("CATALOGIZER_LLM_IMAGE_SEARCH_ENDPOINT", orchestratorSrv.URL)

	r := NewLLMImageSearchResolver(90)
	require.True(t, r.enabled)

	// orchestrator returns an httptest URL whose hostname is 127.0.0.1 - our
	// SSRF guard should reject it.
	_, err := r.Resolve(context.Background(), &resolver.ResolveRequest{
		EntityType: "movie", EntityID: "1", Metadata: map[string]string{"variant": "poster", "title": "Test"},
	})
	require.Error(t, err, "SSRF guard must reject loopback URL")
	assert.Contains(t, err.Error(), "unsafe")
}

func TestLLMResolver_OnePerEntityBudget(t *testing.T) {
	t.Setenv("CATALOGIZER_LLM_IMAGE_SEARCH_ENABLED", "true")
	t.Setenv("CATALOGIZER_LLM_IMAGE_SEARCH_ENDPOINT", "http://example.com")

	r := NewLLMImageSearchResolver(90)
	req := &resolver.ResolveRequest{EntityType: "movie", EntityID: "1", Metadata: map[string]string{"variant": "poster"}}

	assert.True(t, r.CanResolve(context.Background(), req))
	// Simulate a prior attempt that must block further tries for 24h.
	_, _ = r.Resolve(context.Background(), req)
	assert.False(t, r.CanResolve(context.Background(), req), "second CanResolve in the 24h window must be false")
}

func TestAllowPublicURL(t *testing.T) {
	withStrictSSRFGuard(t)
	tests := []struct {
		name string
		url  string
		bad  bool
	}{
		// literal public IP: hostname rows would need live DNS, which hermetic (network none) runs lack
		{"https public ok", "https://93.184.216.34/x.jpg", false},
		{"loopback rejected", "http://127.0.0.1/x.jpg", true},
		{"private range rejected", "http://192.168.1.1/x.jpg", true},
		{"link local rejected", "http://169.254.1.1/x.jpg", true},
		{"file scheme rejected", "file:///etc/passwd", true},
		{"empty host rejected", "https:///x.jpg", true},
		// hostname branch, hermetic: "localhost" is answered from /etc/hosts (no DNS, works with --network none) and must be refused
		{"hostname resolving to loopback rejected", "http://localhost/x.jpg", true},
	}
	for _, tc := range tests {
		err := allowPublicURL(tc.url)
		if tc.bad {
			assert.Error(t, err, tc.name)
		} else {
			assert.NoError(t, err, tc.name)
		}
	}
}

func TestLLMResolver_OrchestratorMalformedResponse(t *testing.T) {
	orchestrator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"nope":"nothing here"}`)
	}))
	defer orchestrator.Close()

	t.Setenv("CATALOGIZER_LLM_IMAGE_SEARCH_ENABLED", "true")
	t.Setenv("CATALOGIZER_LLM_IMAGE_SEARCH_ENDPOINT", orchestrator.URL)

	r := NewLLMImageSearchResolver(90)
	_, err := r.Resolve(context.Background(), &resolver.ResolveRequest{EntityType: "movie", EntityID: "1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no url")
}
