#!/usr/bin/env bash
# Revision: 1 | Created: 2026-10-07 | Task: T022 | Owner decision 2026-10-07 (MemAvailable floor 20 GiB)
# Purpose   Consumer-side host preflight for the code-index writer: floor from config/index/writer_limits.yaml,
#           heap = min(MemAvailable/2 floored at heap_floor_mib, MemTotal*pct/100), concurrent-writer refusal.
#           Read-only: opens no database, starts no indexer.
# Usage     scripts/audit/index_writer_preflight.sh [--limits FILE]    exit 0 PASS, 1 FAIL, 2 usage/config error
# Env       TEST-ONLY: IWP_MEMINFO (meminfo file), IWP_PROC (proc root), both default to /proc.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
LIM=config/index/writer_limits.yaml
[ "${1:-}" = "--limits" ] && { LIM="${2:-}"; shift 2; }
[ -r "$LIM" ] || { echo "ERROR limits file unreadable: $LIM"; exit 2; }
MI="${IWP_MEMINFO:-/proc/meminfo}"; PR="${IWP_PROC:-/proc}"
get() { sed -n "s/^$1:[[:space:]]*\\([0-9][0-9]*\\).*/\\1/p" "$LIM" | head -n1; }
floor_gib="$(get min_mem_avail_gib)"; pct="$(get heap_cap_percent_of_total)"; hfloor="$(get heap_floor_mib)"
for v in "$floor_gib" "$pct" "$hfloor"; do case "$v" in ''|*[!0-9]*) echo "ERROR limits file incomplete: $LIM"; exit 2;; esac; done
avail_kb="$(sed -n 's/^MemAvailable:[[:space:]]*\([0-9]*\).*/\1/p' "$MI")"
total_kb="$(sed -n 's/^MemTotal:[[:space:]]*\([0-9]*\).*/\1/p' "$MI")"
case "$avail_kb$total_kb" in ''|*[!0-9]*) echo "ERROR meminfo unreadable"; exit 2;; esac
ok=0
floor_kb=$((floor_gib * 1048576))
echo "mem_avail_kb=$avail_kb"; echo "min_mem_avail_kb=$floor_kb"
[ "$avail_kb" -ge "$floor_kb" ] || { echo "PREFLIGHT FAIL memory: MemAvailable ${avail_kb} kB < ${floor_gib} GiB"; ok=1; }
cap_mb=$((total_kb * pct / 100 / 1024)); heap_mb=$((avail_kb / 1024 / 2))
[ "$heap_mb" -lt "$hfloor" ] && heap_mb=$hfloor
[ "$heap_mb" -gt "$cap_mb" ] && heap_mb=$cap_mb
echo "heap_mb=$heap_mb"; echo "heap_cap_mb=$cap_mb"
# single writer: real cmdline of other processes (not a substring pgrep): argv0 basename codegraph[_safe.sh] with index|sync|init
for d in "$PR"/[0-9]*; do
  [ "${d##*/}" = "$$" ] && continue
  [ -r "$d/cmdline" ] || continue
  c="$(tr '\0' ' ' < "$d/cmdline" 2>/dev/null)" || continue
  read -r a0 _ <<<"$c"; a0="${a0##*/}"
  case "$a0" in codegraph|codegraph_safe.sh) case " $c " in *" index "*|*" sync "*|*" init "*) echo "PREFLIGHT FAIL writer: live indexer pid ${d##*/}: $c"; ok=1;; esac;; esac
done
[ "$ok" -eq 0 ] && echo "PREFLIGHT PASS"
exit "$ok"
