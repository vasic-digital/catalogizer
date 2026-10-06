#!/usr/bin/env bash
# Test (TDD) for scripts/anti-bluff-scan.sh (owner decision 2026-10-05): wrapper over scripts/audit/anti-bluff-scan.sh with --files-from.
# Usage: bash scripts/audit/tests/test_anti_bluff_scan_wrapper.sh        Env: W=<wrapper under test> (default scripts/anti-bluff-scan.sh;
#   the mutation test points it at a mutated copy placed in a mirror tree so the wrapper finds its inner scanner and lib_safe.sh).
# Fixtures live in a temp dir; the real tree is never scanned. Needs python3-free bash only.
set -u
REPO="$(git rev-parse --show-toplevel)" || exit 2
cd "$REPO" || exit 2
W="${W:-scripts/anti-bluff-scan.sh}"
case "$W" in /*) ;; *) W="$REPO/$W" ;; esac
T="$(mktemp -d "${TMPDIR:-/tmp}/abs_t.XXXXXX")"; trap 'rm -rf "$T"' EXIT
PASSN=0; FAILN=0
ok()  { PASSN=$((PASSN+1)); echo "ok   $1"; }
bad() { FAILN=$((FAILN+1)); echo "FAIL $1"; }
[ -x "$W" ] || echo "NOTE: $W absent or not executable (RED state: every case below must FAIL)"

R="$T/root"; mkdir -p "$R/pkg" "$R/other" "$T/outside"
cat > "$R/pkg/clean_test.go" <<'G'
package pkg

import "testing"

func TestClean(t *testing.T) {
	x := 1
	if x != 1 {
		t.Errorf("x = %d", x)
	}
}
G
cat > "$R/pkg/bad_test.go" <<'G'
package pkg

import "testing"

func TestNothingAsserted(t *testing.T) {
	x := 1
	_ = x
}
G
cat > "$R/other/unlisted_bad_test.go" <<'G'
package other

import "testing"

func TestAlsoNothing(t *testing.T) {
	y := 2
	_ = y
}
G
cp "$R/pkg/bad_test.go" "$T/outside/out_bad_test.go"
ln -s "$T/outside/out_bad_test.go" "$R/pkg/link_test.go"
ln -s "$T/outside" "$R/linkdir"
mkdir "$R/dir_test.go.d"; : > "$R/pkg/plain.txt"; mkfifo "$R/fifo_entry"

run() { # run <name> <args...>   -> RC, OUT (stdout file), ERR
  local name="$1"; shift
  OUT="$T/$name.out"; ERRF="$T/$name.err"
  timeout 20 "$W" "$@" >"$OUT" 2>"$ERRF"; RC=$?
}
lst() { printf '%s\n' "$@" > "$T/list.txt"; }
want() { # want <case> <rc> [stdout-grep] [stdout-must-not-grep]
  local c="$1" rc="$2" g="${3:-}" ng="${4:-}" fine=1
  [ "$RC" -eq "$rc" ] || fine=0
  [ -z "$g" ] || grep -q -- "$g" "$OUT" || fine=0
  [ -z "$ng" ] || ! grep -q -- "$ng" "$OUT" || fine=0
  if [ "$fine" -eq 1 ]; then ok "$c (rc=$RC)"; else bad "$c: want rc=$rc grep=[$g] not=[$ng]; got rc=$RC out=[$(head -c 200 "$OUT" 2>/dev/null | tr '\n\t' '|~')] err=[$(head -c 160 "$ERRF" 2>/dev/null | tr '\n' '|')]"; fi
}
refused() { # refused <case> : rc 2, no finding on stdout, a REFUSED line on stderr
  if [ "$RC" -eq 2 ] && [ ! -s "$OUT" ] && grep -q 'REFUSED' "$ERRF"; then ok "$1 (rc=2, REFUSED, nothing scanned)"
  else bad "$1: want rc=2 + REFUSED + empty stdout; got rc=$RC out=[$(head -c 120 "$OUT" 2>/dev/null)] err=[$(head -c 160 "$ERRF" 2>/dev/null | tr '\n' '|')]"; fi
}

# ---- whole-tree mode = the scanner's behaviour, with a non-zero exit on any finding
run full_bad --root "$R";                                   want "whole tree with violations exits non-zero and reports them" 1 GO_NO_ASSERT
mkdir -p "$T/cleanroot/pkg"; cp "$R/pkg/clean_test.go" "$T/cleanroot/pkg/"
run full_clean --root "$T/cleanroot";                       want "whole tree without violations exits 0 with no output" 0 "" "."
run full_pos "$T/cleanroot";                                want "positional root accepted (clean) -> 0" 0
# ---- --files-from: scoping
lst pkg/clean_test.go;                         run f_clean --root "$R" --files-from "$T/list.txt"
  want "listed clean file, unlisted violations elsewhere IGNORED -> 0, no finding" 0 "" "unlisted_bad"
lst pkg/bad_test.go;                           run f_bad --root "$R" --files-from "$T/list.txt"
  want "violation planted in a LISTED file -> 1 and named" 1 "pkg/bad_test.go.*GO_NO_ASSERT" "unlisted_bad"
lst pkg/clean_test.go pkg/bad_test.go;         run f_both --root "$R" --files-from "$T/list.txt"
  want "clean + bad listed -> 1, only the listed bad file reported" 1 "bad_test.go" "unlisted_bad"
lst pkg/clean_test.go pkg/clean_test.go;       run f_dup --root "$R" --files-from "$T/list.txt"; want "duplicate entries harmless -> 0" 0
lst pkg/plain.txt;                             run f_txt --root "$R" --files-from "$T/list.txt"; want "listed non-test file with no pattern -> 0" 0
printf 'pkg/bad_test.go' > "$T/list_nonl.txt"; run f_nonl --root "$R" --files-from "$T/list_nonl.txt"
  want "last line without newline is still scanned -> 1" 1 GO_NO_ASSERT
lst pkg/clean_test.go;                         run f_eq --root="$R" --files-from="$T/list.txt" --; want "=value forms and -- accepted -> 0" 0
# the real tree is never written
before="$(find "$R" -type f -newer "$T/list.txt" 2>/dev/null | wc -l)"; [ "$before" -eq 0 ] && ok "no file in the scanned tree was created or modified" || bad "tree modified ($before)"

# ---- hostile / unsafe paths: refused as a whole, rc 2, nothing scanned
hostile() { # hostile <label> <exact line>
  printf '%s\n' "$2" > "$T/h.txt"; run "h_$1" --root "$R" --files-from "$T/h.txt"; refused "hostile path: $1"
}
hostile optionlike '-bad_test.go'
hostile longopt '--files-from'
hostile dotdot '../outside/out_bad_test.go'
hostile dotdot_mid 'pkg/../pkg/bad_test.go'
hostile absolute "$T/outside/out_bad_test.go"
hostile dot_prefix './pkg/bad_test.go'
hostile dot_mid 'pkg/./bad_test.go'
hostile trailing_slash 'pkg/'
hostile glob_star 'pkg/*_test.go'
hostile glob_q 'pkg/bad_tes?.go'
hostile glob_bracket 'pkg/[b]ad_test.go'
hostile pathspec_magic ':(glob)pkg/bad_test.go'
hostile double_slash 'pkg//bad_test.go'
hostile backslash 'pkg\bad_test.go'
hostile tab_in_name "$(printf 'pkg/bad\ttest.go')"
hostile empty_line ''
hostile only_dot '.'
hostile nonexistent 'pkg/does_not_exist_test.go'
hostile directory 'dir_test.go.d'
hostile symlink_file 'pkg/link_test.go'
hostile symlink_dir_component 'linkdir/out_bad_test.go'
lst pkg/clean_test.go pkg/bad_test.go -evil; run h_late --root "$R" --files-from "$T/list.txt"; refused "one hostile entry among good ones refuses the whole run"
# ---- options
lst pkg/clean_test.go
mkdir -p "$T/cwd"; printf 'pkg/clean_test.go\n' > "$T/cwd/-x"
OUT="$T/o_listopt.out"; ERRF="$T/o_listopt.err"; ( cd "$T/cwd" && timeout 20 "$W" --root "$R" --files-from -x ) >"$OUT" 2>"$ERRF"; RC=$?
refused "option-like --files-from value (an existing file named -x must still be refused)"
hostile fifo 'fifo_entry'

run o_missing --root "$R" --files-from "$T/none.txt";       refused "missing list file"
: > "$T/empty.txt"; run o_empty --root "$R" --files-from "$T/empty.txt"; refused "empty list file (scans nothing, must not read as clean)"
run o_unknown --root "$R" --nope;                           refused "unknown option"
run o_noval --root "$R" --files-from;                       refused "--files-from without a value"
run o_badroot --root "$T/no_such_dir";                      refused "root that is not a directory"
run o_optroot --root -x;                                    refused "option-like root"
# ---- the unscanned symlink target is never reached: the finding of the outside file must not appear anywhere
lst pkg/link_test.go; run o_link --root "$R" --files-from "$T/list.txt"
grep -q 'out_bad_test.go\|link_test.go.*GO_NO_ASSERT' "$OUT" && bad "symlink target was followed (finding emitted)" || ok "symlink target never scanned (no finding for the outside file)"
# ---- the wrapper trusts a finding even when the inner scanner exits 0, and maps an inner failure to rc 2 (stub inner scanners)
stub() { # stub <tag> <body> : mirror tree (wrapper + lib_safe.sh from the wrapper's own tree) with a stub inner scanner
  local d="$T/mir_$1"; mkdir -p "$d/scripts/audit" "$d/scripts/repo"
  cp "$W" "$d/scripts/anti-bluff-scan.sh"; cp "$(dirname "$W")/repo/lib_safe.sh" "$d/scripts/repo/"
  printf '#!/usr/bin/env bash\n%s\n' "$2" > "$d/scripts/audit/anti-bluff-scan.sh"; chmod +x "$d/scripts/anti-bluff-scan.sh" "$d/scripts/audit/anti-bluff-scan.sh"
  SW="$d/scripts/anti-bluff-scan.sh"
}
stub s_find 'printf "x\t1\tFAKE\ty\n"; exit 0';  OUT="$T/s1.out"; ERRF="$T/s1.err"; "$SW" --root "$T/cleanroot" >"$OUT" 2>"$ERRF"; RC=$?
want "finding printed by the inner scanner with exit 0 still fails the wrapper" 1 FAKE
stub s_rc1 'exit 1';                              OUT="$T/s2.out"; ERRF="$T/s2.err"; "$SW" --root "$T/cleanroot" >"$OUT" 2>"$ERRF"; RC=$?; want "inner exit 1 without output -> 1" 1
stub s_rc3 'echo boom >&2; exit 3';               OUT="$T/s3.out"; ERRF="$T/s3.err"; "$SW" --root "$T/cleanroot" >"$OUT" 2>"$ERRF"; RC=$?; want "inner scanner crash (rc 3) -> 2, never clean" 2
echo "SUMMARY pass=$PASSN fail=$FAILN"
[ "$FAILN" -eq 0 ]
