#!/usr/bin/env bash
# test_probe_host.sh - T001 (local host probe). HOST-ONLY: the /dev and /sys device-id checks (R6) hold on the host, not inside a container (there /dev/null is a bind mount). Runs scripts/containers/probe_host.sh through its real
# invocation path (docs/16 section 8.5, local part) and asserts exit status, JSON output and state delta.
# Oracles: the live host (nproc, /proc/meminfo, ulimit) read independently of the script, plus fixtures
# (PROBE_* path overrides) for absent and errored inputs. Exit non-zero on any failure.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REAL_SUT="$HERE/../probe_host.sh"
FAILS=0; PASSES=0
ok()   { PASSES=$((PASSES+1)); echo "PASS: $1"; }
bad()  { FAILS=$((FAILS+1)); echo "FAIL: $1"; }
check(){ if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
command -v jq >/dev/null 2>&1 || { echo "FAIL: jq is required by this test"; exit 2; }

T="$(mktemp -d "${TMPDIR:-/tmp}/probe-test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
# Evidence under the repo is written only when PROBE_TEST_MUTATION_RECORD is passed explicitly (default: scratch dir).
MUT_RECORD="${PROBE_TEST_MUTATION_RECORD:-$T/probe-mutation.txt}"

probe_suite() { # $1 = script under test, sets SUT; runs the whole assertion body
  SUT="$1"
  # every fixture directory starts empty on EVERY invocation (the suite runs once per mutant; a record or temp file left in a shared directory by an
  # earlier mutant would otherwise make the residue checks fail for the wrong reason, round 4)
  rm -rf "${T:?}"/ev[0-9]* "${T:?}"/ev-mm*
  # --- 1. real host, real invocation path ---
  EV="$T/ev1"; mkdir -p "$EV"
  EV="$EV" bash "$SUT" >"$T/out1.txt" 2>&1; rc=$?
  check "real host: exit status 0" "$rc" "0"
  J="$EV/host-probe.json"
  if [ -s "$J" ] && jq -e . "$J" >/dev/null 2>&1; then ok "real host: host-probe.json exists and is valid JSON"; else bad "real host: host-probe.json missing or invalid"; fi
  check "real host: nproc equals independent nproc" "$(jq -r '.nproc.value' "$J" 2>/dev/null)" "$(nproc)"
  mt_kb=$(sed -n 's/^MemTotal:[[:space:]]*\([0-9][0-9]*\)[[:space:]]*kB.*/\1/p' /proc/meminfo | head -n 1); want_mt=$((mt_kb * 1024))
  check "real host: MemTotal bytes equals /proc/meminfo" "$(jq -r '.mem_total.value' "$J" 2>/dev/null)" "$want_mt"
  ma=$(jq -r '.mem_available.value' "$J" 2>/dev/null)
  if [ "${ma:-0}" -gt 0 ] 2>/dev/null; then ok "real host: MemAvailable is a positive integer"; else bad "real host: MemAvailable '$ma'"; fi
  check "real host: ulimit -u equals independent read" "$(jq -r '.ulimit_u.value' "$J" 2>/dev/null)" "$(ulimit -u)"
  cg=$(jq -r '.cgroup_version.value' "$J" 2>/dev/null)
  if [ -f /sys/fs/cgroup/cgroup.controllers ]; then want_cg=2; else want_cg=1; fi
  check "real host: cgroup version" "$cg" "$want_cg"
  if [ "$(id -u)" -ne 0 ]; then want_rl=true; else want_rl=false; fi
  check "real host: rootless flag" "$(jq -r '.rootless.value' "$J" 2>/dev/null)" "$want_rl"
  for f in kvm selinux ulimit_u cgroup_version rootless user_slice_memory_max nproc mem_total mem_available; do
    st=$(jq -r ".$f.status" "$J" 2>/dev/null)
    case "$st" in present|absent|error) ok "field $f has closed status ($st)";; *) bad "field $f status '$st'";; esac
  done
  # control needle: a path that must not exist is read through the same path and recorded absent
  check "control needle: path that must not exist is recorded absent" "$(jq -r '.control_needle.status' "$J" 2>/dev/null)" "absent"
  check "control needle: records the probed path" "$(jq -r '.control_needle.path|length>0' "$J" 2>/dev/null)" "true"

  # --- 2. fixtures: no SELinux anywhere is 'absent', never 'error' ---
  EV2="$T/ev2"; mkdir -p "$EV2"
  EV="$EV2" PROBE_SELINUX_FS="$T/no-such-selinux" PROBE_GETENFORCE_BIN="$T/no-such-getenforce" PROBE_KVM_PATH="$T/no-such-kvm" bash "$SUT" >/dev/null 2>&1; rc=$?
  check "fixture no SELinux/KVM: exit 0" "$rc" "0"
  check "fixture no SELinux: status absent" "$(jq -r '.selinux.status' "$EV2/host-probe.json" 2>/dev/null)" "absent"
  check "fixture no KVM: status absent" "$(jq -r '.kvm.status' "$EV2/host-probe.json" 2>/dev/null)" "absent"

  # --- 3. fixture: present /dev/kvm stand-in is 'present' ---
  EV3="$T/ev3"; mkdir -p "$EV3"; : >"$T/kvm"
  EV="$EV3" PROBE_KVM_PATH="$T/kvm" bash "$SUT" >/dev/null 2>&1
  check "fixture present kvm stand-in: status present" "$(jq -r '.kvm.status' "$EV3/host-probe.json" 2>/dev/null)" "present"

  # --- 4. fixture: an unparsable meminfo is 'error', never 'absent' or a number ---
  EV4="$T/ev4"; mkdir -p "$EV4"; printf 'garbage\n' >"$T/meminfo"
  EV="$EV4" PROBE_MEMINFO="$T/meminfo" bash "$SUT" >/dev/null 2>&1; rc=$?
  check "fixture garbage meminfo: exit 0 (probe records, does not abort)" "$rc" "0"
  check "fixture garbage meminfo: mem_total status error" "$(jq -r '.mem_total.status' "$EV4/host-probe.json" 2>/dev/null)" "error"
  check "fixture garbage meminfo: mem_available status error" "$(jq -r '.mem_available.status' "$EV4/host-probe.json" 2>/dev/null)" "error"
  # a missing meminfo file is also an error (it must exist on any Linux host), not 'absent'
  EV5="$T/ev5"; mkdir -p "$EV5"
  EV="$EV5" PROBE_MEMINFO="$T/no-such-meminfo" bash "$SUT" >/dev/null 2>&1
  check "fixture missing meminfo: status error" "$(jq -r '.mem_total.status' "$EV5/host-probe.json" 2>/dev/null)" "error"

  # --- 5. user.slice memory.max: 'max' and numeric both captured; missing file absent ---
  EV6="$T/ev6"; mkdir -p "$EV6"; echo 8589934592 >"$T/memmax"
  EV="$EV6" PROBE_USER_SLICE_MEMMAX="$T/memmax" bash "$SUT" >/dev/null 2>&1
  check "fixture user.slice memory.max numeric" "$(jq -r '.user_slice_memory_max.value' "$EV6/host-probe.json" 2>/dev/null)" "8589934592"
  EV7="$T/ev7"; mkdir -p "$EV7"
  EV="$EV7" PROBE_USER_SLICE_MEMMAX="$T/no-such-memmax" bash "$SUT" >/dev/null 2>&1
  check "fixture user.slice memory.max missing: absent" "$(jq -r '.user_slice_memory_max.status' "$EV7/host-probe.json" 2>/dev/null)" "absent"


  # --- 6. review F4/F5: fixtures that make constant/hard-coded probes fail ---
  # independent oracle (sed + shell arithmetic, no awk) on a fixture meminfo with known values
  EV8="$T/ev8"; mkdir -p "$EV8"; printf 'MemTotal:        1000 kB\nMemFree:          100 kB\nMemAvailable:     500 kB\n' >"$T/meminfo-known"
  EV="$EV8" PROBE_MEMINFO="$T/meminfo-known" bash "$SUT" >/dev/null 2>&1
  check "fixture meminfo MemTotal 1000 kB = 1024000 bytes" "$(jq -r '.mem_total.value' "$EV8/host-probe.json" 2>/dev/null)" "1024000"
  check "fixture meminfo MemAvailable 500 kB = 512000 bytes" "$(jq -r '.mem_available.value' "$EV8/host-probe.json" 2>/dev/null)" "512000"
  # getenforce that exists but fails -> error, never absent (T001: an errored probe is error)
  printf '#!/usr/bin/env bash\nexit 1\n' >"$T/getenforce-fail"; chmod +x "$T/getenforce-fail"
  printf '#!/usr/bin/env bash\necho Enforcing\n' >"$T/getenforce-ok"; chmod +x "$T/getenforce-ok"
  EV9="$T/ev9"; mkdir -p "$EV9"
  EV="$EV9" PROBE_SELINUX_FS="$T/no-such-selinux" PROBE_GETENFORCE_BIN="$T/getenforce-fail" bash "$SUT" >/dev/null 2>&1
  check "fixture failing getenforce (no selinuxfs): status error, not absent" "$(jq -r '.selinux.status' "$EV9/host-probe.json" 2>/dev/null)" "error"
  EV10="$T/ev10"; mkdir -p "$EV10"; mkdir -p "$T/selinuxfs"
  EV="$EV10" PROBE_SELINUX_FS="$T/selinuxfs" PROBE_GETENFORCE_BIN="$T/getenforce-fail" bash "$SUT" >/dev/null 2>&1
  check "fixture selinuxfs present + failing getenforce: status error (F5)" "$(jq -r '.selinux.status' "$EV10/host-probe.json" 2>/dev/null)" "error"
  EV11="$T/ev11"; mkdir -p "$EV11"
  EV="$EV11" PROBE_SELINUX_FS="$T/selinuxfs" PROBE_GETENFORCE_BIN="$T/getenforce-ok" bash "$SUT" >/dev/null 2>&1
  check "fixture selinuxfs present + working getenforce: present Enforcing" "$(jq -r '.selinux.status+":"+.selinux.value' "$EV11/host-probe.json" 2>/dev/null)" "present:Enforcing"
  # control needle at an EXISTING path must be reported as an error (the instrument can see)
  EV12="$T/ev12"; mkdir -p "$EV12"; : >"$T/needle-exists"
  EV="$EV12" PROBE_NEEDLE_PATH="$T/needle-exists" bash "$SUT" >/dev/null 2>&1
  check "fixture needle path that exists: status error (needle can see)" "$(jq -r '.control_needle.status' "$EV12/host-probe.json" 2>/dev/null)" "error"
  check "fixture needle path that exists: path recorded" "$(jq -r '.control_needle.path' "$EV12/host-probe.json" 2>/dev/null)" "$T/needle-exists"
  # cgroup v1 vs v2 by controllers file
  EV13="$T/ev13"; mkdir -p "$EV13"; : >"$T/cg-controllers"
  EV="$EV13" PROBE_CGROUP_CONTROLLERS="$T/cg-controllers" bash "$SUT" >/dev/null 2>&1
  check "fixture cgroup.controllers present: cgroup_version 2" "$(jq -r '.cgroup_version.value' "$EV13/host-probe.json" 2>/dev/null)" "2"
  EV14="$T/ev14"; mkdir -p "$EV14"
  EV="$EV14" PROBE_CGROUP_CONTROLLERS="$T/no-such-controllers" bash "$SUT" >/dev/null 2>&1
  check "fixture cgroup.controllers absent: cgroup_version 1" "$(jq -r '.cgroup_version.value' "$EV14/host-probe.json" 2>/dev/null)" "1"
  # root / rootless via the test-only PROBE_UID override
  EV15="$T/ev15"; mkdir -p "$EV15"
  EV="$EV15" PROBE_UID=0 bash "$SUT" >/dev/null 2>&1
  check "fixture uid 0: rootless false" "$(jq -r '.rootless.value' "$EV15/host-probe.json" 2>/dev/null)" "false"
  EV16="$T/ev16"; mkdir -p "$EV16"
  EV="$EV16" PROBE_UID=1000 bash "$SUT" >/dev/null 2>&1
  check "fixture uid 1000: rootless true" "$(jq -r '.rootless.value' "$EV16/host-probe.json" 2>/dev/null)" "true"

  # --- 7. review r2 N5/N6/N7: type, digit-guard, ulimit, duplicate-key and dangling-needle fixtures ---
  # N5/P1: numeric memory.max is a JSON number, "max" is a JSON string (jq -r prints both identically, so assert the type)
  check "N5 user.slice memory.max numeric is a JSON number" "$(jq -r '.user_slice_memory_max.value|type' "$EV6/host-probe.json" 2>/dev/null)" "number"
  EV17="$T/ev17"; mkdir -p "$EV17"; echo max >"$T/memmax-max"
  EV="$EV17" PROBE_USER_SLICE_MEMMAX="$T/memmax-max" bash "$SUT" >/dev/null 2>&1
  check "N5 user.slice memory.max 'max' is present" "$(jq -r '.user_slice_memory_max.status' "$EV17/host-probe.json" 2>/dev/null)" "present"
  check "N5 user.slice memory.max 'max' is a JSON string" "$(jq -r '.user_slice_memory_max.value|type' "$EV17/host-probe.json" 2>/dev/null)" "string"
  # N5/P2: a non-numeric MemTotal value is an error, never a number
  EV18="$T/ev18"; mkdir -p "$EV18"; printf 'MemTotal:        abc kB\nMemAvailable:     500 kB\n' >"$T/meminfo-abc"
  EV="$EV18" PROBE_MEMINFO="$T/meminfo-abc" bash "$SUT" >/dev/null 2>&1; rc=$?
  check "N5 MemTotal 'abc kB': exit 0, record written" "$rc/$([ -s "$EV18/host-probe.json" ] && echo yes || echo no)" "0/yes"
  check "N5 MemTotal 'abc kB': mem_total status error" "$(jq -r '.mem_total.status' "$EV18/host-probe.json" 2>/dev/null)" "error"
  check "N5 MemTotal 'abc kB': mem_available still present (500 kB = 512000)" "$(jq -r '.mem_available.status+":"+(.mem_available.value|tostring)' "$EV18/host-probe.json" 2>/dev/null)" "present:512000"
  # N5/P4: non-numeric ulimit -u is an error; unlimited and numbers keep their types (PROBE_ULIMIT_U test override)
  EV19="$T/ev19"; mkdir -p "$EV19"
  EV="$EV19" PROBE_ULIMIT_U=abc bash "$SUT" >/dev/null 2>&1
  check "N5 ulimit -u 'abc': status error" "$(jq -r '.ulimit_u.status' "$EV19/host-probe.json" 2>/dev/null)" "error"
  EV20="$T/ev20"; mkdir -p "$EV20"
  EV="$EV20" PROBE_ULIMIT_U=unlimited bash "$SUT" >/dev/null 2>&1
  check "N5 ulimit -u 'unlimited': present string" "$(jq -r '.ulimit_u.status+":"+(.ulimit_u.value|type)' "$EV20/host-probe.json" 2>/dev/null)" "present:string"
  EV21="$T/ev21"; mkdir -p "$EV21"
  EV="$EV21" PROBE_ULIMIT_U=4096 bash "$SUT" >/dev/null 2>&1
  check "N5 ulimit -u 4096: present number 4096" "$(jq -r '.ulimit_u.status+":"+(.ulimit_u.value|type)+":"+(.ulimit_u.value|tostring)' "$EV21/host-probe.json" 2>/dev/null)" "present:number:4096"
  # N6: a duplicate MemTotal line is an error FIELD; the record is still written and no .tmp is left behind
  EV22="$T/ev22"; mkdir -p "$EV22"; printf 'MemTotal:        1000 kB\nMemTotal:        2000 kB\nMemAvailable:     500 kB\n' >"$T/meminfo-dup"
  EV="$EV22" PROBE_MEMINFO="$T/meminfo-dup" bash "$SUT" >/dev/null 2>&1; rc=$?
  check "N6 duplicate MemTotal: exit 0 and record written" "$rc/$([ -s "$EV22/host-probe.json" ] && echo yes || echo no)" "0/yes"
  check "N6 duplicate MemTotal: mem_total status error" "$(jq -r '.mem_total.status' "$EV22/host-probe.json" 2>/dev/null)" "error"
  check "N6 duplicate MemTotal: mem_available still present" "$(jq -r '.mem_available.status' "$EV22/host-probe.json" 2>/dev/null)" "present"
  check "N6 duplicate MemTotal: no .tmp left behind" "$(ls "$EV22" | grep -c 'tmp')" "0"
  # N7: a dangling symlink at the needle path EXISTS as a link: the needle can see it (error, not absent)
  EV23="$T/ev23"; mkdir -p "$EV23"; ln -s "$T/target-that-does-not-exist" "$T/needle-dangling"
  EV="$EV23" PROBE_NEEDLE_PATH="$T/needle-dangling" bash "$SUT" >/dev/null 2>&1
  check "N7 needle path is a dangling symlink: status error" "$(jq -r '.control_needle.status' "$EV23/host-probe.json" 2>/dev/null)" "error"

  # --- 8. review W3-r3: W4 concurrency / W5 empty+unreadable memory.max + numeric JSON types / W9 positive control ---
  # W4: 16 concurrent probes into ONE evidence dir all succeed, leave a valid record and no temp residue; a reader polling
  # during the burst never sees an invalid (for example 0-byte) host-probe.json.
  EV24="$T/ev24"; mkdir -p "$EV24"; mkdir -p "$T/rc24"
  ( for i in $(seq 1 16); do ( EV="$EV24" bash "$SUT" >/dev/null 2>&1; echo $? >"$T/rc24/$i" ) & done; wait ) &
  WB=$!; badreads=0; reads=0
  while kill -0 "$WB" 2>/dev/null; do
    if [ -e "$EV24/host-probe.json" ]; then reads=$((reads+1)); jq -e . "$EV24/host-probe.json" >/dev/null 2>&1 || badreads=$((badreads+1)); fi
  done
  wait "$WB" 2>/dev/null
  check "W4 16 concurrent probes: all exit 0" "$(cat "$T"/rc24/* 2>/dev/null | grep -cx 0)" "16"
  check "W4 concurrent probes: record is valid JSON" "$(jq -e . "$EV24/host-probe.json" >/dev/null 2>&1 && echo yes || echo no)" "yes"
  check "W4 concurrent probes: no temp residue in the evidence dir" "$(ls -A "$EV24" | grep -vc '^host-probe.json$')" "0"
  check "W4 concurrent probes: reader saw no invalid record" "$badreads" "0"
  # W5/pR1: an existing but empty memory.max is an error, never absent and never a number; an unreadable one too
  EV25="$T/ev25"; mkdir -p "$EV25"; : >"$T/memmax-empty"
  EV="$EV25" PROBE_USER_SLICE_MEMMAX="$T/memmax-empty" bash "$SUT" >/dev/null 2>&1
  check "W5 empty memory.max: status error (not absent)" "$(jq -r '.user_slice_memory_max.status' "$EV25/host-probe.json" 2>/dev/null)" "error"
  if [ "$(id -u)" -ne 0 ]; then
    EV26="$T/ev26"; mkdir -p "$EV26"; echo 4096 >"$T/memmax-unreadable"; chmod 000 "$T/memmax-unreadable"
    EV="$EV26" PROBE_USER_SLICE_MEMMAX="$T/memmax-unreadable" bash "$SUT" >/dev/null 2>&1
    check "W5 unreadable (mode 000) memory.max: status error (not absent)" "$(jq -r '.user_slice_memory_max.status' "$EV26/host-probe.json" 2>/dev/null)" "error"
    chmod 600 "$T/memmax-unreadable"
  else ok "W5 unreadable memory.max fixture skipped: running as root cannot make a file unreadable (UNCONFIRMED for root)"; fi
  # W5/pR2: jq -r prints a number and a string alike, so the JSON type of every numeric field is asserted explicitly
  check "W5 real host nproc is a JSON number" "$(jq -r '.nproc.value|type' "$J" 2>/dev/null)" "number"
  check "W5 real host mem_total is a JSON number" "$(jq -r '.mem_total.value|type' "$J" 2>/dev/null)" "number"
  check "W5 real host mem_available is a JSON number" "$(jq -r '.mem_available.value|type' "$J" 2>/dev/null)" "number"
  check "W5 fixture mem_total 1024000 is a JSON number" "$(jq -r '.mem_total.value|type' "$EV8/host-probe.json" 2>/dev/null)" "number"
  check "W5 fixture mem_available 512000 is a JSON number" "$(jq -r '.mem_available.value|type' "$EV8/host-probe.json" 2>/dev/null)" "number"
  check "W5 real host cgroup_version is a JSON number" "$(jq -r '.cgroup_version.value|type' "$J" 2>/dev/null)" "number"
  # W9: positive control: a path KNOWN to be present, read through the same -e, must be seen; a blind -e would miss it
  check "W9 positive control present on the real host" "$(jq -r '.control_positive.status' "$J" 2>/dev/null)" "present"
  check "W9 positive control records its path" "$(jq -r '.control_positive.path|length>0' "$J" 2>/dev/null)" "true"
  EV27="$T/ev27"; mkdir -p "$EV27"
  EV="$EV27" PROBE_POSITIVE_PATH="$T/no-such-positive" bash "$SUT" >/dev/null 2>&1; rc=$?
  check "W9 positive control missing: exit 0 and record written" "$rc/$([ -s "$EV27/host-probe.json" ] && echo yes || echo no)" "0/yes"
  check "W9 positive control missing: status error (instrument blind, absences not trustworthy)" "$(jq -r '.control_positive.status' "$EV27/host-probe.json" 2>/dev/null)" "error"


  # --- 9. review r3 M3 / M4 / M1 ---
  # M3/Pf: the digit cap: 15 digits are accepted, 16 are an error (kB*1024 would no longer be exact below 2^63)
  EV28="$T/ev28"; mkdir -p "$EV28"; printf 'MemTotal:  1234567890123456 kB\nMemAvailable:     500 kB\n' >"$T/meminfo-16d"
  EV="$EV28" PROBE_MEMINFO="$T/meminfo-16d" bash "$SUT" >/dev/null 2>&1
  check "M3 MemTotal with 16 digits: status error (cap is 15 digits)" "$(jq -r '.mem_total.status' "$EV28/host-probe.json" 2>/dev/null)" "error"
  EV29="$T/ev29"; mkdir -p "$EV29"; printf 'MemTotal:  999999999999999 kB\nMemAvailable:     500 kB\n' >"$T/meminfo-15d"
  EV="$EV29" PROBE_MEMINFO="$T/meminfo-15d" bash "$SUT" >/dev/null 2>&1
  check "M3 MemTotal with 15 digits: status present" "$(jq -r '.mem_total.status' "$EV29/host-probe.json" 2>/dev/null)" "present"
  # M3/Pc: a missing cgroup root is an error (cannot tell v1 from v2), not a version
  EV30="$T/ev30"; mkdir -p "$EV30"
  EV="$EV30" PROBE_CGROUP_ROOT="$T/no-such-cgroup-root" bash "$SUT" >/dev/null 2>&1
  check "M3 missing cgroup root (no controllers file either): cgroup_version status error" "$(jq -r '.cgroup_version.status' "$EV30/host-probe.json" 2>/dev/null)" "error"
  EV31="$T/ev31"; mkdir -p "$EV31"; mkdir -p "$T/cg-root-v1"
  EV="$EV31" PROBE_CGROUP_ROOT="$T/cg-root-v1" bash "$SUT" >/dev/null 2>&1
  check "M3 cgroup root present without controllers file: cgroup_version 1" "$(jq -r '.cgroup_version.status+":"+(.cgroup_version.value|tostring)' "$EV31/host-probe.json" 2>/dev/null)" "present:1"
  EV32="$T/ev32"; mkdir -p "$EV32"; mkdir -p "$T/cg-root-v2"; : >"$T/cg-root-v2/cgroup.controllers"
  EV="$EV32" PROBE_CGROUP_ROOT="$T/cg-root-v2" bash "$SUT" >/dev/null 2>&1
  check "M3 cgroup root with controllers file: cgroup_version 2" "$(jq -r '.cgroup_version.status+":"+(.cgroup_version.value|tostring)' "$EV32/host-probe.json" 2>/dev/null)" "present:2"
  # M4: positive needles of the SAME filesystem class as the absences they certify (devtmpfs for kvm, sysfs for selinuxfs and cgroup)
  check "M4 real host: devtmpfs positive control present" "$(jq -r '.control_positive_dev.status' "$J" 2>/dev/null)" "present"
  check "M4 real host: sysfs positive control present" "$(jq -r '.control_positive_sys.status' "$J" 2>/dev/null)" "present"
  EV33="$T/ev33"; mkdir -p "$EV33"
  EV="$EV33" PROBE_POSITIVE_DEV="$T/no-such-dev-needle" bash "$SUT" >/dev/null 2>&1
  check "M4 devtmpfs needle not seen: control_positive_dev error, sysfs one still present" "$(jq -r '.control_positive_dev.status+":"+.control_positive_sys.status' "$EV33/host-probe.json" 2>/dev/null)" "error:present"
  EV34="$T/ev34"; mkdir -p "$EV34"
  EV="$EV34" PROBE_POSITIVE_SYS="$T/no-such-sys-needle" bash "$SUT" >/dev/null 2>&1
  check "M4 sysfs needle not seen: control_positive_sys error, devtmpfs one still present" "$(jq -r '.control_positive_sys.status+":"+.control_positive_dev.status' "$EV34/host-probe.json" 2>/dev/null)" "error:present"
  # M1: a TERM while the record is being written removes the temp file (the evidence directory is tracked: no residue may be left)
  EV35="$T/ev35"; mkdir -p "$EV35" "$T/jqshim"
  printf '#!/usr/bin/env bash\ncase "$*" in *host-probe/1*) : >"$PROBE_STARTED"; sleep 3;; esac\nexec "%s" "$@"\n' "$(command -v jq)" >"$T/jqshim/jq"; chmod +x "$T/jqshim/jq"
  rm -f "$EV35.started"
  PATH="$T/jqshim:$PATH" EV="$EV35" PROBE_STARTED="$EV35.started" python3 -c 'import os,signal,sys
for s in (signal.SIGINT,signal.SIGTERM,signal.SIGHUP): signal.signal(s,signal.SIG_DFL)
os.execvp("bash",["bash"]+sys.argv[1:])' "$SUT" >"$T/wr.out" 2>"$T/wr.err" &
  WP=$!; wi=0
  while [ ! -e "$EV35.started" ] && [ "$wi" -lt 100 ]; do sleep 0.1; wi=$((wi+1)); done
  check "M1 record write in progress before the signal" "$([ -e "$EV35.started" ] && echo yes || echo no)" "yes"
  kill -TERM "$WP" 2>/dev/null; wrc=0; wait "$WP" 2>/dev/null; wrc=$?
  sleep 1
  check "M1 TERM during the record write: exit status 143" "$wrc" "143"
  check "M1 TERM during the record write: no temp residue and no partial record in the evidence dir" "$(ls -A "$EV35" | grep -c .)" "0"

  # --- 10. round 4 (WF3-REVIEW R5, R6, R9) ---
  # R9: user.slice memory.max is the word "max" or a plain decimal number; every other content is an ERROR (the probe could not interpret it), never "present"
  local mv mvn=0 EVM
  for mv in 'abc' '12 34' '-1' 'max max' ' max' '0123' '1e5' '0x10'; do
    mvn=$((mvn + 1)); EVM="$T/ev-mm$mvn"; mkdir -p "$EVM"; printf '%s\n' "$mv" >"$T/memmax-bad$mvn"
    EV="$EVM" PROBE_USER_SLICE_MEMMAX="$T/memmax-bad$mvn" bash "$SUT" >/dev/null 2>&1
    check "R9 memory.max '$mv': status error (not present)" "$(jq -r '.user_slice_memory_max.status' "$EVM/host-probe.json" 2>/dev/null)" "error"
  done
  EVM="$T/ev-mm0"; mkdir -p "$EVM"; echo 0 >"$T/memmax-zero"
  EV="$EVM" PROBE_USER_SLICE_MEMMAX="$T/memmax-zero" bash "$SUT" >/dev/null 2>&1
  check "R9 memory.max '0': present JSON number 0 (golden-FALSE: a legal value is not refused)" "$(jq -r '.user_slice_memory_max.status+":"+(.user_slice_memory_max.value|type)+":"+(.user_slice_memory_max.value|tostring)' "$EVM/host-probe.json" 2>/dev/null)" "present:number:0"
  # R6: the DEFAULT positive needles are of the filesystem class they certify (read from the real-host record, classified by the kernel's own statfs)
  local devp sysp
  devp="$(jq -r '.control_positive_dev.path' "$J" 2>/dev/null)"; sysp="$(jq -r '.control_positive_sys.path' "$J" 2>/dev/null)"
  # class = the kernel's own answer: the needle sits on the SAME mount as the directory that holds the absence it certifies (/dev for /dev/kvm,
  # /sys for selinuxfs and cgroup), and statfs names that mount's type (a tmpfs scratch directory is also "tmpfs", so the device id is compared too)
  case "$(stat -f -c %T "$devp" 2>/dev/null)" in tmpfs|devtmpfs) ok "R6 default devtmpfs needle '$devp' is on a tmpfs/devtmpfs filesystem";; *) bad "R6 default devtmpfs needle '$devp' is on '$(stat -f -c %T "$devp" 2>/dev/null)', not tmpfs/devtmpfs";; esac
  check "R6 default devtmpfs needle is on the mount that holds /dev/kvm (same device id as /dev)" "$(stat -c %d "$devp" 2>/dev/null)" "$(stat -c %d /dev 2>/dev/null)"
  check "R6 default sysfs needle '$sysp' is on a sysfs filesystem" "$(stat -f -c %T "$sysp" 2>/dev/null)" "sysfs"
  check "R6 default sysfs needle is on the mount that holds /sys/fs (same device id as /sys)" "$(stat -c %d "$sysp" 2>/dev/null)" "$(stat -c %d /sys 2>/dev/null)"
  # R5: a TERM that lands while mktemp runs (a mktemp stand-in sends TERM to the probe, then creates the file) leaves no .host-probe.* residue
  local EV36="$T/ev36" ; mkdir -p "$EV36" "$T/mkshim"
  printf '#!/usr/bin/env bash\nkill -TERM $PPID\nexec "%s" "$@"\n' "$(command -v mktemp)" >"$T/mkshim/mktemp"; chmod +x "$T/mkshim/mktemp"
  PATH="$T/mkshim:$PATH" EV="$EV36" python3 -c 'import os,signal,sys
for s in (signal.SIGINT,signal.SIGTERM,signal.SIGHUP): signal.signal(s,signal.SIG_DFL)
os.execvp("bash",["bash"]+sys.argv[1:])' "$SUT" >"$T/mk.out" 2>"$T/mk.err" &
  local MP=$! mrc=0; wait "$MP" 2>/dev/null; mrc=$?
  sleep 1
  check "R5 TERM while mktemp runs: exit status 143" "$mrc" "143"
  check "R5 TERM while mktemp runs: no .host-probe.* temp residue and no record in the evidence dir" "$(ls -A "$EV36" | grep -c .)" "0"
}

# =================== paired mutations of probe_host.sh: each must make the suite FAIL ===================
{ echo "probe_host paired mutations $(date -u +%Y-%m-%dT%H:%M:%SZ)"; echo "script under test: $REAL_SUT"; } >"$MUT_RECORD"
probe_suite "$REAL_SUT"
REAL_FAILS=$FAILS
mutate() { # $1 name, $2 sed expression
  local f="$T/mut-$1.sh"
  sed -E "$2" "$REAL_SUT" >"$f" 2>/dev/null
  if [ ! -s "$f" ] || cmp -s "$f" "$REAL_SUT"; then bad "mutation $1 not applied (marker missing)"; echo "mutation $1: NOT APPLIED" >>"$MUT_RECORD"; return; fi
  local kf=$FAILS kp=$PASSES
  probe_suite "$f" >"$T/mut-$1.out" 2>&1
  local caught=$((FAILS-kf)); FAILS=$kf; PASSES=$kp
  if [ "$caught" -gt 0 ]; then ok "mutation $1 observed failing ($caught assertion(s) RED)"; echo "mutation $1: CAUGHT, $caught assertion(s) failed: $(grep '^FAIL' "$T/mut-$1.out" | head -3 | tr '\n' ';')" >>"$MUT_RECORD"
  else bad "mutation $1 survived (suite stayed green)"; echo "mutation $1: SURVIVED" >>"$MUT_RECORD"; fi
}
mutate cgroup-hardcoded-2     '/# MUT:cgroup$/c\CG=$(pnum 2)'
mutate rootless-hardcoded-true '/# MUT:rootless$/c\RL=$(jq -nc '"'"'{status:"present",value:true}'"'"')'
mutate needle-hardcoded-absent 's/if \[ -e "\$NEEDLE" \] \|\| \[ -L "\$NEEDLE" \]; then CN=/if false; then CN=/'
mutate getenforce-failure-as-absent 's/S=\$\(errn "getenforce failed"\)/S=$(absn "getenforce failed")/'
mutate selinuxfs-failure-as-present 's/S=\$\(errn "selinuxfs present, getenforce failed"\)/S=$(pres "selinuxfs present, mode unreadable")/'
mutate usermax-number-as-string 's/\*\) US=\$\(pnum "\$v"\);;/*) US=$(pres "$v");;/'
mutate meminfo-digit-guard-removed 's/ && v ~ \/\^\[0-9\]\+\$\///'
mutate ulimit-nonnumeric-as-present 's/UL=\$\(errn "unparsable ulimit -u: \$u"\)/UL=$(pres "$u")/'
mutate meminfo-duplicate-accepted 's/n==1 \&\& //'
mutate needle-symlink-blind 's/ \|\| \[ -L "\$NEEDLE" \]//'
mutate slice-empty-as-absent '/# MUT:slice-error$/c\  else US=$(absn "$SLICE unreadable"); fi'
mutate nproc-as-string 's/NP=\$\(pnum "\$n"\)/NP=$(pres "$n")/'
mutate memtotal-as-string 's/MT=\$\(pnum "\$mt"\)/MT=$(pres "$mt")/'
mutate memavailable-as-string 's/MA=\$\(pnum "\$ma"\)/MA=$(pres "$ma")/'
mutate fixed-tmp-name '/# MUT:tmpname$/c\TMPF="$OUT.tmp"'
mutate positive-control-hardcoded-present 's/if \[ -e "\$POSITIVE" \]; then CP=/if true; then CP=/'
mutate cgroup-missing-root-as-v1 's/elif \[ -d "\$CGROOT" \]; then CG=\$\(pnum 1\)/elif true; then CG=$(pnum 1)/'
mutate meminfo-digit-cap-removed 's/ \&\& length\(v\)<=15//'
mutate positive-dev-hardcoded-present 's/^pos\(\) \{ if \[ -e "\$1" \]; then/pos() { if true; then/'
mutate write-trap-dropped '/# MUT:write-trap$/c\:'
mutate slice-nonnumeric-as-present '/# MUT:slice-class$/c\    case "$v" in *[!0-9]*) US=$(pres "$v");; *) US=$(pnum "$v");; esac'
mutate trap-after-mktemp 's/^TMPF=""$/:/; /# MUT:write-trap$/{h;d}; /# MUT:tmpname$/{G}'
mutate dev-needle-default-wrong-class 's|POSITIVE_DEV="\$\{PROBE_POSITIVE_DEV:-/dev/null\}"|POSITIVE_DEV="${PROBE_POSITIVE_DEV:-${BASH_SOURCE[0]}}"|'
mutate sys-needle-default-wrong-class 's|POSITIVE_SYS="\$\{PROBE_POSITIVE_SYS:-/sys/kernel\}"|POSITIVE_SYS="${PROBE_POSITIVE_SYS:-/}"|'
check "real script: probe suite has zero failures" "$REAL_FAILS" "0"

echo "probe_host tests: $PASSES passed, $FAILS failed"
[ "$FAILS" -eq 0 ]
