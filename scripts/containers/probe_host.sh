#!/usr/bin/env bash
# probe_host.sh - T001. Local part of the docs/16 section 8.5 host probe.
# Writes $EV/host-probe.json (default EV: specs/001-full-project-audit-remediation/evidence).
# Every field is {status: present|absent|error, value|detail}:
#   present = read and parsed; absent = the thing legitimately does not exist on this host;
#   error   = the probe could not read/parse something that must be readable (never reported as absent).
# Control needles (11.4.201(7)(b)), both read through the same -e path as the probes: a NEGATIVE needle (a path that
# must not exist) recorded absent, and a POSITIVE needle (a path known to be present: this script) recorded present.
# Only the pair certifies the absences: a blind -e would report the negative needle absent too, and misses the positive one.
# The record is written through a unique mktemp name, so concurrent probes into one $EV never clobber each other.
# The positive needle must be of the same filesystem class as the absences it certifies: the script file certifies the -e instrument itself,
# /dev/null (control_positive_dev) certifies the devtmpfs reads (/dev/kvm), /sys/kernel (control_positive_sys) the sysfs reads (selinuxfs, cgroup).
# A TERM/INT/HUP while the temp file is created or the record is written removes the temp file and exits 128+n, so the evidence directory never holds a temp residue.
# Test-only overrides (all default to the real host path): PROBE_KVM_PATH, PROBE_SELINUX_FS,
# PROBE_GETENFORCE_BIN, PROBE_UID, PROBE_NEEDLE_PATH, PROBE_POSITIVE_PATH, PROBE_POSITIVE_DEV, PROBE_POSITIVE_SYS, PROBE_MEMINFO,
# PROBE_USER_SLICE_MEMMAX, PROBE_CGROUP_ROOT, PROBE_CGROUP_CONTROLLERS, PROBE_ULIMIT_U.
# Exit 0 whenever the record was written; non-zero only when it cannot be written.
set -u
command -v jq >/dev/null 2>&1 || { echo "probe_host: jq is required" >&2; exit 2; }
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
EV="${EV:-$ROOT/specs/001-full-project-audit-remediation/evidence}"
KVM="${PROBE_KVM_PATH:-/dev/kvm}"
SEFS="${PROBE_SELINUX_FS:-/sys/fs/selinux}"
GETENF="${PROBE_GETENFORCE_BIN:-getenforce}"
MEMINFO="${PROBE_MEMINFO:-/proc/meminfo}"
SLICE="${PROBE_USER_SLICE_MEMMAX:-/sys/fs/cgroup/user.slice/memory.max}"
CGROOT="${PROBE_CGROUP_ROOT:-/sys/fs/cgroup}"
CGCTL="${PROBE_CGROUP_CONTROLLERS:-$CGROOT/cgroup.controllers}"
NEEDLE="${PROBE_NEEDLE_PATH:-/nonexistent/catalogizer-probe-needle-$$}"
POSITIVE="${PROBE_POSITIVE_PATH:-${BASH_SOURCE[0]}}"
POSITIVE_DEV="${PROBE_POSITIVE_DEV:-/dev/null}"
POSITIVE_SYS="${PROBE_POSITIVE_SYS:-/sys/kernel}"

pres() { jq -nc --arg v "$1" '{status:"present",value:$v}'; }
pnum() { jq -nc --argjson v "$1" '{status:"present",value:$v}'; }
absn() { jq -nc --arg d "$1" '{status:"absent",detail:$d}'; }
errn() { jq -nc --arg d "$1" '{status:"error",detail:$d}'; }

# kvm
if [ -e "$KVM" ]; then K=$(pres "$KVM"); else K=$(absn "$KVM does not exist"); fi

# selinux: filesystem mount or getenforce; neither -> absent (not error)
if [ -d "$SEFS" ]; then
  if v=$("$GETENF" 2>/dev/null); then S=$(pres "$v"); else S=$(errn "selinuxfs present, getenforce failed"); fi
elif command -v "$GETENF" >/dev/null 2>&1 || [ -x "$GETENF" ]; then
  if v=$("$GETENF" 2>/dev/null); then S=$(pres "$v"); else S=$(errn "getenforce failed"); fi
else
  S=$(absn "no selinuxfs and no getenforce")
fi

# ulimit -u (PROBE_ULIMIT_U replaces the real read in tests; the real-host test still compares against `ulimit -u`)
if [ -n "${PROBE_ULIMIT_U+x}" ]; then u="$PROBE_ULIMIT_U"; ur=0; else u=$(ulimit -u 2>/dev/null); ur=$?; fi
if [ "$ur" -eq 0 ] && [ -n "$u" ]; then
  case "$u" in unlimited) UL=$(pres unlimited);; *[!0-9]*) UL=$(errn "unparsable ulimit -u: $u");; *) UL=$(pnum "$u");; esac
else UL=$(errn "ulimit -u unreadable"); fi

# cgroup version
if [ -f "$CGCTL" ]; then CG=$(pnum 2); elif [ -d "$CGROOT" ]; then CG=$(pnum 1); else CG=$(errn "$CGROOT unreadable"); fi   # MUT:cgroup

# rootless: effective uid
PUID="${PROBE_UID:-$(id -u)}"
case "$PUID" in ""|*[!0-9]*) RL=$(errn "unparsable uid: $PUID");; 0) RL=$(jq -nc '{status:"present",value:false}');; *) RL=$(jq -nc '{status:"present",value:true}');; esac   # MUT:rootless

# user.slice memory.max ("max" kept as a string, numeric kept as a number)
if [ -e "$SLICE" ]; then
  if v=$(cat "$SLICE" 2>/dev/null) && [ -n "$v" ]; then
    # cgroup v2 writes either the word "max" or a plain decimal number; anything else is a value the probe cannot interpret: error, never "present" (round 4, R9)
    case "$v" in max) US=$(pres max);; *[!0-9]*|0?*) US=$(errn "unparsable memory.max: $v");; *) US=$(pnum "$v");; esac   # MUT:slice-class
  else US=$(errn "$SLICE unreadable"); fi   # MUT:slice-error
else US=$(absn "$SLICE does not exist"); fi

# nproc
if n=$(nproc 2>/dev/null) && [ -n "$n" ]; then NP=$(pnum "$n"); else NP=$(errn "nproc failed"); fi

# MemTotal / MemAvailable (kB in meminfo -> bytes). Must exist and parse on any Linux host.
# Exactly one line must carry the field (a duplicate is ambiguous -> the field is an error, the record is still written);
# the value is digits only and at most 15 digits, so kB*1024 is exact in awk doubles (a power-of-two multiple) and below 2^63.
meminfo_field() { # $1 field; prints bytes or nothing
  awk -v f="$1:" '$1==f {n++; v=$2} END{ if (n==1 && v ~ /^[0-9]+$/ && length(v)<=15) printf "%.0f\n", v*1024; else exit 1 }' "$MEMINFO" 2>/dev/null
}
if mt=$(meminfo_field MemTotal); then MT=$(pnum "$mt"); else MT=$(errn "MemTotal not parsable (missing, duplicated or non-numeric) in $MEMINFO"); fi
if ma=$(meminfo_field MemAvailable); then MA=$(pnum "$ma"); else MA=$(errn "MemAvailable not parsable (missing, duplicated or non-numeric) in $MEMINFO"); fi

# control needle
if [ -e "$NEEDLE" ] || [ -L "$NEEDLE" ]; then CN=$(jq -nc --arg p "$NEEDLE" '{status:"error",path:$p,detail:"needle path exists; instrument cannot be trusted"}')
else CN=$(jq -nc --arg p "$NEEDLE" '{status:"absent",path:$p,detail:"expected absence confirmed"}'); fi

# positive control: a path known to be present must be seen by the same -e; if not, the instrument is blind
if [ -e "$POSITIVE" ]; then CP=$(jq -nc --arg p "$POSITIVE" '{status:"present",path:$p}')
else CP=$(jq -nc --arg p "$POSITIVE" '{status:"error",path:$p,detail:"positive control path is not seen; instrument blind, recorded absences are not trustworthy"}'); fi

# same-class positive controls: the dev and sys needles are read through the same -e as the kvm / selinuxfs / cgroup absences they certify
pos() { if [ -e "$1" ]; then jq -nc --arg p "$1" '{status:"present",path:$p}'; else jq -nc --arg p "$1" --arg c "$2" '{status:"error",path:$p,detail:("positive control of the " + $c + " filesystem class is not seen; instrument blind there, recorded absences on it are not trustworthy")}'; fi; }
CPD=$(pos "$POSITIVE_DEV" devtmpfs); CPS=$(pos "$POSITIVE_SYS" sysfs)

mkdir -p "$EV" || exit 3
OUT="$EV/host-probe.json"
TMPF=""
# The trap is installed BEFORE mktemp: a signal that lands while mktemp runs is handled after the substitution has assigned TMPF, so the temp file
# is removed (round 4, R5; installed after mktemp it left .host-probe.XXXXXX in the tracked evidence directory).
trap 'rm -f "$TMPF"; exit 143' TERM; trap 'rm -f "$TMPF"; exit 130' INT; trap 'rm -f "$TMPF"; exit 129' HUP   # MUT:write-trap
TMPF="$(mktemp "$EV/.host-probe.XXXXXX")" || exit 3   # MUT:tmpname
jq -n --argjson kvm "$K" --argjson selinux "$S" --argjson ulimit_u "$UL" --argjson cgroup_version "$CG" \
  --argjson rootless "$RL" --argjson user_slice_memory_max "$US" --argjson nproc "$NP" \
  --argjson mem_total "$MT" --argjson mem_available "$MA" --argjson control_needle "$CN" --argjson control_positive "$CP" --argjson control_positive_dev "$CPD" --argjson control_positive_sys "$CPS" \
  --arg host "$(hostname 2>/dev/null)" --arg at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  '{schema:"host-probe/1",scope:"local",host:$host,probed_at:$at,kvm:$kvm,selinux:$selinux,ulimit_u:$ulimit_u,
    cgroup_version:$cgroup_version,rootless:$rootless,user_slice_memory_max:$user_slice_memory_max,
    nproc:$nproc,mem_total:$mem_total,mem_available:$mem_available,control_needle:$control_needle,control_positive:$control_positive,
    control_positive_dev:$control_positive_dev,control_positive_sys:$control_positive_sys}' >"$TMPF" \
  && chmod 0644 "$TMPF" && mv "$TMPF" "$OUT" || { rm -f "$TMPF"; exit 3; }
echo "probe_host: wrote $OUT"
