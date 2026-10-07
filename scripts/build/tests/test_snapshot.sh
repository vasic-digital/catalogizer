#!/usr/bin/env bash
# test_snapshot.sh - T005a (u) (u2) (w) cases for the git-plumbing source snapshot, the input closure, the tar writer and the build-host tree cache
# (scripts/build/lib/snapshot.py driver side, scripts/build/lib/treecache.py build-host side). Host control-plane test (P0-P1 host exception, as T003):
# every fixture is a REAL git repository (root, initialised submodules, a nested submodule, an uninitialised one) in a private temp dir.
# Usage: test_snapshot.sh   |   ONLY="p1 p5" test_snapshot.sh      SNAPSHOT_PY / TREECACHE_PY override the files under test (mutation runs).
set -u
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
SNAP=${SNAPSHOT_PY:-$here/../lib/snapshot.py}
TC=${TREECACHE_PY:-$here/../lib/treecache.py}
[ -f "$SNAP" ] && [ -f "$TC" ] || { echo "FAIL script absent: $SNAP or $TC"; echo "RESULT pass=0 fail=1 skip=0"; exit 1; }
for t in git python3 jq tar sha256sum; do command -v "$t" >/dev/null 2>&1 || { echo "FAIL dependency missing: $t"; echo "RESULT pass=0 fail=1 skip=0"; exit 1; }; done
pass=0; fail=0
ok()  { pass=$((pass+1)); echo "ok   $1"; }
bad() { fail=$((fail+1)); echo "FAIL $1${2:+ -- $2}"; }
chk() { local w=$1; shift; if "$@"; then ok "$w"; else bad "$w"; fi; }
want() { [ -z "${ONLY:-}" ] && return 0; case " $ONLY " in *" $1 "*) return 0;; esac; return 1; }
tmp=$(mktemp -d "${TMPDIR:-/tmp}/tsnap.XXXXXX"); trap 'rm -rf "$tmp"' EXIT
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
G() { git -c protocol.file.allow=always -c user.name=t -c user.email=t@t "$@"; }
S() { python3 -I "$SNAP" "$@"; }
EVP=specs/F/evidence; AUDP=specs/F/audit

mkrepo() { # dir : an empty repository with one commit
  mkdir -p "$1"; ( cd "$1" && G init -q -b main . && printf 'seed\n' > seed.txt && G add -A && G commit -qm seed ); }
# the fixture: root + submodules/lib (initialised, with a NESTED submodule deep/inner) + submodules/other + submodules/uninit (deinitialised)
fixture() { # name : creates $F (root), prints nothing
  F="$tmp/$1/root"; local up="$tmp/$1/up"; mkdir -p "$tmp/$1"
  mkrepo "$up/inner"; mkrepo "$up/lib"; mkrepo "$up/other"; mkrepo "$up/uninit"
  ( cd "$up/lib" && mkdir -p deep && G submodule add -q "$up/inner" deep/inner && printf 'lib code\n' > lib.go && G add -A && G commit -qm lib )
  mkrepo "$F"
  ( cd "$F" && mkdir -p svc img web "$EVP" "$AUDP" submodules
    printf 'module x\n\nrequire example.com/lib v0.0.0\n\nreplace example.com/lib => ../submodules/lib\n' > svc/go.mod
    printf 'package main\n' > svc/main.go
    printf 'FROM scratch\nCOPY submodules/other/data.txt /d\nCOPY web/index.js /w\n' > img/Containerfile
    printf '{"name":"w","dependencies":{"l":"file:../submodules/lib"}}\n' > web/package.json
    printf 'noclosure\n' > web/index.js
    printf 'big\n' > data.bin; printf '*.bin filter=lfs diff=lfs merge=lfs -text\nignored.txt export-ignore\n' > .gitattributes; printf 'kept\n' > ignored.txt
    printf '/scratch/\n' > .gitignore; printf 'ev1\n' > "$EVP/e.txt"; printf 'au1\n' > "$AUDP/a.txt"
    ln -s seed.txt link; chmod +x svc/main.go
    G submodule add -q "$up/lib" submodules/lib; G submodule add -q "$up/other" submodules/other; G submodule add -q "$up/uninit" submodules/uninit
    G submodule update -q --init --recursive; G submodule deinit -q -f submodules/uninit
    G add -A && G commit -qm fixture ); }
SN() { S "$@" --root "$F" --exclude "$EVP" --exclude "$AUDP"; }
dg() { SN digest "$@"; }
realidx() { sha256sum "$F/.git/index" | cut -d' ' -f1; }

# ============ p1: the digest is a function of the content
if want p1; then fixture p1
  a=$(dg); b=$(dg)
  chk "p1 digest is 64 hex and stable across calls" test "${#a}" = 64 -a "$a" = "$b"
  printf 'edit1\n' >> "$F/svc/main.go"; c=$(dg); git -C "$F" checkout -q svc/main.go 2>/dev/null; printf 'edit2\n' >> "$F/svc/main.go"; d=$(dg)
  chk "p1 two different uncommitted edits on one HEAD give two digests" test "$c" != "$d" -a "$c" != "$a" -a "$d" != "$a"
  git -C "$F" checkout -q svc/main.go; chmod +x "$F/svc/main.go"
  chk "p1 reverting the edit returns the original digest" test "$(dg)" = "$a"
  printf 'new\n' > "$F/svc/new_untracked.go"; chk "p1 an untracked non-ignored file changes the digest (TIC: nothing silently left out)" test "$(dg)" != "$a"
  rm "$F/svc/new_untracked.go"; mkdir -p "$F/scratch"; printf 'x\n' > "$F/scratch/i.txt"; chk "p1 an ignored file does not change the digest" test "$(dg)" = "$a"
  rm -f "$F/svc/go.mod"; chk "p1 a deleted tracked file changes the digest" test "$(dg)" != "$a"
fi
# ============ p2: submodules, at every depth
if want p2; then fixture p2
  a=$(dg); printf 'dirty\n' >> "$F/submodules/lib/lib.go"; b=$(dg)
  chk "p2 an edit inside a submodule's working tree changes the digest (the HEAD of the root is unchanged)" test "$a" != "$b"
  git -C "$F/submodules/lib" checkout -q lib.go; printf 'deep\n' > "$F/submodules/lib/deep/inner/new.txt"; c=$(dg)
  chk "p2 an untracked file in a NESTED submodule (depth 2) changes the digest" test "$c" != "$a"
  m=$(SN manifest | jq -r '[.repos[].path] | join(",")')
  chk "p2 the manifest lists the root and every INITIALISED submodule at every depth, in path order, and not the deinitialised one" test "$m" = ".,submodules/lib,submodules/lib/deep/inner,submodules/other"
fi
# ============ p3: the snapshot equals git's own tree, never touches the real index, never runs a filter
if want p3; then fixture p3
  printf 'wip\n' >> "$F/svc/main.go"; printf 'unt\n' > "$F/svc/u.go"; i0=$(realidx); st0=$(git -C "$F" status --porcelain=v1 | sha256sum)
  root=$(S manifest --root "$F" --class full | jq -r '.repos[0].tree')
  # independent oracle: the same worktree committed by git add -A in a scratch clone of the directory (filters disabled the same way)
  cp -a "$F" "$tmp/p3/oracle"; ( cd "$tmp/p3/oracle" && G -c filter.lfs.clean=cat -c filter.lfs.smudge=cat -c filter.lfs.process= -c filter.lfs.required=false add -A >/dev/null 2>&1 )
  want=$(git -C "$tmp/p3/oracle" write-tree)
  chk "p3 the root tree id equals git add -A + write-tree (full class)" test "$root" = "$want"
  chk "p3 the real index is byte-identical after the snapshot" test "$(realidx)" = "$i0"
  chk "p3 git status of the real checkout is unchanged" test "$(git -C "$F" status --porcelain=v1 | sha256sum)" = "$st0"
fi
# ============ p4: LFS clean filter never runs (control needle: the same shim does write its marker when git add runs it)
if want p4; then fixture p4
  printf '#!/bin/sh\ntouch %s/marker\ncat\n' "$tmp/p4" > "$tmp/p4/clean.sh"; chmod +x "$tmp/p4/clean.sh"
  git -C "$F" config filter.lfs.clean "$tmp/p4/clean.sh"; git -C "$F" config filter.lfs.required false
  printf 'BIG\n' > "$F/data.bin"; rm -f "$tmp/p4/marker"   # same size as the committed 'big', different content: git must COMPARE content, which is where a clean filter would run
  m=$(S manifest --root "$F" --class full); id=$(git -C "$F" hash-object --no-filters data.bin)
  chk "p4 the snapshot of a file under a filter=lfs pattern ran NO clean filter (marker absent)" test ! -e "$tmp/p4/marker"
  chk "p4 the file's blob in the tree is the raw (no-filter) blob" test "$(git -C "$F" ls-tree -r "$(jq -r '.repos[0].tree' <<<"$m")" -- data.bin | awk '{print $3}')" = "$id"
  cp -a "$F" "$tmp/p4/c2"; ( cd "$tmp/p4/c2" && printf 'AGAI\n' > data.bin && GIT_CONFIG_GLOBAL=/dev/null git add data.bin ) >/dev/null 2>&1
  chk "p4 CONTROL NEEDLE: the same shim does write its marker when git add runs the filter" test -e "$tmp/p4/marker"
fi
# ============ p5: tar from the tree id, attribute-free
if want p5; then fixture p5
  printf 'wip\n' >> "$F/svc/main.go"
  M="$tmp/p5/m.json"; SN manifest > "$M"; mkdir -p "$tmp/p5/out"
  SN tar --manifest "$M" --repos ".,submodules/lib,submodules/lib/deep/inner,submodules/other" | tar -x -C "$tmp/p5/out"
  chk "p5 a changed file arrives with its changed content" grep -q wip "$tmp/p5/out/svc/main.go"
  chk "p5 a file marked export-ignore still arrives" test -f "$tmp/p5/out/ignored.txt"
  chk "p5 a submodule's files arrive at its path, nested depth included" test -f "$tmp/p5/out/submodules/lib/lib.go" -a -f "$tmp/p5/out/submodules/lib/deep/inner/seed.txt"
  chk "p5 mode, symlink and exec bit survive" test -L "$tmp/p5/out/link" -a -x "$tmp/p5/out/svc/main.go" -a ! -x "$tmp/p5/out/svc/go.mod"
  chk "p5 no .git entry is shipped" test -z "$(find "$tmp/p5/out" -name .git | head -1)"
  chk "p5 the evidence and audit trees are excluded from the compile-class tar" test ! -e "$tmp/p5/out/$EVP"
fi
# ============ p6: closure
if want p6; then fixture p6
  c=$(SN closure --component svc | jq -r '[.repos[]] | join(",")')
  chk "p6 a Go replace into submodules/lib brings that submodule (the nested one it holds too)" test "$c" = ".,submodules/lib,submodules/lib/deep/inner"
  c=$(SN closure --component web | jq -r '[.repos[]] | join(",")'); chk "p6 an npm file: dependency brings its submodule" test "$c" = ".,submodules/lib,submodules/lib/deep/inner"
  c=$(SN closure --component web/index.js 2>/dev/null | jq -r '[.repos[]] | join(",")'); chk "p6 the closure is per component: a component without references ships the root only" test "$(SN closure --component svc/main.go | jq -r '.repos|join(",")')" = "."
  c=$(SN closure --component img --containerfile img/Containerfile --context . | jq -r '[.repos[]] | join(",")')
  chk "p6 (u2) a Containerfile whose COPY reads a file inside a submodule ships that submodule's tree" test "$c" = ".,submodules/other"
  printf 'replace example.com/u => ../submodules/uninit\n' >> "$F/svc/go.mod"
  out=$(SN closure --component svc 2>&1); rc=$?
  chk "p6 (u) an input needing an UNINITIALISED submodule is refused: exit 20 submodule_uninitialised" test "$rc" = 20 -a "${out#*submodule_uninitialised}" != "$out"
  git -C "$F" checkout -q svc/go.mod
  M="$tmp/p6/m.json"; SN manifest > "$M"
  out=$(SN tar --manifest "$M" --repos "." --component img --containerfile img/Containerfile --context . 2>&1 >/dev/null); rc=$?
  chk "p6 (u2) a path an input references inside a gitlink OUTSIDE the shipped set is refused: exit 20 closure_incomplete" test "$rc" = 20 -a "${out#*closure_incomplete}" != "$out"
fi
# ============ p7: evidence and audit changes never enter a compile-class snapshot (w)
if want p7; then fixture p7
  a=$(dg); printf 'ev2\n' >> "$F/$EVP/e.txt"; printf 'new\n' > "$F/$AUDP/n.txt"; b=$(dg)
  chk "p7 a changed or new file under the evidence and audit trees changes no compile-class digest" test "$a" = "$b"
  chk "p7 the full class does see it" test "$(S manifest --root "$F" --class full | jq -r '.repos[0].tree')" != "$(git -C "$F" rev-parse HEAD^{tree})"
fi
# ============ p8: CPA mode: exactly the declared change set, seeded from HEAD
if want p8; then fixture p8
  h=$(S manifest --root "$F" --class full --mode cpa | jq -r '.repos[0].tree')
  chk "p8 a CPA snapshot with no declared path is exactly HEAD's tree" test "$h" = "$(git -C "$F" rev-parse HEAD^{tree})"
  printf 'staged\n' > "$F/seed.txt"; git -C "$F" add seed.txt; printf 'unstaged\n' >> "$F/web/index.js"
  chk "p8 undeclared STAGED content never enters a CPA snapshot" test "$(S manifest --root "$F" --class full --mode cpa | jq -r '.repos[0].tree')" = "$h"
  d=$(S manifest --root "$F" --class full --mode cpa --declare seed.txt | jq -r '.repos[0].tree')
  chk "p8 exactly the declared path is loaded" test "$d" != "$h" -a "$(git -C "$F" ls-tree -r "$d" -- seed.txt | awk '{print $3}')" = "$(git -C "$F" hash-object seed.txt)"
  git -C "$F" reset -q; git -C "$F" checkout -q seed.txt
  printf 'dirty\n' >> "$F/svc/main.go"
  out=$(SN manifest --mode cpa --component svc 2>&1); rc=$?
  chk "p8 an undeclared dirty file inside a compile-class check's input closure is refused: exit 20 undeclared_dirty_input" test "$rc" = 20 -a "${out#*undeclared_dirty_input}" != "$out"
  chk "p8 the same file declared is accepted" test "$(SN manifest --mode cpa --component svc --declare svc/main.go >/dev/null 2>&1; echo $?)" = 0
fi
# ============ p9: the build-host tree cache: object transfer, recompute, tamper (u) (w)
if want p9; then fixture p9
  CACHE="$tmp/p9/cache"; mkdir -p "$CACHE"; REPOS=".,submodules/lib,submodules/lib/deep/inner"
  ship() { # prints "<objects sent> <bytes>" ; materialises the closure into the cache
    local M="$tmp/p9/m.json"; SN manifest > "$M"
    SN plan --manifest "$M" --repos "$REPOS" > "$tmp/p9/ids"
    python3 -I "$TC" missing "$CACHE" < "$tmp/p9/ids" > "$tmp/p9/miss"
    SN pack --manifest "$M" --repos "$REPOS" < "$tmp/p9/miss" > "$tmp/p9/pack.tar"
    python3 -I "$TC" store "$CACHE" < "$tmp/p9/pack.tar" >/dev/null || return 1
    printf '%s %s\n' "$(wc -l < "$tmp/p9/miss")" "$(stat -c %s "$tmp/p9/pack.tar")"; }
  first=$(ship); n1=${first% *}
  chk "p9 the first ship sends objects" test "$n1" -gt 3
  second=$(ship); chk "p9 (w) a second ship of an unchanged checkout sends NO object" test "${second% *}" = 0
  printf 'one changed line\n' >> "$F/svc/main.go"
  third=$(ship); chk "p9 (w) one changed source file ships its blob and the trees above it (svc, root), nothing else" test "${third% *}" = 3
  M="$tmp/p9/m.json"; D="$tmp/p9/work"; python3 -I "$TC" materialize "$CACHE" "$M" --repos "$REPOS" --dest "$D" >/dev/null; rc=$?
  chk "p9 the materialised tree matches the manifest (remote recompute passes)" test "$rc" = 0 -a -f "$D/svc/main.go" -a -f "$D/submodules/lib/deep/inner/seed.txt"
  chk "p9 the materialised tree equals the driver's own tar of the same snapshot" test "$(cd "$D" && find . -type f -o -type l | LC_ALL=C sort | sha256sum)" = "$(SN tar --manifest "$M" --repos "$REPOS" | tar -t | sed 's#^\./##;/\/$/d' | sed 's#^#./#' | LC_ALL=C sort | sha256sum)"
  python3 -I "$TC" verify "$D" "$M" --repos "$REPOS" >/dev/null; chk "p9 verify of an untouched tree passes" test "$?" = 0
  printf 'tamper\n' >> "$D/svc/main.go"; out=$(python3 -I "$TC" verify "$D" "$M" --repos "$REPOS" 2>&1); rc=$?
  chk "p9 (u) a tree altered on the build host is refused: exit 20 snapshot_mismatch" test "$rc" = 20 -a "${out#*snapshot_mismatch}" != "$out"
  rm -rf "$D"; blob=$(git -C "$F" hash-object svc/main.go); printf 'forged blob\n' > "$CACHE/objs/${blob:0:2}/$blob"
  out=$(python3 -I "$TC" materialize "$CACHE" "$M" --repos "$REPOS" --dest "$D" 2>&1); rc=$?
  chk "p9 (u) an object altered in the cache is refused at materialisation (exit 20 snapshot_mismatch, the message names the object that does not hash to its id)" test "$rc" = 20 -a "${out#*snapshot_mismatch}" != "$out" -a "${out#*does not hash to its id}" != "$out" -a ! -e "$D"
  python3 - "$tmp/p9/bad.tar" <<'PYEOF'
import io, sys, tarfile
t = tarfile.open(sys.argv[1], "w")
ti = tarfile.TarInfo("b/" + "0123456789abcdef0123456789abcdef01234567"); data = b"bytes that do not hash to that id\n"; ti.size = len(data)
t.addfile(ti, io.BytesIO(data)); t.close()
PYEOF
  out=$(python3 -I "$TC" store "$CACHE" < "$tmp/p9/bad.tar" 2>&1); rc=$?
  chk "p9 an object whose bytes do not hash to its id is refused at store (exit 20 snapshot_mismatch) and nothing is stored" test "$rc" = 20 -a "${out#*snapshot_mismatch}" != "$out" -a ! -e "$CACHE/objs/01/0123456789abcdef0123456789abcdef01234567"
fi
echo "RESULT pass=$pass fail=$fail skip=0"
[ "$fail" = 0 ]
