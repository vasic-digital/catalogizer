#!/usr/bin/env bash
# Paired mutation test for scripts/safe_submodule_init.sh (constitution §1.1).
# Copies the script and its fixture test into a scratch tree, applies one mutant at a time, and
# requires the fixture test (scripts/tests/test_safe_submodule_init.sh) to FAIL on every mutant
# and PASS on the unmutated copy. Exit 0 = every mutant killed; 1 = a mutant survived or the
# baseline failed; 2 = a mutant did not apply (the sed pattern no longer matches the script).
set -uo pipefail
HERE="$(cd "$(dirname "$0")/.." && pwd)"
W="$(mktemp -d)"; trap 'rm -rf "$W"' EXIT
mkdir -p "$W/scripts/tests"
cp "$HERE/tests/test_safe_submodule_init.sh" "$W/scripts/tests/"
run() { bash "$W/scripts/tests/test_safe_submodule_init.sh" >/dev/null 2>&1; }
cp "$HERE/safe_submodule_init.sh" "$W/scripts/"
if run; then echo "BASELINE PASS"; else echo "BASELINE FAIL"; exit 1; fi
rc=0
mutant() {  # $1 = name, $2 = sed expression
  sed "$2" "$HERE/safe_submodule_init.sh" > "$W/scripts/safe_submodule_init.sh"
  if cmp -s "$HERE/safe_submodule_init.sh" "$W/scripts/safe_submodule_init.sh"; then
    echo "NOT-APPLIED $1"; rc=2; return
  fi
  if run; then echo "SURVIVED $1"; [ "$rc" -eq 0 ] && rc=1; else echo "KILLED $1"; fi
}
mutant no_gitlink_check   's/is_gitlink "\$repo" "\$rel" \&\& continue/:/'
mutant no_constitution_glob 's| "\$repo"/constitution/; do|; do|'
mutant no_root_scan       's/^scan_leftovers \.$/:/'
mutant no_engine_scan     's|for e in constitution/submodules/\*/; do scan_leftovers "\${e%/}"; done|:|'
mutant moves_leftover     's|^scan_leftovers \.$|scan_leftovers .; mv constitution/submodules/stale_hub constitution/submodules/stale_hub.moved 2>/dev/null|'
mutant exit_status_changed 's/^exit \$rc$/exit 1/'
cp "$HERE/safe_submodule_init.sh" "$W/scripts/"
exit $rc
