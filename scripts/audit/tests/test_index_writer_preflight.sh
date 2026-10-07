#!/usr/bin/env bash
# Revision: 1 | Created: 2026-10-07 | T022 test: floor 20 GiB, heap cap total*60/100, concurrent-writer refusal (real script invocation).
# Usage  bash scripts/audit/tests/test_index_writer_preflight.sh   Env: IWP=<script under test>
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
IWP="${IWP:-scripts/audit/index_writer_preflight.sh}"
T="$(mktemp -d "${TMPDIR:-/tmp}/iwp.XXXXXX")"; trap 'rm -rf "$T"' EXIT
P=0; F=0; ok(){ P=$((P+1)); echo "ok   $1"; }; bad(){ F=$((F+1)); echo "FAIL $1"; }
mi(){ printf 'MemTotal:       %s kB\nMemAvailable:   %s kB\n' "$1" "$2" > "$T/mi"; }
run(){ IWP_MEMINFO="$T/mi" IWP_PROC="${1:-$T/proc}" bash "$IWP" >"$T/out" 2>&1; echo $?; }
mkdir -p "$T/proc"
G=1048576; TOT=$((30*G+200000))   # ~30.2 GiB
mi $TOT $((19*G)); r=$(run); [ "$r" = 1 ] && grep -q 'MemAvailable .* < 20 GiB' "$T/out" && ok "below floor (19 GiB) refused" || bad "below floor refused rc=$r"
mi $TOT $((20*G-1)); r=$(run); [ "$r" = 1 ] && ok "floor-1 kB refused" || bad "floor-1 rc=$r"
mi $TOT $((20*G)); r=$(run); [ "$r" = 0 ] && grep -q PREFLIGHT\ PASS "$T/out" && ok "exactly floor allowed" || bad "at floor rc=$r"
mi $TOT $((25*G)); r=$(run); [ "$r" = 0 ] && ok "above floor allowed" || bad "above floor rc=$r"
# heap: cap = TOT*60/100/1024 MiB; avail/2 = 12800 MiB at 25 GiB (< cap)
exp=$((TOT*60/100/1024)); grep -q "^heap_cap_mb=$exp$" "$T/out" && ok "cap == MemTotal*60/100 ($exp MiB)" || bad "cap value"
grep -q '^heap_mb=12800$' "$T/out" && ok "heap = avail/2 below cap" || bad "heap below cap"
mi $((20*G)) $((30*G)); r=$(run); grep -q "^heap_mb=$((20*G*60/100/1024))$" "$T/out" && ok "heap clamped to 60% cap (synthetic avail>total fixture; real hosts never bind it)" || bad "heap clamp: $(grep heap "$T/out"|tr '\n' ' ')"
mi $((64*G)) $((20*G)); r=$(run); grep -q '^heap_mb=10240$' "$T/out" && grep -q "^heap_cap_mb=$((64*G*60/100/1024))$" "$T/out" && ok "cap follows MemTotal (64 GiB host)" || bad "cap follows total"
# concurrent writer
mi $TOT $((25*G)); mkdir -p "$T/proc/4242"; printf 'codegraph\0index\0--force\0' > "$T/proc/4242/cmdline"
r=$(run); [ "$r" = 1 ] && grep -q 'live indexer pid 4242' "$T/out" && ok "live codegraph index refused" || bad "writer refused rc=$r"
printf 'node\0-e\0codegraph index\0' > "$T/proc/4242/cmdline"
r=$(run); [ "$r" = 0 ] && ok "carrier mentioning codegraph index NOT refused (real argv0)" || bad "carrier false positive rc=$r"
# config error
r=$(IWP_MEMINFO="$T/mi" IWP_PROC="$T/proc" bash "$IWP" --limits "$T/missing" >"$T/out" 2>&1; echo $?); [ "$r" = 2 ] && ok "missing limits file -> rc 2" || bad "config rc=$r"
echo "passed=$P failed=$F"; [ "$F" -eq 0 ]
