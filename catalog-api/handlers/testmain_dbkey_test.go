package handlers

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
	os.Setenv("CATALOGIZER_OUTBOUND_ALLOW_CIDRS", "127.0.0.0/8,::1/128") // SECQ-1/W5: loopback WebDAV fixtures (the SECQ1 test clears it)
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
