#!/usr/bin/env bash
# host_checklist.sh - WP-10 (T100 on the build host, docs/16 section 9.5 checklist items 1 to 8).
# Usage: host_checklist.sh [--strict]
# Writes $EV/wp10/host-checklist.json (schema host-checklist/1; default EV: specs/001-full-project-audit-remediation/evidence).
# OWNER DECISION (recorded in the T100 task note of this run): the build and measurement host is THIS host (anton); there is NO remote
# host. Checks that only mean something against a remote host (SSH reachability and key login, the two-way artifact path, host-key
# pinning, remote roles) are therefore recorded status=not_applicable_local_host, verdict=na, citing that decision. They are never
# faked as pass and never silently dropped. Checks that cannot be decided without network use or an owner answer are status=unconfirmed.
# Every item: {id, section_9_5_item, status, verdict, detail, value?}. status: present | absent | error | not_applicable_local_host | unconfirmed.
# verdict: pass | fail | na | unconfirmed. Control needles (11.4.201(7)(b)): the same -e instrument sees a known-present path (this script)
# and does not see a known-absent one; the lock-file reader must return at least one image id (a positive needle for its parser).
# Test overrides (default to the real host): CHK_KVM, CHK_USERNS, CHK_SUBUID, CHK_SUBGID, CHK_NEWUIDMAP, CHK_MEMINFO, CHK_LOADAVG, CHK_LOCK,
# CHK_CONF, CHK_USER, CHK_ULIMIT_U, CHK_ULIMIT_N, CHK_CGROOT, CHK_REPO_PATH; podman, df, timedatectl are found through PATH (shims in tests).
# This script never contacts a network and never pulls an image. Exit 0 when the record is written; with --strict, 1 if any verdict is fail.
set -u
command -v jq >/dev/null 2>&1 || { echo "host_checklist: jq is required" >&2; exit 2; }   # python3 with PyYAML reads the lock (item C13); without it C13 is error/fail, never a pass
STRICT=0
case "${1:-}" in "") ;; --strict) STRICT=1;; *) echo "host_checklist: usage: host_checklist.sh [--strict]" >&2; exit 2;; esac
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
EV="${EV:-$ROOT/specs/001-full-project-audit-remediation/evidence}"
KVM="${CHK_KVM:-/dev/kvm}"; USERNS="${CHK_USERNS:-/proc/sys/user/max_user_namespaces}"
SUBUID="${CHK_SUBUID:-/etc/subuid}"; SUBGID="${CHK_SUBGID:-/etc/subgid}"; NEWUIDMAP="${CHK_NEWUIDMAP:-newuidmap}"
MEMINFO="${CHK_MEMINFO:-/proc/meminfo}"; LOADAVG="${CHK_LOADAVG:-/proc/loadavg}"
LOCK="${CHK_LOCK:-$ROOT/build/containers/images.lock.yaml}"; CONF="${CHK_CONF:-$ROOT/scripts/containers/disk_headroom.conf}"
CUSER="${CHK_USER:-$(id -un)}"; CGROOT="${CHK_CGROOT:-/sys/fs/cgroup}"; REPOP="${CHK_REPO_PATH:-$ROOT}"
DECISION="owner decision 2026-10-06: build and measurement host is this host anton, no remote host exists"

ITEMS=()
# add <id> <9.5 item> <status> <verdict> <detail> [value-json]
add() { local v="${6:-null}"; ITEMS+=("$(jq -nc --arg id "$1" --arg it "$2" --arg st "$3" --arg vd "$4" --arg d "$5" --argjson v "$v" '{id:$id,section_9_5_item:$it,status:$st,verdict:$vd,detail:$d} + (if $v==null then {} else {value:$v} end)')"); }
na() { add "$1" "$2" not_applicable_local_host na "$3; $DECISION"; }
num() { case "$1" in ''|*[!0-9]*) return 1;; *) return 0;; esac; }

# 1 reachability and key login: remote only
na C01_ssh_reachability_key_login 1 "SSH reachability and key login apply to a remote host only"   # MUT:c01
# 2 podman
if pv=$(podman --version 2>/dev/null) && [ -n "$pv" ]; then add C02_podman_version 2 present pass "$pv" "$(jq -nc --arg v "$pv" '$v')"; else add C02_podman_version 2 error fail "podman --version failed or empty"; fi   # MUT:c02
if r=$(podman info --format '{{.Host.Security.Rootless}}' 2>/dev/null); then
  case "$r" in true) add C03_podman_rootless 2 present pass "rootless=true" true;; false) add C03_podman_rootless 2 present fail "podman reports rootless=false" false;; *) add C03_podman_rootless 2 error fail "unparsable rootless value: $r";; esac   # MUT:c03
else add C03_podman_rootless 2 error fail "podman info failed"; fi
if [ -f "$CGROOT/cgroup.controllers" ]; then add C04_cgroup_v2 2 present pass "cgroup v2 (cgroup.controllers present)" "$(jq -nc --arg c "$(cat "$CGROOT/cgroup.controllers" 2>/dev/null)" '{version:2,controllers:$c}')"   # MUT:c04
elif [ -d "$CGROOT" ]; then add C04_cgroup_v2 2 present fail "cgroup v1 or hybrid ($CGROOT has no cgroup.controllers)" '{"version":1}'
else add C04_cgroup_v2 2 error fail "$CGROOT unreadable"; fi
mt=$(awk '$1=="MemTotal:"{print $2*1024}' "$MEMINFO" 2>/dev/null); ma=$(awk '$1=="MemAvailable:"{print $2*1024}' "$MEMINFO" 2>/dev/null)
if num "$mt" && num "$ma" && [ "$mt" -gt 0 ]; then add C05_memory 2 present pass "MemTotal and MemAvailable read (bytes)" "$(jq -nc --argjson t "$mt" --argjson a "$ma" '{mem_total:$t,mem_available:$a}')"   # MUT:c05
else add C05_memory 2 error fail "MemTotal/MemAvailable not parsable in $MEMINFO"; fi
if n=$(nproc 2>/dev/null) && num "$n" && [ "$n" -gt 0 ]; then add C06_cpu_count 2 present pass "nproc" "$n"; else add C06_cpu_count 2 error fail "nproc failed"; fi   # MUT:c06
# kvm: exists AND the user can open it read-write
if [ -e "$KVM" ]; then if [ -r "$KVM" ] && [ -w "$KVM" ]; then add C07_kvm 2 present pass "$KVM present and readable+writable by $CUSER"; else add C07_kvm 2 present fail "$KVM present but not readable+writable by $CUSER"; fi   # MUT:c07
else add C07_kvm 2 absent fail "$KVM does not exist"; fi
# user namespaces: max_user_namespaces > 0, subuid and subgid ranges, newuidmap
un=$(cat "$USERNS" 2>/dev/null); su=$(grep -c "^$CUSER:" "$SUBUID" 2>/dev/null); sg=$(grep -c "^$CUSER:" "$SUBGID" 2>/dev/null)
nm=0; command -v "$NEWUIDMAP" >/dev/null 2>&1 || [ -x "$NEWUIDMAP" ] && nm=1
if num "$un" && [ "$un" -gt 0 ] && [ "${su:-0}" -ge 1 ] && [ "${sg:-0}" -ge 1 ] && [ "$nm" = 1 ]; then add C08_user_namespaces 2 present pass "max_user_namespaces=$un, subuid and subgid ranges for $CUSER, newuidmap present" "$(jq -nc --argjson u "$un" --argjson a "$su" --argjson g "$sg" '{max_user_namespaces:$u,subuid_ranges:$a,subgid_ranges:$g}')"   # MUT:c08
elif ! num "$un"; then add C08_user_namespaces 2 error fail "$USERNS unreadable"
else add C08_user_namespaces 2 present fail "max_user_namespaces=$un subuid_ranges=${su:-0} subgid_ranges=${sg:-0} newuidmap=$nm: rootless podman cannot run" ; fi
# ulimits
if [ -n "${CHK_ULIMIT_U+x}" ]; then uu="$CHK_ULIMIT_U"; else uu=$(ulimit -u 2>/dev/null); fi; if [ -n "${CHK_ULIMIT_N+x}" ]; then un2="$CHK_ULIMIT_N"; else un2=$(ulimit -n 2>/dev/null); fi
ulok() { case "$1" in unlimited) return 0;; esac; num "$1" && [ "$1" -gt 0 ]; }
if ulok "$uu" && ulok "$un2"; then
  if [ "$uu" != unlimited ] && [ "$uu" -lt 4096 ]; then add C09_ulimits 2 present fail "ulimit -u $uu is below 4096" "$(jq -nc --arg u "$uu" --arg n "$un2" '{nproc:$u,nofile:$n}')"   # MUT:c09
  else add C09_ulimits 2 present pass "ulimit -u and -n read" "$(jq -nc --arg u "$uu" --arg n "$un2" '{nproc:$u,nofile:$n}')"; fi
else add C09_ulimits 2 error fail "ulimit -u/-n unreadable or unparsable ('$uu' '$un2')"; fi
# disk: graphroot and repo free, headroom rule from disk_headroom.conf (absolute min_free_bytes, need 0)
MIN=$(grep -E '^min_free_bytes=' "$CONF" 2>/dev/null | head -n 2); mcount=$(printf '%s\n' "$MIN" | grep -c .); MINV=${MIN#min_free_bytes=}
free_of() { df -B1 --output=avail "$1" 2>/dev/null | tail -n 1 | tr -d ' '; }
GR=$(podman info --format '{{.Store.GraphRoot}}' 2>/dev/null)
if [ -n "$GR" ] && gf=$(free_of "$GR") && num "$gf"; then add C10_disk_graphroot_free 2 present pass "free bytes under graphroot $GR" "$(jq -nc --arg p "$GR" --argjson f "$gf" '{path:$p,free_bytes:$f}')"   # MUT:c10
else add C10_disk_graphroot_free 2 error fail "graphroot or its free space unreadable"; gf=""; fi
if rf=$(free_of "$REPOP") && num "$rf"; then add C11_disk_repo_free 2 present pass "free bytes on the repository filesystem" "$(jq -nc --arg p "$REPOP" --argjson f "$rf" '{path:$p,free_bytes:$f}')"; else add C11_disk_repo_free 2 error fail "repo free space unreadable"; rf=""; fi   # MUT:c11
if [ "$mcount" = 1 ] && num "$MINV" && [ "$MINV" -gt 0 ]; then
  if num "$gf" && num "$rf"; then
    if [ "$gf" -ge "$MINV" ] && [ "$rf" -ge "$MINV" ]; then hv=pass; hd="both filesystems keep at least min_free_bytes free"; else hv=fail; hd="a filesystem is below min_free_bytes"; fi   # MUT:c12
    add C12_disk_headroom_rule 2 present "$hv" "$hd" "$(jq -nc --argjson m "$MINV" --argjson g "$gf" --argjson r "$rf" '{min_free_bytes:$m,graphroot_free_bytes:$g,repo_free_bytes:$r,graphroot_margin_bytes:($g-$m),repo_margin_bytes:($r-$m)}')"
  else add C12_disk_headroom_rule 2 error fail "free space unreadable, the rule cannot be evaluated"; fi
else add C12_disk_headroom_rule 2 error fail "min_free_bytes missing, duplicated or not a positive integer in $CONF"; fi
# images from the lock file: id, reference, tag_intent, digest, platform_digest. Present AND pinned = the stored .Digest or a RepoDigest matches.
# Round 6 (F3, F6): the lock is read with PyYAML, the reader run_pinned.sh uses, NOT an awk field splitter: a `|` or an odd key order can no longer shift a
# value into another field, and an entry whose first key is not `id` is seen. python3 validates the SHAPE of every entry (a mapping, an id, a reference path,
# an OCI tag, string values, no duplicate id); a violation is BAD (lock_malformed). Digests are validated here (hexd). Any failure to read is C13 error/fail.
LOCKREAD=$(python3 -I - "$LOCK" <<'PY' 2>/dev/null
import re, sys
try:
    import yaml
    d = yaml.safe_load(open(sys.argv[1]))
except Exception:
    print("ERR|lock_unreadable"); sys.exit(0)
imgs = d.get("images") if isinstance(d, dict) else None
if not isinstance(imgs, list):
    print("ERR|lock_unreadable"); sys.exit(0)
ID = re.compile(r"IMG-[A-Z0-9][A-Za-z0-9-]*\Z")
REF = re.compile(r"[A-Za-z0-9][A-Za-z0-9._-]*(:[0-9]+)?(/[A-Za-z0-9][A-Za-z0-9._-]*)+\Z")
TAG = re.compile(r"[A-Za-z0-9_][A-Za-z0-9._-]{0,127}\Z")
DIG = re.compile(r"[A-Za-z0-9:._-]*\Z")
ids = set(); n = 0
for i, e in enumerate(imgs, 1):  # MUT:c13idfirst
    n += 1; bad = 0
    if not isinstance(e, dict):
        print("BAD|entry#%d|||||" % i); continue
    def val(k, rx=None, opt=False):
        global bad
        v = e.get(k)
        if v is None and opt: return ""
        if not isinstance(v, str) or (rx is not None and not rx.match(v)):
            bad = 1; return ""
        return v
    i_d = val("id", ID)
    if not i_d: i_d = "entry#%d" % i
    elif i_d in ids: bad = 1   # MUT:c13dup
    ids.add(i_d)
    ref = val("reference", REF); tag = val("tag_intent", TAG)
    dg = val("digest", DIG, opt=True); pd = val("platform_digest", DIG, opt=True)
    print("%s|%s|%s|%s|%s|%s" % ("BAD" if bad else "OK", i_d, ref, tag, dg, pd))
print("END|%d" % n)
PY
)
# hexd: a well-formed digest is exactly sha256: followed by 64 lowercase hex characters (empty, absent, short, long or malformed values are never a pattern)
hexd() { case "$1" in sha256:*) ;; *) return 1;; esac; [ "${#1}" -eq 71 ] || return 1; case "${1#sha256:}" in *[!0-9a-f]*) return 1;; esac; return 0; }   # MUT:c13hexlen
IMGJ="[]"; ibad=0; icount=0; lockerr=""
case "$(printf '%s\n' "$LOCKREAD" | tail -n 1)" in
  "END|"[0-9]*) ;;
  "ERR|"*) lockerr=lock_unreadable;;
  *) lockerr=lock_reader_failed;;   # MUT:c13readerfail
esac
if [ -z "$lockerr" ]; then
while IFS='|' read -r kind id ref tag dg pd; do
  case "$kind" in OK|BAD) ;; *) continue;; esac
  icount=$((icount+1))   # MUT:c13seen
  if [ "$kind" = BAD ]; then st=lock_malformed; ibad=$((ibad+1))   # MUT:c13shape
  elif ! hexd "$dg"; then st=lock_malformed; ibad=$((ibad+1))   # MUT:c13dg
  elif [ -n "$pd" ] && ! hexd "$pd"; then st=lock_malformed; ibad=$((ibad+1))   # MUT:c13pd
  elif sd=$(podman image inspect --format '{{.Digest}} {{range .RepoDigests}}{{.}} {{end}}' "$ref:$tag" 2>/dev/null </dev/null); then
    # fail closed: only non-empty, well-formed digests ever become a pattern (an empty one would match every image)
    m=0
    case " $sd " in *"@$dg "*) m=1;; esac   # MUT:c13repo
    case " $sd " in *" $dg "*) m=1;; esac   # MUT:c13dig MUT:c13digdrop
    if [ -n "$pd" ]; then case " $sd " in *" $pd "*) m=1;; esac; fi
    if [ "$m" = 1 ]; then st=present_pinned; else st=digest_mismatch; ibad=$((ibad+1)); fi   # MUT:c13
  else st=absent; ibad=$((ibad+1)); fi   # MUT:c13b
  IMGJ=$(jq -c --arg id "$id" --arg r "$ref:$tag" --arg st "$st" --arg dg "$dg" '. + [{id:$id,reference:$r,status:$st,pinned_digest:$dg}]' <<<"$IMGJ")
done <<<"$LOCKREAD"
fi
if [ -n "$lockerr" ]; then add C13_images_from_lock 3 error fail "the lock $LOCK could not be read ($lockerr: python3 with PyYAML is required)"
elif [ "$icount" -eq 0 ]; then add C13_images_from_lock 3 error fail "no image parsed from $LOCK (parser blind or lock empty)"
elif [ "$ibad" -eq 0 ]; then add C13_images_from_lock 3 present pass "all $icount lock images present locally and pinned" "$IMGJ"
else add C13_images_from_lock 3 present fail "$ibad of $icount lock images absent, malformed in the lock, or with a digest mismatch (no pull is done here: T005d pulls)" "$IMGJ"; fi
# outbound pull policy: no network use here, so undecidable by this script
add C14_outbound_pull_policy 3 unconfirmed unconfirmed "UNCONFIRMED: not probed, this checklist uses no network and pulls nothing (T005d records whether a pull works); images already present are item C13"   # MUT:c14
# shared host
if la=$(cat "$LOADAVG" 2>/dev/null) && [ -n "$la" ]; then add C15_host_sharing 4 unconfirmed unconfirmed "UNCONFIRMED: whether other work shares this host is an owner fact; only the instantaneous load is recorded" "$(jq -nc --arg l "$la" '{loadavg:$l}')"; else add C15_host_sharing 4 error fail "$LOADAVG unreadable"; fi   # MUT:c15
# time sync
if ts=$(timedatectl show -p NTPSynchronized --value 2>/dev/null) && [ -n "$ts" ]; then
  case "$ts" in yes) add C16_time_sync 5 present pass "NTPSynchronized=yes";; no) add C16_time_sync 5 present fail "NTPSynchronized=no";; *) add C16_time_sync 5 error fail "unparsable NTPSynchronized: $ts";; esac   # MUT:c16
else add C16_time_sync 5 error fail "timedatectl show failed"; fi
na C17_remote_roles_and_capacity 6 "roles of thinker.local and amber.local do not apply, no remote host"   # MUT:c17
na C18_artifact_return_path 7 "the two-way artifact path is a remote-host check; builds and artifacts stay on this host"   # MUT:c18
na C19_host_key_pinning 8 "known_hosts fingerprint pinning is a remote-host check"   # MUT:c19
# control needles: same -e instrument on a known-present and a known-absent path; lock parser must see images
NEG="/nonexistent/catalogizer-checklist-needle-$$"; POS="${BASH_SOURCE[0]}"
if [ -e "$NEG" ] || [ -L "$NEG" ]; then cn=error; cnd="negative needle exists: instrument cannot be trusted"; else cn=absent; cnd="expected absence confirmed"; fi
if [ -e "$POS" ]; then cp=present; cpd="positive needle seen"; else cp=error; cpd="positive needle not seen: instrument blind"; fi
CTRL=$(jq -nc --arg n "$cn" --arg nd "$cnd" --arg p "$cp" --arg pd "$cpd" --argjson li "$icount" '{negative:{status:$n,detail:$nd},positive:{status:$p,detail:$pd},lock_parser_images_seen:$li}')

sha() { [ -f "$1" ] && sha256sum "$1" | cut -d' ' -f1 || echo absent; }
mkdir -p "$EV/wp10" || exit 3
OUT="$EV/wp10/host-checklist.json"; TMPF="$(mktemp "$EV/wp10/.host-checklist.XXXXXX")" || exit 3
trap 'rm -f "$TMPF"; exit 143' TERM; trap 'rm -f "$TMPF"; exit 130' INT; trap 'rm -f "$TMPF"; exit 129' HUP
printf '%s\n' "${ITEMS[@]}" | jq -s --argjson ctrl "$CTRL" --arg host "$(hostname 2>/dev/null)" --arg at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" --arg dec "$DECISION" \
  --arg ssha "$(sha "${BASH_SOURCE[0]}")" --arg lsha "$(sha "$LOCK")" --arg csha "$(sha "$CONF")" \
  '{schema:"host-checklist/1",scope:"local-host-only",host:$host,probed_at:$at,owner_decision:$dec,
    identity:{script_sha256:$ssha,lock_sha256:$lsha,disk_headroom_conf_sha256:$csha},
    control:$ctrl,items:.,
    summary:{pass:(map(select(.verdict=="pass"))|length),fail:(map(select(.verdict=="fail"))|length),na:(map(select(.verdict=="na"))|length),unconfirmed:(map(select(.verdict=="unconfirmed"))|length)}}' >"$TMPF" \
  && chmod 0644 "$TMPF" && mv "$TMPF" "$OUT" || { rm -f "$TMPF"; exit 3; }
echo "host_checklist: wrote $OUT"
if [ "$STRICT" = 1 ] && jq -e '.summary.fail > 0' "$OUT" >/dev/null; then exit 1; fi
exit 0
