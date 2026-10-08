#!/usr/bin/env bash
# roundtrip.sh - T132. One real round trip against one protocol of a running test-infrastructure project, from IMG-INFRA-CLIENT on the project network
# (run_client.sh): PostgreSQL (create/insert/read/update/delete), Redis (SET/GET/INCR/EXPIRE/DEL), FTP in passive mode, SMB, WebDAV, and NFS (the unprivileged user-space server of T134, libnfs clients) (corpus files compared with
# the corpus manifest sha256, upload, download back, delete). Every step prints `STEP <name> ok|FAIL`; the verdict is the last line `PASS|FAIL roundtrip ...`.
# MinIO is BLOCKED (no obtainable image, evidence/wp12/minio-blocked.txt): `BLOCKED roundtrip minio reason=image_unavailable`, exit 3, never a pass.
# Usage:  roundtrip.sh --build-id <id> --protocol postgres|redis|ftp|smb|webdav|nfs|minio [--record <evidence-dir> --iteration <n>]
#   --record DIR   (made absolute against the caller's cwd) run the round trip through the evidence recorder (tools/evidence/evrec): one `ev/1` record is appended to DIR/ledger.jsonl (anchor and blobs
#                  beside it), polarity GREEN, evidence class runtime, oracle `specified` (the answers are defined by the protocols and the corpus manifest, independent
#                  of the code under test), test source the client script. The recorder's inputs are pinned (scratch repository root, no host entry).
# Exit:   0 PASS; 1 FAIL (without --record); 3 BLOCKED; 2 usage. With --record: 0 PASS and one record appended; 65 the run did not pass, so evrec REFUSES to store a GREEN record (nothing is appended); 1 the run passed but the
#         ledger did not grow by exactly one record (`record_not_appended`). Env: TI_RT_CLIENT_DIR (repo-relative client script directory; a test hook for mutation copies), TI_PROBE_HOST_<PROTO>.
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$HERE/lib.sh"
BID=""; PROTO=""; REC=""; ITER=1
while [ $# -gt 0 ]; do
  case "$1" in --build-id) ti_optval "$1" $# "${2:-}"; BID=$2; shift 2;; --protocol) ti_optval "$1" $# "${2:-}"; PROTO=$2; shift 2;; --record) ti_optval "$1" $# "${2:-}"; REC=$2; shift 2;; --iteration) ti_optval "$1" $# "${2:-}"; ITER=$2; shift 2;; *) ti_die "unknown argument '$1'" 2;; esac
done
ti_valid_id "$BID" || ti_die "--build-id must match ^[a-z0-9][a-z0-9-]{0,30}\$" 2
ti_uint "$ITER" 9 --iteration
[ -z "$REC" ] || REC="$(ti_abs "$REC")"   # a relative --record resolves against the CALLER's cwd, never against the recorder's scratch directory (WF17 TI-D3)
case "$PROTO" in
  minio) echo "BLOCKED roundtrip minio reason=image_unavailable no registry serves the MinIO image (evidence/wp12/minio-blocked.txt); not a pass"; exit 3;;
  postgres|redis|ftp|smb|webdav|nfs) ;;
  *) ti_die "unknown protocol '$PROTO'" 2;;
esac
CDIR="${TI_RT_CLIENT_DIR:-scripts/test-infra/client}"
EVF=(); for h in POSTGRES REDIS FTP SMB WEBDAV NFS; do v="TI_PROBE_HOST_$h"; [ -z "${!v:-}" ] || EVF+=("$v=${!v}"); done
[ -z "${TI_RT_FAILFAST:-}" ] || EVF+=("TI_RT_FAILFAST=$TI_RT_FAILFAST")   # stop at the first failing step (the sabotage runs of the tests)
RUN=(bash "$HERE/run_client.sh" --build-id "$BID" -- env "${EVF[@]}" bash "/src/$CDIR/roundtrip_$PROTO.sh")
if [ -z "$REC" ]; then exec "${RUN[@]}"; fi
ti_need python3
mkdir -p "$REC" || ti_die "cannot create $REC"
H="$(mktemp -d "${TMPDIR:-/tmp}/ti-evrec.XXXXXX")"; ti_exit_on_signals; trap 'rm -rf -- "${H:?}"' EXIT
BEFORE="$(grep -c . "$REC/ledger.jsonl" 2>/dev/null || true)"; BEFORE=${BEFORE:-0}
mkdir -p "$H/repo/.audit" "$H/home"; printf '#!/usr/bin/env bash\nexit 99\n' >"$H/host_entry.sh"; chmod +x "$H/host_entry.sh"
SRC="$TI_ROOT/$CDIR/roundtrip_$PROTO.sh"; ITEM=RUN-132; [ "$PROTO" != nfs ] || ITEM=RUN-134   # the register item of the task whose evidence this is (T132 round trips, T134 NFS)
# not `env -i`: the round trip itself needs the caller's HOME and XDG_RUNTIME_DIR for rootless podman; only the recorder's own inputs are pinned here
cd "$H" && env -u EV -u EV_SCHEMA -u EV_TEST_SOURCES -u EV_LEDGER_BOUND -u EV_BLOB_BOUND -u EVREC_FAULT -u EVREC_TURN_RUN_ID \
  CPA_HOST_ENTRY="$H/host_entry.sh" EVREC_REPO_ROOT="$H/repo" EV_LEDGER="$REC/ledger.jsonl" EV_ANCHOR="$REC/anchor.jsonl" EV_BLOBS="$REC/blobs" \
  python3 "$TI_ROOT/tools/evidence/evrec" run "$ITEM" GREEN "$ITER" shell_script "$SRC" --evidence-class runtime --oracle specified --oracle-independent --test-source "$SRC" -- "${RUN[@]}" 9>&-
ERC=$?
# the record is verified, not assumed: a recorded pass must have GROWN the ledger by exactly one record (WF17 TI-D3)
if [ "$ERC" = 0 ]; then
  [ "$(grep -c . "$REC/ledger.jsonl" 2>/dev/null || echo 0)" = "$((BEFORE + 1))" ] || { echo "test-infra: REFUSED reason=record_not_appended the ledger $REC/ledger.jsonl did not grow by exactly one record" >&2; exit 1; }
fi
exit "$ERC"
