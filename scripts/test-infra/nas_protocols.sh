#!/usr/bin/env bash
# nas_protocols.sh - WP-12. READ-ONLY protocol survey of the owner's seven Synology hosts (docs/infrastructure/synology-hosts.md): for each host NFS (portmap, versions, exports, MNT attempts and,
# where a path is mountable, a bounded walk + read sample), FTP, explicit FTPS and SFTP (bounded breadth-first listing, listing latency p50/p95, the first 1 MiB of ONE file per share read and discarded).
# SMB is covered by nas_readonly_leg.sh and the earlier survey. Nothing is ever written, created, deleted, renamed or changed on a NAS; at most one request per second per host.
# Clients run in rootless containers through scripts/containers/run_pinned.sh: IMG-GO (python3 stdlib + ssh for FTP/FTPS/SFTP/RPC: client/nas_proto.py) and IMG-INFRA-CLIENT (libnfs user-space nfs-ls/nfs-cat:
# client/nas_proto_nfs.sh). No kernel mount, no sudo, no host client.
# Credentials: ONLY from the gitignored env file (default <repo>/.env: SYNOLOGY_SMB_USER, SYNOLOGY_SMB_PASSWORD, SYNOLOGY_IP_<n>); parsed with dotenv_get.py, never sourced. They go to a 0600 file in a 0700 run directory on the
# per-user tmpfs ($XDG_RUNTIME_DIR), read by the client, removed on every exit path; never argv, never the environment, never a log or an evidence file. Before the evidence is written it is scanned for the user name, the
# password and every host address, and the run is REFUSED if one is found. Evidence records aliases (Synology<n>), share names, counts, sha256 of names, extension histograms, latencies and throughputs only.
# Usage:  nas_protocols.sh [--hosts 1,2,3,4,5,6,7] [--protocols nfs,ftp,ftps,sftp] [--ev-dir DIR] [--jobs N]     Env: TI_ENV_FILE (default <repo>/.env)
#   --ev-dir  default specs/001-full-project-audit-remediation/evidence/wp12/nas-protocols (per-host-per-protocol JSON, summary.json, SHA256SUMS)
#   --jobs    hosts measured in parallel (default 4, max 7); a host is always measured sequentially, so the per-host rate stays <= 1 request/second
# Exit:   0 finished (a protocol that is refused or unreachable is a RESULT, recorded); 1 a leak scan or an assembly step failed; 2 usage. SKIP (exit 0) when the env file or a variable is absent.
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$HERE/lib.sh"
HOSTS="1,2,3,4,5,6,7"; PROTOS="nfs,ftp,ftps,sftp"; EVDIR=""; JOBS=4
while [ $# -gt 0 ]; do case "$1" in
  --hosts) ti_optval "$1" $# "${2:-}"; HOSTS=$2; shift 2;; --protocols) ti_optval "$1" $# "${2:-}"; PROTOS=$2; shift 2;;
  --ev-dir) ti_optval "$1" $# "${2:-}"; EVDIR=$2; shift 2;; --jobs) ti_optval "$1" $# "${2:-}"; JOBS=$2; shift 2;; *) ti_die "unknown argument '$1'" 2;; esac; done
[[ "$HOSTS" =~ ^[1-7](,[1-7])*$ ]] || ti_die "--hosts must be a comma list of 1..7" 2
[[ "$PROTOS" =~ ^(nfs|ftp|ftps|sftp)(,(nfs|ftp|ftps|sftp))*$ ]] || ti_die "--protocols must be a comma list of nfs,ftp,ftps,sftp" 2
[[ "$JOBS" =~ ^[1-7]$ ]] || ti_die "--jobs must be 1..7" 2
[ -n "$EVDIR" ] || EVDIR="$TI_ROOT/specs/001-full-project-audit-remediation/evidence/wp12/nas-protocols"
EVDIR="$(ti_abs "$EVDIR")"
ENVF="${TI_ENV_FILE:-$TI_ROOT/.env}"
if [ ! -r "$ENVF" ]; then echo "SKIP nas_protocols reason=env_absent the gitignored env file is absent; the real-NAS survey is optional"; exit 0; fi
ti_need jq sha256sum python3 podman
envval() { python3 -I "$HERE/dotenv_get.py" "$ENVF" "$1" 2>/dev/null; }
missing=""
[ -n "$(envval SYNOLOGY_SMB_USER)" ] || missing="$missing SYNOLOGY_SMB_USER"
[ -n "$(envval SYNOLOGY_SMB_PASSWORD)" ] || missing="$missing SYNOLOGY_SMB_PASSWORD"
IFS=, read -r -a IDX <<<"$HOSTS"
for i in "${IDX[@]}"; do [ -n "$(envval "SYNOLOGY_IP_$i")" ] || missing="$missing SYNOLOGY_IP_$i"; done
if [ -n "$missing" ]; then echo "SKIP nas_protocols reason=credentials_absent variables:$missing"; exit 0; fi
RTD="${XDG_RUNTIME_DIR:-}"; [ -n "$RTD" ] && [ -d "$RTD" ] && [ -w "$RTD" ] || ti_refuse runtime_dir_unavailable "XDG_RUNTIME_DIR is not a writable directory; the credentials must not be written under the checkout" 1
RUN="$RTD/catalogizer-nas-proto.$$"; mkdir -m 700 "$RUN" || ti_die "cannot create $RUN"
mkdir -p "$TI_ROOT/.audit/scratch" && VIEW="$(mktemp -d "$TI_ROOT/.audit/scratch/ti-view.XXXXXX")" || ti_die "cannot create the client view"
ti_view_dir "$VIEW" scripts/test-infra/client || ti_die "cannot build the client view"
cleanup() { case "$RUN" in "$RTD"/catalogizer-nas-proto.*) rm -rf -- "${RUN:?}";; esac; case "$VIEW" in "$TI_ROOT"/.audit/scratch/ti-view.*) rm -rf -- "$VIEW";; esac; }
ti_exit_on_signals
trap cleanup EXIT
USER_V="$(envval SYNOLOGY_SMB_USER)"; PASS_V="$(envval SYNOLOGY_SMB_PASSWORD)"
SURVEY="$TI_ROOT/specs/001-full-project-audit-remediation/evidence/wp12/nas-survey"
want() { case ",$PROTOS," in *",$1,"*) return 0;; *) return 1;; esac; }
# crun <host dir> <tag> <image> <command word>...: one container run from the scratch view, /out = the host's run directory
crun() { local hd=$1 tag=$2 img=$3; shift 3; (cd "$VIEW" && ti_runpinned --out "$hd" --op-id "nasproto-$$-${hd##*/}-$tag" "$img" -- "$@") >"$hd/log-$tag.txt" 2>&1; }
run_host() {
  local n=$1 hd="$RUN/h$n" ip; ip="$(envval "SYNOLOGY_IP_$n")"; mkdir -m 700 "$hd"
  ( umask 077; printf '%s\n%s\n' "$USER_V" "$PASS_V" >"$hd/cred" ); chmod 600 "$hd/cred"
  if [ -r "$SURVEY/survey-$n.json" ]; then jq -r '.shares[]? | select(.status=="listed") | .share' "$SURVEY/survey-$n.json" >"$hd/shares-$n.txt" 2>/dev/null; fi
  if want nfs; then
    crun "$hd" rpc IMG-GO python3 -I /src/scripts/test-infra/client/nas_proto.py rpc "$n" "$ip" /out
    if [ -s "$hd/exports-$n.txt" ]; then
      local vers=3; jq -e '.nfs_versions_tcp_2049.v4.null_call == "ok"' "$hd/nfs-rpc-$n.json" >/dev/null 2>&1 && vers=3,4
      crun "$hd" nfs IMG-INFRA-CLIENT bash /src/scripts/test-infra/client/nas_proto_nfs.sh "$ip" "$n" "/out/exports-$n.txt" "$vers"
    fi
  fi
  want ftp  && crun "$hd" ftp  IMG-GO python3 -I /src/scripts/test-infra/client/nas_proto.py ftp  "$n" "$ip" /out
  want ftps && crun "$hd" ftps IMG-GO python3 -I /src/scripts/test-infra/client/nas_proto.py ftps "$n" "$ip" /out
  want sftp && crun "$hd" sftp IMG-GO python3 -I /src/scripts/test-infra/client/nas_proto.py sftp "$n" "$ip" /out
  rm -f -- "$hd/cred" "$hd/askpass" "$hd/ssh_config" "$hd/ssh_stderr"
  echo "host Synology$n done"
}
echo "nas_protocols: hosts=$HOSTS protocols=$PROTOS jobs=$JOBS (read-only, <=1 request/s per host)"
running=0
for n in "${IDX[@]}"; do
  run_host "$n" &
  running=$((running+1))
  if [ "$running" -ge "$JOBS" ]; then wait -n; running=$((running-1)); fi
done
wait
# ---- assemble the evidence ----
HEAD_C="$(git -C "$TI_ROOT" rev-parse HEAD 2>/dev/null)"; AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
mkdir -p "$EVDIR" || ti_die "cannot create $EVDIR"
TOOLS="$(for f in scripts/test-infra/nas_protocols.sh scripts/test-infra/client/nas_proto.py scripts/test-infra/client/nas_proto_nfs.sh; do printf '{"file":"%s","sha256":"%s"}\n' "$f" "$(sha256sum "$TI_ROOT/$f" | cut -d' ' -f1)"; done | jq -s .)"
ident() { jq -n --arg head "$HEAD_C" --arg at "$AT" --argjson tools "$TOOLS" '{schema_owner:"wp12-nas-protocols", head:$head, run_at:$at, tools:$tools, images:["IMG-GO","IMG-INFRA-CLIENT"], rootless:true, writes_performed:0, content_stored:false, names_recorded:false, host_addresses_recorded:false, credentials_recorded:false}'; }
for n in "${IDX[@]}"; do
  hd="$RUN/h$n"
  for p in ftp ftps sftp; do
    want "$p" || continue
    if [ -s "$hd/$p-$n.json" ]; then jq --argjson id "$(ident)" '. + {identity:$id}' "$hd/$p-$n.json" >"$EVDIR/$p-$n.json"
    else jq -n --argjson id "$(ident)" --arg a "Synology$n" --arg p "$p" '{alias:$a, protocol:$p, status:"no_result", reason:"the client container produced no result file (see the run log)", identity:$id}' >"$EVDIR/$p-$n.json"; fi
  done
  if want nfs; then
    base="$hd/nfs-rpc-$n.json"; [ -s "$base" ] || echo '{"status":"no_result"}' >"$base"
    if [ -s "$hd/nfs-$n.tsv" ]; then
      walk="$(jq -R -s --arg a x '
        split("\n") | map(select(length>0) | split("\t")) as $r
        | [$r[] | select(.[0]=="R" and .[1]!="-") | .[1]+"|"+.[2]] | unique | map(. as $k | ($k|split("|")) as $kv
          | {export:$kv[0], version:("v"+$kv[1]),
             result: ([$r[] | select(.[0]=="R" and .[1]==$kv[0] and .[2]==$kv[1]) | {(.[3]): .[4]}] | add),
             listing_latency_ms: ([$r[] | select(.[0]=="LAT" and .[1]==$kv[0] and .[2]==$kv[1]) | (.[3]|tonumber)] | sort | {n:length, p50:(if length>0 then .[((length-1)*0.5|round)] else null end), p95:(if length>0 then .[((length-1)*0.95|round)] else null end), max:(if length>0 then .[-1] else null end)}),
             ext_histogram_top25: ([$r[] | select(.[0]=="EXT" and .[1]==$kv[0] and .[2]==$kv[1]) | {ext:.[3], count:(.[4]|tonumber)}] | sort_by(-.count) | .[:25]),
             mode_histogram_files: ([$r[] | select(.[0]=="MODEF" and .[1]==$kv[0] and .[2]==$kv[1]) | {(.[3]): (.[4]|tonumber)}] | add),
             mode_histogram_dirs: ([$r[] | select(.[0]=="MODED" and .[1]==$kv[0] and .[2]==$kv[1]) | {(.[3]): (.[4]|tonumber)}] | add)})' <"$hd/nfs-$n.tsv")"
    else walk='"not_run: no export listed and no path proved mountable"'; fi
    jq --argjson id "$(ident)" --argjson walk "$walk" '. + {walk:$walk, identity:$id}' "$base" >"$EVDIR/nfs-$n.json"
  fi
done
# leak scan: the evidence must hold no credential and no host address
leak=0
for pat in "$USER_V" "$PASS_V"; do [ -z "$pat" ] || ! grep -rqF -- "$pat" "$EVDIR"/*.json 2>/dev/null || { leak=1; echo "FAIL leak scan: a credential value occurs in the evidence"; }; done
for n in "${IDX[@]}"; do ip="$(envval "SYNOLOGY_IP_$n")"; ! grep -rqF -- "$ip" "$EVDIR"/*.json 2>/dev/null || { leak=1; echo "FAIL leak scan: the address of host $n occurs in the evidence"; }; done
[ "$leak" = 0 ] || { rm -f -- "$EVDIR"/nfs-[1-7].json "$EVDIR"/ftp-[1-7].json "$EVDIR"/ftps-[1-7].json "$EVDIR"/sftp-[1-7].json; ti_die "leak scan failed: the evidence files were removed" 1; }
# summary (only the evidence files of THIS run: a subset run does not read the files of an earlier full run)
SUMFILES=(); for n in "${IDX[@]}"; do for p in nfs ftp ftps sftp; do want "$p" && SUMFILES+=("$EVDIR/$p-$n.json"); done; done
jq -s --argjson id "$(ident)" '
  def med(a): (a|sort) as $s | if ($s|length)==0 then null else $s[(($s|length)-1)/2|floor] end;
  def shares_of: (.shares // []) | map(select(.read_sample.status? == "ok"));
  def ftpsum: {status: .status, works: (.status=="ok"), reason: (.error // .login_reply // null),
      listing_latency_ms: (.listing_latency_all // null),
      read_mib_per_s_median_total: med([shares_of[] | .read_sample.mib_per_s_total]),
      read_mib_per_s_median_after_first_byte: med([shares_of[] | .read_sample.mib_per_s_after_first_byte]),
      shares_listed: ((.shares // []) | map(select(.status=="listed")) | length),
      server: (.ssh.server_software // .banner // null), tls: (.tls | if . then {negotiated:.negotiated_version, accepts:.accepts, self_signed:.self_signed, cert_sha256:.cert_sha256} else null end),
      host_key_types: ((.host_keys // []) | map(.type)), mlsd: (.mlsd.supported // null)};
  def nfssum: {status: .status, works: ((.walk|type)=="array" and ((.walk|map(select(.result.status=="listed"))|length) > 0)),
      reason: (if .status=="ok" then null else .status end),
      versions: (.nfs_versions_tcp_2049 // {} | map_values(.null_call)), exports_listed: ((.exports // [])|length),
      mount_attempt_codes: ((.mount_attempts // []) | map(.mnt3_name) | unique), mountable_paths: ((.mountable_paths // [])|length),
      control_path_code: ((.mount_attempts // []) | map(select(.path=="(control)")) | (.[0].mnt3_name // null))};
  { identity:$id,
    hosts: ( group_by(.alias) | map({alias: .[0].alias, protocols: (map({(.protocol): (if .protocol=="nfs-rpc" or .protocol=="nfs" then nfssum else ftpsum end)}) | add)}
        | . + {best_protocol_by_median_read_mib_per_s: ([ .protocols | to_entries[] | select(.value.read_mib_per_s_median_total != null) | {p:.key, v:.value.read_mib_per_s_median_total} ] | sort_by(-.v) | (.[0].p // null)), best_protocol_by_median_read_mib_per_s_after_first_byte: ([ .protocols | to_entries[] | select(.value.read_mib_per_s_median_after_first_byte != null) | {p:.key, v:.value.read_mib_per_s_median_after_first_byte} ] | sort_by(-.v) | (.[0].p // null)), caveat: "one 1 MiB sample per share: a single observation; the first request of a share can include disk spin-up (compare the two rankings)"}) | sort_by(.alias) ) }
  ' "${SUMFILES[@]}" >"$EVDIR/summary.json" 2>"$RUN/summary.err" || { cat "$RUN/summary.err"; ti_die "summary assembly failed" 1; }
( cd "$EVDIR" && ls -1 ./*.json | sed 's#^\./##' | LC_ALL=C sort | xargs sha256sum >SHA256SUMS ) || ti_die "SHA256SUMS failed" 1
( cd "$EVDIR" && sha256sum -c --quiet SHA256SUMS ) || ti_die "SHA256SUMS does not verify" 1
echo "nas_protocols: evidence written to ${EVDIR#"$TI_ROOT"/}"
jq -r '.hosts[] | .alias + " " + (.protocols | to_entries | map(.key + "=" + (if .value.works then "works" else "no(" + (.value.status // "?") + ")" end)) | join(" "))' "$EVDIR/summary.json"
exit 0
