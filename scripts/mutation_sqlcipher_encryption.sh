#!/usr/bin/env bash
# mutation_sqlcipher_encryption.sh - paired §1.1 mutations for CAT-001 (SQLCipher
# encryption at rest). Applies each mutation to catalog-api/database in place,
# runs the TestEncryption suite through the given Go runner and REQUIRES it to
# FAIL; the original file is restored after every mutation (copy kept as .orig).
#
# Usage:   scripts/mutation_sqlcipher_encryption.sh <go-runner>
#          <go-runner> is a command invoked as: <go-runner> test -count=1 -run TestEncryption ./database/
#          (e.g. a wrapper that runs `go` inside rootless podman, §11.4.173).
# Exit:    0 when EVERY mutation was killed by the suite, 1 otherwise.
set -u
RUNNER="${1:?usage: $0 <go-runner>}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENC="$ROOT/catalog-api/database/encryption.go"
CONN="$ROOT/catalog-api/database/connection.go"
survived=0

mutate() { # mutate <label> <file> <old> <new>
  local label="$1" file="$2" old="$3" new="$4"
  cp -p "$file" "$file.orig" || exit 2
  if ! python3 - "$file" "$old" "$new" <<'PY'
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
  if "$RUNNER" test -count=1 -run TestEncryption ./database/ >"/tmp/mut_$label.log" 2>&1; then
    echo "MUTATION $label: SURVIVED (suite still green)  <-- FAIL"; survived=$((survived+1))
  else
    echo "MUTATION $label: killed"
  fi
  mv -f "$file.orig" "$file"
}

mutate M1_no_key_on_connect "$ENC" 'return path + "?" + params + "&_pragma_key=" + url.QueryEscape("x'"'"'"+keyHex+"'"'"'")' 'return path + "?" + params + "&_ignored=" + url.QueryEscape("x'"'"'"+keyHex+"'"'"'")'
mutate M2_no_perm_check "$ENC" 'info.Mode().Perm()&0o077 != 0' 'false'
mutate M3_wrong_key_accepted "$ENC" 'return errors.New("database cannot be decrypted with the configured key (wrong key or corrupted file)")' 'return nil'
mutate M4_backup_skips_sidecars "$ENC" 'if err != nil || st.Size() == 0 {' 'if true || err != nil || st.Size() == 0 {'
mutate M5_rekey_noop "$ENC" 'if _, err := db.Exec(fmt.Sprintf(`PRAGMA rekey = "x'"'"'%s'"'"'"`, newKeyHex)); err != nil {' 'if false {'
mutate M6_migration_drops_backup "$ENC" 'bak := uniquePath(path + ".plaintext-backup-" + ts)
	if err = os.Rename(path, bak); err != nil {' 'bak := uniquePath(path + ".plaintext-backup-" + ts)
	if err = os.Rename(path, bak); err != nil {
		return err
	}
	if err = os.Remove(bak); err != nil {'
mutate M7_missing_key_regenerated_for_existing_db "$ENC" "return loadOrCreateKey(keyPath, false)" "return loadOrCreateKey(keyPath, true)"
mutate M8_key_file_world_readable "$ENC" 'if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		cleanup()' 'if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		cleanup()'
mutate M9_plaintext_fallback_for_files "$CONN" 'if isMemoryPath(cfg.Path) {' 'if true {'

if [ "$survived" -ne 0 ]; then echo "$survived mutation(s) survived"; exit 1; fi
echo "all mutations killed"
