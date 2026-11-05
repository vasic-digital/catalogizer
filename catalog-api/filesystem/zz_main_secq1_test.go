package filesystem

import (
	"os"
	"testing"
)

// TestMain re-admits loopback for the in-process WebDAV fixtures (SECQ-1/W5: the WebDAV client refuses loopback unless
// the operator allow-list names it). The SECQ1 tests clear the variable to prove the guard.
func TestMain(m *testing.M) {
	os.Setenv("CATALOGIZER_OUTBOUND_ALLOW_CIDRS", "127.0.0.0/8,::1/128")
	os.Exit(m.Run())
}
