#!/usr/bin/env bash
# cm_cgo_usleep_flag.sh - gate: every tracked script/Dockerfile/Containerfile/Makefile (outside submodules/ and
# constitution/) holding a `go build|test|vet|run|install` line must be classified in scripts/lib/cgo_flag_scope.tsv,
# and a `covered` file must have an UNCOMMENTED executable line that sources scripts/lib/cgo_flags.sh (with `|| ...` loud failure)
# or assigns CGO_CPPFLAGS=...-DHAVE_USLEEP=1. Sweep: *.sh *.bash Dockerfile* Containerfile* Makefile *.mk docker-compose*.yml .air.toml
# and extension-less files with a shebang. Remaining limits (disclosed in the decision log D2): go invoked through other variables,
# go inside a here-doc or generated script text, `# ` inside a quoted string is read as a comment, a define assigned but never
# exported to the go line, and file-level (not per-line) satisfaction.
# Usage: cm_cgo_usleep_flag.sh [--root DIR] [--scope FILE]     Exit: 0 PASS, 1 FAIL, 2 BLIND (control needles), 64 usage
# Control needles (sweep instrument): positive = scripts/distribute.sh must be found holding a go line;
# negative = a fabricated path must not be. Either failing means the sweep cannot see: BLIND, never a PASS.
set -u
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd); SCOPE=""
while [ $# -gt 0 ]; do case "$1" in --root) ROOT=$2; shift 2;; --scope) SCOPE=$2; shift 2;; *) echo "usage: $0 [--root DIR] [--scope FILE]" >&2; exit 64;; esac; done
SCOPE=${SCOPE:-$ROOT/scripts/lib/cgo_flag_scope.tsv}
[ -r "$SCOPE" ] || { echo "BLIND: scope file unreadable: $SCOPE"; exit 2; }
cd "$ROOT" || exit 2
SUBCMD='(build|test|vet|run|install|generate)'
# a go command: `go <sub>`, `/path/go <sub>`, `$GO <sub>` / `"$GO" <sub>`; any run of blanks between the words
GO_RE="(^|[^A-Za-z0-9_.-])go[[:space:]]+${SUBCMD}([^A-Za-z0-9_]|\$)|\"?\\$\\{?GO\\}?\"?[[:space:]]+${SUBCMD}([^A-Za-z0-9_]|\$)"
# executable logical lines of a file: line continuations joined, full-line and trailing comments dropped,
# echo/printf/log_ lines dropped (a mention is not an execution)
logical() {
  awk '{ l=$0; if (cont) buf=buf l; else buf=l; if (buf ~ /\\$/) { sub(/\\$/,"",buf); cont=1; next } cont=0; print buf; buf="" }
       END { if (cont) print buf }' "$1" |
    sed -E 's/[[:space:]]+#.*$//' | grep -vE '^[[:space:]]*(#|echo|printf|log_[a-z]+)([[:space:]]|$)'
}
SRC_RE='^[[:space:]]*(\.|source)[[:space:]]+.*cgo_flags\.sh.*\|\|'      # sourcing line WITH a loud-failure branch
DEF_RE='(^|[[:space:],;&"(]|ENV[[:space:]]+)CGO_CPPFLAGS[=:][[:space:]]*"?[^[:space:]"]*-DHAVE_USLEEP=1'   # assignment of the define on an executable line
holders=$(mktemp) || exit 2; trap 'rm -f "$holders"' EXIT
git ls-files 2>/dev/null | grep -vE '^(submodules|constitution)/' |
while IFS= read -r f; do
  [ -f "$f" ] || continue
  case "$f" in
    *.sh|*.bash|*Dockerfile*|*Containerfile*|*/Makefile|Makefile|*.mk|*docker-compose*.yml|*docker-compose*.yaml|*.air.toml) ;;
    *.*) continue;;
    *) [ "$(head -c2 "$f" 2>/dev/null)" = '#!' ] || continue;;
  esac
  if logical "$f" | grep -E "$GO_RE" | grep -vE 'go install [^ ]*@' | grep -q .; then printf '%s\n' "$f"; fi
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
    covered) if ! logical "$f" | grep -qE -e "$SRC_RE" -e "$DEF_RE"; then echo "FAIL: $f is covered but has no UNCOMMENTED executable line that sources lib/cgo_flags.sh with a failure branch or assigns CGO_CPPFLAGS with -DHAVE_USLEEP=1"; fail=1; fi;;
    *) echo "FAIL: $f has an invalid class '$cls'"; fail=1;;
  esac
done < "$holders"
[ -r "$ROOT/scripts/lib/cgo_flags.sh" ] || { echo "FAIL: scripts/lib/cgo_flags.sh missing"; fail=1; }
if [ $fail = 0 ]; then echo "PASS: $n files with go lines, all classified, every covered file carries the define"; exit 0; fi
exit 1
