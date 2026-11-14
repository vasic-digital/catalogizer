#!/usr/bin/env bash
# CAT-REL-CHECK-3 tests (constitution 11.4.224): snapshot extraction safety (I1, class: no content-derived write may
# resolve outside the snapshot), working-tree-independent existence (M1), reviewer-mutant killers (RM1, RM17),
# size caps and git timeouts (N2). Unit-class evidence: fixtures are input data for the checker (11.4.289(C)(3)).
# Also run by test_check_release_consistency.sh. CHECKER=<path> selects the checker under test.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHECKER="${CHECKER:-$HERE/check_release_consistency.sh}"
# shellcheck source=scripts/release/release_fixture.sh
. "$HERE/release_fixture.sh"
BASE="${RELCHK_TMP:-${XDG_CACHE_HOME:-${HOME:?}/.cache}/relchk-tmp}"
mkdir -p "$BASE" || exit 2
ROOT="$(mktemp -d "$BASE/rc4test.XXXXXX")" || exit 2
case "$ROOT" in "$BASE"/rc4test.*) ;; *) echo "refusing odd ROOT $ROOT" >&2; exit 2 ;; esac
trap 'rm -rf "${ROOT:?}"' EXIT
PASS=0; FAIL=0
ok()  { PASS=$((PASS+1)); }
bad() { FAIL=$((FAIL+1)); echo "  FAIL: $1"; }
GOLD="$ROOT/gold"; make_fixture "$GOLD" || { echo "fixture build failed" >&2; exit 2; }
fresh() { local t; t="$(mktemp -d "$ROOT/c.XXXXXX")" && cp -a "$GOLD/." "$t/" && echo "$t"; }
REALGIT="$(command -v git)"
# run <dir> [args]: checker on committed state only; output on stdout, rc in $RC (no auto-commit: states are built by hand)
run() { local dir="$1"; shift; OUT_="$("$CHECKER" --repo "$dir" "$@" 2>&1)"; RC=$?; }
# want <label> <rc> <needle>: asserts on the last run
want() {
  if [ "$RC" != "$2" ]; then bad "$1: rc=$RC want $2"; return; fi
  if [ -n "$3" ] && ! printf '%s\n' "$OUT_" | grep -qF -- "$3"; then bad "$1: output lacks '$3'"; return; fi
  printf '%s\n' "$OUT_" | grep -q '^VERDICT: ' || { bad "$1: no VERDICT line"; return; }
  ok
}
# rawtree <repo>: stdin lines "mode<TAB>name<TAB>sha" (name: python escapes allowed) -> sha of a raw tree object (no sorting,
# no name validation, like a hostile client could write with git hash-object --literally)
rawtree() { python3 -I -c '
import sys, subprocess
repo = sys.argv[1]; raw = b""
for ln in sys.stdin.read().split("\n"):
    if not ln:
        continue
    m, n, h = ln.split("\t")
    raw += m.encode() + b" " + n.encode().decode("unicode_escape").encode("latin1") + b"\0" + bytes.fromhex(h)
print(subprocess.run(["git", "-C", repo, "hash-object", "-t", "tree", "-w", "--literally", "--stdin"], input=raw, capture_output=True, check=True).stdout.decode().strip())
' "$1"; }
blob() { printf '%s' "$2" | git -C "$1" hash-object -w --stdin; }
# evil_head <repo>: stdin = extra root entries "mode<TAB>name<TAB>sha"; HEAD becomes a commit whose root tree is the old
# root plus those entries (appended in the given order), with the working tree untouched
evil_head() {
  local r="$1" base ex t c
  ex="$(cat)"
  base="$(git -C "$r" ls-tree HEAD | awk -F'\t' '{split($1,a," "); m=a[1]; sub(/^0/,"",m); printf "%s\t%s\t%s\n", m, $2, a[3]}')"
  t="$(printf '%s\n%s\n' "$base" "$ex" | rawtree "$r")" || return 1
  c="$(git -C "$r" commit-tree "$t" -p HEAD -m evil)" || return 1
  git -C "$r" update-ref HEAD "$c"
}
gitstub() { local sd; sd="$(mktemp -d "$ROOT/gstub.XXXXXX")"; printf '#!/bin/sh\n%s\nexec %s "$@"\n' "$1" "$REALGIT" > "$sd/git"; chmod +x "$sd/git"; echo "$sd"; }

echo "== I1: snapshot extraction must never write outside the snapshot (class, not instance)"
# esc <label> <extras-builder-function>: fresh repo, hostile HEAD, canary directory that must stay empty
canary_empty() { [ -z "$(ls -A "$1" 2>/dev/null)" ]; }
mkcase() { CASE_D="$(fresh)"; CASE_OUT="$(mktemp -d "$ROOT/outside.XXXXXX")"; CASE_TMP="$(mktemp -d "$ROOT/tmpbase.XXXXXX")"; }
runcase() { # <label> <guard message> ; extras on stdin ; expects BLIND rc 2 with THAT guard message (not merely the generic prefix:
  # an unexpected internal error also carries "cannot write the HEAD snapshot"), nothing written outside
  local label="$1" needle="$2"
  evil_head "$CASE_D" || { bad "$label: fixture"; return; }
  RELCHK_TMP="$CASE_TMP" run "$CASE_D" --check changelog
  want "$label -> BLIND" 2 "snapshot refused: $needle"
  if canary_empty "$CASE_OUT" && [ -z "$(ls -A "$CASE_TMP")" ]; then ok; else bad "$label: something was written outside the snapshot ($(ls -A "$CASE_OUT" "$CASE_TMP" | tr '\n' ' '))"; fi
}
# A1: the reviewer's reproduction: symlink l -> absolute dir, then tree l/ holding pwn
mkcase; L="$(blob "$CASE_D" "$CASE_OUT")"; P="$(blob "$CASE_D" PWNED)"; SUB="$(printf '100644\tpwn\t%s\n' "$P" | rawtree "$CASE_D")"
runcase "A1 symlink l then tree l/ (escape reproduction)" "a path is both an entry and a directory" < <(printf '120000\tl\t%s\n40000\tl\t%s\n' "$L" "$SUB")
mkcase; L="$(blob "$CASE_D" "$CASE_OUT")"; P="$(blob "$CASE_D" PWNED)"; SUB="$(printf '100644\tpwn\t%s\n' "$P" | rawtree "$CASE_D")"
runcase "A2 tree l/ then symlink l (other order)" "a path is both an entry and a directory" < <(printf '40000\tl\t%s\n120000\tl\t%s\n' "$SUB" "$L")
# A3: the same duplicate nested one level down
mkcase; L="$(blob "$CASE_D" "$CASE_OUT")"; P="$(blob "$CASE_D" PWNED)"; SUB="$(printf '100644\tpwn\t%s\n' "$P" | rawtree "$CASE_D")"
D2="$(printf '120000\ts\t%s\n40000\ts\t%s\n' "$L" "$SUB" | rawtree "$CASE_D")"
runcase "A3 nested duplicate link/tree under zz/" "a path is both an entry and a directory" < <(printf '40000\tzz\t%s\n' "$D2")
# A4: a symlink entry and a flat entry whose name goes THROUGH it (name with a slash), both orders
mkcase; L="$(blob "$CASE_D" "$CASE_OUT")"; P="$(blob "$CASE_D" PWNED)"
runcase "A4 symlink m then flat entry m/pwn" "a path is both an entry and a directory" < <(printf '120000\tm\t%s\n100644\tm/pwn\t%s\n' "$L" "$P")
mkcase; L="$(blob "$CASE_D" "$CASE_OUT")"; P="$(blob "$CASE_D" PWNED)"
runcase "A5 flat entry m/pwn then symlink m" "a path is both an entry and a directory" < <(printf '100644\tm/pwn\t%s\n120000\tm\t%s\n' "$P" "$L")
# A6: duplicate regular entries
mkcase; P="$(blob "$CASE_D" one)"; Q="$(blob "$CASE_D" two)"
runcase "A6 two blobs with one name" "duplicate tree entry" < <(printf '100644\tdup.txt\t%s\n100644\tdup.txt\t%s\n' "$P" "$Q")
# A7: gitlink and tree with one name, and gitlink plus a flat entry below it
mkcase; P="$(blob "$CASE_D" one)"; H="$(git -C "$CASE_D" rev-parse HEAD)"; SUB="$(printf '100644\tf\t%s\n' "$P" | rawtree "$CASE_D")"
runcase "A7 gitlink g and tree g/" "a path is both an entry and a directory" < <(printf '160000\tg\t%s\n40000\tg\t%s\n' "$H" "$SUB")
mkcase; P="$(blob "$CASE_D" one)"; H="$(git -C "$CASE_D" rev-parse HEAD)"
runcase "A8 gitlink g and flat entry g/f" "a path is both an entry and a directory" < <(printf '160000\tg\t%s\n100644\tg/f\t%s\n' "$H" "$P")
# A9: ordinary directory/file clash (file x and tree x/)
mkcase; P="$(blob "$CASE_D" one)"; SUB="$(printf '100644\tf\t%s\n' "$P" | rawtree "$CASE_D")"
runcase "A9 file x and tree x/" "a path is both an entry and a directory" < <(printf '100644\tx\t%s\n40000\tx\t%s\n' "$P" "$SUB")

echo "== reserved and hostile components (RM1: .git guard)"
# B1: .git/info/exclude in HEAD naming a manifest entry that is missing (RM1: without the guard, the exclude file hides it -> PASS)
d=$(fresh); echo "$(printf g | sha256sum | cut -d' ' -f1)  gone.txt" >> "$d/docs/qa/run2/SHA256SUMS"; ( cd "$d" && git add -A && git commit -q -m gone )
EX="$(blob "$d" $'gone.txt\n')"; INFO="$(printf '100644\texclude\t%s\n' "$EX" | rawtree "$d")"; GIT_T="$(printf '40000\tinfo\t%s\n' "$INFO" | rawtree "$d")"
printf '40000\t.git\t%s\n' "$GIT_T" | evil_head "$d"; RELCHK_TMP="$ROOT" run "$d" --check changelog
want "B1 HEAD holds .git/info/exclude naming the missing entry -> BLIND, not PASS (RM1)" 2 "cannot write the HEAD snapshot"
d=$(fresh); EX="$(blob "$d" x)"; for nm in .git .GIT .Git; do
  T="$(printf '100644\tconfig\t%s\n' "$EX" | rawtree "$d")"; 
  e=$(fresh); printf '40000\t%s\t%s\n' "$nm" "$T" | evil_head "$e"; run "$e" --check changelog
  want "B2 root component $nm -> BLIND" 2 "cannot write the HEAD snapshot"
done
e=$(fresh); EX="$(blob "$e" x)"; printf '100644\tdocs/.git/x\t%s\n' "$EX" | evil_head "$e"; run "$e" --check changelog
want "B3 nested component docs/.git/x -> BLIND" 2 "cannot write the HEAD snapshot"
e=$(fresh); EX="$(blob "$e" x)"; printf '100644\tdocs/.GIT\t%s\n' "$EX" | evil_head "$e"; run "$e" --check changelog
want "B3b nested file component .GIT -> BLIND" 2 "cannot write the HEAD snapshot"
# B4: dot-dot and dot components, absolute and empty names, newline in a name; nothing may appear next to the snapshot
for nm in '../pwn_dd' 'docs/../pwn_dd2' './pwn_dot' '/abs_pwn' 'a//b'; do
  e=$(fresh); tb="$(mktemp -d "$ROOT/tb.XXXXXX")"; EX="$(blob "$e" x)"
  printf '100644\t%s\t%s\n' "$nm" "$EX" | evil_head "$e"; RELCHK_TMP="$tb" run "$e" --check changelog
  want "B4 hostile name '$nm' -> BLIND" 2 "cannot write the HEAD snapshot"
  if [ -z "$(ls -A "$tb")" ] && [ ! -e "$ROOT/pwn_dd" ] && [ ! -e "$ROOT/pwn_dd2" ] && [ ! -e "$ROOT/pwn_dot" ]; then ok; else bad "B4 '$nm': a file appeared outside the snapshot"; fi
done
e=$(fresh); EX="$(blob "$e" x)"; T="$(printf '100644\tpwn_sub\t%s\n' "$EX" | rawtree "$e")"; tb="$(mktemp -d "$ROOT/tb.XXXXXX")"
printf '40000\t..\t%s\n' "$T" | evil_head "$e"; RELCHK_TMP="$tb" run "$e" --check changelog
want "B5 subtree named .. -> BLIND" 2 "cannot write the HEAD snapshot"
if [ -z "$(ls -A "$tb")" ]; then ok; else bad "B5: a file appeared next to the snapshot"; fi
# B6: names that differ only in case are two distinct files on this filesystem: no escape, no snapshot refusal
e=$(fresh); P="$(blob "$e" one)"; Q="$(blob "$e" two)"; printf '100644\tCASE.txt\t%s\n100644\tcase.txt\t%s\n' "$P" "$Q" | evil_head "$e"; run "$e" --check version
if printf '%s\n' "$OUT_" | grep -qF "cannot write the HEAD snapshot"; then bad "B6 case-variant siblings must not be refused as a snapshot error"; else ok; fi

# B7: a newline inside a name is a legal tracked name (the changelog helper escapes it): the snapshot is written, nothing escapes
e=$(fresh); tb="$(mktemp -d "$ROOT/tb.XXXXXX")"; EX="$(blob "$e" x)"; printf '100644\ta\\nb\t%s\n' "$EX" | evil_head "$e"; RELCHK_TMP="$tb" run "$e" --check version
if printf '%s\n' "$OUT_" | grep -qF "cannot write the HEAD snapshot"; then bad "B7 newline in a name must not be refused"; else ok; fi

echo "== M1: existence of a cited path comes from HEAD's snapshot, not from the working tree"
mkother() { mkdir -p "$1/docs/other"; echo x > "$1/docs/other/z.txt"; }
addcite() { sed -i "s#^- Tool \`scripts/tool.sh\`#- Extra \`$2\`; Tool \`scripts/tool.sh\`#" "$1/docs/CHANGELOG.md"; }
cmp_clone() { # <label> <dir> ... : local rc equals the plain clone's rc
  local label="$1" d="$2"; shift 2; local lrc crc
  "$CHECKER" --repo "$d" "$@" >/dev/null 2>&1; lrc=$?
  rm -rf "${d:?}.clone"; git clone -q "$d" "$d.clone" >/dev/null 2>&1; "$CHECKER" --repo "$d.clone" "$@" >/dev/null 2>&1; crc=$?
  if [ "$lrc" = "$crc" ]; then ok; else bad "$label: local rc=$lrc, plain clone rc=$crc"; fi
}
# M1a: file in HEAD, deleted from the working tree (uncommitted): clone PASSes, so does the checker
d=$(fresh); mkother "$d"; addcite "$d" docs/other/z.txt; ( cd "$d" && git add -A && git commit -q -m cl ); rm "$d/docs/other/z.txt"
run "$d" --check changelog; want "M1a HEAD file missing in the working tree -> PASS like a clone" 0 "OK    docs/other/z.txt"
cmp_clone "M1a clone parity" "$d" --check changelog
# M1b: sparse-like state (skip-worktree and removed): the same
d=$(fresh); mkother "$d"; addcite "$d" docs/other/z.txt; ( cd "$d" && git add -A && git commit -q -m cl && git update-index --skip-worktree docs/other/z.txt ); rm "$d/docs/other/z.txt"
run "$d" --check changelog; want "M1b skip-worktree + removed locally -> PASS like a clone" 0 "OK    docs/other/z.txt"
cmp_clone "M1b clone parity" "$d" --check changelog
# M1c: the cited path is a dangling symlink in HEAD: MISS (a clone fails too), whatever the working tree holds
d=$(fresh); mkother "$d"; ln -s no_such_target "$d/docs/other/dang"; addcite "$d" docs/other/dang; ( cd "$d" && git add -A && git commit -q -m cl )
run "$d" --check changelog; want "M1c dangling HEAD symlink -> MISS" 1 "MISS  docs/other/dang (not in HEAD)"
cmp_clone "M1c clone parity" "$d" --check changelog
# M1d: HEAD lacks the path but the working tree has it (untracked): MISS, as in a clone
d=$(fresh); mkother "$d"; addcite "$d" docs/other/z.txt; ( cd "$d" && git add docs/CHANGELOG.md && git commit -q -m cl )
run "$d" --check changelog; want "M1d untracked-only path -> MISS (a clone lacks it)" 1 "MISS  docs/other/z.txt (not in HEAD)"
cmp_clone "M1d clone parity" "$d" --check changelog

echo "== RM17: only the committed .gitignore decides; XDG default ignore file must not count"
d=$(fresh); echo "$(printf g | sha256sum | cut -d' ' -f1)  gone.txt" >> "$d/docs/qa/run2/SHA256SUMS"; ( cd "$d" && git add -A && git commit -q -m gone )
xd="$(mktemp -d "$ROOT/xdg.XXXXXX")"; mkdir -p "$xd/git"; echo gone.txt > "$xd/git/ignore"
XDG_CONFIG_HOME="$xd" run "$d" --check changelog
want "RM17 XDG_CONFIG_HOME/git/ignore names the missing entry -> still SHAFAIL" 1 "docs/qa/run2/gone.txt (missing/unreadable, not gitignored)"
# control: the same ignore file is honoured by plain git in the working tree (so the test would see it if it leaked)
( cd "$d" && mkdir -p ctl && : > ctl/gone.txt && cd ctl && XDG_CONFIG_HOME="$xd" GIT_CONFIG_GLOBAL=/dev/null git init -q . && XDG_CONFIG_HOME="$xd" GIT_CONFIG_GLOBAL=/dev/null git check-ignore -q gone.txt ) \
  && ok || bad "RM17 control: the XDG ignore file is not read by git here, the test above proves nothing"

echo "== N2: size caps and git timeouts end BLIND, never hang or pass"
d=$(fresh); RELCHK_MAX_BYTES=10 run "$d" --check changelog; want "N2a byte cap exceeded -> BLIND" 2 "more than 10 bytes"
d=$(fresh); RELCHK_MAX_FILES=3 run "$d" --check changelog; want "N2b entry cap exceeded -> BLIND" 2 "more than 3 tree entries"
d=$(fresh); RELCHK_MAX_BYTES=100000000 RELCHK_MAX_FILES=100000 run "$d" --check changelog; want "N2c generous caps -> PASS" 0 "VERDICT: PASS"
sd=$(gitstub 'case "$*" in *"ls-tree -r -z --full-tree"*) exec sleep 25;; esac'); t0=$SECONDS
PATH="$sd:$PATH" RELCHK_GIT_TIMEOUT=2 run "$(fresh)" --check changelog
want "N2d hung ls-tree in the snapshot writer -> BLIND" 2 "timed out"
[ $((SECONDS - t0)) -lt 20 ] && ok || bad "N2d took $((SECONDS - t0))s: the timeout did not bound the run"
sd=$(gitstub 'case "$*" in *" status "*) exec sleep 25;; esac'); t0=$SECONDS
PATH="$sd:$PATH" RELCHK_GIT_TIMEOUT=2 run "$(fresh)" --check changelog
want "N2e hung git status -> BLIND, bounded" 2 ""
[ $((SECONDS - t0)) -lt 40 ] && ok || bad "N2e took $((SECONDS - t0))s"
sd=$(gitstub 'case "$*" in *"ls-tree -r -z --name-only --full-tree HEAD"*) exec sleep 80;; esac'); t0=$SECONDS   # longer than the 40 s bound: an unbounded call fails the bound
PATH="$sd:$PATH" RELCHK_GIT_TIMEOUT=2 run "$(fresh)" --check changelog
want "N2f hung ls-tree in the changelog helper -> BLIND, bounded" 2 ""
[ $((SECONDS - t0)) -lt 40 ] && ok || bad "N2f took $((SECONDS - t0))s"

sd=$(gitstub 'case "$*" in *" cat-file --batch"*) exec sleep 25;; esac'); t0=$SECONDS
PATH="$sd:$PATH" RELCHK_SNAP_TIMEOUT=2 run "$(fresh)" --check changelog
want "N2g hung cat-file in the snapshot writer -> BLIND at the writer's own time limit" 2 "time limit"
[ $((SECONDS - t0)) -lt 20 ] && ok || bad "N2g took $((SECONDS - t0))s"

echo "== F2: every reader of snapshot content refuses non-regular targets, targets outside the snapshot and oversize files (memory-capped)"
# runcap: the checker under a 1 GiB address-space cap and a wall-clock bound; an unbounded read fails by MemoryError or timeout, never by luck
runcap() { local dir="$1"; shift; OUT_="$(prlimit --as=$((1 << 30)) -- timeout 60 "$CHECKER" --repo "$dir" "$@" 2>&1)"; RC=$?; }
want_refused() { # <label> : not a PASS, not a hang (rc 124/137), a VERDICT line, no traceback, no MemoryError
  if [ "$RC" = 0 ] || [ "$RC" = 124 ] || [ "$RC" = 137 ]; then bad "$1: rc=$RC"; return; fi
  printf '%s\n' "$OUT_" | grep -q '^VERDICT: ' || { bad "$1: no VERDICT line (rc=$RC)"; return; }
  if printf '%s\n' "$OUT_" | grep -qE 'Traceback|MemoryError'; then bad "$1: traceback or MemoryError in output"; return; fi
  ok
}
SPARSE="$ROOT/huge_sparse"; truncate -s 100G "$SPARSE"; FIFO="$ROOT/fifo_outside"; mkfifo "$FIFO"
lnk() { # <label> <repo-relative file> <symlink target> <message the NEW guard prints> [check args...]
  # the message is what discriminates: the pre-fix checker (4369773168) also ended BLIND/FAIL on some of these through other layers,
  # but never said this
  local label="$1" f="$2" tgt="$3" needle="$4"; shift 4
  local d; d=$(fresh); ( cd "$d" && rm -f -- "$f" && ln -s "$tgt" "$f" && git add -A && git commit -q -m lnk )
  runcap "$d" "$@"; want_refused "$label"
  printf '%s\n' "$OUT_" | grep -qF -- "$needle" && ok || bad "$label: output lacks the guard message '$needle'"
}
lnk "F2a versions.json -> /dev/zero" versions.json /dev/zero "resolves outside the snapshot"
lnk "F2b versions.json -> FIFO outside" versions.json "$FIFO" "resolves outside the snapshot"
lnk "F2c versions.json -> 100 GiB sparse file outside" versions.json "$SPARSE" "resolves outside the snapshot"
lnk "F2d CONTROL (the earlier [ -f ] layer already refuses it; not a discriminator vs 4369773168): docs/CHANGELOG.md -> /dev/zero" docs/CHANGELOG.md /dev/zero "VERDICT:" --check changelog
lnk "F2e docs/CHANGELOG.md -> sparse outside" docs/CHANGELOG.md "$SPARSE" "resolves outside the snapshot" --check changelog
lnk "F2f CONTROL (the earlier [ -f ] layer already refuses it; not a discriminator vs 4369773168): docs/qa/run2/SHA256SUMS -> /dev/zero" docs/qa/run2/SHA256SUMS /dev/zero "VERDICT:" --check changelog
lnk "F2g docs/qa/run2/SHA256SUMS -> sparse outside" docs/qa/run2/SHA256SUMS "$SPARSE" "not a regular file inside the snapshot" --check changelog
lnk "F2h CONTROL (the earlier [ -f ] layer already refuses it; not a discriminator vs 4369773168): docs/qa/run2/SHA256SUMS -> FIFO outside" docs/qa/run2/SHA256SUMS "$FIFO" "VERDICT:" --check changelog
OUTC="$ROOT/outside_changelog.md"; cp "$GOLD/docs/CHANGELOG.md" "$OUTC"
lnk "F2o docs/CHANGELOG.md -> small regular file outside the snapshot (valid changelog bytes)" docs/CHANGELOG.md "$OUTC" "resolves outside the snapshot" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa"; ( cd "$d" && ln -s /dev/zero docs/qa/EVIDENCE_WAIVERS.txt && git add -A && git commit -q -m w ); runcap "$d" --check changelog; want_refused "F2i CONTROL waiver file -> /dev/zero (the earlier [ -f ] layer already refuses it; not a discriminator vs 4369773168)"
d=$(fresh); ( cd "$d" && ln -s "$SPARSE" docs/qa/EVIDENCE_WAIVERS.txt && git add -A && git commit -q -m w ); runcap "$d" --check changelog; want_refused "F2j waiver file -> sparse outside"
printf '%s\n' "$OUT_" | grep -qF "not a regular file inside the snapshot" && ok || bad "F2j: output lacks the waiver guard message"
# F2n: a small REGULAR file outside the snapshot, reached through an absolute HEAD symlink, is never read (it holds valid versions.json bytes)
OUTV="$ROOT/outside_versions.json"; cp "$GOLD/versions.json" "$OUTV"
lnk "F2n versions.json -> small regular file outside the snapshot" versions.json "$OUTV" "resolves outside the snapshot"
d=$(fresh); RELCHK_MAX_READ=10 runcap "$d"; want_refused "F2k a regular snapshot file above RELCHK_MAX_READ is refused"
d=$(fresh); RELCHK_MAX_READ=10 runcap "$d" --check changelog; want_refused "F2l changelog above RELCHK_MAX_READ is refused"
d=$(fresh); runcap "$d" --check changelog; want "F2m negative control: the same fixture under the same cap, unmodified -> PASS" 0 "VERDICT: PASS"
d=$(fresh); RELCHK_MAX_READ=notanumber runcap "$d" --check changelog; want "F6a bad RELCHK_MAX_READ -> clear BLIND" 2 "RELCHK_MAX_READ must be a positive integer"
d=$(fresh); RELCHK_SNAP_TIMEOUT=abc runcap "$d" --check changelog; want "F6b bad RELCHK_SNAP_TIMEOUT -> clear BLIND" 2 "RELCHK_SNAP_TIMEOUT must be a positive integer"
d=$(fresh); RELCHK_MAX_FILES=-5 runcap "$d" --check changelog; want "F6c bad RELCHK_MAX_FILES -> clear BLIND" 2 "RELCHK_MAX_FILES must be a positive integer"
# F6d: a NUL byte in a symlink target: a clean refusal, never a traceback
d=$(fresh); NB="$(printf 'a\0b' | git -C "$d" hash-object -w --stdin)"; printf '120000\tnul_link\t%s\n' "$NB" | evil_head "$d"
run "$d" --check changelog; want "F6d NUL byte in a symlink target -> BLIND" 2 "cannot write the HEAD snapshot"
if printf '%s\n' "$OUT_" | grep -q Traceback; then bad "F6d: traceback in output"; else ok; fi

# F6e: an unexpected exception in the writer (a 300-byte file name: ENAMETOOLONG) is a clean refusal, never a traceback
d=$(fresh); LN="$(head -c 300 /dev/zero | tr '\0' 'x')"; EX="$(blob "$d" x)"; printf '100644\t%s\t%s\n' "$LN" "$EX" | evil_head "$d"
run "$d" --check changelog; want "F6e unexpected writer exception -> BLIND with a reason" 2 "snapshot refused: internal error in the snapshot writer"
if printf '%s\n' "$OUT_" | grep -q Traceback; then bad "F6e: traceback in output"; else ok; fi

echo "== F4a/F3: the post-extraction verification layer is reachable (path beyond PATH_MAX) and hung children do not outlive the writer"
d=$(fresh); prev="$(blob "$d" leaf)"; for i in $(seq 1 20); do
  nm="$(printf 'n%02d' "$i")$(head -c 246 /dev/zero | tr '\0' 'a')"
  if [ "$i" = 1 ]; then prev="$(printf '100644\t%s\t%s\n' "$nm" "$prev" | rawtree "$d")"; else prev="$(printf '40000\t%s\t%s\n' "$nm" "$prev" | rawtree "$d")"; fi
done
printf '40000\tdeep\t%s\n' "$prev" | evil_head "$d"; run "$d" --check changelog
want "D1 20 levels of 249-char names (beyond PATH_MAX) -> BLIND from the extraction verification" 2 "snapshot refused: extracted entry"
sd=$(gitstub 'case "$*" in *" cat-file --batch"*) exec sleep 27;; esac'); t0=$SECONDS
PATH="$sd:$PATH" RELCHK_SNAP_TIMEOUT=2 run "$(fresh)" --check changelog
sleep 1; n="$(pgrep -fc '^sleep 27$' || true)"
[ "${n:-0}" = 0 ] && ok || { bad "D2 hung cat-file child outlived the writer ($n orphan sleep processes)"; pgrep -f '^sleep 27$' | xargs -r kill 2>/dev/null || true; }

echo "== F5: a failing or hung ls-tree in raw_tree_check is BLIND, never an empty (clean) tree"
sd=$(gitstub 'case "$*" in *"ls-tree -r -z HEAD -- "*) exec sleep 25;; esac'); t0=$SECONDS
PATH="$sd:$PATH" RELCHK_GIT_TIMEOUT=2 run "$(fresh)"
want "F5a hung ls-tree -r in raw_tree_check -> BLIND" 2 "ls-tree -r failed or timed out"
[ $((SECONDS - t0)) -lt 60 ] && ok || bad "F5a took $((SECONDS - t0))s"
sd=$(gitstub 'case "$*" in *"ls-tree -r -z HEAD -- "*) exit 3;; esac')
PATH="$sd:$PATH" run "$(fresh)"
want "F5b failing ls-tree -r in raw_tree_check -> BLIND" 2 "ls-tree -r failed or timed out"

echo "== R2-2: sibling-directory prefix (RELCHK_SNAP_DIR=.../snapx next to .../snapx2) and in-snapshot oversize manifest / waiver"
sibcase() { # <label> <repo-relative file> <content> <needle> [check args...]: HEAD symlink to a file in a SIBLING of the snapshot dir
  local label="$1" f="$2" content="$3" needle="$4"; shift 4
  local base d sn; base="$(mktemp -d "$ROOT/sib.XXXXXX")"; sn="$base/snapx"; mkdir "$base/snapx2"
  printf '%s' "$content" > "$base/snapx2/target"
  d=$(fresh); mkdir -p "$d/$(dirname "$f")"; ( cd "$d" && rm -f -- "$f" && ln -s "$base/snapx2/target" "$f" && git add -A && git commit -q -m sib )
  RELCHK_SNAP_DIR="$sn" runcap "$d" "$@"
  want_refused "$label"
  printf '%s\n' "$OUT_" | grep -qF -- "$needle" && ok || bad "$label: output lacks '$needle'"
}
sibcase "S1 python reader: changelog -> file in a sibling dir sharing the snapshot prefix" docs/CHANGELOG.md "$(cat "$GOLD/docs/CHANGELOG.md")" "resolves outside the snapshot" --check changelog
sibcase "S2 bash reader: waiver file -> file in a sibling dir sharing the snapshot prefix" docs/qa/EVIDENCE_WAIVERS.txt "# none
" "not a regular file inside the snapshot" --check changelog
sibcase "S3 bash reader: manifest -> file in a sibling dir sharing the snapshot prefix" docs/qa/run2/SHA256SUMS "# none
" "not a regular file inside the snapshot" --check changelog
big() { head -c 300000 /dev/zero | tr '\0' 'x' | sed 's/^/# /' | fold -w 100; }
d=$(fresh); { cat "$d/docs/qa/run2/SHA256SUMS"; big | sed 's/^/# /'; } > "$d/m.new"; mv "$d/m.new" "$d/docs/qa/run2/SHA256SUMS"; ( cd "$d" && git add -A && git commit -q -m bigm )
RELCHK_MAX_READ=200000 runcap "$d" --check changelog; want_refused "S4 manifest above RELCHK_MAX_READ inside the snapshot"
printf '%s\n' "$OUT_" | grep -qF "exceeds RELCHK_MAX_READ" && ok || bad "S4: output lacks the manifest guard message"
d=$(fresh); mkdir -p "$d/docs/qa"; big | sed 's/^/# /' > "$d/docs/qa/EVIDENCE_WAIVERS.txt"; ( cd "$d" && git add -A && git commit -q -m bigw )
RELCHK_MAX_READ=200000 runcap "$d" --check changelog; want_refused "S5 waiver file above RELCHK_MAX_READ inside the snapshot"
printf '%s\n' "$OUT_" | grep -qF "exceeds RELCHK_MAX_READ" && ok || bad "S5: output lacks the waiver guard message"
d=$(fresh); RELCHK_MAX_READ=200000 runcap "$d" --check changelog; want "S6 control: same cap, unpadded fixture -> PASS" 0 "VERDICT: PASS"

echo "== R2-3/R2-5/R2-6: no scratch left behind on an early return; every numeric knob is validated"
d=$(fresh); H="$(git -C "$d" rev-parse HEAD)"; mkdir "$d/docs/qa/run2/gl"; ( cd "$d" && git update-index --add --cacheinfo "160000,$H,docs/qa/run2/gl" && git commit -q -m gl )
tb="$(mktemp -d "$ROOT/leak.XXXXXX")"; RELCHK_TMP="$tb" run "$d" --check changelog
want "R2-3a gitlink in a manifest tree -> BLIND (early return of raw_tree_check)" 2 "contains the submodule"
[ -z "$(ls -A "$tb")" ] && ok || bad "R2-3b scratch left behind in RELCHK_TMP: $(ls -A "$tb" | tr '\n' ' ')"
for v in abc 0 007 -3; do d=$(fresh); RELCHK_GIT_TIMEOUT=$v run "$d" --check changelog; want "R2-5 RELCHK_GIT_TIMEOUT=$v -> clear BLIND" 2 "RELCHK_GIT_TIMEOUT must be a positive integer"; done
d=$(fresh); RELCHK_GIT_TIMEOUT= run "$d" --check changelog; want "R2-5 empty RELCHK_GIT_TIMEOUT -> default, PASS" 0 "VERDICT: PASS"
d=$(fresh); RELCHK_MAX_READ=99999999999999999999 run "$d" --check changelog; want "R2-6 absurdly long RELCHK_MAX_READ -> clear BLIND" 2 "RELCHK_MAX_READ is too large"

echo "== valid snapshot shapes still work (negative controls for the new refusals)"
d=$(fresh); mkdir -p "$d/docs/other/sub"; echo x > "$d/docs/other/sub/f.txt"; ln -s sub/f.txt "$d/docs/other/lnk"; ln -s sub "$d/docs/other/dlnk"; chmod +x "$d/docs/other/sub/f.txt"
addcite "$d" docs/other/dlnk/f.txt; ( cd "$d" && git add -A && git commit -q -m shapes )
run "$d" --check changelog; want "G1 symlink to file, symlink to dir, executable file, nested dirs -> PASS" 0 "OK    docs/other/dlnk/f.txt"
echo "== result: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
