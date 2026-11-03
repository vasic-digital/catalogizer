package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	root_config "catalogizer/config"
	"catalogizer/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// addLegacyAdmin inserts an admin row in the legacy hash shape the old
// POST /users wrote: hex SHA-256(password+salt). CheckPassword accepts it at
// login, so the compromised-admin refusal and the offline rotation MUST see it.
func addLegacyAdmin(t *testing.T, db *database.DB, user, password string) {
	t.Helper()
	salt := "00112233445566778899aabbccddee" + "ff"
	sum := sha256.Sum256([]byte(password + salt))
	_, err := db.Exec(`INSERT INTO users (username,email,password_hash,salt,role_id,first_name,last_name,display_name,is_active) VALUES (?,?,?,?,1,'a','b','c',1)`,
		user, user+"@x", hex.EncodeToString(sum[:]), salt)
	require.NoError(t, err)
}

// F1: a legacy (SHA-256) admin row holding a published default password logs in
// (CheckPassword accepts it), so startup must refuse it and admin-reset rotate it.
func TestRefuseCompromisedAdminHashes_LegacySHA256EveryCandidate(t *testing.T) {
	for _, c := range root_config.PlaceholderPasswordCandidates() {
		db := seedTestDB(t)
		addLegacyAdmin(t, db, "legacyadmin", c)
		assert.True(t, errors.Is(refuseCompromisedAdminHashes(db), errCompromisedAdmin), "legacy candidate %q", c)
	}
}

func TestAdminReset_RotatesLegacySHA256Admin(t *testing.T) {
	const newPw = "Rotated-Op3rator-Value-7q"
	for _, c := range root_config.PlaceholderPasswordCandidates() {
		db := seedTestDB(t)
		addLegacyAdmin(t, db, "legacyadmin", c)
		t.Setenv("ADMIN_NEW_PASSWORD", newPw)
		var out, errb bytes.Buffer
		require.Equal(t, 0, runAdminReset(db, []string{"admin-reset-password"}, &out, &errb), c)
		assert.Contains(t, out.String(), "rotated", c)
		assert.NoError(t, refuseCompromisedAdminHashes(db), c)
		assert.True(t, verifyStored(t, db, "legacyadmin", newPw), c)
		assert.False(t, verifyStored(t, db, "legacyadmin", c), c)
	}
}

// A legacy admin row with a strong password is neither refused nor rotated.
func TestLegacyAdminWithStrongPasswordUntouched(t *testing.T) {
	db := seedTestDB(t)
	addLegacyAdmin(t, db, "legacyadmin", "Str0ng!Operator-Value-91")
	assert.NoError(t, refuseCompromisedAdminHashes(db))
}
