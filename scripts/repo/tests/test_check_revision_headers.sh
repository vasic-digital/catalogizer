#!/usr/bin/env bash
# T040 test (TDD): scripts/repo/check_revision_headers.sh - section 11.4.44 revision header check (new and edited Markdown files).
# Usage: bash scripts/repo/tests/test_check_revision_headers.sh   Env: H=<helper>
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
D0="$(pwd)"; H="${H:-scripts/repo/check_revision_headers.sh}"; case "$H" in /*) ;; *) H="$D0/$H" ;; esac
. "$D0/scripts/repo/tests/lib_wp04b.sh"
T="$(mktemp -d "${TMPDIR:-/tmp}/rh_test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
[ -x "$H" ] || echo "NOTE: $H is absent or not executable (RED state: every case must FAIL)"
R="$T/r"; mkrepo "$R"; mkdir -p "$R/docs/register" "$R/docs/other" "$R/docs/issues"
run() { ( cd "$R" && "$H" --root "$R" "$@" ) >"$T/out" 2>"$T/err"; RC=$?; }
w() { mkdir -p "$(dirname "$R/$1")"; printf '%b' "$2" > "$R/$1"; }
HT='# T\n\n| Field | Value |\n|---|---|\n| Revision | 3 |\n| Last modified | 2026-10-05 |\n\nbody\n'
w new_ok.md "$HT"
w bold_a.md '# T\n\n**Revision:** 3\n**Last modified:** 2026-10-05\n\nbody\n'
w bold_b.md '# T\n\n**Revision**: 3\n**Last modified**: 2026-10-05\n\nbody\n'
w no_header.md '# T\n\nbody\n'
w rev_only.md '# T\n\n| Revision | 3 |\n\nbody\n'
w plain_line.md '# T\n\nRevision: 3\nLast modified: x\n'
w deep.md "$(printf '# T\n'; for i in $(seq 1 45); do echo "line $i"; done; printf '| Revision | 3 |\n| Last modified | x |\n')\n"
w docs/register/Issues.md '# Issues\n\nno header here\n'
w docs/other/Issues.md '# Issues\n\nno header here\n'
w docs/issues/T-1.md 'ticket without header\n'
w ALL_ISSUES_FIXED.md 'legacy report without header\n'
w notmd.txt 'not markdown\n'
printf '%s\n' new_ok.md bold_a.md bold_b.md > "$T/good.lst"
run --files-from "$T/good.lst"; eq "header table and both bold forms pass: 0" "$RC" 0
run no_header.md;  eq "no header: 10" "$RC" 10; has "failing path named" "$(cat "$T/out")" "no_header.md"
run rev_only.md;   eq "Revision without Last modified: 10" "$RC" 10
run plain_line.md; eq "plain 'Revision:' lines are not the header form: 10" "$RC" 10
run deep.md;       eq "header beyond the first 40 lines: 10" "$RC" 10
run docs/register/Issues.md; eq "class generated (docs/register): 0" "$RC" 0
run docs/other/Issues.md;    eq "same file one directory away (class source): 10" "$RC" 10
run docs/issues/T-1.md;      eq "class legacy-collection (docs/issues ticket): 0" "$RC" 0
run ALL_ISSUES_FIXED.md;     eq "class legacy-collection (legacy root report): 0" "$RC" 0
run notmd.txt;               eq "a non-Markdown file is never judged: 0" "$RC" 0
run new_ok.md no_header.md;  eq "mixed list: 10" "$RC" 10; hasnot "passing file not named" "$(cat "$T/out")" "new_ok.md"
run missing_declared.md;     eq "declared but absent file (a deletion) is not judged: 0" "$RC" 0
# measure mode: baseline rows for every tracked Markdown file whose class applies the check, never a refusal
commit_all "$R" all
run --measure; eq "--measure exits 0" "$RC" 0
eq "--measure rows (no_header, rev_only, plain_line, deep, docs/other/Issues)" "$(grep -c '^revision_header	' "$T/out")" 5
has "--measure key shape" "$(cat "$T/out")" "revision_header	no_header.md	missing"
hasnot "--measure skips generated" "$(cat "$T/out")" "docs/register/Issues.md"
hasnot "--measure skips legacy-collection" "$(cat "$T/out")" "ALL_ISSUES_FIXED.md"
# refusals
printf '../escape.md\n' > "$T/bad.lst"; run --files-from "$T/bad.lst"; eq "path with .. : 20" "$RC" 20
printf '/abs.md\n' > "$T/bad.lst"; run --files-from "$T/bad.lst"; eq "absolute path: 20" "$RC" 20
printf -- '-rf.md\n' > "$T/bad.lst"; run --files-from "$T/bad.lst"; eq "path starting with a dash: 20" "$RC" 20
printf 'a\nb.md\n' > "$T/bad.lst"; run --files-from "$T/nonexistent.lst"; eq "unreadable --files-from: 20" "$RC" 20
run --bogus; eq "unknown option: 20" "$RC" 20
mkdir -p "$T/cwd"; mkrepo "$T/cwd/-x"; ( cd "$T/cwd" && "$H" --root -x ) >/dev/null 2>&1; eq "an existing repository named -x is refused as an option-like value: 20" "$?" 20

# (WF5 F3) --measure with a failing git listing is no "0 files measured": 20, named
mkdir -p "$T/fshim"; rg_="$(command -v git)"; printf '#!/bin/sh\ncase "$*" in *ls-files*) echo "fatal: shim" >&2; exit 128 ;; esac\nexec "%s" "$@"\n' "$rg_" > "$T/fshim/git"; chmod +x "$T/fshim/git"
PATH="$T/fshim:$PATH" run --measure; eq "--measure with git ls-files failing: 20 (F3)" "$RC" 20; has "named git_listing_failed" "$(cat "$T/err")" git_listing_failed

# (WF6 W6-5) a terminated --measure run leaves no temp file behind: SIGTERM during a slow listing
mkdir -p "$T/slowshim"; printf '#!/bin/sh\ncase "$*" in *ls-files*) sleep 2 ;; esac\nexec "%s" "$@"\n' "$rg_" > "$T/slowshim/git"; chmod +x "$T/slowshim/git"
rm -rf "$T/tmpd"; mkdir -p "$T/tmpd"; PATH="$T/slowshim:$PATH" TMPDIR="$T/tmpd" "$H" --root "$R" --measure >/dev/null 2>&1 & hp=$!
n=0; until [ -n "$(ls -A "$T/tmpd" 2>/dev/null)" ] || [ "$n" -ge 100 ]; do sleep 0.1; n=$((n+1)); done
eq "control: the listing temp file exists while the listing runs" "$([ -n "$(ls -A "$T/tmpd")" ] && echo yes || echo no)" yes
kill -TERM "$hp"; wait "$hp" 2>/dev/null
eq "no temp file left behind by the terminated --measure run (W6-5)" "$(ls -A "$T/tmpd" | tr '\n' ' ')" ""
fin
