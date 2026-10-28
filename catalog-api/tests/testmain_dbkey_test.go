package tests

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain keeps the SQLCipher key file used by this package's tests out of
// the developer's real config directory (CAT-001).
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "cat-dbkey-")
	if err != nil {
		panic(err)
	}
	os.Setenv("DATABASE_ENCRYPTION_KEY_FILE", filepath.Join(dir, "db.key"))
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
