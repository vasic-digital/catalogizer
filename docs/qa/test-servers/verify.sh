#!/usr/bin/env bash
# Download every manifest file over each protocol from the LAN IP, compare sha256, 3 runs, + negative controls.
# NFS check is opt-in (kernel mount needs privilege): TS_NFS_MOUNT=1 uses `sudo -n mount`.
set -uo pipefail
HERE=$(cd "$(dirname "$0")" && pwd); . "$HERE/ports.env"
ROOT=${TS_ROOT:-/home/milosvasic/Testing-servers}
ENVF=${TS_ENV:-/home/milosvasic/.cache/claude-tmp/catqa3/testservers.env}
. "$HERE/../lib/lan_ip.sh"
_ts_host_caller="${TS_HOST:-}"
set -a; . "$ENVF"; set +a
# TS_HOST: caller env wins, else the auto-detected LAN address (a value stored in the env file is not trusted; it goes stale)
if [ -n "$_ts_host_caller" ]; then TS_HOST="$_ts_host_caller"; else TS_HOST="$(qa_lan_ip)" || exit 1; fi
OUT=${TS_EVID:-$ROOT/evidence}; mkdir -p "$OUT"; W=$(mktemp -d); trap 'rm -rf "${W:?}"' EXIT
LOG=$OUT/verify_$(date +%Y%m%dT%H%M%S).log; FAIL=0
say(){ echo "$*" | tee -a "$LOG"; }
enc(){ python3 -I -c 'import sys,urllib.parse as u;print("/".join(u.quote(s) for s in sys.argv[1].split("/")))' "$1"; }
fetch(){ # proto path dest
  set -- "$1" "${2#./}" "$3"
  local p=$2 d=$3 e; e=$(enc "$2")
  case $1 in
    smb) smbclient "//$TS_HOST/corpus" -p "$TS_SMB_PORT" -U "$TS_USER%$TS_PASS" -c "get \"$p\" \"$d\"" >/dev/null 2>&1;;
    ftp) curl -sf --max-time 30 -u "$TS_USER:$TS_PASS" "ftp://$TS_HOST:$TS_FTP_PORT/$e" -o "$d";;
    ftps) curl -sf --max-time 30 -k --ssl-reqd -u "$TS_USER:$TS_PASS" "ftp://$TS_HOST:$TS_FTPS_PORT/$e" -o "$d";;
    sftp) curl -sf --max-time 30 -k -u "$TS_USER:$TS_PASS" "sftp://$TS_HOST:$TS_SFTP_PORT/$e" -o "$d";;
    webdav) curl -sf --max-time 30 -u "$TS_USER:$TS_PASS" "http://$TS_HOST:$TS_WEBDAV_PORT/$e" -o "$d";;
    http) curl -sf --max-time 30 "http://$TS_HOST:$TS_HTTP_PORT/$e" -o "$d";;
    nfs) cp "$NFSMNT/$p" "$d";;
  esac
}
check_proto(){
  local proto=$1 run n=0 bad=0 sum
  for run in 1 2 3; do
    : > "$W/sums.$proto.$run"
    while read -r h p; do
      mkdir -p "$W/dl"; rm -f "$W/dl/f"
      fetch "$proto" "$p" "$W/dl/f" || { say "  $proto run$run FETCHFAIL $p"; bad=1; continue; }
      g=$(sha256sum "$W/dl/f" | cut -d' ' -f1); [ "$g" = "$h" ] || { say "  $proto run$run MISMATCH $p"; bad=1; }
      echo "$g $p" >> "$W/sums.$proto.$run"
    done < <(sed 's/  / /' "$ROOT/corpus/manifest.sha256")
    n=$(wc -l < "$W/sums.$proto.$run")
  done
  cmp -s "$W/sums.$proto.1" "$W/sums.$proto.2" && cmp -s "$W/sums.$proto.2" "$W/sums.$proto.3" || bad=1
  if [ $bad = 0 ]; then say "PASS $proto: $n files x3 runs identical, sha256 == manifest"; else say "FAIL $proto"; FAIL=1; fi
}
neg(){ # name, command that MUST fail
  local n=$1; shift
  if "$@" >/dev/null 2>&1; then say "FAIL neg-control $n (wrong credential accepted)"; FAIL=1; else say "PASS neg-control $n (wrong credential rejected)"; fi
}
NFSMNT=
if [ -n "${TS_NFS_MNT:-}" ] && grep -q " ${TS_NFS_MNT} nfs" /proc/mounts; then
  NFSMNT=$TS_NFS_MNT; check_proto nfs   # pre-mounted by the operator (mount needs privilege)
elif [ "${TS_NFS_MOUNT:-0}" = 1 ]; then
  NFSMNT=$W/nfs; mkdir -p "$NFSMNT"
  if sudo -n mount -t nfs -o ro,nfsvers=3,tcp,nolock,port=$TS_NFS_PORT,mountport=$TS_NFS_PORT "$TS_HOST:/" "$NFSMNT"; then
    trap 'sudo -n umount "$NFSMNT" 2>/dev/null; rm -rf "${W:?}"' EXIT
    check_proto nfs
  else say "SKIP nfs (mount failed / no privilege)"; NFSMNT=; fi
else say "SKIP nfs (set TS_NFS_MNT=<premounted dir> or TS_NFS_MOUNT=1)"; fi
for p in smb ftp ftps sftp webdav http; do check_proto $p; done
BAD=wrongpass
neg smb smbclient "//$TS_HOST/corpus" -p "$TS_SMB_PORT" -U "$TS_USER%$BAD" -c ls
neg ftp curl -sf --max-time 15 -u "$TS_USER:$BAD" "ftp://$TS_HOST:$TS_FTP_PORT/docs/readme.txt"
neg ftps curl -sf --max-time 15 -k --ssl-reqd -u "$TS_USER:$BAD" "ftp://$TS_HOST:$TS_FTPS_PORT/docs/readme.txt"
neg sftp curl -sf --max-time 15 -k -u "$TS_USER:$BAD" "sftp://$TS_HOST:$TS_SFTP_PORT/docs/readme.txt"
neg webdav curl -sf --max-time 15 -u "$TS_USER:$BAD" "http://$TS_HOST:$TS_WEBDAV_PORT/docs/readme.txt"
# write control: corpus is read-only outside rw/; rw/ writable (webdav PUT)
if curl -sf --max-time 15 -u "$TS_USER:$TS_PASS" -T "$HERE/ports.env" "http://$TS_HOST:$TS_WEBDAV_PORT/rw/_probe.txt" -o /dev/null && \
   ! curl -sf --max-time 15 -u "$TS_USER:$TS_PASS" -T "$HERE/ports.env" "http://$TS_HOST:$TS_WEBDAV_PORT/docs/_probe.txt" -o /dev/null; then
  say "PASS write-control: rw/ writable, corpus read-only"
else say "FAIL write-control"; FAIL=1; fi
curl -s --max-time 15 -u "$TS_USER:$TS_PASS" -X DELETE "http://$TS_HOST:$TS_WEBDAV_PORT/rw/_probe.txt" -o /dev/null
say "RESULT fail=$FAIL log=$LOG"; exit $FAIL
