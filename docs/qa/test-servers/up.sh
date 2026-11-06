#!/usr/bin/env bash
# Idempotent: start (or replace) the rootless test-server containers. Never touches other stacks.
set -euo pipefail
HERE=$(cd "$(dirname "$0")" && pwd)
. "$HERE/ports.env"
ROOT=${TS_ROOT:-/home/milosvasic/Testing-servers}
ENVF=${TS_ENV:-/home/milosvasic/.cache/claude-tmp/catqa3/testservers.env}
IMG=localhost/cat-testservers:3
[ -d "$ROOT/corpus" ] && [ -f "$ENVF" ] || { echo "missing corpus or env"; exit 1; }
set -a; . "$ENVF"; set +a
podman image exists "$IMG" || nice -n 10 podman build -q -t "$IMG" -f "$HERE/Containerfile" "$HERE"
run() { # name, cmd...
  local n=$1; shift
  podman rm -f "cattsrv-$n" >/dev/null 2>&1 || true
  podman run -d --name "cattsrv-$n" --network host --restart on-failure:3 \
    --env-file "$ENVF" \
    -v "$ROOT/corpus:/srv/corpus:ro" -v "$ROOT/corpus/rw:/srv/corpus/rw:rw" \
    -v "$ROOT/tls:/tls:ro" -v "$HERE/smb.conf:/etc/samba/smb.conf:ro" -v "$HERE/ftps_server.py:/etc/ftps_server.py:ro" \
    "$IMG" sh -c "$*" >/dev/null
}
run smb 'mkdir -p /tmp/private; adduser -D catsvc 2>/dev/null; printf "%s\n%s\n" "$TS_PASS" "$TS_PASS" | smbpasswd -a -s catsvc >/dev/null; exec smbd -F --no-process-group -s /etc/samba/smb.conf'
run ftp "exec rclone serve ftp /srv/corpus --addr :$TS_FTP_PORT --user \$TS_USER --pass \$TS_PASS --passive-port $TS_PASV_RANGE --public-ip \$TS_HOST"
run ftps "LO=${TS_PASV_RANGE_FTPS%-*} HI=${TS_PASV_RANGE_FTPS#*-} PORT=$TS_FTPS_PORT exec python3 /etc/ftps_server.py"
run sftp "exec rclone serve sftp /srv/corpus --key /tls/ssh_host_ed25519 --addr :$TS_SFTP_PORT --user \$TS_USER --pass \$TS_PASS"
run webdav "exec rclone serve webdav /srv/corpus --addr :$TS_WEBDAV_PORT --user \$TS_USER --pass \$TS_PASS"
run http "exec rclone serve http /srv/corpus --addr :$TS_HTTP_PORT --read-only"
run nfs "exec rclone serve nfs /srv/corpus --addr :$TS_NFS_PORT --read-only"
podman ps --filter name=cattsrv- --format '{{.Names}} {{.Status}}'
