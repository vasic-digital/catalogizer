package filesystem

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWebDAVClient(t *testing.T) {
	tests := []struct {
		name    string
		config  *WebDAVConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &WebDAVConfig{
				URL:      "http://example.com/webdav",
				Username: "user",
				Password: "pass",
				Path:     "/files",
			},
			wantErr: false,
		},
		{
			name: "valid config without auth",
			config: &WebDAVConfig{
				URL: "http://example.com/webdav",
			},
			wantErr: false,
		},
		{
			name: "invalid URL",
			config: &WebDAVConfig{
				URL: "://invalid-url",
			},
			wantErr: true,
			errMsg:  "invalid WebDAV URL",
		},
		{
			name: "empty path defaults to root",
			config: &WebDAVConfig{
				URL:  "http://example.com/webdav",
				Path: "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewWebDAVClient(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.NotNil(t, client.client)
				assert.NotNil(t, client.baseURL)
				assert.Equal(t, tt.config, client.config)
			}
		})
	}
}

func TestWebDAVClient_Connect_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PROPFIND", r.Method)
		assert.Equal(t, "0", r.Header.Get("Depth"))
		w.WriteHeader(http.StatusMultiStatus)
	}))
	defer server.Close()

	config := &WebDAVConfig{
		URL:      server.URL,
		Username: "testuser",
		Password: "testpass",
	}

	client, err := NewWebDAVClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.Connect(ctx)

	assert.NoError(t, err)
	assert.True(t, client.connected)
}

func TestWebDAVClient_Connect_OKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &WebDAVConfig{
		URL: server.URL,
	}

	client, err := NewWebDAVClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.Connect(ctx)

	assert.NoError(t, err)
	assert.True(t, client.connected)
}

func TestWebDAVClient_Connect_AuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	config := &WebDAVConfig{
		URL: server.URL,
	}

	client, err := NewWebDAVClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.Connect(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401")
	assert.False(t, client.connected)
}

func TestWebDAVClient_Connect_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := &WebDAVConfig{
		URL: server.URL,
	}

	client, err := NewWebDAVClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.Connect(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestWebDAVClient_Connect_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := &WebDAVConfig{
		URL: server.URL,
	}

	client, err := NewWebDAVClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.Connect(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestWebDAVClient_Connect_WithTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusMultiStatus)
	}))
	defer server.Close()

	config := &WebDAVConfig{
		URL: server.URL,
	}

	client, err := NewWebDAVClient(config)
	require.NoError(t, err)

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = client.Connect(ctx)
	assert.Error(t, err)
}

func TestWebDAVClient_IsConnected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultiStatus)
	}))
	defer server.Close()

	config := &WebDAVConfig{
		URL: server.URL,
	}

	client, err := NewWebDAVClient(config)
	require.NoError(t, err)

	// Initially not connected
	assert.False(t, client.IsConnected())

	// Connect
	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, client.IsConnected())

	// Disconnect
	err = client.Disconnect(ctx)
	require.NoError(t, err)
	assert.False(t, client.IsConnected())
}

func TestWebDAVClient_TestConnection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultiStatus)
	}))
	defer server.Close()

	config := &WebDAVConfig{
		URL: server.URL,
	}

	client, err := NewWebDAVClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Test without connection should fail
	err = client.TestConnection(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")

	// Connect first
	err = client.Connect(ctx)
	require.NoError(t, err)

	// Test with connection should succeed
	err = client.TestConnection(ctx)
	assert.NoError(t, err)
}

func TestWebDAVClient_ReadFile_Success(t *testing.T) {
	expectedContent := "Hello, WebDAV!"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PROPFIND" {
			w.WriteHeader(http.StatusMultiStatus)
			return
		}

		// Check authentication
		user, pass, ok := r.BasicAuth()
		if ok {
			assert.Equal(t, "testuser", user)
			assert.Equal(t, "testpass", pass)
		}

		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(expectedContent))
		}
	}))
	defer server.Close()

	config := &WebDAVConfig{
		URL:      server.URL,
		Username: "testuser",
		Password: "testpass",
	}

	client, err := NewWebDAVClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)

	reader, err := client.ReadFile(ctx, "/test.txt")
	require.NoError(t, err)
	assert.NotNil(t, reader)

	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, expectedContent, string(content))

	reader.Close()
}

func TestWebDAVClient_ReadFile_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PROPFIND" {
			w.WriteHeader(http.StatusMultiStatus)
			return
		}
		if r.Method == "GET" {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	config := &WebDAVConfig{
		URL: server.URL,
	}

	client, err := NewWebDAVClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)

	_, err = client.ReadFile(ctx, "/nonexistent.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestWebDAVClient_ReadFile_NotConnected(t *testing.T) {
	config := &WebDAVConfig{
		URL: "http://example.com/webdav",
	}

	client, err := NewWebDAVClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.ReadFile(ctx, "/test.txt")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestWebDAVClient_WriteFile_Success(t *testing.T) {
	var receivedContent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PROPFIND" {
			w.WriteHeader(http.StatusMultiStatus)
			return
		}
		if r.Method == "PUT" {
			body, _ := io.ReadAll(r.Body)
			receivedContent = string(body)
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer server.Close()

	config := &WebDAVConfig{
		URL:      server.URL,
		Username: "testuser",
		Password: "testpass",
	}

	client, err := NewWebDAVClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)

	content := "test content"
	reader := strings.NewReader(content)
	err = client.WriteFile(ctx, "/test.txt", reader)

	assert.NoError(t, err)
	assert.Equal(t, content, receivedContent)
}

func TestWebDAVClient_WriteFile_NotConnected(t *testing.T) {
	config := &WebDAVConfig{
		URL: "http://example.com/webdav",
	}

	client, err := NewWebDAVClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	reader := strings.NewReader("test")
	err = client.WriteFile(ctx, "/test.txt", reader)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestWebDAVClient_Disconnect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultiStatus)
	}))
	defer server.Close()

	config := &WebDAVConfig{
		URL: server.URL,
	}

	client, err := NewWebDAVClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, client.IsConnected())

	err = client.Disconnect(ctx)
	assert.NoError(t, err)
	assert.False(t, client.IsConnected())
}

func TestWebDAVClient_resolveURL(t *testing.T) {
	config := &WebDAVConfig{
		URL:  "http://example.com/webdav",
		Path: "/base",
	}

	client, err := NewWebDAVClient(config)
	require.NoError(t, err)

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "simple path",
			path:     "/file.txt",
			expected: "http://example.com/webdav/base/file.txt", // WF22 F5: Path is joined UNDER the URL path (the old expectation encoded the replacement bug)
		},
		{
			name:     "nested path",
			path:     "/folder/subfolder/file.txt",
			expected: "http://example.com/webdav/base/folder/subfolder/file.txt",
		},
		{
			name:     "path with traversal attempt",
			path:     "/../../../etc/passwd",
			expected: "http://example.com/webdav/base/etc/passwd", // ".." segments cannot climb out of the base
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.resolveURL(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWebDAVClient_WithTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusMultiStatus)
	}))
	defer server.Close()

	config := &WebDAVConfig{
		URL: server.URL,
	}

	client, err := NewWebDAVClient(config)
	require.NoError(t, err)

	// Reduce timeout for testing
	client.client.Timeout = 100 * time.Millisecond

	ctx := context.Background()
	err = client.Connect(ctx)

	assert.Error(t, err)
}

// Benchmark tests
func BenchmarkWebDAVClient_resolveURL(b *testing.B) {
	config := &WebDAVConfig{
		URL:  "http://example.com/webdav",
		Path: "/base",
	}

	client, _ := NewWebDAVClient(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.resolveURL("/folder/subfolder/file.txt")
	}
}
