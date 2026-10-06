#!/usr/bin/env bash
# Sourced by test_wrap_*.sh (T051, T053). Shared check helpers for the runner-wrapper parser tests.
#   wsetup NAME            sets T (tools/evidence), FX (fixtures dir), W (the wrapper under test), S (scratch), TOK (a fresh 32-hex token)
#   wcheck NAME RC ARGS... PAT...   runs "$W --run-token $TOK ARGS", requires exit status RC and every PAT (grep -E) in the
#                          summary part of stdout (the lines before `wrap: raw-begin`); a PAT starting with ! must NOT match.
# Anti-vacuity (a PASS without the tool under test is the bluff this task exists to prevent): while the wrapper is absent no
# check passes, whatever the expected status is.
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd); T=$(cd "$here/.." && pwd); FX=$here/fixtures
fails=0; n=0
ok()  { n=$((n+1)); if [ -x "$W" ]; then echo "ok   $1"; else fails=$((fails+1)); echo "FAIL $1 (vacuous: $W absent or not executable)"; fi; }
bad() { n=$((n+1)); fails=$((fails+1)); echo "FAIL $1"; }
wsetup() {
  W=$T/$1; S=$(mktemp -d); trap 'rm -rf "$S"' EXIT
  TOK=$(python3 -c 'import os;print(os.urandom(16).hex())')
  unset EVREC_RUN_TOKEN WRAP_GO_BIN WRAP_VITEST_BIN WRAP_GRADLE_BIN WRAP_CARGO_BIN
}
wcheck() {   # wcheck NAME RC ARGS... -- PAT...   (the first `--` splits wrapper args from patterns)
  local name=$1 want=$2; shift 2; local args=() pats=() seen=0 a
  for a in "$@"; do if [ "$a" = "::" ] && [ $seen = 0 ]; then seen=1; elif [ $seen = 0 ]; then args+=("$a"); else pats+=("$a"); fi; done
  local out rc sum p
  out=$("$W" --run-token "$TOK" "${args[@]}" 2>"$S/err"); rc=$?
  sum=$(printf '%s\n' "$out" | sed '/^wrap: raw-begin$/,$d')
  if [ "$rc" != "$want" ]; then bad "$name (exit $rc, want $want; $(head -c 160 "$S/err" | tr '\n' ' '))"; return; fi
  for p in "${pats[@]}"; do
    case $p in
      '!'*) if printf '%s\n' "$sum" | grep -Eq -- "${p#!}"; then bad "$name (summary has forbidden /${p#!}/)"; return; fi ;;
      *)    if ! printf '%s\n' "$sum" | grep -Eq -- "$p"; then bad "$name (summary lacks /$p/: $(printf '%s' "$sum" | head -c 200 | tr '\n' '|'))"; return; fi ;;
    esac
  done
  ok "$name"
}
wrefuse() {  # wrefuse NAME ARGS... : the wrapper must exit 126 (no test outcome, never 0 and never a test failure) and say reason=
  local name=$1; shift; local rc
  "$W" "$@" >"$S/o" 2>"$S/e"; rc=$?
  if [ "$rc" = 126 ] && grep -q 'reason=' "$S/e"; then ok "$name"; else bad "$name (exit $rc, stderr: $(head -c 120 "$S/e" | tr '\n' ' '))"; fi
}
wrefuse_r() {  # wrefuse_r NAME REASON ARGS... : exit 126 AND reason=REASON named on stderr
  local name=$1 want=$2; shift 2; local rc
  "$W" "$@" >"$S/o" 2>"$S/e"; rc=$?
  if [ "$rc" = 126 ] && grep -q "reason=$want" "$S/e"; then ok "$name"; else bad "$name (exit $rc, want 126 reason=$want; stderr: $(head -c 120 "$S/e" | tr '\n' ' '))"; fi
}
wdone() { echo "checks=$n failures=$fails"; [ "$fails" -eq 0 ]; }
