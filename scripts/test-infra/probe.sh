#!/usr/bin/env bash
# probe.sh - T128. Protocol-level probes of a running test-infrastructure project: every probe speaks the service's own protocol with the
# generated credentials and checks an ANSWER (SELECT 1 -> 1, PING -> PONG, FTP passive transfer sha256, SMB share list, WebDAV PROPFIND 207).
# A probe that only checks an open TCP port is a carrier and is not accepted: the postgres probe pointed at a bare TCP listener FAILs.
# Every client runs inside IMG-INFRA-CLIENT through scripts/test-infra/run_client.sh (run_pinned.sh, the project's compose network): never a host client,
# never an `exec` into a service container.
# Usage:  probe.sh --build-id <id> [--services postgres,redis,minio,ftp,smb,webdav,nfs | all]   (all = the first six; nfs only when named)
# Output: one line per probe `PROBE <proto> PASS|FAIL|BLOCKED <detail>`, then `PROBES pass=<n> fail=<n> blocked=<n>`.
# Exit:   0 no probe failed and at least one passed; 1 a probe failed; 3 every requested probe is BLOCKED (minio alone); 2 usage;
#         1 REFUSED reason=probe_runner_not_container when TI_PROBE_RUNNER is set to anything but `container`.
# MinIO: BLOCKED with reason image_unavailable (no registry serves the image; specs/001-full-project-audit-remediation/evidence/wp12/minio-blocked.txt). Never PASS.
# Env: TI_PROBE_RUNNER (default container), TI_PROBE_CLIENT_DIR (repo-relative directory of the client scripts, test hook for mutation copies),
#      TI_PROBE_HOST_<PROTO> (override one service host for the carrier fixture).
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$HERE/lib.sh"
BID=""; SVC="all"
while [ $# -gt 0 ]; do
  case "$1" in --build-id) BID=${2:-}; shift 2;; --services) SVC=${2:-}; shift 2;; *) ti_die "unknown argument '$1'" 2;; esac
done
ti_valid_id "$BID" || ti_die "--build-id must match ^[a-z0-9][a-z0-9-]{0,30}\$" 2
[ "${TI_PROBE_RUNNER:-container}" = container ] || ti_refuse probe_runner_not_container "TI_PROBE_RUNNER=${TI_PROBE_RUNNER} (a probe never runs on the host or in a service-class image)"
[ "$SVC" != all ] || SVC="postgres,redis,minio,ftp,smb,webdav"   # nfs is probed only when asked for: it belongs to the T134 project
CDIR="${TI_PROBE_CLIENT_DIR:-scripts/test-infra/client}"
EVF=()
for h in POSTGRES REDIS FTP SMB WEBDAV NFS; do v="TI_PROBE_HOST_$h"; [ -z "${!v:-}" ] || EVF+=("$v=${!v}"); done
pass=0; fail=0; blocked=0
IFS=, read -r -a LIST <<<"$SVC"
for p in "${LIST[@]}"; do
  case "$p" in
    minio) echo "PROBE minio BLOCKED reason=image_unavailable no registry serves the MinIO image (evidence/wp12/minio-blocked.txt); not a pass"; blocked=$((blocked+1)); continue;;
    postgres|redis|ftp|smb|webdav|nfs) ;;
    *) ti_die "unknown protocol '$p'" 2;;
  esac
  out="$(bash "$HERE/run_client.sh" --build-id "$BID" -- env "${EVF[@]}" bash "/src/$CDIR/probe_$p.sh" 2>&1)"; rc=$?
  line="$(printf '%s\n' "$out" | grep -E '^(PASS|FAIL) ' | tail -1)"
  if [ "$rc" = 0 ] && [[ "$line" == PASS* ]]; then echo "PROBE $p PASS ${line#PASS }"; pass=$((pass+1))
  else echo "PROBE $p FAIL rc=$rc ${line#FAIL } $(printf '%s' "$out" | grep -v -E '^(PASS|FAIL) ' | tail -2 | tr '\n' ' ' | cut -c1-160)"; fail=$((fail+1)); fi
done
echo "PROBES pass=$pass fail=$fail blocked=$blocked"
[ "$fail" -eq 0 ] || exit 1
[ "$pass" -gt 0 ] || exit 3
exit 0
