#!/usr/bin/env bash
# Idempotent: start (or replace) the rootless test-server containers. Never touches other stacks.
set -euo pipefail
HERE=$(cd "$(dirname "$0")" && pwd)
. "$HERE/ports.env"
ROOT=${TS_ROOT:-/home/milosvasic/Testing-servers}
ENVF=${TS_ENV:-/home/milosvasic/.cache/claude-tmp/catqa3/testservers.env}
IMG=localhost/cat-testservers:3
[ -d "$ROOT/corpus" ] && [ -f "$ENVF" ] || { echo "missing corpus or env"; exit 1; }
. "$HERE/../lib/lan_ip.sh"
_ts_host_caller="${TS_HOST:-}"
set -a; . "$ENVF"; set +a
# TS_HOST: caller env wins, else the auto-detected LAN address (docs/qa/lib/lan_ip.sh). A TS_HOST stored in the env file is NOT trusted (it goes stale when DHCP changes the address).
if [ -n "$_ts_host_caller" ]; then TS_HOST="$_ts_host_caller"; else TS_HOST="$(qa_lan_ip)" || exit 1; fi
qa_ip_is_ipv4 "$TS_HOST" || { echo "TS_HOST '$TS_HOST' is not an IPv4 address"; exit 1; }
# Self-signed TLS cert (FTPS): regenerate when its SAN lacks the current address. New pair is generated into temp files in the SAME dir and verified
# (SAN has the address, key matches cert) BEFORE anything live is touched; the old pair is then hard-linked to unique *.bak-* names (never deleted) and the
# new files are renamed over it, so a failure at any step leaves the running pair untouched.
cert_has_ip() { openssl x509 -in "${1:-$ROOT/tls/cert.pem}" -noout -ext subjectAltName 2>/dev/null | tr ',' '\n' | grep -Eq "^[[:space:]]*IP Address:${TS_HOST//./\\.}[[:space:]]*$"; }
if ! cert_has_ip; then
  [ -d "$ROOT/tls" ] || { echo "missing $ROOT/tls"; exit 1; }
  tmpk="" tmpc=""
  cert_fail() { rm -f -- "${tmpk:-}" "${tmpc:-}"; echo "$1 (live cert/key untouched)"; exit 1; }
  tmpk="$(umask 077; mktemp "$ROOT/tls/.key.new.XXXXXX")" && tmpc="$(umask 077; mktemp "$ROOT/tls/.cert.new.XXXXXX")" || cert_fail "cannot create temp files"
  openssl req -x509 -newkey rsa:2048 -nodes -days 365 -subj "/CN=anton.local" \
    -addext "subjectAltName=DNS:anton.local,IP:$TS_HOST" -keyout "$tmpk" -out "$tmpc" >/dev/null 2>&1 || cert_fail "cert generation failed"
  cert_has_ip "$tmpc" || cert_fail "generated cert lacks SAN IP $TS_HOST"
  pk_c="$(openssl x509 -in "$tmpc" -noout -pubkey 2>/dev/null)" && pk_k="$(openssl pkey -in "$tmpk" -pubout 2>/dev/null)" && [ -n "$pk_c" ] && [ "$pk_c" = "$pk_k" ] || cert_fail "generated key does not match generated cert"
  chmod 600 "$tmpk"; chmod 644 "$tmpc"
  stamp="$(date +%Y%m%dT%H%M%S.%N)-$$"
  for f in cert.pem key.pem; do [ -f "$ROOT/tls/$f" ] && { ln -- "$ROOT/tls/$f" "$ROOT/tls/$f.bak-$stamp" || cert_fail "cannot back up $f"; }; done
  mv -f -- "$tmpk" "$ROOT/tls/key.pem" && mv -f -- "$tmpc" "$ROOT/tls/cert.pem" || cert_fail "cannot move new pair into place"
  echo "TLS cert regenerated for $TS_HOST (old pair kept as *.bak-$stamp)"
fi
podman image exists "$IMG" || nice -n 10 podman build -q -t "$IMG" -f "$HERE/Containerfile" "$HERE"
run() { # name, cmd...
  local n=$1; shift
  podman rm -f "cattsrv-$n" >/dev/null 2>&1 || true
  podman run -d --name "cattsrv-$n" --network host --restart on-failure:3 \
    --env-file "$ENVF" -e "TS_HOST=$TS_HOST" \
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
