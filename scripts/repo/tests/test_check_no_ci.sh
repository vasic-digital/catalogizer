#!/usr/bin/env bash
# T040 test (TDD): scripts/repo/check_no_ci.sh - the single home of the section 11.4.156 condition (no CI pipeline definition at a
# repository root). Throwaway repositories only. Usage: bash scripts/repo/tests/test_check_no_ci.sh   Env: H=<helper> (mutation driver)
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
D0="$(pwd)"; H="${H:-scripts/repo/check_no_ci.sh}"; case "$H" in /*) ;; *) H="$D0/$H" ;; esac
. "$D0/scripts/repo/tests/lib_wp04b.sh"
T="$(mktemp -d "${TMPDIR:-/tmp}/nci_test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
[ -x "$H" ] || echo "NOTE: $H is absent or not executable (RED state: every case must FAIL)"
run() { "$H" "$@" >"$T/out" 2>"$T/err"; RC=$?; }
mkrepo "$T/r"; mkdir -p "$T/r/.github/workflows" "$T/r/.specify/extensions/superspec/.github/workflows" "$T/r/src"
echo "docs" > "$T/r/.github/workflows/README.md"
echo "on: push" > "$T/r/.specify/extensions/superspec/.github/workflows/ci.yml"   # the negative needle: nested, not at a root
echo x > "$T/r/src/a.go"; commit_all "$T/r" init
# 1 golden-true: README in .github/workflows and a nested ci.yml are not pipeline definitions at the root
run --root "$T/r"; eq "clean root (README + nested needle): 0" "$RC" 0
# 2 one fixture per file form, each planted in a throwaway clone (control needle: the same form at a nested depth stays 0)
form() { # form <relative path>
  rm -rf "$T/c"; git clone -q "$T/r" "$T/c"; mkdir -p "$(dirname "$T/c/$1")"; echo "x" > "$T/c/$1"
  run --root "$T/c"; eq "planted $1: 10" "$RC" 10; has "report names $1" "$(cat "$T/out")" "$1"
}
for f in .github/workflows/a.yml .github/workflows/b.yaml .gitlab-ci.yml .gitea/workflows/w.yml .gitea/workflows/noext .woodpecker.yml .drone.yml .circleci/config.yml .circleci/sub/x.txt; do form "$f"; done
rm -rf "$T/c"; git clone -q "$T/r" "$T/c"; mkdir -p "$T/c/pkg/.github/workflows"; echo x > "$T/c/pkg/.github/workflows/a.yml"; echo x > "$T/c/pkg/.gitlab-ci.yml"
run --root "$T/c"; eq "nested pipeline files at a deeper path (never a root): 0" "$RC" 0
# 3 tracked and untracked-unignored files count, ignored ones do not (they cannot reach a remote)
rm -rf "$T/c"; git clone -q "$T/r" "$T/c"; echo '.drone.yml' > "$T/c/.gitignore"; echo x > "$T/c/.drone.yml"
run --root "$T/c"; eq "git-ignored .drone.yml: 0" "$RC" 0
rm -rf "$T/c"; git clone -q "$T/r" "$T/c"; mkdir -p "$T/c/.github/workflows"; echo x > "$T/c/.github/workflows/t.yml"; git -C "$T/c" add -f .github/workflows/t.yml; git -C "$T/c" commit -qm t
run --root "$T/c"; eq "tracked workflow file: 10" "$RC" 10
# 4 several roots: each judged at its own root; one violating root gives 10 and is named
rm -rf "$T/c"; git clone -q "$T/r" "$T/c"; echo x > "$T/c/.travis-not-listed.yml"; mkrepo "$T/d"; echo x > "$T/d/.circleci-not.yml"; mkdir -p "$T/d/.circleci"; echo x > "$T/d/.circleci/config.yml"; commit_all "$T/d" d
run --root "$T/c" --root "$T/d"; eq "two roots, second violates: 10" "$RC" 10; has "violating root named" "$(cat "$T/out")" "$T/d"
# 5 refusals (20): not a repository, option-like root, unknown option, no root
run --root "$T/nonexistent"; eq "missing root: 20" "$RC" 20
mkdir -p "$T/plain"; run --root "$T/plain"; eq "not a git repository: 20" "$RC" 20
run --root "-x"; eq "root starting with a dash: 20" "$RC" 20
mkdir -p "$T/cwd"; mkrepo "$T/cwd/-x"; echo x > "$T/cwd/-x/a"; commit_all "$T/cwd/-x" d
( cd "$T/cwd" && "$H" --root -x ) >/dev/null 2>&1; eq "an existing repository named -x is still refused as an option-like value: 20" "$?" 20
run --bogus; eq "unknown option: 20" "$RC" 20
run; eq "no --root: 20" "$RC" 20
# a git shim that FAILS (rc 128) when the argument list matches a glob, else runs the real git: a failing listing must never read as clean
mkshim() { mkdir -p "$T/fshim"; local rg; rg="$(command -v git)"; printf '#!/bin/sh\ncase "$*" in $FAILPAT) echo "fatal: shim" >&2; exit 128 ;; esac\nexec "%s" "$@"\n' "$rg" > "$T/fshim/git"; chmod +x "$T/fshim/git"; }
# 6 (WF5 F3) a failing git listing is no "clean": 20 with a named reason, never 0
mkshim; rm -rf "$T/c"; git clone -q "$T/r" "$T/c"; echo x > "$T/c/.gitlab-ci.yml"
run --root "$T/c"; eq "control, real git: pipeline file found: 10" "$RC" 10
PATH="$T/fshim:$PATH" FAILPAT='*ls-files*' run --root "$T/c"; eq "git ls-files failing: 20, not 0 (F3)" "$RC" 20; has "named git_listing_failed" "$(cat "$T/err")" git_listing_failed
PATH="$T/fshim:$PATH" FAILPAT='*ls-files*' run --root "$T/r"; eq "git ls-files failing on a clean root: 20, never 0" "$RC" 20
eq "no clean-looking stdout on failure" "$(cat "$T/out")" ""

# 7 (WF6 W6-5) a terminated run leaves no temp file behind: SIGTERM during a slow listing (the shim sleeps, then runs the real git)
mkdir -p "$T/slowshim" "$T/tmpd"; rg_="$(command -v git)"; printf '#!/bin/sh\ncase "$*" in *ls-files*) sleep 2 ;; esac\nexec "%s" "$@"\n' "$rg_" > "$T/slowshim/git"; chmod +x "$T/slowshim/git"
rm -rf "$T/tmpd"; mkdir -p "$T/tmpd"; PATH="$T/slowshim:$PATH" TMPDIR="$T/tmpd" "$H" --root "$T/r" >/dev/null 2>&1 & hp=$!
n=0; until [ -n "$(ls -A "$T/tmpd" 2>/dev/null)" ] || [ "$n" -ge 100 ]; do sleep 0.1; n=$((n+1)); done
eq "control: the listing temp file exists while the listing runs" "$([ -n "$(ls -A "$T/tmpd")" ] && echo yes || echo no)" yes
kill -TERM "$hp"; wait "$hp" 2>/dev/null; trc=$?
eq "terminated run exits 143 (TERM trap)" "$trc" 143; eq "no temp file left behind by the terminated run (W6-5)" "$(ls -A "$T/tmpd" | tr '\n' ' ')" ""
fin
