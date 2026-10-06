#!/bin/bash
# smoke_probe.sh - T008, runs INSIDE an image (through run_pinned.sh, source mounted read-only at /src, output at /out).
# Writes /out/toolchain.json: per tool presence + version (absent | error recorded as such, never as present), python import probes,
# the cache-write probe as the mapped user, and the control needle: a write to the read-only /src mount that MUST fail and is recorded.
# Usage: bash /src/scripts/containers/smoke_probe.sh <IMG-ID>   (SMOKE_OUT / SMOKE_SRC override /out and /src for the host test)
set -u
IMG="${1:?image id}"
OUTD="${SMOKE_OUT:-/out}"; SRCD="${SMOKE_SRC:-/src}"   # test inputs; inside an image run they are the real /out and /src
esc() { printf '%s' "$1" | tr -d '\000-\037' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
ver() { # $1 name, rest: command that prints a version. Exit codes accepted as success: $VER_OK (default "0"; a stated per-tool exception)
  local name="$1"; shift
  if ! command -v "$1" >/dev/null 2>&1 && [ ! -x "$1" ]; then printf '"%s":{"status":"absent"}' "$name"; return; fi
  local out rc good=0 c; out="$("$@" 2>&1)"; rc=$?
  for c in ${VER_OK:-0}; do [ "$rc" = "$c" ] && good=1; done   # MUT:probe-status
  local had=0; [ -n "$out" ] && had=1
  out="$(printf '%s\n' "$out" | head -2 | tr '\n' ' ')"
  # present = the tool itself exited with an accepted status AND printed something; any other outcome is an error, never present
  if [ "$good" = 1 ] && [ "$had" = 1 ]; then printf '"%s":{"status":"present","version":"%s"}' "$name" "$(esc "$out")"
  else printf '"%s":{"status":"error","exit":%s,"output":"%s"}' "$name" "$rc" "$(esc "$out")"; fi
}
imp() { # $1 label, $2 python, $3 code
  if ! command -v "$2" >/dev/null 2>&1 && [ ! -x "$2" ]; then printf '"%s":"absent"' "$1"; return; fi
  if "$2" -c "$3" >/dev/null 2>&1; then printf '"%s":"present"' "$1"; else printf '"%s":"error"' "$1"; fi
}
T=()
T+=("$(ver bash bash --version)") ; T+=("$(ver git git --version)"); T+=("$(ver sqlite3 sqlite3 --version)"); T+=("$(ver python3 python3 --version)"); T+=("$(ver jq jq --version)")
# gofmt -h prints its usage and exits 2: the stated per-tool exception (VER_OK)
T+=("$(ver go go version)"); T+=("$(VER_OK="0 2" ver gofmt gofmt -h)"); T+=("$(ver kcov kcov --version)"); T+=("$(ver script script --version)"); T+=("$(ver realpath realpath --version)")
T+=("$(ver detect-secrets /opt/cpa-tools/bin/detect-secrets --version)"); T+=("$(ver check-yaml /opt/cpa-tools/bin/check-yaml --help)"); T+=("$(ver detect-private-key /opt/cpa-tools/bin/detect-private-key --help)")
T+=("$(ver definitely-not-a-tool definitely-not-a-tool --version)")   # control needle: must be absent
T+=("$(ver errored-needle sh -c 'echo needle-output >&2; exit 3')")   # control needle: a tool that errors, must be error
IMPS=("$(imp pytest_yaml_jsonschema python3 'import pytest, yaml, jsonschema')" "$(imp cpa_tools_identify /opt/cpa-tools/bin/python3 'import identify')")
GLIBC="$(ldd --version 2>/dev/null | head -1 | grep -o '[0-9]\+\.[0-9]\+$')"
UIDN="$(id -u)"
# cache-write probe as the mapped user: HOME and XDG_CACHE_HOME are under the writable tmpfs
if mkdir -p "$XDG_CACHE_HOME/probe" 2>/dev/null && : >"$XDG_CACHE_HOME/probe/x" 2>/dev/null; then CW=ok; else CW=failed; fi
# control needle: a write to the read-only source mount must fail; "failed" is the expected value, "succeeded" voids the probe
if { : >"$SRCD/.smoke-probe-needle"; } 2>/dev/null; then ROW=succeeded; rm -f "$SRCD/.smoke-probe-needle"; else ROW=failed; fi
{
  printf '{"schema":1,"image_id":"%s","uid":%s,"glibc":"%s","cache_write_probe":"%s","ro_mount_write_probe":"%s","ro_mount_write_expected":"failed","tools":{' "$IMG" "$UIDN" "${GLIBC:-unknown}" "$CW" "$ROW"
  ( IFS=,; printf '%s' "${T[*]}" )
  printf '},"python_imports":{'; ( IFS=,; printf '%s' "${IMPS[*]}" ); printf '}}\n'
} >"$OUTD/toolchain.json"
cat "$OUTD/toolchain.json"
