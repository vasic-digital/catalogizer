package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"

	root_config "catalogizer/config"
	"catalogizer/database"

	"golang.org/x/crypto/bcrypt"
)

// errAdminHashCheck means the startup check of stored admin hashes could not
// be completed; startup fails closed (CAT-021).
var errAdminHashCheck = errors.New("cannot verify stored admin password hashes")

// handleSeedAdminError escalates the two refusal classes to fatal and treats
// any other seeding problem as a warning.
func handleSeedAdminError(err error, fatal, warn func(msg string, err error)) {
	if err == nil {
		return
	}
	if errors.Is(err, errCompromisedAdmin) || errors.Is(err, errAdminHashCheck) {
		fatal("Refusing to start", err)
		return
	}
	warn("Failed to seed admin user", err)
}

// newAdminPasswordFromEnv reads the replacement password from
// ADMIN_NEW_PASSWORD (by NAME; never printed) and validates it.
func newAdminPasswordFromEnv() (string, error) {
	pw := os.Getenv("ADMIN_NEW_PASSWORD")
	if pw == "" {
		return "", errors.New("ADMIN_NEW_PASSWORD must be set to the new admin password")
	}
	if root_config.IsPlaceholderPassword(pw) && !root_config.AllowInsecureDefaults() {
		return "", errors.New("ADMIN_NEW_PASSWORD is a known placeholder/default value; choose a real password")
	}
	return pw, nil
}

// runAdminReset implements the offline `admin-reset-password [username]`
// subcommand (server stopped). With no username it rotates every admin account
// whose stored hash matches a published default; with a username it rotates
// that admin. It prints account names only, never passwords. Returns the exit code.
func runAdminReset(db *database.DB, args []string, stdout, stderr io.Writer) int {
	pw, err := newAdminPasswordFromEnv()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	target := ""
	if len(args) > 1 {
		target = args[1]
	}
	rows, err := db.Query("SELECT id, username, password_hash, COALESCE(salt,'') FROM users WHERE role_id = 1")
	if err != nil {
		fmt.Fprintf(stderr, "admin-reset-password FAILED: %v\n", err)
		return 1
	}
	type acct struct {
		id               int64
		user, hash, salt string
	}
	var todo []acct
	for rows.Next() {
		var a acct
		if err := rows.Scan(&a.id, &a.user, &a.hash, &a.salt); err != nil {
			rows.Close()
			fmt.Fprintf(stderr, "admin-reset-password FAILED: %v\n", err)
			return 1
		}
		if target != "" {
			if a.user == target {
				todo = append(todo, a)
			}
			continue
		}
		for _, c := range root_config.PlaceholderPasswordCandidates() {
			if bcrypt.CompareHashAndPassword([]byte(a.hash), []byte(c+a.salt)) == nil {
				todo = append(todo, a)
				break
			}
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		fmt.Fprintf(stderr, "admin-reset-password FAILED: %v\n", err)
		return 1
	}
	rows.Close()
	if len(todo) == 0 {
		if target != "" {
			fmt.Fprintf(stderr, "admin-reset-password FAILED: no admin account named %q\n", target)
			return 1
		}
		fmt.Fprintln(stdout, "no compromised admin account to rotate")
		return 0
	}
	for _, a := range todo {
		sb := make([]byte, 16)
		if _, err := rand.Read(sb); err != nil {
			fmt.Fprintf(stderr, "admin-reset-password FAILED: %v\n", err)
			return 1
		}
		salt := hex.EncodeToString(sb)
		h, err := bcryptHash([]byte(pw + salt))
		if err != nil {
			fmt.Fprintf(stderr, "admin-reset-password FAILED: %v\n", err)
			return 1
		}
		// Revoke every session/refresh token FIRST: if the hash update then fails the
		// account is merely logged out, never left with live old-credential sessions.
		if _, err := db.Exec("UPDATE user_sessions SET is_active = 0 WHERE user_id = ?", a.id); err != nil {
			fmt.Fprintf(stderr, "admin-reset-password FAILED (session revoke): %v\n", err)
			return 1
		}
		if _, err := db.Exec("UPDATE users SET password_hash = ?, salt = ? WHERE id = ?", string(h), salt, a.id); err != nil {
			fmt.Fprintf(stderr, "admin-reset-password FAILED: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "rotated admin account: %s\n", a.user)
	}
	return 0
}
