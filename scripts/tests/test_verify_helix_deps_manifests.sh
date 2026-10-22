#!/usr/bin/env bash
# Test for scripts/verify_helix_deps_manifests.sh, the thin wrapper that delegates to the canonical
# constitution engine (constitution/scripts/deps_manifest/verify_helix_deps_manifests.sh, 11.4.31).
# Written first (RED against the old duplicated engine). Drives the REAL wrapper, copied into a
# throw-away project tree whose constitution/ holds a STUB canonical that records its argv and exits
# with a code chosen by the test. Then runs mutants of the wrapper; the suite must catch each one.
# Optional live section: set LIVE=1 to also run the wrapper against the real constitution engine.
set -uo pipefail
HERE="$(cd "$(dirname "$0")/.." && pwd)"
SUT="${SUT:-$HERE/verify_helix_deps_manifests.sh}"
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
fail=0; total=0
chk() { total=$((total+1)); if eval "$2"; then echo "PASS $1"; else echo "FAIL $1"; fail=$((fail+1)); fi; }

# Build a project tree: <p>/scripts/<wrapper copy>, optional stub canonical.
mkproj() { # name with_stub(1|0) -> echoes project dir
  local p="$T/$1"; mkdir -p "$p/scripts"
  cp "$SUT" "$p/scripts/verify_helix_deps_manifests.sh"
  if [ "$2" = 1 ]; then
    mkdir -p "$p/constitution/scripts/deps_manifest"
    cat > "$p/constitution/scripts/deps_manifest/verify_helix_deps_manifests.sh" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$@" > "$STUB_ARGS_FILE"
echo "STUB-CANON-RAN"
exit "${STUB_RC:-0}"
STUB
  fi
  echo "$p"
}
runw() { # proj [args...] ; env STUB_RC passes through
  local p="$1"; shift
  out="$(STUB_ARGS_FILE="$T/args.txt" bash "$p/scripts/verify_helix_deps_manifests.sh" "$@" 2>&1)"; rc=$?
}

P="$(mkproj ok 1)"; rm -f "$T/args.txt"
STUB_RC=0 runw "$P"
chk "delegates: canonical stub ran" 'echo "$out" | grep -q STUB-CANON-RAN'
chk "delegates: --root <project root> is first args" '[ "$(sed -n 1p "$T/args.txt")" = "--root" ] && [ "$(sed -n 2p "$T/args.txt")" = "$P" ]'
chk "exit 0 passes through" '[ $rc -eq 0 ]'

STUB_RC=0 runw "$P" --own-orgs a,b
chk "extra args forwarded after --root" '[ "$(sed -n 3p "$T/args.txt")" = "--own-orgs" ] && [ "$(sed -n 4p "$T/args.txt")" = "a,b" ]'

for c in 1 2 64; do
  STUB_RC=$c runw "$P"
  chk "exit code $c passes through unchanged" "[ \$rc -eq $c ]"
done

# canon missing -> loud, actionable, non-zero, never 0
P2="$(mkproj nocanon 0)"; runw "$P2"
chk "missing canon exits non-zero (2 = BLIND)" '[ $rc -eq 2 ]'
chk "missing canon message names the expected path" 'echo "$out" | grep -q "constitution/scripts/deps_manifest/verify_helix_deps_manifests.sh"'
chk "missing canon message is actionable (submodule update hint)" 'echo "$out" | grep -qi "submodule"'
chk "missing canon does not claim PASS" '! echo "$out" | grep -q "^PASS"'

# ---- mutants: each broken copy of the wrapper must be CAUGHT -------------------------------------
if [ -z "${MUTANT_RUN:-}" ]; then
  mut() { # name sed-expr
    local f="$T/mutant_$1.sh"; sed -E "$2" "$SUT" > "$f"
    if cmp -s "$SUT" "$f"; then chk "mutant $1 actually changes the wrapper" 'false'; return; fi
    MUTANT_RUN=1 SUT="$f" bash "${BASH_SOURCE[0]}" >/dev/null 2>&1; local mrc=$?
    chk "mutant $1 is caught" '[ $mrc -ne 0 ]'
  }
  mut swallow_exit_code 's/^exec bash (.*)$/bash \1; exit 0/'
  mut drop_root_arg 's/--root "\$ROOT" //'
  mut drop_forwarded_args 's/ "\$@"$//'
  mut missing_canon_ok 's/exit 2/exit 0/'
  echo "---- with mutants: $((total-fail))/$total checks passed"
fi

if [ "${LIVE:-0}" = 1 ] && [ -z "${MUTANT_RUN:-}" ]; then
  out="$(bash "$SUT" 2>&1)"; rc=$?
  echo "$out" | tail -n 5
  chk "live: canonical engine produced a VERDICT line (rc=$rc)" 'echo "$out" | grep -q "^VERDICT:"'
fi

echo "---- $((total-fail))/$total checks passed (SUT=$SUT)"
[ $fail -eq 0 ]
