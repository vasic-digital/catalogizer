#!/usr/bin/env bash
# mutation_sqlcipher_encryption.sh - paired §1.1 mutations for CAT-001 (SQLCipher
# encryption at rest). Applies each mutation to catalog-api/database in place,
# runs the TestEncryption + TestHardening suites through the given Go runner and
# REQUIRES them to FAIL; the original file is restored after every mutation
# (copy kept as <file>.orig during the run).
#
# Usage:   scripts/mutation_sqlcipher_encryption.sh <go-runner>
#          <go-runner> is a command invoked as:
#            <go-runner> test -count=1 -run 'TestEncryption|TestHardening' ./database/
#          (e.g. a wrapper that runs `go` inside rootless podman, §11.4.173).
# Logs:    one file per mutation in a fresh mktemp directory (printed at start).
# Exit:    0 when EVERY mutation was killed by the suite, 1 otherwise.
set -u
RUNNER="${1:?usage: $0 <go-runner>}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DB="$ROOT/catalog-api/database"
ENC="$DB/encryption.go"
CONN="$DB/connection.go"
UNIX="$DB/encryption_owner_unix.go"
LOGDIR="$(mktemp -d "${TMPDIR:-/tmp}/cat001-mut.XXXXXX")" || exit 2
echo "mutation logs: $LOGDIR"
survived=0

mutate() { # mutate <label> <file> <old> <new>
  local label="$1" file="$2" old="$3" new="$4"
  cp -p "$file" "$file.orig" || exit 2
  if ! python3 -I - "$file" "$old" "$new" <<'PY'
import sys
p, old, new = sys.argv[1:4]
s = open(p).read()
if old not in s:
    sys.exit(3)
open(p, "w").write(s.replace(old, new, 1))
PY
  then
    echo "MUTATION $label: could not apply (pattern missing)"; mv -f "$file.orig" "$file"; survived=$((survived+1)); return
  fi
  if "$RUNNER" test -count=1 -run 'TestEncryption|TestHardening' ./database/ >"$LOGDIR/$label.log" 2>&1; then
    echo "MUTATION $label: SURVIVED (suite still green)  <-- FAIL"; survived=$((survived+1))
  else
    if grep -q "build failed" "$LOGDIR/$label.log"; then
      echo "MUTATION $label: build error (not a valid kill)  <-- FAIL"; survived=$((survived+1))
    else
      echo "MUTATION $label: killed"
    fi
  fi
  mv -f "$file.orig" "$file"
}

# --- original mutants (M1-M9) ---
mutate M1_no_key_on_connect "$ENC" 'return path + "?" + params + "&_pragma_key=" + url.QueryEscape("x'"'"'"+keyHex+"'"'"'")' 'return path + "?" + params + "&_ignored=" + url.QueryEscape("x'"'"'"+keyHex+"'"'"'")'
mutate M2_no_perm_check "$ENC" 'info.Mode().Perm()&0o077 != 0' 'false'
mutate M3_wrong_key_accepted "$ENC" 'return errors.New("database cannot be decrypted with the configured key (wrong key or corrupted file)")' 'return nil'
mutate M4_backup_skips_sidecars "$ENC" 'if serr != nil || st.Size() == 0 {' 'if true || serr != nil || st.Size() == 0 {'
mutate M5_rekey_noop "$ENC" 'if _, err := db.Exec(fmt.Sprintf(`PRAGMA rekey = "x'"'"'%s'"'"'"`, newKeyHex)); err != nil {' 'if false {'
mutate M6_migration_drops_backup "$ENC" '_ = os.Chmod(bak, 0o600)' '_ = os.Chmod(bak, 0o600); _ = os.Remove(bak)'
mutate M7_missing_key_regenerated_for_existing_db "$ENC" "k, err := loadOrCreateKey(keyPath, false)" "k, err := loadOrCreateKey(keyPath, true)"
mutate M8_key_file_world_readable "$ENC" 'if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		cleanup()' 'if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		cleanup()'
mutate M9_plaintext_fallback_for_files "$CONN" 'if isMemoryPath(cfg.Path) {' 'if true {'

# --- review round 1 mutants (RM1-RM7 and the new code paths) ---
mutate RM1_migration_rowcount_check_off "$ENC" 'if err := equalCounts(wantCounts, got); err != nil {' 'if err := equalCounts(wantCounts, got); err != nil && false {'
mutate RM2_rekey_old_key_check_off "$ENC" 'if oerr == nil {' 'if false && oerr == nil {'
mutate RM3_scrubdsn_off "$ENC" 'if i := strings.Index(msg, "_pragma_key="); i >= 0 {' 'if i := strings.Index(msg, "_pragma_key="); i >= 1<<30 {'
mutate RM4_owner_check_off "$UNIX" 'if int(fileUID) != want {' 'if false {'
mutate RM6_rekey_rowcount_check_off "$ENC" 'if err := equalCounts(before, after); err != nil {' 'if err := equalCounts(before, after); err != nil && false {'
mutate R1_backup_cleanup_removes_foreign_files "$ENC" 'removeAll(created) // only what this call created' 'removeAll(append(created, dest, dest+"-wal", dest+"-shm"))'
mutate R2_rekey_no_exclusive_lock "$ENC" 'if err := acquireExclusive(db); err != nil {
		closeDB()' 'if err := acquireExclusive(db); false && err != nil {
		closeDB()'
mutate R3_restore_no_staged_verification "$ENC" 'if err = verifyStaged(staging, key); err != nil {' 'if false {'
mutate R3_restore_no_running_server_probe "$ENC" 'if rel, perr := probeInUse(target, key); perr != nil {' 'if rel, perr := probeInUse(target, key); false && perr != nil {'
mutate R3_restore_accepts_plaintext_backup "$ENC" '} else if st == dbPlaintext {' '} else if false && st == dbPlaintext {'
mutate L1_key_symlink_followed "$UNIX" 'os.O_RDONLY|syscall.O_NOFOLLOW' 'os.O_RDONLY'
mutate L2_key_dir_perms_unchecked "$UNIX" 'st.Mode().Perm()&0o022 != 0' 'false'
mutate L3_no_staged_key_hint "$ENC" 'if _, err := os.Lstat(keyPath + ".new"); err == nil {
		return fmt.Sprintf(' 'if _, err := os.Lstat(keyPath + ".new"); false && err == nil {
		return fmt.Sprintf('
mutate L4_rekey_installs_key_without_verify "$ENC" 'if err := verifyRekey(cfg.Path, oldKey, newKeyHex, before); err != nil {' 'if err := verifyRekey(cfg.Path, oldKey, newKeyHex, before); false && err != nil {'

# --- review round 2 mutants (reviewer X1-X8 plus the R4/R6/F3 fixes) ---
mutate X1_no_exclusive_locking_mode "$ENC" 'if _, err := db.Exec(`PRAGMA locking_mode=EXCLUSIVE`); err != nil {' 'if _, err := db.Exec(`PRAGMA locking_mode=NORMAL`); err != nil {'
mutate X2_journal_mode_check_off "$ENC" 'if !strings.EqualFold(mode, "delete") {
		return fmt.Errorf("journal mode stayed' 'if false {
		return fmt.Errorf("journal mode stayed'
mutate X3_restore_no_aside_link "$ENC" 'if err = linkOrExplain(target, aside); err != nil {' 'if false {'
mutate X4_restore_integrity_check_off "$ENC" 'if res != "ok" {' 'if false {'
mutate X5_backup_deferred_no_lock "$ENC" '`BEGIN IMMEDIATE`' '`BEGIN DEFERRED`'
mutate X6_migration_sidecar_check_off "$ENC" 'if st, serr := os.Stat(s); serr == nil && st.Size() > 0 {
			return fmt.Errorf("plaintext' 'if st, serr := os.Stat(s); false && serr == nil && st.Size() > 0 {
			return fmt.Errorf("plaintext'
mutate X7_rekey_no_oldkey_backup "$ENC" 'if err := copyFile(keyPath, res.OldKeyBackupPath); err != nil {' 'if false {'
mutate X8_probe_skips_exclusive_lock "$ENC" 'if err := acquireExclusive(db); err != nil {
		db.Close()
		return nil, err
	}
	return func() { _ = db.Close() }, nil' 'db.Close()
	return func() {}, nil'
mutate R4_migration_source_not_locked "$ENC" 'if err = acquireExclusive(src); err != nil {' 'if false {'
mutate R6_probe_busy_fails_open "$ENC" 'return nil, fmt.Errorf("database is in use by another process (stop the server first): %v", err)
	case isNotADBErr' 'return func() {}, nil
	case isNotADBErr'
mutate F3_restore_releases_lock_early "$ENC" 'defer rel()' 'rel()'

# --- review round 3 mutants (Windows lock release, fail-closed default) ---
mutate Y2_unknown_probe_error_treated_as_notadb "$ENC" 'return strings.Contains(m, "not a database") || strings.Contains(m, "encrypted")' 'return len(m) >= 0'
mutate R7_migration_keeps_lock_through_rename_on_windows "$ENC" 'if closeBeforeRename(hostOS) {
		_ = src.Close()' 'if false {
		_ = src.Close()'
mutate R7_restore_keeps_lock_through_rename_on_windows "$ENC" '} else if closeBeforeRename(hostOS) {' '} else if false {'
mutate R7_close_before_rename_never "$ENC" 'func closeBeforeRename(goos string) bool { return goos == "windows" }' 'func closeBeforeRename(goos string) bool { return false }'

# --- review round 4 follow-ups (descriptor leaks, wording, swap-hook coverage) ---
mutate Z2_probe_notadb_path_leaks_handle "$ENC" 'Scan(&n); err != nil {
		db.Close()
		return classifyProbeError(err)' 'Scan(&n); err != nil {
		return classifyProbeError(err)'
mutate Z6_probe_lock_failure_leaks_handle "$ENC" 'if err := acquireExclusive(db); err != nil {
		db.Close()
		return nil, err' 'if err := acquireExclusive(db); err != nil {
		return nil, err'
mutate W1_probe_open_error_reported_as_missing_cipher "$ENC" 'cannot open the database to probe it: %v' 'SQLCipher is not active in this build: %v'
mutate R7_migration_swap_hook_never_fires "$ENC" 'if beforeSwapHook != nil {
		beforeSwapHook()
	}
	// Swap: keep' 'if false {
		beforeSwapHook()
	}
	// Swap: keep'
mutate R7_restore_swap_hook_never_fires "$ENC" 'if beforeSwapHook != nil {
		beforeSwapHook()
	}
	if err = os.Rename(staging, target)' 'if false {
		beforeSwapHook()
	}
	if err = os.Rename(staging, target)'

if [ "$survived" -ne 0 ]; then echo "$survived mutation(s) survived"; exit 1; fi
echo "all mutations killed"
