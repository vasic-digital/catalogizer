#!/usr/bin/env bash
# test_host_checklist.sh - WP-10 / T100 (local-host checklist, docs/16 section 9.5). Drives scripts/containers/host_checklist.sh through its real
# invocation path with podman, df, nproc and timedatectl shims first on PATH and scratch files for every /proc, /sys and /etc input.
# HOST_CHECKLIST_SUT overrides the script under test (the RED run points it at an absent path). A real-host leg runs the unshimmed
# script. Paired mutations (a copy of the script with one marked behaviour changed) must each make at least one fixture FAIL;
# HOST_CHECKLIST_MUTATION_RECORD names the mutation record file (default: scratch). Exit non-zero on any failure.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SUT_REAL="$HERE/../host_checklist.sh"
SUT="${HOST_CHECKLIST_SUT:-$SUT_REAL}"
FAILS=0; PASSES=0
ok(){ PASSES=$((PASSES+1)); [ "${QUIET:-0}" = 1 ] || echo "PASS: $1"; }
bad(){ FAILS=$((FAILS+1)); echo "FAIL: $1"; }
check(){ if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
command -v jq >/dev/null 2>&1 || { echo "FAIL: jq is required by this test"; exit 2; }
T="$(mktemp -d "${TMPDIR:-/tmp}/hc-test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
MUT_RECORD="${HOST_CHECKLIST_MUTATION_RECORD:-$T/mutation.txt}"; : >"$MUT_RECORD"
SH="$T/shims"; mkdir -p "$SH"
cat >"$SH/podman" <<'S'
#!/usr/bin/env bash
echo "podman $*" >>"$SHIM_LOG"
case "$1" in
  --version) [ -n "${SHIM_PODMAN_VERSION_FAIL:-}" ] && exit 1; echo "podman version 9.9.9";;
  info) case "$*" in *Rootless*) [ -n "${SHIM_ROOTLESS_FAIL:-}" ] && exit 1; echo "${SHIM_ROOTLESS:-true}";; *GraphRoot*) echo "${SHIM_GRAPHROOT-/graph}";; *) exit 1;; esac;;
  image) ref="${@: -1}"; line=$(grep -F "$ref|" "$SHIM_IMGDB" 2>/dev/null | head -n 1) || exit 1; [ -n "$line" ] || exit 1; echo "${line#*|}";;
  *) exit 1;;
esac
S
cat >"$SH/df" <<'S'
#!/usr/bin/env bash
if [ "$#" -ne 3 ] || [ "$1" != "-B1" ] || [ "$2" != "--output=avail" ]; then echo "df shim: unexpected args $*" >&2; echo garbage; exit 1; fi
echo Avail; if [ "$3" = "${SHIM_GRAPHROOT-/graph}" ]; then [ -n "${SHIM_DF_GRAPH_FAIL:-}" ] && exit 1; echo "${SHIM_DF_GRAPH:-500000}"; else [ -n "${SHIM_DF_REPO_FAIL:-}" ] && exit 1; echo "${SHIM_DF_REPO:-500000}"; fi
S
cat >"$SH/nproc" <<'S'
#!/usr/bin/env bash
[ -n "${SHIM_NPROC_FAIL:-}" ] && exit 1; echo "${SHIM_NPROC:-8}"
S
cat >"$SH/timedatectl" <<'S'
#!/usr/bin/env bash
[ -n "${SHIM_NTP_FAIL:-}" ] && exit 1; [ "$*" = "show -p NTPSynchronized --value" ] || exit 1; echo "${SHIM_NTP:-yes}"
S
REAL_PY="$(command -v python3)"; export REAL_PY
cat >"$SH/python3" <<'S'
#!/usr/bin/env bash
[ -n "${SHIM_PY_FAIL:-}" ] && exit 1; exec "$REAL_PY" "$@"
S
chmod +x "$SH"/*

# fixture builder: writes a healthy world under $1
mkworld() {
  local w="$1"; rm -rf "$w"; mkdir -p "$w/cg" "$w/ev" "$w/repo"
  : >"$w/kvm"; echo "0-9" >"$w/cg/cgroup.controllers"; echo 100 >"$w/userns"
  echo "tester:100000:65536" >"$w/subuid"; echo "tester:100000:65536" >"$w/subgid"; printf '#!/bin/sh\n' >"$w/newuidmap"; chmod +x "$w/newuidmap"
  printf 'MemTotal:       16000000 kB\nMemAvailable:    8000000 kB\n' >"$w/meminfo"; echo "0.10 0.20 0.30 1/200 1234" >"$w/loadavg"
  printf 'min_free_bytes=1000\n' >"$w/conf"
  cat >"$w/lock" <<L
schema: 1
images:
- id: IMG-A
  reference: example.invalid/a
  tag_intent: t1
  digest: $DA
  platform_digest: $DA1
  size_bytes: 1
- id: IMG-B
  reference: example.invalid/b
  tag_intent: t2
  digest: $DB
  platform_digest: $DB1
  size_bytes: 1
L
  printf 'example.invalid/a:t1|'"$DA1"' example.invalid/a@'"$DA"' \nexample.invalid/b:t2|'"$DB1"' example.invalid/b@'"$DB"' \n' >"$w/imgdb"
}
hx() { printf "$1%.0s" $(seq 64); }   # a well-formed digest needs exactly 64 lowercase hex characters (round 6, F6)
DA="sha256:$(hx a)"; DA1="sha256:$(hx 1)"; DB="sha256:$(hx b)"; DB1="sha256:$(hx 2)"
W="$T/w"
# run_hc <script> [ENV=VAL ...] -- [args]; sets RC OUT(json path) ERRF
run_hc() {
  local script="$1"; shift; local envs=()
  while [ "$1" != "--" ]; do envs+=("$1"); shift; done; shift
  rm -rf "$W/ev"; mkdir -p "$W/ev"; : >"$T/shim.log"; RC=0
  env PATH="$SH:$PATH" SHIM_LOG="$T/shim.log" SHIM_IMGDB="$W/imgdb" EV="$W/ev" \
    CHK_KVM="$W/kvm" CHK_USERNS="$W/userns" CHK_SUBUID="$W/subuid" CHK_SUBGID="$W/subgid" CHK_NEWUIDMAP="$W/newuidmap" CHK_MEMINFO="$W/meminfo" \
    CHK_LOADAVG="$W/loadavg" CHK_LOCK="$W/lock" CHK_CONF="$W/conf" CHK_USER=tester CHK_ULIMIT_U=100000 CHK_ULIMIT_N=1048576 CHK_CGROOT="$W/cg" CHK_REPO_PATH="$W/repo" \
    "${envs[@]}" timeout 30 bash "$script" "$@" >"$T/o.txt" 2>"$T/e.txt" || RC=$?
  OUT="$W/ev/wp10/host-checklist.json"
}
iv() { jq -r --arg id "$1" '.items[]|select(.id|startswith($id))|.verdict' "$OUT" 2>/dev/null; }
is() { jq -r --arg id "$1" '.items[]|select(.id|startswith($id))|.status' "$OUT" 2>/dev/null; }

# ---- A. baseline
mkworld "$W"; run_hc "$SUT" -- 
check "A1 baseline exit 0" "$RC" 0
check "A2 baseline record written" "$([ -f "$OUT" ] && echo y || echo n)" y
check "A3 baseline schema" "$(jq -r .schema "$OUT" 2>/dev/null)" host-checklist/1
check "A4 19 unique item ids" "$(jq -r '[.items[].id]|unique|length' "$OUT" 2>/dev/null)" 19
for c in C02 C03 C04 C05 C06 C07 C08 C09 C10 C11 C12 C13 C16; do check "A5 baseline $c pass" "$(iv $c)" pass; done
for c in C01 C17 C18 C19; do check "A6 $c verdict na" "$(iv $c)" na; check "A6 $c status not_applicable_local_host" "$(is $c)" not_applicable_local_host; check "A6 $c cites owner decision" "$(jq -r --arg id $c '.items[]|select(.id|startswith($id))|.detail|test("owner decision")' "$OUT")" true; done
for c in C14 C15; do check "A7 $c unconfirmed" "$(iv $c)" unconfirmed; done
check "A8 summary pass count" "$(jq -r .summary.pass "$OUT")" 13
check "A9 summary fail count" "$(jq -r .summary.fail "$OUT")" 0
check "A10 control negative absent" "$(jq -r .control.negative.status "$OUT")" absent
check "A11 control positive present" "$(jq -r .control.positive.status "$OUT")" present
check "A12 lock parser saw 2 images" "$(jq -r .control.lock_parser_images_seen "$OUT")" 2
check "A13 identity script sha" "$(jq -r .identity.script_sha256 "$OUT")" "$(sha256sum "$SUT" | cut -d' ' -f1)"
check "A14 identity lock sha" "$(jq -r .identity.lock_sha256 "$OUT")" "$(sha256sum "$W/lock" | cut -d' ' -f1)"
check "A15 no pull and no network verb used" "$(grep -cE 'podman (pull|run|build|push|login)' "$T/shim.log")" 0
check "A16 no temp residue" "$(ls -A "$W/ev/wp10" | grep -c '^\.host-checklist\.')" 0
check "A17 values: mem_total bytes" "$(jq -r '.items[]|select(.id|startswith("C05"))|.value.mem_total' "$OUT")" 16384000000
check "A18 values: disk margins" "$(jq -r '.items[]|select(.id|startswith("C12"))|.value.graphroot_margin_bytes' "$OUT")" 499000
check "A19 images both present_pinned" "$(jq -r '.items[]|select(.id|startswith("C13"))|.value|map(.status)|unique|join(",")' "$OUT")" present_pinned

# ---- B. usage and strict
run_hc "$SUT" -- --bogus; check "B1 unknown arg exit 2" "$RC" 2
mkworld "$W"; run_hc "$SUT" SHIM_NTP=no -- --strict; check "B2 --strict with a fail exits 1" "$RC" 1
run_hc "$SUT" SHIM_NTP=no --; check "B3 default exit 0 with a fail" "$RC" 0
mkworld "$W"; run_hc "$SUT" -- --strict; check "B4 --strict clean exits 0" "$RC" 0

# ---- C. one broken input per check: that item fails, and ONLY that item changes verdict
only_fails() { # <C-id> ; every other pass-item still pass
  local id="$1" other
  other=$(jq -r --arg id "$id" '[.items[]|select((.id|startswith($id)|not) and .verdict=="fail")|.id]|join(",")' "$OUT" 2>/dev/null)
  case "$id" in C10|C11) other=$(printf '%s' "$other" | sed 's/^C12_disk_headroom_rule,\?//;s/,C12_disk_headroom_rule//');; esac   # C12 legitimately depends on C10 and C11
  check "$id collateral fails" "$other" ""
}
fx() { # <C-id> <label> <expect-verdict> [ENV...]   (world is rebuilt healthy first; callers adjust files before via PRE)
  local id="$1" lab="$2" want="$3"; shift 3
  run_hc "$SUT" "$@" --; check "C $id $lab" "$(iv $id)" "$want"; only_fails "$id"
}
mkworld "$W"; fx C02 "podman --version fails" fail SHIM_PODMAN_VERSION_FAIL=1
mkworld "$W"; fx C03 "rootless=false" fail SHIM_ROOTLESS=false
mkworld "$W"; fx C03 "rootless unparsable" fail SHIM_ROOTLESS=maybe
mkworld "$W"; fx C03 "podman info fails" fail SHIM_ROOTLESS_FAIL=1
mkworld "$W"; rm "$W/cg/cgroup.controllers"; fx C04 "cgroup v1 (no controllers file)" fail
mkworld "$W"; fx C04 "cgroot unreadable" fail CHK_CGROOT="$W/nope"
mkworld "$W"; printf 'MemTotal: abc kB\n' >"$W/meminfo"; fx C05 "meminfo unparsable" fail
mkworld "$W"; fx C06 "nproc prints 0" fail SHIM_NPROC=0
mkworld "$W"; fx C06 "nproc fails" fail SHIM_NPROC_FAIL=1
mkworld "$W"; fx C07 "kvm absent" fail CHK_KVM="$W/nokvm"
mkworld "$W"; chmod 000 "$W/kvm"; fx C07 "kvm present but not rw" fail
mkworld "$W"; : >"$W/subuid"; fx C08 "no subuid range" fail
mkworld "$W"; echo 0 >"$W/userns"; fx C08 "max_user_namespaces=0" fail
mkworld "$W"; fx C08 "no newuidmap" fail CHK_NEWUIDMAP="$W/nonewuidmap"
mkworld "$W"; fx C09 "ulimit -u 1000" fail CHK_ULIMIT_U=1000
mkworld "$W"; fx C09 "ulimit unreadable" fail CHK_ULIMIT_N=oops
mkworld "$W"; fx C10 "graphroot empty" fail SHIM_GRAPHROOT=
mkworld "$W"; fx C10 "df graphroot fails" fail SHIM_DF_GRAPH_FAIL=1
mkworld "$W"; fx C11 "df repo fails" fail SHIM_DF_REPO_FAIL=1
mkworld "$W"; fx C12 "graphroot below min" fail SHIM_DF_GRAPH=900
mkworld "$W"; fx C12 "repo below min (graphroot fine)" fail SHIM_DF_REPO=900
mkworld "$W"; printf 'min_free_bytes=1000\nmin_free_bytes=2000\n' >"$W/conf"; fx C12 "duplicate min_free_bytes" fail
mkworld "$W"; fx C12 "exactly at the minimum passes" pass SHIM_DF_GRAPH=1000 SHIM_DF_REPO=1000
mkworld "$W"; printf 'example.invalid/b:t2|'"$DB1"' example.invalid/b@'"$DB"' \n' >"$W/imgdb"; fx C13 "image a absent" fail
mkworld "$W"; printf 'example.invalid/a:t1|sha256:zzzz example.invalid/a@sha256:yyyy \nexample.invalid/b:t2|'"$DB1"' example.invalid/b@'"$DB"' \n' >"$W/imgdb"; fx C13 "image a digest mismatch" fail
mkworld "$W"; : >"$W/lock"; fx C13 "empty lock (parser blind)" fail
# N1 (round 5): a lock entry with an absent, empty or malformed digest field must FAIL C13, never match every image
mklock2() { # <digest-line-or-empty> <platform-digest-line-or-empty> for IMG-A; IMG-B stays complete
  { printf 'schema: 1\nimages:\n- id: IMG-A\n  reference: example.invalid/a\n  tag_intent: t1\n'; [ -z "$1" ] || printf '%s\n' "$1"; [ -z "$2" ] || printf '%s\n' "$2"
    printf '  size_bytes: 1\n- id: IMG-B\n  reference: example.invalid/b\n  tag_intent: t2\n  digest: '"$DB"'\n  platform_digest: '"$DB1"'\n  size_bytes: 1\n'; } >"$W/lock"; }
mkworld "$W"; mklock2 "" ""; fx C13 "N1 lock entry with neither digest nor platform_digest" fail
check "N1 ...and its status is lock_malformed, not present_pinned" "$(jq -r '.items[]|select(.id|startswith("C13"))|.value[]|select(.id=="IMG-A")|.status' "$OUT")" lock_malformed
mkworld "$W"; mklock2 "  digest:" "  platform_digest: $DA1"; fx C13 "N1 empty digest value (platform_digest alone is not enough)" fail
mkworld "$W"; mklock2 "  digest: garbage" "  platform_digest: $DA1"; fx C13 "N1 malformed digest (no sha256: prefix)" fail
mkworld "$W"; mklock2 "  digest: sha256:" "  platform_digest: $DA1"; fx C13 "N1 digest with an empty hex part" fail
mkworld "$W"; mklock2 "  digest: sha256:ZZZZ" "  platform_digest: $DA1"; fx C13 "N1 digest with non-hex characters" fail
mkworld "$W"; mklock2 "  digest: $DA" "  platform_digest: nonsense"; fx C13 "N1 malformed platform_digest" fail
mkworld "$W"; mklock2 "  digest: $DA" ""; printf 'example.invalid/a:t1|sha256:zzzz example.invalid/a@sha256:yyyy \nexample.invalid/b:t2|'"$DB1"' example.invalid/b@'"$DB"' \n' >"$W/imgdb"; fx C13 "N1 no platform_digest and the local image does not match the digest" fail
mkworld "$W"; mklock2 "  digest: $DA" ""; fx C13 "N1 guard against false refusal: no platform_digest but the digest matches the local image" pass

# ---- F3 / F6 (round 6): the lock is read with PyYAML (the reader run_pinned.sh uses); every field is validated, fail closed; a digest is exactly sha256:<64 lowercase hex>
imgstatus() { jq -r --arg i "$1" '.items[]|select(.id|startswith("C13"))|.value[]?|select(.id==$i)|.status' "$OUT" 2>/dev/null; }
seen() { jq -r .control.lock_parser_images_seen "$OUT" 2>/dev/null; }
IMGB_YAML="- id: IMG-B
  reference: example.invalid/b
  tag_intent: t2
  digest: $DB
  platform_digest: $DB1
  size_bytes: 1"
lock_raw() { printf 'schema: 1\nimages:\n%s\n' "$1" >"$W/lock"; }   # $1 = the images list body
# F3-1 (probe H6): an entry with NO digest key whose tag_intent carries `|<digest>` used to shift the field and read as present_pinned
mkworld "$W"; lock_raw "- id: IMG-A
  reference: example.invalid/a
  tag_intent: t1|$DA
  size_bytes: 1
$IMGB_YAML"; fx C13 "F3 field shift (digest key absent, tag_intent t1|<digest>) fails" fail
check "F3 ...and the shifted entry is lock_malformed, never present_pinned" "$(imgstatus IMG-A)" lock_malformed
mkworld "$W"; lock_raw "- id: IMG-A
  reference: example.invalid/a
  tag_intent: t1|$DA
  size_bytes: 1
$IMGB_YAML"; run_hc "$SUT" -- --strict; check "F3 ...and --strict exits 1" "$RC" 1
# F3-2 (probe H1): an entry whose first key is not `id` is valid YAML; it must be SEEN and checked, not fused into its neighbour
mkworld "$W"; lock_raw "- id: IMG-A
  reference: example.invalid/a
  tag_intent: t1
  digest: $DA
  platform_digest: $DA1
  size_bytes: 1
- reference: example.invalid/b
  id: IMG-B
  tag_intent: t2
  digest: $DB
  platform_digest: $DB1
  size_bytes: 1"; printf 'example.invalid/a:t1|%s example.invalid/a@%s \n' "$DA1" "$DA" >"$W/imgdb"
fx C13 "F3 entry with id not first and absent locally fails (was invisible)" fail
check "F3 ...both entries are counted by the parser" "$(seen)" 2
check "F3 ...IMG-B is checked: absent" "$(imgstatus IMG-B)" absent
mkworld "$W"; lock_raw "- id: IMG-A
  reference: example.invalid/a
  tag_intent: t1
  digest: $DA
  platform_digest: $DA1
  size_bytes: 1
- reference: example.invalid/b
  id: IMG-B
  tag_intent: t2
  digest: $DB
  platform_digest: $DB1
  size_bytes: 1"; fx C13 "F3 guard against false refusal: id-not-first entries that are present and pinned pass" pass
check "F3 ...both counted" "$(seen)" 2
# F3-3..: malformed entries are lock_malformed and counted
mkworld "$W"; lock_raw "- id: IMG-A
  reference: example.invalid/a
  tag_intent: t1
  digest: $DA
  platform_digest: $DA1
- reference: example.invalid/b
  tag_intent: t2
  digest: $DB
  platform_digest: $DB1"; fx C13 "F3 entry with no id at all fails" fail
check "F3 ...counted (2 entries seen)" "$(seen)" 2
mkworld "$W"; lock_raw "$IMGB_YAML
- id: IMG-B
  reference: example.invalid/a
  tag_intent: t1
  digest: $DA
  platform_digest: $DA1"; fx C13 "F3 duplicate id fails (run_pinned.sh refuses lock_duplicate_id too)" fail
mkworld "$W"; lock_raw "$IMGB_YAML
- just a string"; fx C13 "F3 a list entry that is not a mapping fails" fail
mk_a() { # <id> <reference> <tag_intent> <digest> <platform_digest>: raw YAML values for entry A (IMG-B stays healthy)
  lock_raw "- id: $1
  reference: $2
  tag_intent: $3
  digest: $4
  platform_digest: $5
  size_bytes: 1
$IMGB_YAML"; }
mkworld "$W"; mk_a IMG-A example.invalid/a t1 "$DA" "$DA1"; fx C13 "F3 control: the mk_a builder with healthy values passes" pass
mkworld "$W"; mk_a IMG-A '"example.invalid/a b"' t1 "$DA" "$DA1"; fx C13 "F3 malformed field fails: reference with a space" fail
check "F3 ...and the entry is lock_malformed" "$(imgstatus "$(jq -r '.items[]|select(.id|startswith("C13"))|.value[]|select(.status=="lock_malformed")|.id' "$OUT" | head -n 1)")" lock_malformed
mkworld "$W"; mk_a IMG-A '"example.invalid/a@sha256:abc"' t1 "$DA" "$DA1"; fx C13 "F3 malformed field fails: reference carrying @digest" fail
check "F3 ...and the entry is lock_malformed" "$(imgstatus "$(jq -r '.items[]|select(.id|startswith("C13"))|.value[]|select(.status=="lock_malformed")|.id' "$OUT" | head -n 1)")" lock_malformed
mkworld "$W"; mk_a IMG-A '"-example.invalid/a"' t1 "$DA" "$DA1"; fx C13 "F3 malformed field fails: reference with a leading dash" fail
check "F3 ...and the entry is lock_malformed" "$(imgstatus "$(jq -r '.items[]|select(.id|startswith("C13"))|.value[]|select(.status=="lock_malformed")|.id' "$OUT" | head -n 1)")" lock_malformed
mkworld "$W"; mk_a IMG-A '"example.invalid"' t1 "$DA" "$DA1"; fx C13 "F3 malformed field fails: reference without a path" fail
check "F3 ...and the entry is lock_malformed" "$(imgstatus "$(jq -r '.items[]|select(.id|startswith("C13"))|.value[]|select(.status=="lock_malformed")|.id' "$OUT" | head -n 1)")" lock_malformed
mkworld "$W"; mk_a IMG-A example.invalid/a '"t 1"' "$DA" "$DA1"; fx C13 "F3 malformed field fails: tag_intent with a space" fail
check "F3 ...and the entry is lock_malformed" "$(imgstatus "$(jq -r '.items[]|select(.id|startswith("C13"))|.value[]|select(.status=="lock_malformed")|.id' "$OUT" | head -n 1)")" lock_malformed
mkworld "$W"; mk_a IMG-A example.invalid/a '"t1|x"' "$DA" "$DA1"; fx C13 "F3 malformed field fails: tag_intent with a pipe" fail
check "F3 ...and the entry is lock_malformed" "$(imgstatus "$(jq -r '.items[]|select(.id|startswith("C13"))|.value[]|select(.status=="lock_malformed")|.id' "$OUT" | head -n 1)")" lock_malformed
mkworld "$W"; mk_a IMG-A example.invalid/a 1.20 "$DA" "$DA1"; fx C13 "F3 malformed field fails: unquoted tag_intent 1.20 (YAML float, would silently read 1.2)" fail
check "F3 ...and the entry is lock_malformed" "$(imgstatus "$(jq -r '.items[]|select(.id|startswith("C13"))|.value[]|select(.status=="lock_malformed")|.id' "$OUT" | head -n 1)")" lock_malformed
mkworld "$W"; mk_a IMG-A example.invalid/a '""' "$DA" "$DA1"; fx C13 "F3 malformed field fails: empty tag_intent" fail
check "F3 ...and the entry is lock_malformed" "$(imgstatus "$(jq -r '.items[]|select(.id|startswith("C13"))|.value[]|select(.status=="lock_malformed")|.id' "$OUT" | head -n 1)")" lock_malformed
mkworld "$W"; mk_a '"IMG-a b"' example.invalid/a t1 "$DA" "$DA1"; fx C13 "F3 malformed field fails: id with a space" fail
check "F3 ...and the entry is lock_malformed" "$(imgstatus "$(jq -r '.items[]|select(.id|startswith("C13"))|.value[]|select(.status=="lock_malformed")|.id' "$OUT" | head -n 1)")" lock_malformed
mkworld "$W"; mk_a IMG-A example.invalid/a t1 '"sha256:1 2"' "$DA1"; fx C13 "F3 malformed field fails: digest with a space" fail
check "F3 ...and the entry is lock_malformed" "$(imgstatus "$(jq -r '.items[]|select(.id|startswith("C13"))|.value[]|select(.status=="lock_malformed")|.id' "$OUT" | head -n 1)")" lock_malformed
mkworld "$W"; mk_a IMG-A example.invalid/a t1 12345 "$DA1"; fx C13 "F3 malformed field fails: digest that is a YAML integer" fail
check "F3 ...and the entry is lock_malformed" "$(imgstatus "$(jq -r '.items[]|select(.id|startswith("C13"))|.value[]|select(.status=="lock_malformed")|.id' "$OUT" | head -n 1)")" lock_malformed
# a YAML-quoted digest is read like run_pinned.sh reads it (it used to be a spurious lock_malformed)
mkworld "$W"; lock_raw "- id: IMG-A
  reference: example.invalid/a
  tag_intent: t1
  digest: \"$DA\"
  platform_digest: '$DA1'
  size_bytes: 1
$IMGB_YAML"; fx C13 "F3 YAML-quoted digest and platform_digest are read, not misparsed, and pass" pass
# unusable reader inputs: C13 is error/fail, never a silent pass
mkworld "$W"; printf 'schema: 1\nimages: {}\n' >"$W/lock"; fx C13 "F3 images is not a list fails" fail
check "F3 ...nothing seen" "$(seen)" 0
mkworld "$W"; printf '::: not yaml [\n' >"$W/lock"; fx C13 "F3 a lock that is not YAML fails" fail
mkworld "$W"; fx C13 "F3 the lock reader (python3) cannot run: fails closed" fail SHIM_PY_FAIL=1
check "F3 ...status is error" "$(is C13)" error
# F6: a digest is exactly sha256: + 64 lowercase hex (the old reader took any length >= 1 and relied on the space anchors)
for sd in "sha256:a" "${DA:0:70}" "${DA}0" "sha256:$(printf 'A%.0s' $(seq 64))"; do
  mkworld "$W"; mklock2 "  digest: $sd" "  platform_digest: $DA1"; fx C13 "F6 digest of the wrong length or case ($(printf '%s' "$sd" | wc -c) chars: ${sd:0:14}...) fails" fail
  check "F6 ...as lock_malformed" "$(imgstatus IMG-A)" lock_malformed
done
mkworld "$W"; mklock2 "  digest: $DA" "  platform_digest: sha256:$(printf 'f%.0s' $(seq 8))"; fx C13 "F6 a truncated platform_digest fails" fail
# F6: the local image's digest must be the lock digest EXACTLY: a local digest that merely starts with it is a mismatch
mkworld "$W"; mklock2 "  digest: $DA" ""; printf 'example.invalid/a:t1|%s example.invalid/a@%s \nexample.invalid/b:t2|%s example.invalid/b@%s \n' "$DA1" "${DA}ff" "$DB1" "$DB" >"$W/imgdb"
fx C13 "F6 a local RepoDigest that merely starts with the lock digest is a mismatch" fail
mkworld "$W"; mklock2 "  digest: $DA" ""; printf 'example.invalid/a:t1|%sff \nexample.invalid/b:t2|%s example.invalid/b@%s \n' "$DA" "$DB1" "$DB" >"$W/imgdb"
fx C13 "F6 a local .Digest that merely starts with the lock digest is a mismatch" fail
# F6: an image stored with only its .Digest (no RepoDigests) matches that digest
mkworld "$W"; mklock2 "  digest: $DA" ""; printf 'example.invalid/a:t1|%s \nexample.invalid/b:t2|%s example.invalid/b@%s \n' "$DA" "$DB1" "$DB" >"$W/imgdb"
fx C13 "F6 an image whose only record is its .Digest matches the lock digest (false-refusal guard)" pass
# ---- round 7 (WF7 F-A, HM1, HM4): the lock digest `run_pinned.sh` runs must itself be present; the platform_digest never substitutes for it
DAW="sha256:$(hx c)"   # a well-formed digest that is NOT the one of the local image
mkworld "$W"; mklock2 "  digest: $DA" "  platform_digest: $DA1"; printf 'example.invalid/a:t1|%s example.invalid/a@%s \nexample.invalid/b:t2|%s example.invalid/b@%s \n' "$DA1" "$DAW" "$DB1" "$DB" >"$W/imgdb"
fx C13 "R7 F-A: a well-formed lock digest that is NOT present locally fails even though the platform_digest matches the local .Digest" fail
check "R7 F-A ...as digest_mismatch" "$(imgstatus IMG-A)" digest_mismatch
mkworld "$W"; mklock2 "  digest: $DA" "  platform_digest: $DA1"; printf 'example.invalid/a:t1|%s example.invalid/a@%s \nexample.invalid/b:t2|%s example.invalid/b@%s \n' "$DAW" "$DA" "$DB1" "$DB" >"$W/imgdb"
fx C13 "R7 F-A: the digest is present but a listed platform_digest is not: fails" fail
check "R7 F-A ...as digest_mismatch" "$(imgstatus IMG-A)" digest_mismatch
mkworld "$W"; mklock2 "  digest: $DA" "  platform_digest: $DA1"; fx C13 "R7 F-A guard against false refusal: digest and platform_digest both present pass" pass
# HM1: a yaml.py on PYTHONPATH must not replace PyYAML (python3 -I): the forged reader would report only IMG-A and hide the absent IMG-B
mkdir -p "$T/forge"; printf 'def safe_load(f):\n    return {"images": [{"id": "IMG-A", "reference": "example.invalid/a", "tag_intent": "t1", "digest": "%s", "platform_digest": "%s"}]}\n' "$DA" "$DA1" >"$T/forge/yaml.py"
mkworld "$W"; printf 'example.invalid/a:t1|%s example.invalid/a@%s \n' "$DA1" "$DA" >"$W/imgdb"
fx C13 "R7 HM1: a forged yaml.py on PYTHONPATH is not imported by the lock reader (the absent IMG-B is still seen)" fail PYTHONPATH="$T/forge"
check "R7 HM1 ...both lock entries counted" "$(seen)" 2
# HM4: a digest with an embedded newline is not sha256:<64 hex>; the Python regex must end at the string end (\Z)
mkworld "$W"; mk_a IMG-A example.invalid/a t1 "\"${DA}\\nextra\"" "$DA1"; fx C13 "R7 HM4: a digest carrying an embedded newline fails" fail
check "R7 HM4 ...as lock_malformed" "$(imgstatus IMG-A)" lock_malformed
mkworld "$W"; fx C15 "loadavg unreadable" fail CHK_LOADAVG="$W/noload"
mkworld "$W"; fx C16 "ntp not synchronised" fail SHIM_NTP=no
mkworld "$W"; fx C16 "timedatectl fails" fail SHIM_NTP_FAIL=1
mkworld "$W"; fx C16 "ntp garbage" fail SHIM_NTP=wat
# the badly-pinned image fixture names the image in the value (verifiable, not just a verdict)
mkworld "$W"; printf 'example.invalid/b:t2|'"$DB1"' example.invalid/b@'"$DB"' \n' >"$W/imgdb"; run_hc "$SUT" --
check "C13 value names the absent image" "$(jq -r '.items[]|select(.id|startswith("C13"))|.value|map(select(.status=="absent")|.id)|join(",")' "$OUT")" IMG-A

# ---- D. real host (no shims): the same instrument against this machine
RC=0; RWE="$T/real-ev"; mkdir -p "$RWE"
env EV="$RWE" timeout 60 bash "$SUT" >"$T/o.txt" 2>"$T/e.txt" || RC=$?
OUT="$RWE/wp10/host-checklist.json"
check "D1 real host exit 0" "$RC" 0
check "D2 real nproc equals nproc" "$(jq -r '.items[]|select(.id|startswith("C06"))|.value' "$OUT" 2>/dev/null)" "$(nproc)"
check "D3 real kvm verdict agrees with the filesystem" "$(iv C07)" "$([ -r /dev/kvm ] && [ -w /dev/kvm ] && echo pass || echo fail)"
check "D4 real rootless agrees with podman" "$(iv C03)" "$([ "$(podman info --format '{{.Host.Security.Rootless}}' 2>/dev/null)" = true ] && echo pass || echo fail)"
check "D5 real record has 19 items" "$(jq -r '.items|length' "$OUT" 2>/dev/null)" 19
check "D6 real lock parser saw the lock images" "$(jq -r .control.lock_parser_images_seen "$OUT" 2>/dev/null)" "$(grep -c '^- id:' "$HERE/../../../build/containers/images.lock.yaml")"

# ---- E. paired mutations: a copy of the script with one marked behaviour changed; the fixture set must FAIL
mutate() { # <marker> <old> <new> ; prints path or fails when not applied exactly once
  local m="$1" old="$2" new="$3" out="$T/mut_$1.sh"
  python3 - "$SUT_REAL" "$out" "$m" "$old" "$new" <<'PY' || return 1
import sys
src,out,m,old,new=sys.argv[1:6]; lines=open(src).read().split('\n'); n=0
for i,l in enumerate(lines):
    tail=l.rstrip().rsplit('#',1)[-1].split() if '#' in l else []   # a line may carry several markers: "# MUT:a MUT:b"
    if ('MUT:'+m) in tail and old in l: lines[i]=l.replace(old,new,1); n+=1
if n!=1: sys.exit(1)
open(out,'w').write('\n'.join(lines))
PY
  printf '%s' "$out"
}
# each mutation names: marker, old, new, and a fixture setup+expectation that the mutant must violate
mut_case() { # <marker> <old> <new> <setup-fn> <check-id> <expected-verdict-or-status> <status|verdict> [ENV...]
  local m="$1" old="$2" new="$3" setup="$4" id="$5" want="$6" kind="$7"; shift 7
  local f; if ! f=$(mutate "$m" "$old" "$new"); then bad "E $m mutation not applied exactly once"; return; fi
  [ -z "${SUT_REAL_ONLY:-}" ] || true
  mkworld "$W"; $setup; run_hc "$f" "$@" --
  local got; if [ "$kind" = status ]; then got=$(is "$id"); elif [ "${kind#img:}" != "$kind" ]; then got=$(imgstatus "${kind#img:}"); elif [ "$kind" = seen ]; then got=$(seen); elif [ "$kind" = detail ]; then got=$(jq -r '.items[]|select(.id|startswith("C13"))|.detail|test("could not be read")' "$OUT" 2>/dev/null); else got=$(iv "$id"); fi
  if [ "$got" != "$want" ] || [ ! -f "$OUT" ]; then echo "CAUGHT $m: mutant gives '${got:-<no record>}' (fixture wants '$want')" >>"$MUT_RECORD"; ok "E mutation $m caught"
  else echo "SURVIVED $m" >>"$MUT_RECORD"; bad "E mutation $m survived (mutant still gives '$got')"; fi
}
s_none(){ :; }
s_nocg(){ rm "$W/cg/cgroup.controllers"; }
s_kvmperm(){ chmod 000 "$W/kvm"; }
s_nosubuid(){ : >"$W/subuid"; }
s_dupconf(){ printf 'min_free_bytes=1000\nmin_free_bytes=2000\n' >"$W/conf"; }
s_noimga(){ printf 'example.invalid/b:t2|'"$DB1"' example.invalid/b@'"$DB"' \n' >"$W/imgdb"; }
s_badimga(){ printf 'example.invalid/a:t1|sha256:zzzz example.invalid/a@sha256:yyyy \nexample.invalid/b:t2|'"$DB1"' example.invalid/b@'"$DB"' \n' >"$W/imgdb"; }
mut_case c01 'na C01_ssh_reachability_key_login 1 ' 'add C01_ssh_reachability_key_login 1 present pass ' s_none C01 not_applicable_local_host status
mut_case c17 'na C17_remote_roles_and_capacity 6 ' 'add C17_remote_roles_and_capacity 6 present pass ' s_none C17 not_applicable_local_host status
mut_case c18 'na C18_artifact_return_path 7 ' 'add C18_artifact_return_path 7 present pass ' s_none C18 not_applicable_local_host status
mut_case c19 'na C19_host_key_pinning 8 ' 'add C19_host_key_pinning 8 present pass ' s_none C19 not_applicable_local_host status
mut_case c02 'error fail "podman --version' 'error pass "podman --version' s_none C02 fail verdict SHIM_PODMAN_VERSION_FAIL=1
mut_case c03 'present fail "podman reports' 'present pass "podman reports' s_none C03 fail verdict SHIM_ROOTLESS=false
mut_case c04 '[ -f "$CGROOT/cgroup.controllers" ]' '[ -d "$CGROOT" ]' s_nocg C04 fail verdict
mut_case c05 'num "$mt" && num "$ma" && [ "$mt" -gt 0 ]' 'true' s_none C05 fail verdict CHK_MEMINFO=/dev/null
mut_case c06 '&& num "$n" && [ "$n" -gt 0 ]' '' s_none C06 fail verdict SHIM_NPROC=0
mut_case c07 'present fail "$KVM present but' 'present pass "$KVM present but' s_kvmperm C07 fail verdict
mut_case c08 '[ "${su:-0}" -ge 1 ]' 'true' s_nosubuid C08 fail verdict
mut_case c09 '-lt 4096' '-lt 0' s_none C09 fail verdict CHK_ULIMIT_U=1000
mut_case c10 '[ -n "$GR" ] && ' '' s_none C10 fail verdict SHIM_GRAPHROOT=
mut_case c11 'error fail "repo free' 'error pass "repo free' s_none C11 fail verdict SHIM_DF_REPO_FAIL=1
mut_case c12 ' && [ "$rf" -ge "$MINV" ]' '' s_none C12 fail verdict SHIM_DF_REPO=900
mut_case c13 'st=digest_mismatch; ibad=$((ibad+1))' 'st=digest_mismatch' s_badimga C13 fail verdict
s_nodig2(){ sed -i "s/^  platform_digest: $DA1\$/  platform_digest: nonsense/" "$W/lock"; }
s_nodig(){ printf 'schema: 1\nimages:\n- id: IMG-A\n  reference: example.invalid/a\n  tag_intent: t1\n  size_bytes: 1\n- id: IMG-B\n  reference: example.invalid/b\n  tag_intent: t2\n  digest: '"$DB"'\n  platform_digest: '"$DB1"'\n  size_bytes: 1\n' >"$W/lock"; }
mut_case c13dg '! hexd "$dg"' 'false' s_nodig C13 fail verdict
mut_case c13pd '[ -n "$pd" ] && ! hexd "$pd"' 'false' s_nodig2 IMG-A lock_malformed img:IMG-A   # round 7: the platform_digest requirement also fails it, so the STATUS (lock_malformed) is what tells the two apart
mut_case c13b 'st=absent; ibad=$((ibad+1))' 'st=absent' s_noimga C13 fail verdict
mut_case c14 'unconfirmed unconfirmed "UNCONFIRMED: not probed' 'present pass "not probed' s_none C14 unconfirmed verdict
mut_case c15 'error fail "$LOADAVG' 'error pass "$LOADAVG' s_none C15 fail verdict CHK_LOADAVG=/nonexistent/x
mut_case c16 'present fail "NTPSynchronized=no"' 'present pass "NTPSynchronized=no"' s_none C16 fail verdict SHIM_NTP=no

# round 6 (F3, F6): paired mutations of the new reader and matcher
s_shift(){ lock_raw "- id: IMG-A
  reference: example.invalid/a
  tag_intent: t1|$DA
  size_bytes: 1
$IMGB_YAML"; }
s_badtag(){ mk_a IMG-A example.invalid/a '"t1|x"' "$DA" "$DA1"; }   # a healthy entry in every respect but its tag: only the shape check can refuse it
s_shortdig(){ mklock2 "  digest: sha256:a" "  platform_digest: $DA1"; }
s_repolong(){ mklock2 "  digest: $DA" ""; printf 'example.invalid/a:t1|%s example.invalid/a@%s \nexample.invalid/b:t2|%s example.invalid/b@%s \n' "$DA1" "${DA}ff" "$DB1" "$DB" >"$W/imgdb"; }
s_diglong(){ mklock2 "  digest: $DA" ""; printf 'example.invalid/a:t1|%sff \nexample.invalid/b:t2|%s example.invalid/b@%s \n' "$DA" "$DB1" "$DB" >"$W/imgdb"; }
s_digonly(){ mklock2 "  digest: $DA" ""; printf 'example.invalid/a:t1|%s \nexample.invalid/b:t2|%s example.invalid/b@%s \n' "$DA" "$DB1" "$DB" >"$W/imgdb"; }
s_idnotfirst(){ lock_raw "- id: IMG-A
  reference: example.invalid/a
  tag_intent: t1
  digest: $DA
  platform_digest: $DA1
- reference: example.invalid/b
  id: IMG-B
  tag_intent: t2
  digest: $DB
  platform_digest: $DB1"; printf 'example.invalid/a:t1|%s example.invalid/a@%s \n' "$DA1" "$DA" >"$W/imgdb"; }
s_dupid(){ lock_raw "$IMGB_YAML
- id: IMG-B
  reference: example.invalid/a
  tag_intent: t1
  digest: $DA
  platform_digest: $DA1"; }
s_readerfail(){ :; }
mut_case c13shape '[ "$kind" = BAD ]' 'false' s_badtag IMG-A lock_malformed img:IMG-A
mut_case c13hexlen '-eq 71' '-ge 8' s_shortdig IMG-A lock_malformed img:IMG-A
mut_case c13repo '*"@$dg "*) m=1;;' '*"@$dg"*) m=1;;' s_repolong C13 fail verdict
mut_case c13dig '*" $dg "*) m=1;;' '*" $dg"*) m=1;;' s_diglong C13 fail verdict
mut_case c13digdrop '*" $dg "*) m=1;;' '*" $dg "*) m=0;;' s_digonly C13 pass verdict
mut_case c13seen 'icount=$((icount+1))' 'icount=$((icount+0))' s_idnotfirst IMG-B 2 seen
mut_case c13dup 'elif i_d in ids: bad = 1' 'elif False: bad = 1' s_dupid C13 fail verdict
mut_case c13idfirst 'for i, e in enumerate(imgs, 1):' 'for i, e in enumerate(imgs[:1], 1):' s_idnotfirst C13 fail verdict
mut_case c13readerfail 'lockerr=lock_reader_failed' 'lockerr=""' s_none C13 true detail SHIM_PY_FAIL=1
s_pdwrong(){ mklock2 "  digest: $DA" "  platform_digest: $DA1"; printf 'example.invalid/a:t1|%s example.invalid/a@%s \nexample.invalid/b:t2|%s example.invalid/b@%s \n' "$DA1" "$DAW" "$DB1" "$DB" >"$W/imgdb"; }
s_pdmissing(){ mklock2 "  digest: $DA" "  platform_digest: $DA1"; printf 'example.invalid/a:t1|%s example.invalid/a@%s \nexample.invalid/b:t2|%s example.invalid/b@%s \n' "$DAW" "$DA" "$DB1" "$DB" >"$W/imgdb"; }
s_forge(){ printf 'example.invalid/a:t1|%s example.invalid/a@%s \n' "$DA1" "$DA" >"$W/imgdb"; }
s_digeol(){ mk_a IMG-A example.invalid/a t1 "\"${DA}\\nextra\"" "$DA1"; }
# round 7 (WF7): mutations of the exact-digest rule and of the reader isolation
mut_case c13pdalone '*" $pd "*) ;;' '*" $pd "*) m=1;;' s_pdwrong C13 fail verdict
mut_case c13pdreq '*) m=0;; esac; fi' '*) ;; esac; fi' s_pdmissing C13 fail verdict
mut_case c13iso 'python3 -I -' 'python3 -' s_forge C13 fail verdict PYTHONPATH="$T/forge"
mut_case c13digz '*\Z")' '*")' s_digeol C13 fail verdict

echo "host_checklist test: $PASSES passed, $FAILS failed (mutation record: $MUT_RECORD)"
[ "$FAILS" -eq 0 ]
