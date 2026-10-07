#!/usr/bin/env bash
# mutate_exclusions.sh - T200 paired mutation (G-GATE) run against EACH application's real exclusion file. For every coverage/exclusions/<app>.yaml:
#   poisoned copy = the file plus one first-party entry WITHOUT a tracked item;
#   the real gate must FAIL on the poisoned copy (rc 1, naming the entry) and give on the unpoisoned file the verdict it gives today (recorded);
#   a gate copy that ignores the tracked-item check must ACCEPT the poisoned copy (rc 0): that difference is what test_check_exclusions.sh asserts, so the mutation is CAUGHT.
# Writes <EVDIR>/<app>/exclusions-mutation.txt per application. Usage: mutate_exclusions.sh [--evdir DIR] [--apps "a b ..."]
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; REPO="$(cd "$HERE/../../.." && pwd)"
EVDIR="$REPO/specs/001-full-project-audit-remediation/evidence/coverage_baseline"
APPS="catalog-api catalog-web catalogizer-desktop installer-wizard catalogizer-android catalogizer-androidtv catalogizer-api-client website build-scripts"
while [ $# -gt 0 ]; do case "$1" in --evdir) EVDIR="$2"; shift 2;; --apps) APPS="$2"; shift 2;; *) echo "usage: mutate_exclusions.sh [--evdir DIR] [--apps LIST]" >&2; exit 2;; esac; done
T="$(mktemp -d "${TMPDIR:-/tmp}/mutexcl.XXXXXX")"; trap 'rm -rf "$T"' EXIT
mkdir -p "$T/mut"; cp "$HERE/../check_exclusions.sh" "$HERE/../fence_lib.py" "$T/mut/"; python3 -I - "$HERE/../check_exclusions.py" "$T/mut/check_exclusions.py" <<'PY' || { echo "mutation anchor missing" >&2; exit 3; }
import sys
s=open(sys.argv[1]).read(); old='if not item_ok and not mby_ok:'
if s.count(old)!=1: sys.exit(1)
open(sys.argv[2],"w").write(s.replace(old,"if False:"))
PY
FAILS=0
for app in $APPS; do
  f="$REPO/coverage/exclusions/$app.yaml"; [ -f "$f" ] || { echo "no fence file $f" >&2; FAILS=$((FAILS+1)); continue; }
  d="$T/$app"; mkdir -p "$d"
  cp "$f" "$d/$app.yaml"
  if grep -q '^exclusions: \[\]' "$d/$app.yaml"; then sed -i 's/^exclusions: \[\]/exclusions:/' "$d/$app.yaml"; fi
  cat >>"$d/$app.yaml" <<'Y'
  - path: "poison/first_party_without_item.src"
    class: first-party
    justification: "a first-party entry written without a tracked item, the case the gate must refuse"
Y
  bash "$REPO/scripts/coverage/check_exclusions.sh" "$d/$app.yaml" --lanes "$REPO/scripts/containers/lanes.tsv" >"$d/real.out" 2>"$d/real.err"; rc_real=$?
  bash "$T/mut/check_exclusions.sh" "$d/$app.yaml" --lanes "$REPO/scripts/containers/lanes.tsv" >"$d/mut.out" 2>"$d/mut.err"; rc_mut=$?
  bash "$REPO/scripts/coverage/check_exclusions.sh" "$f" --used "$REPO/coverage/exclusions/used/$app.txt" >/dev/null 2>&1; rc_today=$?
  verdict=CAUGHT; { [ "$rc_real" = 1 ] && [ "$rc_mut" = 0 ] && grep -q 'first_party_without_item' "$d/real.err"; } || { verdict=SURVIVED; FAILS=$((FAILS+1)); }
  mkdir -p "$EVDIR/$app"
  {
    echo "mutation: tracked-item check ignored (if not item_ok and not mby_ok -> if False), application $app"
    echo "date_utc: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "fence_file: coverage/exclusions/$app.yaml sha256 $(sha256sum <"$f" | cut -d' ' -f1)"
    echo "poisoned copy (the file plus a first-party entry without a tracked item): real gate rc=$rc_real (want 1), mutant gate rc=$rc_mut (want 0: the mutant accepts it)"
    echo "real gate stderr: $(grep first_party_without_item "$d/real.err" | head -1)"
    echo "real gate on the unmodified file with its used list: rc=$rc_today"
    echo "result: $verdict"
  } >"$EVDIR/$app/exclusions-mutation.txt"
  echo "$app: $verdict (real=$rc_real mutant=$rc_mut)"
done
[ "$FAILS" = 0 ]
