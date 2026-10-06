#!/usr/bin/env bash
# lib/wrap_common.sh - shared part of tools/evidence/wrap-{go,bash,vitest,gradle,cargo}.sh (tasks.md T051, T053). Sourced, never run.
#
#   wrap-X.sh --run-token TOKEN [--from-report PATH] [--rc N] [--min-tests N] [--results-dir DIR] [--no-raw] [--] RUNNER_ARGS...
#
#   --run-token T    REQUIRED. The per-run unique evidence token printed by `evrec token` (32 lowercase hex). The wrapper exports it to the
#                    test as EVREC_RUN_TOKEN (T051a, constitution 7.1). The caller passes the same value after `--run-token` in the argv
#                    that `evrec run` records, so the recorded argv holds it. The wrapper never prints it.
#   --from-report P  parse a recorded report instead of running the runner (the parser tests of T051/T053 use this). With --rc N the
#                    recorded runner exit status takes part in the verdict.
#   --min-tests N    fewer tests than N (default 1) is an error, never a pass.
# Exit status: 0 pass, 1 test failure, 126 no test outcome (usage error, runner absent, zero tests, build failure, cut stream). The
# wrapper never exits 64/127: the recorder would read those as a failing assertion (1..125) or hide the cause.
# Output: the summary lines (`wrap: runner=... status=...`), then `wrap: raw-begin` and the runner's own report, so the recorder's
# stdout blob holds both the verdict-bearing summary and everything the test printed (the captured post-state).

WRAP_HERE=$(cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")/.." && pwd)

wrap_refuse() { echo "wrap: refused reason=$1${2:+ $2}" >&2; exit 126; }

wrap_parse() {
  W_TOKEN=""; W_FROM=""; W_RC=""; W_MIN=1; W_RESULTS="."; W_NORAW=""; W_REST=()
  while [ $# -gt 0 ]; do
    case $1 in
      --run-token)   [ $# -ge 2 ] || wrap_refuse usage_error "--run-token needs a value"; W_TOKEN=$2; shift 2 ;;
      --from-report) [ $# -ge 2 ] || wrap_refuse usage_error "--from-report needs a path"; W_FROM=$2; shift 2 ;;
      --rc)          [ $# -ge 2 ] || wrap_refuse usage_error "--rc needs a number"; W_RC=$2; shift 2 ;;
      --min-tests)   [ $# -ge 2 ] || wrap_refuse usage_error "--min-tests needs a number"; W_MIN=$2; shift 2 ;;
      --results-dir) [ $# -ge 2 ] || wrap_refuse usage_error "--results-dir needs a directory"; W_RESULTS=$2; shift 2 ;;
      --no-raw)      W_NORAW=--no-raw; shift ;;
      --)            shift; W_REST=("$@"); break ;;
      *)             wrap_refuse usage_error "unknown argument $1 (runner arguments go after --)" ;;
    esac
  done
  case $W_TOKEN in
    "") wrap_refuse run_token_missing "--run-token TOKEN is required: take it from \`evrec token\` (T051a, a run without a token cannot pass)" ;;
  esac
  [[ $W_TOKEN =~ ^[0-9a-f]{32}$ ]] || wrap_refuse run_token_malformed "--run-token must be 32 lowercase hex digits (the output of \`evrec token\`)"
  [[ $W_MIN =~ ^[1-9][0-9]*$ ]] || wrap_refuse usage_error "--min-tests must be a positive integer"
  [ -z "$W_RC" ] || [[ $W_RC =~ ^[0-9]+$ ]] || wrap_refuse usage_error "--rc must be a non-negative integer"
}

# wrap_report KIND PATH [extra evparse args...]: parse and print; the exit status of evparse is the wrapper's
wrap_report() {
  local kind=$1 path=$2; shift 2
  local args=("$kind" "$path" --min-tests "$W_MIN")
  [ -z "$W_RC" ] || args+=(--rc "$W_RC")
  [ -z "$W_NORAW" ] || args+=("$W_NORAW")
  python3 -I "$WRAP_HERE/evparse.py" "${args[@]}" "$@"
  return $?
}

# wrap_need CMD...: the runner must be executable, else no outcome (never a test failure)
wrap_need() {
  local c=$1
  if [[ $c == */* ]]; then [ -x "$c" ] || wrap_refuse runner_not_executable "$c is not an executable file"
  else command -v -- "$c" >/dev/null 2>&1 || wrap_refuse runner_not_executable "$c is not in PATH"; fi
}

wrap_tmp() { W_TMP=$(mktemp -d "${TMPDIR:-/tmp}/wrap.XXXXXX") || wrap_refuse usage_error "no scratch directory"; trap 'rm -rf "$W_TMP"' EXIT; }
