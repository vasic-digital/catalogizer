#!/usr/bin/env bash
# Shared test harness for the WP-04 second-slice helper tests (T040/T040b/T041). Sourced, never run.
# Provides: ok/bad/eq/has/hasnot counters, `fin` (summary and exit status), `hp <name>` (helper path, overridable with H),
# `mkrepo <dir>` (throwaway repository with a known identity), `G` (git with identity, no hooks path, no global config).
# Every test works in a throwaway tree under $TMPDIR; none touches the real repository's state or any real remote.
PASSN=0; FAILN=0
ok()  { PASSN=$((PASSN+1)); echo "ok   $1"; }
bad() { FAILN=$((FAILN+1)); echo "FAIL $1"; }
eq()  { if [ "$2" = "$3" ]; then ok "$1 (=$2)"; else bad "$1 (got '$2', want '$3')"; fi; }
has() { case "$2" in *"$3"*) ok "$1";; *) bad "$1 (no '$3' in '${2:0:200}')";; esac; }
hasnot() { case "$2" in *"$3"*) bad "$1 (unexpected '$3' in '${2:0:200}')";; *) ok "$1";; esac; }
fin() { echo "---- $PASSN ok, $FAILN failed"; [ "$FAILN" = 0 ] && [ -x "${H:-/nonexistent}" ]; }
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
export GIT_CONFIG_COUNT=2 GIT_CONFIG_KEY_0=protocol.file.allow GIT_CONFIG_VALUE_0=always GIT_CONFIG_KEY_1=safe.directory GIT_CONFIG_VALUE_1='*'   # safe.directory: a container user that does not own the bind-mounted tree (container portability)
mkrepo() { git init -q -b main "$1" && git -C "$1" config user.email t@t && git -C "$1" config user.name t; }
commit_all() { git -C "$1" add -A && git -C "$1" commit -qm "${2:-c}"; }
