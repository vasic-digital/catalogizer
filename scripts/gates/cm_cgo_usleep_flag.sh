#!/usr/bin/env bash
# cm_cgo_usleep_flag.sh - gate: every tracked script/Dockerfile/Containerfile/Makefile (outside submodules/ and
# constitution/) holding a `go build|test|vet|run|install` line must be classified in scripts/lib/cgo_flag_scope.tsv,
# and a `covered` file must carry the define (-DHAVE_USLEEP=1 inline) or source scripts/lib/cgo_flags.sh.
# Usage: cm_cgo_usleep_flag.sh [--root DIR] [--scope FILE]     Exit: 0 PASS, 1 FAIL, 2 BLIND (control needles), 64 usage
# Control needles (sweep instrument): positive = scripts/distribute.sh must be found holding a go line;
# negative = a fabricated path must not be. Either failing means the sweep cannot see: BLIND, never a PASS.
set -u
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd); SCOPE=""
while [ $# -gt 0 ]; do case "$1" in --root) ROOT=$2; shift 2;; --scope) SCOPE=$2; shift 2;; *) echo "usage: $0 [--root DIR] [--scope FILE]" >&2; exit 64;; esac; done
SCOPE=${SCOPE:-$ROOT/scripts/lib/cgo_flag_scope.tsv}
[ -r "$SCOPE" ] || { echo "BLIND: scope file unreadable: $SCOPE"; exit 2; }
cd "$ROOT" || exit 2
GO_RE='(^|[^A-Za-z0-9_./-])go (build|test|vet|run|install)([^A-Za-z0-9_]|$)'
holders=$(mktemp) || exit 2; trap 'rm -f "$holders"' EXIT
git ls-files 2>/dev/null | grep -vE '^(submodules|constitution)/' | grep -E '(\.sh$|(^|/)Dockerfile|(^|/)Containerfile|(^|/)Makefile$|\.mk$)' |
while IFS= read -r f; do
  [ -f "$f" ] || continue
  if grep -vE '^[[:space:]]*#' "$f" | grep -E "$GO_RE" | grep -vE 'go install [^ ]*@' | grep -q .; then printf '%s\n' "$f"; fi
done > "$holders"
if [ "${CGO_GATE_TEST_NO_NEEDLE:-0}" != 1 ]; then
  grep -qx 'scripts/distribute.sh' "$holders" || { echo "BLIND: positive control scripts/distribute.sh not found by the sweep"; exit 2; }
  grep -qx 'zz_no_such_go_file.sh' "$holders" && { echo "BLIND: negative control matched"; exit 2; }
fi
fail=0; n=0
while IFS= read -r f; do
  n=$((n+1))
  cls=$(awk -F'\t' -v f="$f" '$1==f {print $2; exit}' "$SCOPE")
  case "$cls" in
    "") echo "FAIL: $f holds a go build/test/vet/run line and is not classified in $(basename "$SCOPE")"; fail=1;;
    exempt:*) ;;
    covered) if ! grep -q -e '-DHAVE_USLEEP=1' -e 'lib/cgo_flags\.sh' "$f"; then echo "FAIL: $f is covered but neither carries -DHAVE_USLEEP=1 nor sources scripts/lib/cgo_flags.sh"; fail=1; fi;;
    *) echo "FAIL: $f has an invalid class '$cls'"; fail=1;;
  esac
done < "$holders"
[ -r "$ROOT/scripts/lib/cgo_flags.sh" ] || { echo "FAIL: scripts/lib/cgo_flags.sh missing"; fail=1; }
if [ $fail = 0 ]; then echo "PASS: $n files with go lines, all classified, every covered file carries the define"; exit 0; fi
exit 1
