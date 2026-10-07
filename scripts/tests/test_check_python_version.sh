#!/usr/bin/env bash
# test_check_python_version.sh - tests of scripts/check_python_version.sh (owner decision 2026-10-07 (2)): fake interpreters of every version shape, a blind
# interpreter, the declaration file, and the REAL interpreter of the environment the test runs in.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; REPO="$(cd "$HERE/../.." && pwd)"; CK="$REPO/scripts/check_python_version.sh"
T="$(mktemp -d "${TMPDIR:-/tmp}/cpv.XXXXXX")"; trap 'rm -rf "$T"' EXIT
P=0; F=0
ok() { P=$((P+1)); echo "  ok   $1"; }
no() { F=$((F+1)); echo "  FAIL $1 :: ${2:-}"; }
fake() { printf '#!/bin/sh\n# fake interpreter: answers the sys.version_info probe\necho "%s"\n' "$2" >"$T/$1"; chmod +x "$T/$1"; }
chk() { OUT="$("$CK" "${@:2}" 2>&1)"; RC=$?; if [ "$RC" = "$1" ]; then ok "$TITLE"; else no "$TITLE" "rc=$RC want=$1 out=$OUT"; fi; }
[ "$(cat "$REPO/scripts/python-min-version")" = "3.11" ] && ok "declaration file says 3.11" || no "declaration file says 3.11"
fake p310 3.10; fake p311 3.11; fake p312 3.12; fake p400 4.0; fake p39 3.9; fake pbad "not a version"; printf '#!/bin/sh\nexit 7\n' >"$T/pdead"; chmod +x "$T/pdead"
TITLE="3.10 is below 3.11 -> 1"; chk 1 --python "$T/p310"
TITLE="3.9 is below 3.11 -> 1"; chk 1 --python "$T/p39"
TITLE="3.11 meets the minimum -> 0"; chk 0 --python "$T/p311"
TITLE="3.12 meets the minimum -> 0"; chk 0 --python "$T/p312"
TITLE="4.0 meets the minimum (major compare) -> 0"; chk 0 --python "$T/p400"
TITLE="--min 3.12 rejects 3.11 -> 1"; chk 1 --python "$T/p311" --min 3.12
TITLE="unparseable output is refused (blind probe) -> 3"; chk 3 --python "$T/pbad"
TITLE="interpreter that fails to run is refused -> 3"; chk 3 --python "$T/pdead"
TITLE="missing interpreter is refused -> 3"; chk 3 --python "$T/nope"
TITLE="bad --min -> 2"; chk 2 --python "$T/p311" --min x.y
TITLE="bad --min 3.11.2 -> 2"; chk 2 --python "$T/p311" --min 3.11.2
TITLE="unknown argument -> 2"; chk 2 --bogus
# real interpreter of this environment (the tooling image must be >= 3.11)
TITLE="REAL python3 of this environment ($(python3 -c 'import sys;print(sys.version.split()[0])' 2>/dev/null)) meets the declared minimum"; chk 0
# mutation: a checker whose comparison is inverted must disagree with the table above
sed 's/-ge "\$Mm"/-lt "$Mm"/' "$CK" >"$T/mut.sh"; cp "$REPO/scripts/python-min-version" "$T/python-min-version"; chmod +x "$T/mut.sh"
if "$T/mut.sh" --python "$T/p310" >/dev/null 2>&1; then ok "MUTATION: inverted comparison now accepts 3.10 (so the 3.10->1 test above is load-bearing)"; else no "MUTATION not effective"; fi
echo "PASS=$P FAIL=$F"; [ "$F" = 0 ]
