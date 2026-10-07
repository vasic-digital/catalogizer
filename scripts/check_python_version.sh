#!/usr/bin/env bash
# check_python_version.sh - owner decision 2026-10-07 (2): the project tooling needs Python >= the version declared in scripts/python-min-version (3.11).
# Fails (exit 1) when the interpreter is older, 3 when the interpreter cannot be run or reports nothing parseable (a blind probe never passes, 11.4.201).
# Usage: check_python_version.sh [--python CMD] [--min X.Y]
#   --python CMD  interpreter to judge (default: $PYTHON, else python3); --min X.Y overrides scripts/python-min-version (test hook, same format)
# Reads the interpreter's own sys.version_info through `python -I` (isolated: a planted module in the working directory is never imported); never parses `--version` text.
# Exit: 0 interpreter >= minimum; 1 too old; 2 usage / bad declaration; 3 interpreter missing, not runnable or output unparseable.
# Contract: docs/scripts/check_python_version.md. Test: scripts/tests/test_check_python_version.sh.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PY="${PYTHON:-python3}"; MIN=""
while [ $# -gt 0 ]; do
  case "$1" in
    --python) [ $# -ge 2 ] || { echo "check_python_version: --python needs a value" >&2; exit 2; }; PY="$2"; shift 2 ;;
    --min) [ $# -ge 2 ] || { echo "check_python_version: --min needs a value" >&2; exit 2; }; MIN="$2"; shift 2 ;;
    *) echo "check_python_version: unknown argument $1" >&2; exit 2 ;;
  esac
done
if [ -z "$MIN" ]; then
  [ -r "$HERE/python-min-version" ] || { echo "check_python_version: declaration scripts/python-min-version missing" >&2; exit 2; }
  MIN="$(head -n1 "$HERE/python-min-version")"
fi
case "$MIN" in [0-9]*.[0-9]*) : ;; *) echo "check_python_version: bad minimum '$MIN' (want X.Y)" >&2; exit 2 ;; esac
case "$MIN" in *[!0-9.]*|*.*.*) echo "check_python_version: bad minimum '$MIN' (want X.Y)" >&2; exit 2 ;; esac
command -v "$PY" >/dev/null 2>&1 || { echo "check_python_version: REFUSED reason=interpreter_missing python=$PY" >&2; exit 3; }
GOT="$("$PY" -I -c 'import sys; print("%d.%d" % sys.version_info[:2])' 2>/dev/null)"; rc=$?
if [ "$rc" != 0 ]; then echo "check_python_version: REFUSED reason=interpreter_not_runnable python=$PY rc=$rc" >&2; exit 3; fi
case "$GOT" in [0-9]*.[0-9]*) : ;; *) echo "check_python_version: REFUSED reason=version_unparseable got='$GOT'" >&2; exit 3 ;; esac
GM="${GOT%%.*}"; Gm="${GOT##*.}"; MM="${MIN%%.*}"; Mm="${MIN##*.}"
if [ "$GM" -gt "$MM" ] || { [ "$GM" -eq "$MM" ] && [ "$Gm" -ge "$Mm" ]; }; then
  echo "check_python_version: OK python=$PY version=$GOT minimum=$MIN"; exit 0
fi
echo "check_python_version: FAIL python=$PY version=$GOT is below the declared minimum $MIN (scripts/python-min-version)" >&2
exit 1
