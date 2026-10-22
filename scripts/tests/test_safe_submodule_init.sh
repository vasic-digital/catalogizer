#!/usr/bin/env bash
# Fixture test: safe_submodule_init.sh must init a normal submodule recursively but never
# initialise constitution/'s nested submodules (unless named as an engine, one level only).
set -uo pipefail
HERE="$(cd "$(dirname "$0")/.." && pwd)"
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
export GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=protocol.file.allow GIT_CONFIG_VALUE_0=always
G="git -c user.name=t -c user.email=t@t"
mk() { git init -q "$1" && $G -C "$1" commit -q --allow-empty -m init; }
mk "$T/leaf"; mk "$T/engine"; mk "$T/other_inner"
# engine has its own nested submodule (stands in for a vendored constitution cycle)
$G -C "$T/engine" submodule add -q "$T/leaf" nested && $G -C "$T/engine" commit -q -m n
mk "$T/const"; mkdir -p "$T/const/x"
$G -C "$T/const" submodule add -q "$T/engine" submodules/engine && $G -C "$T/const" commit -q -m e
mk "$T/other"; $G -C "$T/other" submodule add -q "$T/other_inner" inner && $G -C "$T/other" commit -q -m i
mk "$T/parent"
$G -C "$T/parent" submodule add -q "$T/const" constitution
$G -C "$T/parent" submodule add -q "$T/other" submodules/other
$G -C "$T/parent" commit -q -m s
git clone -q "$T/parent" "$T/work"
fail=0; chk() { if eval "$2"; then echo "PASS $1"; else echo "FAIL $1"; fail=1; fi; }
( cd "$T/work" && bash "$HERE/safe_submodule_init.sh" ) >/dev/null 2>&1; rc_default=$?
chk "default run exits 0" '[ "$rc_default" -eq 0 ]'
chk "non-constitution submodule initialised recursively" '[ -d "$T/work/submodules/other/inner/.git" ] || [ -f "$T/work/submodules/other/inner/.git" ]'
chk "constitution initialised one level" '[ -e "$T/work/constitution/.git" ]'
chk "constitution engine NOT initialised by default" '[ ! -e "$T/work/constitution/submodules/engine/.git" ]'
( cd "$T/work" && bash "$HERE/safe_submodule_init.sh" engine ) >/dev/null 2>&1; rc_engine=$?
chk "named-engine run exits 0" '[ "$rc_engine" -eq 0 ]'
chk "named engine initialised" '[ -e "$T/work/constitution/submodules/engine/.git" ]'
chk "engine nested (cycle) NOT initialised" '[ ! -e "$T/work/constitution/submodules/engine/nested/.git" ]'
# unknown engine name must be reported as a failure (non-zero) and must not initialise anything extra
( cd "$T/work" && bash "$HERE/safe_submodule_init.sh" no_such_engine ) >/dev/null 2>&1; rc_unknown=$?
chk "unknown engine name exits non-zero" '[ "$rc_unknown" -ne 0 ]'
chk "unknown engine name initialises nothing under constitution/submodules/no_such_engine" '[ ! -e "$T/work/constitution/submodules/no_such_engine" ]'
# leftover detection: a populated directory that is NOT a gitlink in its repo's index but holds a .git
# (what git leaves behind when an engine drops a gitlink) must be WARNED about; exit status unchanged.
git init -q "$T/work/submodules/stale_root"
git init -q "$T/work/constitution/submodules/stale_hub"
mkdir -p "$T/work/constitution/submodules/engine/submodules"; git init -q "$T/work/constitution/submodules/engine/submodules/dropped"
git init -q "$T/work/constitution/submodules/engine/constitution"   # engine dropped its nested constitution gitlink (the claude_toolkit case)
mkdir -p "$T/work/submodules/plain_dir"   # no .git: must NOT be reported
out="$( cd "$T/work" && bash "$HERE/safe_submodule_init.sh" engine 2>&1 )"; rc_left=$?
chk "leftover run exit status unchanged (0)" '[ "$rc_left" -eq 0 ]'
chk "root-level leftover warned" 'printf "%s\n" "$out" | grep -q "WARN leftover: submodules/stale_root"'
chk "hub-level leftover warned" 'printf "%s\n" "$out" | grep -q "WARN leftover: constitution/submodules/stale_hub"'
chk "engine-level dropped-gitlink leftover warned" 'printf "%s\n" "$out" | grep -q "WARN leftover: constitution/submodules/engine/submodules/dropped"'
chk "engine-level dropped constitution leftover warned" 'printf "%s\n" "$out" | grep -q "WARN leftover: constitution/submodules/engine/constitution holds"'
chk "real gitlink not warned" '! printf "%s\n" "$out" | grep -q "WARN leftover: submodules/other holds"'
chk "real hub engine gitlink not warned" '! printf "%s\n" "$out" | grep -q "WARN leftover: constitution/submodules/engine holds"'
chk "directory without .git not warned" '! printf "%s\n" "$out" | grep -q "plain_dir"'
chk "nothing moved or deleted" '[ -e "$T/work/submodules/stale_root/.git" ] && [ -e "$T/work/constitution/submodules/stale_hub/.git" ] && [ -e "$T/work/constitution/submodules/engine/submodules/dropped/.git" ] && [ -e "$T/work/constitution/submodules/engine/constitution/.git" ]'
exit $fail
