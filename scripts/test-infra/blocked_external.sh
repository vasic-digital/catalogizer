#!/usr/bin/env bash
# blocked_external.sh - T135. Records the legs of the real-service stack that need an EXTERNAL provider credential or device, with their state: `available`, or `blocked-unavailable` with
# the reason (`credentials_absent` and the variable NAMES, BLOCKED-ON ODG-01; `image_unavailable`; `odg08_unanswered`), never `pass` (availability is not a result: the leg itself records pass/fail).
# Legs: nas_smb_readonly (SYNOLOGY_SMB_USER, SYNOLOGY_SMB_PASSWORD, SYNOLOGY_IP_3, SYNOLOGY_IP_4 in the gitignored env file), minio (no obtainable image), nfs_owner_host (ODG-08: the owner NFS host).
# Credential VALUES are never read into the record or printed: only whether each named variable has a non-empty value.
# Usage:  blocked_external.sh [--out FILE]      Env: TI_ENV_FILE (default <repo>/.env)
# Exit:   0 the record was written; 2 usage.
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="${TI_ROOT:-$(cd "$HERE/../.." && pwd)}"
ENVF="${TI_ENV_FILE:-$ROOT/.env}"; OUT=""
while [ $# -gt 0 ]; do case "$1" in --out) OUT=${2:-}; shift 2;; *) echo "blocked_external: unknown argument '$1'" >&2; exit 2;; esac; done
command -v jq >/dev/null 2>&1 || { echo "blocked_external: jq is required" >&2; exit 2; }
envval() { [ -r "$ENVF" ] && sed -n "s/^$1=//p" "$ENVF" | head -1 | sed -e 's/^"\(.*\)"$/\1/' -e "s/^'\(.*\)'$/\1/"; }
missing=()
for v in SYNOLOGY_SMB_USER SYNOLOGY_SMB_PASSWORD SYNOLOGY_IP_3 SYNOLOGY_IP_4; do
  if [ -n "$(envval "$v")" ]; then :; else missing+=("$v"); fi
done
if [ "${#missing[@]}" -eq 0 ]; then nas='{"leg":"nas_smb_readonly","state":"available","note":"credentials present in the gitignored env file (names only checked); the leg itself records its own result"}'
else nas="$(printf '%s\n' "${missing[@]}" | jq -R . | jq -sc '{leg:"nas_smb_readonly", state:"blocked-unavailable", reason:"credentials_absent", variables:., blocked_on:"ODG-01"}')"; fi
legs="$(jq -nc --argjson nas "$nas" '[$nas,
  {leg:"minio", state:"blocked-unavailable", reason:"image_unavailable", blocked_on:"owner choice of an S3 image", evidence:"wp12/minio-blocked.txt"},
  {leg:"nfs_owner_host", state:"blocked-unavailable", reason:"odg08_unanswered", blocked_on:"ODG-08", note:"the owner NFS host that ODG-08 names; NFS is closed from this host on all seven Synology hosts"}]')"
rec="$(jq -n --argjson legs "$legs" --arg at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" '{schema:"blocked-external/1", task:"T135", run_at:$at, legs:$legs}')"
if [ -n "$OUT" ]; then printf '%s\n' "$rec" >"$OUT"; else printf '%s\n' "$rec"; fi
