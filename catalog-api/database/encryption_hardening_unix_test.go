//go:build !windows

package database

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// RM4: a key file owned by another user is refused.
func TestHardening_RM4_OwnerCheck(t *testing.T) {
	if !keyPermissionsEnforced() {
		t.Skip("owner checks are not enforced on this platform (documented)")
	}
	me := os.Getuid()
	assert.NoError(t, checkOwnerUID(uint32(me), me))
	assert.Error(t, checkOwnerUID(uint32(me+1), me))
}
