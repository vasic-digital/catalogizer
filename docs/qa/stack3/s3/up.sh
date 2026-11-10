#!/usr/bin/env bash
# up.sh - start a REAL Garage S3 server (rootless podman) for the Catalogizer live S3 tests (CAT-S3-1).
# Own network (catqa-s3live), own container; does not touch catqa1/2/3 stacks or the shared podman network.
# Writes credentials ONLY to ${S3_STATE:?}/s3.env (mode 0600), never to stdout.
set -euo pipefail
S3_STATE="${S3_STATE:-$HOME/.cache/claude-tmp/catqa-s3live}"
. "$(cd "$(dirname "${BASH_SOURCE[0]}")/../../lib" && pwd)/lan_ip.sh"
S3_BIND="${S3_BIND:-127.0.0.1}"   # LAN opt-in: S3_BIND=lan (auto-detected host LAN address; override with QA_BIND/LAN_BIND) or an explicit IPv4
if [ "$S3_BIND" = lan ]; then S3_BIND="$(qa_lan_ip)" || exit 1; fi
S3_PORT="${S3_PORT:-39320}"
IMAGE="${S3_IMAGE:-docker.io/dxflrs/garage@sha256:a5706cf1f3d7b349ac5133ec59ad8181b709270b75e5f2fa7b3e1a5c07d67137}"  # pinned by digest (= v1.0.1)
NAME=catqa-s3live-garage
NET=catqa-s3live
BUCKET=catalogizer-live
: "${S3_STATE:?}"
umask 077
mkdir -p "$S3_STATE/meta" "$S3_STATE/data"
RPC_SECRET="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
if [ ! -f "$S3_STATE/garage.toml" ]; then
cat > "$S3_STATE/garage.toml" <<TOML
metadata_dir = "/var/lib/garage/meta"
data_dir = "/var/lib/garage/data"
db_engine = "sqlite"
replication_factor = 1
rpc_bind_addr = "127.0.0.1:3901"
rpc_public_addr = "127.0.0.1:3901"
rpc_secret = "$RPC_SECRET"
[s3_api]
s3_region = "garage"
api_bind_addr = "0.0.0.0:3900"
root_domain = ".s3.garage.invalid"
TOML
fi
podman network exists "$NET" || podman network create "$NET" >/dev/null
podman rm -f "$NAME" >/dev/null 2>&1 || true
podman run -d --name "$NAME" --network "$NET" --memory 1g --memory-swap 1g --cpus 1 \
  -p "$S3_BIND:$S3_PORT:3900" \
  -v "$S3_STATE/garage.toml:/etc/garage.toml:ro,Z" -v "$S3_STATE/meta:/var/lib/garage/meta:Z" -v "$S3_STATE/data:/var/lib/garage/data:Z" \
  "$IMAGE" /garage -c /etc/garage.toml server >/dev/null
for i in $(seq 1 40); do podman exec "$NAME" /garage -c /etc/garage.toml status >/dev/null 2>&1 && break; sleep 1; done
g() { podman exec "$NAME" /garage -c /etc/garage.toml "$@"; }
NODE="$(g status 2>/dev/null | awk '/^[0-9a-f]{16}/{print $1; exit}')"
: "${NODE:?garage node id not found}"
if ! g layout show 2>/dev/null | grep -q "$NODE"; then
  g layout assign -z dc1 -c 1G "$NODE" >/dev/null
  g layout apply --version 1 >/dev/null
fi
sleep 2
g bucket create "$BUCKET" >/dev/null 2>&1 || true
out="$(g key create catalogizer-live 2>&1 || g key info catalogizer-live --show-secret 2>&1)"
KID="$(printf '%s\n' "$out" | awk -F': *' '/Key ID/{print $2; exit}')"
KSEC="$(printf '%s\n' "$out" | awk -F': *' '/Secret key/{print $2; exit}')"
if [ -z "$KSEC" ]; then out="$(g key info catalogizer-live --show-secret 2>&1)"; KSEC="$(printf '%s\n' "$out" | awk -F': *' '/Secret key/{print $2; exit}')"; fi
: "${KID:?no key id}"; : "${KSEC:?no secret}"
g bucket allow --read --write --owner "$BUCKET" --key catalogizer-live >/dev/null
{
  echo "S3_LIVE_ENDPOINT=http://$S3_BIND:$S3_PORT"
  echo "S3_LIVE_REGION=garage"
  echo "S3_LIVE_BUCKET=$BUCKET"
  echo "S3_LIVE_ACCESS_KEY=$KID"
  echo "S3_LIVE_SECRET_KEY=$KSEC"
  echo "S3_LIVE_ALLOW_CIDR=$S3_BIND/32"
} > "$S3_STATE/s3.env"
chmod 0600 "$S3_STATE/s3.env"
echo "garage up: endpoint http://$S3_BIND:$S3_PORT bucket $BUCKET env-file $S3_STATE/s3.env (0600)"
