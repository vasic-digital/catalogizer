package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"catalogizer/internal/pathguard"
	"catalogizer/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Review round 6 (class-exhaustive sink pass): DownloadJobFile served job.TargetPath with
// c.File, which follows symlinks and does not re-check the path. A stored job naming a file
// outside the conversion write roots (written before the confinement existed, edited in the
// database, or a symlink planted at the target after the conversion) leaked that file.
func TestDownloadJobFile_RV6_ServesOnlyConfinedRegularFiles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	base := t.TempDir()
	root := filepath.Join(base, "conv")
	outside := filepath.Join(base, "outside")
	require.NoError(t, os.MkdirAll(root, 0o755))
	require.NoError(t, os.MkdirAll(outside, 0o755))
	t.Setenv(pathguard.EnvConversionRoots, root)
	secret := filepath.Join(outside, "secret.txt")
	require.NoError(t, os.WriteFile(secret, []byte("TOPSECRET"), 0o600))
	good := filepath.Join(root, "good.txt")
	require.NoError(t, os.WriteFile(good, []byte("CONVERTED"), 0o644))
	link := filepath.Join(root, "link.txt")
	require.NoError(t, os.Symlink(secret, link))
	// a link that resolves inside the root passes the confine; only the regular-file check
	// refuses it (reviewer mutant R6-13 survived without this case)
	inRootLink := filepath.Join(root, "inroot-link.txt")
	require.NoError(t, os.Symlink(good, inRootLink))

	run := func(target string) *httptest.ResponseRecorder {
		conv := &MockConversionService{}
		auth := &MockConversionAuthService{}
		auth.On("GetCurrentUser", "t").Return(&models.User{ID: 1}, nil)
		conv.On("GetJob", 7, 1).Return(&models.ConversionJob{ID: 7, UserID: 1, Status: "completed", TargetPath: target}, nil)
		h := NewConversionHandler(conv, auth)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/conversion/jobs/7/download", nil)
		c.Request.Header.Set("Authorization", "Bearer t")
		c.Params = gin.Params{{Key: "id", Value: "7"}}
		h.DownloadJobFile(c)
		return w
	}

	for _, tc := range []struct{ name, target string }{
		{"outside-roots", secret},
		{"symlink-in-root", link},
		{"root-itself", root},
		{"in-root-symlink", inRootLink},
	} {
		w := run(tc.target)
		assert.NotEqual(t, http.StatusOK, w.Code, tc.name)
		assert.NotContains(t, w.Body.String(), "TOPSECRET", tc.name)
	}

	// golden-FALSE: a regular file inside the root is still served
	w := run(good)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "CONVERTED", w.Body.String())
}
