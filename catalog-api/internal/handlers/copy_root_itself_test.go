package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Review round 5 (RV5-B): a destination that resolves to the copy root itself is not a file
// below the root. ContainedJoin accepted it, then MkdirAll(Dir(dest)) and the SMB download wrote
// at the root's PARENT level. It must be refused by input validation, before any service runs
// (the services are nil here; reaching one would turn into a 500).
func TestRoutedCopyToLocal_RefusesTheRootItselfAsDestination(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "sub"), 0o755))
	require.NoError(t, os.Symlink(".", filepath.Join(root, "self")))
	r := newProductionCopyRouter(t, root)
	for _, dest := range []string{".", "./", "sub/..", "self", "a/../."} {
		t.Run(dest, func(t *testing.T) {
			w := postJSON(r, "/api/v1/copy/local", map[string]any{
				"source_path": "nas:/share/file.bin", "destination_path": dest, "overwrite": true,
			})
			assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
		})
	}
}
