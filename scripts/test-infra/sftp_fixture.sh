#!/usr/bin/env bash
# sftp_fixture.sh - WP-12 PA-05. Runs the integration tests of submodules/filesystem/pkg/sftp against a REAL OpenSSH (internal-sftp, chrooted)
# server in a rootless container with a READ-ONLY data volume (docker-compose.test-infra.sftp.yml), end to end and self-cleaning.
#
# Usage:  scripts/test-infra/sftp_fixture.sh run [--log FILE] [--keep] -- <go test arguments>      (arguments are appended to
#         `go test -count=1 -tags integration`, e.g.  -race -run 'Integration' -v ./pkg/sftp/ )
#         scripts/test-infra/sftp_fixture.sh selftest        (generate, start, assert the server answers with the pinned host key, stop; no go test)
# What one run does, in order (every step is undone on any exit, including INT/TERM/HUP):
#   1. creates .audit/scratch/<project>/ (mode 0700): per-run password, ed25519 + rsa host keys, an ed25519 client key, users.conf (0600),
#      the seeded read-only data tree (text file with a fixed mtime, a 1 MiB random binary with its sha256, a nested dir, a symlink that
#      leaves the client's root) and a copy of submodules/filesystem (the container sees a scratch VIEW, never the real .env of the checkout);
#   2. registers a long operation (scripts/longops/register.sh, owner pid = this script) so the anti-mess sweep matches the containers;
#   3. docker-compose.test-infra.sftp.yml up (podman-compose, in_pod false) and waits for sshd;
#   4. runs `go test` in the pinned IMG-GO image (scripts/containers/run_pinned.sh) attached to the project network, with the credentials
#      in a mode 0600 env file (never argv). The host key fingerprint the test pins is computed HERE, outside the container, from the
#      public key file: that is the "owner confirmation" of the pin workflow;
#   5. tears the project down (compose down -v), checks nothing labelled for it remains, releases the operation, removes the scratch.
# Every run prints (and, with --log, writes as the first lines of the log) the sha256 of the exact tree the container tested, and of its pkg/sftp subtree alone. SFTP_FIXTURE_SRC
# (a copy of the module) is honoured only together with SFTP_FIXTURE_ALLOW_SRC=1.
# Secrets are never printed. Exit: the go test exit code; 1 REFUSED/failed (`sftp-fixture: REFUSED reason=<code>` on stderr); 2 usage;
# 3 selftest stopped by the test hook SFTP_FIXTURE_STOP_AFTER_HASH=1 (see below: refused in `run` mode, so a run can never exit 0 without a test run).
# The complete list of environment variables this script reads (review WF24 S06: a stray exported variable must not change what a run means):
#   SFTP_FIXTURE_SRC + SFTP_FIXTURE_ALLOW_SRC=1   test another copy of the module (declared);   SFTP_FIXTURE_STOP_AFTER_HASH=1   selftest only (exit 3).
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
COMPOSE_FILE="$ROOT/docker-compose.test-infra.sftp.yml"
refuse() { echo "sftp-fixture: REFUSED reason=$1 ${2:-}" >&2; exit 1; }
usage() { echo "sftp-fixture: usage: sftp_fixture.sh run [--log FILE] [--keep] -- <go test args> | selftest" >&2; exit 2; }

MODE="${1:-}"; [ -n "$MODE" ] || usage; shift
# the stop hook belongs to the gate test (selftest); in `run` mode it would end the script with a success-looking exit and no test run
if [ "${SFTP_FIXTURE_STOP_AFTER_HASH:-}" = 1 ] && [ "$MODE" != selftest ]; then
  refuse stop_hook_only_in_selftest "SFTP_FIXTURE_STOP_AFTER_HASH=1 is a test hook of the selftest mode; unset it (a run must execute go test)"
fi
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

for t in podman podman-compose ssh-keygen openssl python3 tar; do command -v "$t" >/dev/null 2>&1 || refuse tool_missing "$t"; done
[ -r "$COMPOSE_FILE" ] || refuse compose_missing "$COMPOSE_FILE"

ID="$(date -u +%Y%m%d%H%M%S)-$$"
PROJECT="catalogizer-sftp-$ID"
OPID="$PROJECT"
SD="$ROOT/.audit/scratch/$PROJECT"
USER_NAME="sftpuser"
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
    SFTP_OP_ID="$OPID" SFTP_PROJECT="$PROJECT" SFTP_DIR="$SD" SFTP_USER="$USER_NAME" \
      podman-compose -p "$PROJECT" -f "$COMPOSE_FILE" down -v >/dev/null 2>&1
    # anything of this project that survived is a leak, reported and removed by exact label (never by name pattern)
    local left
    left="$(podman ps -a -q --filter "label=catalogizer.test_project=$PROJECT" 2>/dev/null)"
    if [ -n "$left" ]; then
      echo "sftp-fixture: leak: containers of $PROJECT survived compose down: $left (removing)" >&2
      # shellcheck disable=SC2086
      podman rm -f $left >/dev/null 2>&1
      rc_state=failed; verdict=leak_after_down
    fi
    podman network rm -f "${PROJECT}_test-network" >/dev/null 2>&1
    podman network exists "${PROJECT}_test-network" 2>/dev/null && { echo "sftp-fixture: leak: network ${PROJECT}_test-network still exists" >&2; rc_state=failed; verdict=network_leak; }
  fi
  if [ "$OP_REGISTERED" = 1 ]; then
    lo release --op-id "$OPID" --state "$rc_state" --verdict "$verdict" >/dev/null 2>&1 || true
  fi
  # the scratch directory is removed only when it is exactly this run's directory under .audit/scratch (never an empty or unresolved variable)
  if [ "$KEEP" = 0 ] && [ -n "$ROOT" ] && [ -d "$SD" ]; then
    case "$SD" in "$ROOT"/.audit/scratch/catalogizer-sftp-?*) rm -rf -- "$SD";; *) echo "sftp-fixture: refusing to remove unexpected scratch path" >&2;; esac
  fi
  [ "$rc_state" = complete ]
}
trap 'cleanup' EXIT
trap 'cleanup; exit 130' INT TERM HUP

# ---- 1. per-run material ----
( umask 077; mkdir -p "$SD/data/sub/deep" "$SD/view/fixture" ) || refuse scratch_uncreatable "$SD"
chmod 700 "$SD" "$SD/view" "$SD/view/fixture"
PASS="$(openssl rand -hex 16)"
( umask 077; printf '%s:%s:1001:100\n' "$USER_NAME" "$PASS" >"$SD/users.conf" )
ssh-keygen -q -t ed25519 -N '' -C sftp-fixture-host -f "$SD/ssh_host_ed25519_key" || refuse keygen_failed host_ed25519
ssh-keygen -q -t rsa -b 2048 -N '' -C sftp-fixture-host -f "$SD/ssh_host_rsa_key" || refuse keygen_failed host_rsa
ssh-keygen -q -t ed25519 -N '' -C sftp-fixture-client -f "$SD/client_key" || refuse keygen_failed client
chmod 600 "$SD"/ssh_host_*_key "$SD/client_key"
chmod 644 "$SD"/*.pub
HOSTFP="$(ssh-keygen -lf "$SD/ssh_host_ed25519_key.pub" | awk '{print $2}')"
case "$HOSTFP" in SHA256:*) ;; *) refuse fingerprint_unreadable "$HOSTFP";; esac

# the read-only data tree (readable by the container user: dirs 755, files 644)
printf 'hello sftp\n' >"$SD/data/a.txt"
head -c 1048576 /dev/urandom >"$SD/data/sub/b.bin"
printf 'c\n' >"$SD/data/sub/deep/c.txt"
touch -d '2020-05-17 10:30:00 UTC' "$SD/data/a.txt"
ln -s .. "$SD/data/escape"                      # leaves the client's root (/data) but stays inside the chroot
ln -s a.txt "$SD/data/inside-link"
find "$SD/data" -type d -exec chmod 755 {} +
find "$SD/data" -type f -exec chmod 644 {} +
B_SHA="$(sha256sum "$SD/data/sub/b.bin" | awk '{print $1}')"
A_MTIME="$(stat -c %Y "$SD/data/a.txt")"
cp "$SD/client_key" "$SD/view/fixture/client_key"
# SFTP_FIXTURE_SRC (a directory holding a copy of the module) replaces the checked-in module in the view: the mutation harness uses it.
# It must be DECLARED with SFTP_FIXTURE_ALLOW_SRC=1: a stray exported SFTP_FIXTURE_SRC would otherwise test other bytes than the checkout
# while every header of the run still describes the checkout (review SFTP-19).
if [ -n "${SFTP_FIXTURE_SRC:-}" ] && [ "${SFTP_FIXTURE_ALLOW_SRC:-}" != 1 ]; then
  refuse src_override_not_declared "SFTP_FIXTURE_SRC is set; set SFTP_FIXTURE_ALLOW_SRC=1 to declare a mutation or verification run on a copy"
fi
SRC_MODULE="${SFTP_FIXTURE_SRC:-$ROOT/submodules/filesystem}"
[ -d "$SRC_MODULE" ] || refuse src_module_missing "$SRC_MODULE"
mkdir -p "$SD/view/filesystem"
tar --exclude=.git -C "$SRC_MODULE" -cf - . | tar -C "$SD/view/filesystem" -xf - || refuse view_copy_failed
# the hash of exactly the tree the container will test (sorted file list, so it is reproducible from a checkout with
# `cd submodules/filesystem && find . -path ./.git -prune -o -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum`)
TREE_SHA="$(cd "$SD/view/filesystem" && find . -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum | awk '{print $1}')"
[ "${#TREE_SHA}" = 64 ] || refuse tree_hash_failed "$TREE_SHA"
# the same for the package under test alone: the whole module is shared with other work (pkg/ftp, pkg/factory ...) that changes while this one is tested,
# so only this second hash is stable across runs and comparable with the checkout
PKG_SHA="$(cd "$SD/view/filesystem/pkg/sftp" 2>/dev/null && find . -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum | awk '{print $1}')"
[ "${#PKG_SHA}" = 64 ] || refuse tree_hash_failed "pkg/sftp: $PKG_SHA"
echo "sftp-fixture: tested tree sha256=$TREE_SHA pkg/sftp sha256=$PKG_SHA source=$SRC_MODULE" >&2
# test hook (sftp_fixture_gate_test.sh, selftest mode only - refused above otherwise): stop here, before any container, after the hash
# line, with the distinct exit code 3 so that nobody can mistake it for a passed run
[ "${SFTP_FIXTURE_STOP_AFTER_HASH:-}" = 1 ] && exit 3

ENVF="$SD/test.env"
( umask 077; cat >"$ENVF" <<E
SFTP_TEST_HOST=sftp
SFTP_TEST_PORT=22
SFTP_TEST_USER=$USER_NAME
SFTP_TEST_PASSWORD=$PASS
SFTP_TEST_ROOT=/data
SFTP_TEST_HOSTKEY_FP_ED25519=$HOSTFP
SFTP_TEST_KEY_FILE=/src/fixture/client_key
SFTP_TEST_B_SHA256=$B_SHA
SFTP_TEST_B_SIZE=1048576
SFTP_TEST_A_MTIME=$A_MTIME
E
)

# ---- 2. registered long operation ----
lo register --purpose "$PROJECT" --owner sftp-fixture --op-id "$OPID" --pid "$$" --container-label "$PROJECT" --no-progress-s 900 >/dev/null 2>"$SD/register.err" \
  || refuse register_failed "$(tr '\n' ' ' <"$SD/register.err" | cut -c1-200)"
OP_REGISTERED=1
echo "sftp-fixture: registered op_id=$OPID" >&2

# ---- 3. start ----
UP=1
export SFTP_OP_ID="$OPID" SFTP_PROJECT="$PROJECT" SFTP_DIR="$SD" SFTP_USER="$USER_NAME"
podman-compose -p "$PROJECT" -f "$COMPOSE_FILE" up -d >"$SD/up.log" 2>&1 || { sed 's/^/  up: /' "$SD/up.log" >&2; refuse compose_up_failed "$PROJECT"; }
READY=0
for _ in $(seq 1 60); do
  if podman logs "${PROJECT}_sftp_1" 2>&1 | grep -q 'Executing sshd'; then READY=1; break; fi
  sleep 1
done
[ "$READY" = 1 ] || { podman logs "${PROJECT}_sftp_1" 2>&1 | tail -15 | sed 's/^/  sftp: /' >&2; refuse sshd_not_ready "$PROJECT"; }
NET="${PROJECT}_test-network"
podman network exists "$NET" || refuse network_absent "$NET"

# ---- 4. run in the pinned Go image on the project network ----
GOCMD='cd /src/filesystem && go test -count=1 -tags integration'
for a in "${TESTARGS[@]:-}"; do [ -n "$a" ] && GOCMD+=" $(printf '%q' "$a")"; done
if [ "$MODE" = selftest ]; then GOCMD='cd /src/filesystem && go test -count=1 -tags integration -run TestIntegrationSelftest -v ./pkg/sftp/'; fi
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
  printf '# tested-tree-sha256: %s\n# tested-pkg-sftp-sha256: %s\n# tested-tree-source: %s\n' "$TREE_SHA" "$PKG_SHA" "$SRC_MODULE" >"$LOG"
  ( cd "$SD/view" && "${NEW[@]}" ) 2>&1 | tee -a "$LOG"; RC=${PIPESTATUS[0]}
else
  ( cd "$SD/view" && "${NEW[@]}" ); RC=$?
fi
cleanup || RC=1
exit "$RC"
