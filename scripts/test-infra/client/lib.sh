#!/usr/bin/env bash
# client/lib.sh - helpers of the scripts that run INSIDE IMG-INFRA-CLIENT (sourced). The container user is the host uid, HOME=/tmp.
# Hosts of the services on the compose network are the service names; TI_PROBE_HOST_<PROTO> overrides one (the carrier fixture of T128 points
# the postgres probe at a TCP-only listener to prove that an open port is not a pass).
set -u
export LC_ALL=C.UTF-8   # the corpus has unicode names: bash $'\u...' escapes and the clients' file-name encoding need a UTF-8 locale (the image ships C.UTF-8)
host_of() { local v="TI_PROBE_HOST_$1"; echo "${!v:-$2}"; }
fail() { echo "FAIL $*"; exit 1; }
pass() { echo "PASS $*"; exit 0; }
# a credentials file for tools that take one (0600, in the container's private /tmp; the values never appear in argv)
secret_file() { local f; f="$(mktemp /tmp/ti-secret.XXXXXX)"; chmod 600 "$f"; printf '%s\n' "$@" >"$f"; echo "$f"; }
# round trips: `step <name> <command...>` runs one command, prints `STEP <name> ok` or `STEP <name> FAIL <detail>` and counts; `finish <proto>` prints the verdict
STEPS=0; BAD=0
step() { local n=$1; shift; local o d; STEPS=$((STEPS+1)); if o="$("$@" 2>&1)"; then echo "STEP $n ok"; else BAD=$((BAD+1)); d="$(printf '%s\n' "$o" | grep -v gencache_init | tr '\n' ' ')"; echo "STEP $n FAIL ${d:0:160}"; [ "${TI_RT_FAILFAST:-0}" != 1 ] || { echo "FAIL roundtrip failfast steps=$STEPS failed=$BAD"; exit 1; }; fi; }   # the harmless smbclient gencache_init warning never fills the 160-character detail that should show the real status
expect_eq() { [ "$1" = "$2" ] || { echo "got '${1:0:80}' want '${2:0:80}'"; return 1; }; }
nonce() { cat /proc/sys/kernel/random/uuid; }
# the manifest sha256 of a corpus file: manifest_sha <relative path>
manifest_sha() { local line; while IFS= read -r line; do if [ "${line:66}" = "./$1" ]; then echo "${line:0:64}"; return 0; fi; done </manifest.sha256; return 1; }
finish() { if [ "$BAD" -eq 0 ] && [ "$STEPS" -gt 0 ]; then echo "PASS roundtrip $1 steps=$STEPS"; exit 0; else echo "FAIL roundtrip $1 steps=$STEPS failed=$BAD"; exit 1; fi; }
