#!/usr/bin/env bash
# ftps_fixture.sh - WP-12 PA-04. Runs the integration tests of submodules/filesystem/pkg/ftp against a REAL pure-ftpd (clear text + explicit
# FTPS on one server, docker-compose.test-infra.ftps.yml) in a rootless container, end to end and self-cleaning.
#
# Usage:  scripts/test-infra/ftps_fixture.sh run [--log FILE] [--keep] -- <go test arguments>      (arguments are appended to
#         `go test -count=1 -tags integration`, e.g.  -race -v ./pkg/ftp/ )
#         scripts/test-infra/ftps_fixture.sh selftest        (generate, start, assert the server presents the generated certificate; no go test)
# What one run does, in order (every step is undone on any exit, including INT/TERM/HUP):
#   1. creates .audit/scratch/<project>/ (mode 0700): per-run user + password, an RSA certificate+key (pure-ftpd.pem, 0600), the seeded data
#      tree (text file with a fixed mtime, a 1 MiB random binary with its sha256, a nested dir, a UTF-8 named file) and a copy of
#      submodules/filesystem (the container sees a scratch VIEW, never the real .env of the checkout); the sha256 of the certificate is
#      computed HERE with openssl: that is the "owner confirmation" of the pin workflow;
#   2. registers a long operation (scripts/longops/register.sh, owner pid = this script) so the anti-mess sweep matches the containers;
#   3. docker-compose.test-infra.ftps.yml up (podman-compose, in_pod false) and waits for the server;
#   4. runs `go test` in the pinned IMG-GO image (scripts/containers/run_pinned.sh) attached to the project network, with the credentials
#      in a mode 0600 env file (never argv);
#   5. SINK-SIDE CHECK: the digest of the served data tree (names, sizes, mtimes, sha256 of every file) taken before and after the run
#      must be identical: the read-only scan path of the client changed nothing on the server;
#   6. tears the project down (compose down -v), checks nothing labelled for it remains, releases the operation, removes the scratch.
# Secrets are never printed. Exit: the go test exit code; 1 REFUSED/failed (`ftps-fixture: REFUSED reason=<code>` on stderr); 2 usage.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
COMPOSE_FILE="$ROOT/docker-compose.test-infra.ftps.yml"
refuse() { echo "ftps-fixture: REFUSED reason=$1 ${2:-}" >&2; exit 1; }
usage() { echo "ftps-fixture: usage: ftps_fixture.sh run [--log FILE] [--keep] -- <go test args> | selftest" >&2; exit 2; }

MODE="${1:-}"; [ -n "$MODE" ] || usage; shift
LOG=""; KEEP=0
case "$MODE" in
  run)
    while [ $# -gt 0 ]; do case "$1" in
      --log) [ $# -ge 2 ] || usage; LOG="$2"; shift 2;;
      --keep) KEEP=1; shift;;
      --) shift; break;;
      *) usage;; esac; done
    [ $# -ge 1 ] || usage;;
  selftest) [ $# -eq 0 ] || usage;;
  *) usage;;
esac
TESTARGS=("$@")

for t in podman podman-compose openssl python3 tar sha256sum; do command -v "$t" >/dev/null 2>&1 || refuse tool_missing "$t"; done
[ -r "$COMPOSE_FILE" ] || refuse compose_missing "$COMPOSE_FILE"

ID="$(date -u +%Y%m%d%H%M%S)-$$"
PROJECT="catalogizer-ftps-$ID"
OPID="$PROJECT"
SD="$ROOT/.audit/scratch/$PROJECT"
USER_NAME="ftpuser"
OP_REGISTERED=0
UP=0
CLEANED=0

lo() { local s=$1; shift; bash "$ROOT/scripts/longops/$s.sh" "$@"; }

cleanup() {
  [ "$CLEANED" = 0 ] || return 0
  CLEANED=1
  trap - EXIT INT TERM HUP
  local rc_state=complete verdict=ok
  if [ "$UP" = 1 ]; then
    FTPS_OP_ID="$OPID" FTPS_PROJECT="$PROJECT" FTPS_DIR="$SD" FTPS_USER="$USER_NAME" FTPS_PASSWORD="${PASS:-x}" \
      podman-compose -p "$PROJECT" -f "$COMPOSE_FILE" down -v >/dev/null 2>&1
    # anything of this project that survived is a leak, reported and removed by exact label (never by name pattern)
    local left
    left="$(podman ps -a -q --filter "label=catalogizer.test_project=$PROJECT" 2>/dev/null)"
    if [ -n "$left" ]; then
      echo "ftps-fixture: leak: containers of $PROJECT survived compose down: $left (removing)" >&2
      # shellcheck disable=SC2086
      podman rm -f $left >/dev/null 2>&1
      rc_state=failed; verdict=leak_after_down
    fi
    podman network rm -f "${PROJECT}_test-network" >/dev/null 2>&1
    podman network exists "${PROJECT}_test-network" 2>/dev/null && { echo "ftps-fixture: leak: network ${PROJECT}_test-network still exists" >&2; rc_state=failed; verdict=network_leak; }
  fi
  if [ "$OP_REGISTERED" = 1 ]; then
    lo release --op-id "$OPID" --state "$rc_state" --verdict "$verdict" >/dev/null 2>&1 || true
  fi
  # the scratch directory is removed only when it is exactly this run's directory under .audit/scratch (never an empty or unresolved variable)
  if [ "$KEEP" = 0 ] && [ -n "$ROOT" ] && [ -d "$SD" ]; then
    case "$SD" in "$ROOT"/.audit/scratch/catalogizer-ftps-?*)
      # pure-ftpd chowns the served directory to a sub-uid of the rootless user namespace: remove it from inside that namespace
      podman unshare rm -rf -- "$SD" 2>/dev/null || rm -rf -- "$SD";; *) echo "ftps-fixture: refusing to remove unexpected scratch path" >&2;; esac
  fi
  [ "$rc_state" = complete ]
}
trap 'cleanup' EXIT
trap 'cleanup; exit 130' INT TERM HUP

# ---- 1. per-run material ----
( umask 077; mkdir -p "$SD/data/sub/deep" "$SD/view" "$SD/disk" ) || refuse scratch_uncreatable "$SD"
chmod 700 "$SD" "$SD/view" "$SD/disk"
# the disk head-room records of this run go to the per-run scratch (removed at the end), never into the tracked evidence tree
export DISK_HEADROOM_OUT_DIR="${DISK_HEADROOM_OUT_DIR:-$SD/disk}"
PASS="$(openssl rand -hex 16)"
# one RSA certificate+key file for pure-ftpd; SAN covers the compose service name the test connects to
( umask 077; openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj "/CN=ftp" -addext "subjectAltName=DNS:ftp,DNS:localhost" \
    -keyout "$SD/key.pem" -out "$SD/cert.pem" >/dev/null 2>"$SD/openssl.err" ) || refuse cert_failed "$(tr '\n' ' ' <"$SD/openssl.err" | cut -c1-200)"
( umask 077; cat "$SD/key.pem" "$SD/cert.pem" >"$SD/pure-ftpd.pem" )
chmod 600 "$SD/pure-ftpd.pem"; chmod 644 "$SD/cert.pem"
CERTFP="$(openssl x509 -in "$SD/cert.pem" -noout -fingerprint -sha256 | sed 's/^.*=//')"
case "$CERTFP" in [0-9A-F][0-9A-F]:*) CERTFP="SHA256:$CERTFP";; *) refuse fingerprint_unreadable "$CERTFP";; esac

# the data tree served by the FTP user (readable by the container's ftp account: dirs 755, files 644)
printf 'hello ftp\n' >"$SD/data/a.txt"
head -c 1048576 /dev/urandom >"$SD/data/sub/b.bin"
printf 'c\n' >"$SD/data/sub/deep/c.txt"
printf 'utf8\n' >"$SD/data/Čšž_日本.txt"
touch -d '2020-05-17 10:30:00 UTC' "$SD/data/a.txt"
touch -d '2021-01-02 03:04:05 UTC' "$SD/data/sub/deep/c.txt"
find "$SD/data" -type d -exec chmod 755 {} +
find "$SD/data" -type f -exec chmod 644 {} +
B_SHA="$(sha256sum "$SD/data/sub/b.bin" | awk '{print $1}')"
A_MTIME="$(stat -c %Y "$SD/data/a.txt")"
C_MTIME="$(stat -c %Y "$SD/data/sub/deep/c.txt")"
# digest of the served tree: relative name, size, mtime, content hash of every entry. Compared before and after the run (sink side).
tree_digest() { ( cd "$SD/data" && find . -mindepth 1 -printf '%P\t%y\t%s\t%T@\t' -exec sh -c 'if [ -f "$1" ]; then sha256sum < "$1"; else echo -; fi' _ {} \; | LC_ALL=C sort | sha256sum | awk '{print $1}' ); }
TREE_BEFORE="$(tree_digest)"
[ -n "$TREE_BEFORE" ] || refuse tree_digest_empty
# FTPS_FIXTURE_SRC (a directory holding a copy of the module) replaces the checked-in module in the view: the mutation harness uses it
SRC_MODULE="${FTPS_FIXTURE_SRC:-$ROOT/submodules/filesystem}"
[ -d "$SRC_MODULE" ] || refuse src_module_missing "$SRC_MODULE"
mkdir -p "$SD/view/filesystem"
tar --exclude=.git -C "$SRC_MODULE" -cf - . | tar -C "$SD/view/filesystem" -xf - || refuse view_copy_failed

ENVF="$SD/test.env"
( umask 077; cat >"$ENVF" <<E
FTP_TEST_HOST=ftp
FTP_TEST_PORT=21
FTP_TEST_USER=$USER_NAME
FTP_CRED_FIXTURE_PASSWORD=$PASS
FTP_TEST_CERT_FP=$CERTFP
FTP_TEST_B_SHA256=$B_SHA
FTP_TEST_B_SIZE=1048576
FTP_TEST_A_MTIME=$A_MTIME
FTP_TEST_C_MTIME=$C_MTIME
E
)

# ---- 2. registered long operation ----
lo register --purpose "$PROJECT" --owner ftps-fixture --op-id "$OPID" --pid "$$" --container-label "$PROJECT" --no-progress-s 900 >/dev/null 2>"$SD/register.err" \
  || refuse register_failed "$(tr '\n' ' ' <"$SD/register.err" | cut -c1-200)"
OP_REGISTERED=1
echo "ftps-fixture: registered op_id=$OPID" >&2

# ---- 3. start ----
UP=1
export FTPS_OP_ID="$OPID" FTPS_PROJECT="$PROJECT" FTPS_DIR="$SD" FTPS_USER="$USER_NAME" FTPS_PASSWORD="$PASS"
podman-compose -p "$PROJECT" -f "$COMPOSE_FILE" up -d >"$SD/up.log" 2>&1 || { sed 's/^/  up: /' "$SD/up.log" >&2; refuse compose_up_failed "$PROJECT"; }
READY=0
for _ in $(seq 1 60); do
  # ready = the server answers with its 220 banner on its own control port (probed from inside the container)
  if podman exec "${PROJECT}_ftp_1" bash -c 'read -r -t 3 l < /dev/tcp/127.0.0.1/21 && [ "${l:0:3}" = 220 ]' >/dev/null 2>&1; then READY=1; break; fi
  sleep 1
done
[ "$READY" = 1 ] || { podman logs "${PROJECT}_ftp_1" 2>&1 | tail -15 | sed 's/^/  ftp: /' >&2; refuse ftpd_not_ready "$PROJECT"; }
NET="${PROJECT}_test-network"
podman network exists "$NET" || refuse network_absent "$NET"

# ---- 4. run in the pinned Go image on the project network ----
GOCMD='cd /src/filesystem && go test -count=1 -tags integration'
for a in "${TESTARGS[@]:-}"; do [ -n "$a" ] && GOCMD+=" $(printf '%q' "$a")"; done
if [ "$MODE" = selftest ]; then GOCMD='cd /src/filesystem && go test -count=1 -tags integration -run TestIntegrationSelftest -v ./pkg/ftp/'; fi
OUTD="$SD/out"; mkdir -p "$OUTD"
ARGV=()
while IFS= read -r line; do ARGV+=("$line"); done < <(cd "$SD/view" && RUNP_PRINT_ARGV=1 bash "$ROOT/scripts/containers/run_pinned.sh" --out "$OUTD" --op-id "$OPID" IMG-GO -- \
    env GOMAXPROCS=3 GOTOOLCHAIN=local CGO_ENABLED=1 GOFLAGS=-buildvcs=false sh -c "$GOCMD")
[ "${#ARGV[@]}" -gt 5 ] && [ "${ARGV[0]}" = podman ] || refuse run_pinned_failed "run_pinned.sh printed no podman argv"
NEW=("${ARGV[0]}" "${ARGV[1]}" --network "$NET" --env-file "$ENVF")
for ((i = 2; i < ${#ARGV[@]}; i++)); do NEW+=("${ARGV[$i]}"); done
# disk head-room gate that run_pinned would run in exec mode (print mode skips it)
DISK_HEADROOM_REPO_ROOT="$ROOT" bash "$ROOT/scripts/containers/disk_headroom.sh" --need 1500000000 --op-id "$OPID-run" >/dev/null 2>&1 || refuse disk_headroom "go image run"
RC=0
if [ -n "$LOG" ]; then
  ( cd "$SD/view" && "${NEW[@]}" ) 2>&1 | tee "$LOG"; RC=${PIPESTATUS[0]}
else
  ( cd "$SD/view" && "${NEW[@]}" ); RC=$?
fi
TREE_AFTER="$(tree_digest)"
if [ "$MODE" = run ] && [ "$TREE_AFTER" != "$TREE_BEFORE" ]; then
  echo "ftps-fixture: SINK-SIDE FAIL: the served data tree changed during the run (before $TREE_BEFORE after $TREE_AFTER)" >&2
  RC=1
else
  echo "ftps-fixture: sink-side check: served tree digest unchanged ($TREE_AFTER)" >&2
fi
cleanup || RC=1
exit "$RC"
