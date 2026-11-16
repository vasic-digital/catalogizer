package handlers

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"catalogizer/filesystem"
	"catalogizer/internal/config"
	"catalogizer/internal/services"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// failShare stands in for the SMB server only (unit-class fault injection): Open and Create fail with err, which the real SMBService wraps
// with %w; the routed handler, SMBService and the status mapping are real.
type failShare struct{ err error }

func (s failShare) ReadDir(string) ([]os.FileInfo, error)    { return nil, s.err }
func (s failShare) Open(string) (services.RemoteFile, error) { return nil, s.err }
func (s failShare) Create(string) (io.WriteCloser, error)    { return nil, s.err }
func (s failShare) Stat(string) (os.FileInfo, error)         { return nil, s.err }
func (s failShare) Mkdir(string, os.FileMode) error          { return nil }
func (s failShare) Remove(string) error                      { return nil }
func (s failShare) Umount() error                            { return nil }

func copyErrRouter(t *testing.T, shareErr error) (*CopyHandler, string) {
	cfg := &config.Config{}
	cfg.SMB.Hosts = []config.SMBHost{{Name: "nas", Host: "x", Port: 445, Share: "Public"}}
	cfg.SMB.ChunkSize = 1024
	svc := services.NewSMBServiceWithOpener(cfg, zap.NewNop(), func(string, string) (services.ShareAPI, func(), error) {
		return failShare{shareErr}, func() {}, nil
	})
	root := t.TempDir()
	return NewCopyHandler(nil, svc, root, zap.NewNop()), root
}

func uploadErrCase(r http.Handler, dest string) *httptest.ResponseRecorder {
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	fw, _ := mw.CreateFormFile("file", "a.txt")
	_, _ = fw.Write([]byte("data"))
	_ = mw.WriteField("destination", dest)
	_ = mw.WriteField("overwrite", "true")
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/copy/upload", &b)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// SMBCOPYR-F7: the filesystem.ErrCopy* refusals reach the client as 403/409/422, never as a blanket 500, on BOTH routed copy endpoints.
func TestRoutedCopy_ErrCopyRefusalsMapToHTTPStatus(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"read-only destination", filesystem.ErrCopyDestReadOnly, http.StatusForbidden},
		{"same file", filesystem.ErrCopySameFile, http.StatusConflict},
		{"destination is directory", filesystem.ErrCopyDestIsDirectory, http.StatusConflict},
		{"source not regular", filesystem.ErrCopySourceNotRegular, http.StatusUnprocessableEntity},
		{"other error stays 500", fmt.Errorf("boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := fmt.Errorf("%w: secret/path.txt", tc.err)
			h, _ := copyErrRouter(t, wrapped)
			r := newProductionCopyRouterWith(h)

			w := postJSON(r, "/api/v1/copy/local", map[string]any{"source_path": "nas:a/b.txt", "destination_path": "out/b.txt"})
			require.Equal(t, tc.want, w.Code, "download: %s", w.Body.String())
			require.NotContains(t, w.Body.String(), "secret/path.txt", "the raw error text (paths) must not reach the client")

			w = uploadErrCase(r, "nas:a/b.txt")
			require.Equal(t, tc.want, w.Code, "upload: %s", w.Body.String())
			require.NotContains(t, w.Body.String(), "secret/path.txt")
		})
	}
}

func TestCopyErrorStatus_UnrelatedErrorNotMapped(t *testing.T) {
	_, _, ok := copyErrorStatus(fmt.Errorf("x"))
	require.False(t, ok)
	_, _, ok = copyErrorStatus(nil)
	require.False(t, ok)
}
