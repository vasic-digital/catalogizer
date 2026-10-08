#!/usr/bin/env bash
# blocked_external.sh - T135. Records the legs of the real-service stack that need an EXTERNAL provider credential or device, with their state: `available`, or `blocked-unavailable` with
# the reason (`credentials_absent` and the variable NAMES, BLOCKED-ON ODG-01; `image_unavailable`; `odg08_unanswered`), never `pass` (availability is not a result: the leg itself records pass/fail).
# Legs: nas_smb_readonly (SYNOLOGY_SMB_USER, SYNOLOGY_SMB_PASSWORD, SYNOLOGY_IP_3, SYNOLOGY_IP_4 in the gitignored env file), minio (no obtainable image), nfs_owner_host (ODG-08: the owner NFS host).
# Credential VALUES are never read into the record or printed: only whether each named variable has a non-empty value.
# The nfs_owner_host leg is DERIVED from the T134a record (evidence wp10/nfs-fallback.json, written by nfs_fallback_state.sh), never hard-coded (WF12 F7): `blocked` -> blocked-unavailable with
# the record's reason; `not_needed` -> not_needed; an absent or unreadable record -> blocked-unavailable `nfs_fallback_record_missing`. Two records can no longer disagree about the same leg.
# Usage:  blocked_external.sh [--out FILE]   (--out needs a non-empty value; a failed write exits 1)      Env: TI_ENV_FILE (default <repo>/.env), TI_NFS_FALLBACK (default <repo>/specs/001-full-project-audit-remediation/evidence/wp10/nfs-fallback.json)
# Exit:   0 the record was written; 1 the record could not be written; 2 usage.
# The env file is read by scripts/test-infra/dotenv_get.py (CRLF, `export`, spaces, quotes, inline comments, last duplicate wins); an IP variable that is not an IPv4 address is `credentials_invalid` (the leg refuses it too).
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export LC_ALL=C
ROOT="${TI_ROOT:-$(cd "$HERE/../.." && pwd)}"
ENVF="${TI_ENV_FILE:-$ROOT/.env}"; OUT=""
while [ $# -gt 0 ]; do
  case "$1" in
    --out) if [ $# -lt 2 ] || [ -z "${2:-}" ]; then echo "blocked_external: --out needs a non-empty value" >&2; exit 2; fi; OUT=$2; shift 2;;
    *) echo "blocked_external: unknown argument '$1'" >&2; exit 2;;
  esac
done
command -v jq >/dev/null 2>&1 || { echo "blocked_external: jq is required" >&2; exit 2; }
command -v python3 >/dev/null 2>&1 || { echo "blocked_external: python3 is required (dotenv reader)" >&2; exit 2; }
# ONE dotenv reader for every script (CRLF, `export`, spaces around =, quotes, inline comments, last duplicate wins): scripts/test-infra/dotenv_get.py
envval() { [ -r "$ENVF" ] && python3 -I "$HERE/dotenv_get.py" "$ENVF" "$1" 2>/dev/null; }
missing=(); invalid=()
for v in SYNOLOGY_SMB_USER SYNOLOGY_SMB_PASSWORD SYNOLOGY_IP_3 SYNOLOGY_IP_4; do
  val="$(envval "$v")"
  if [ -z "$val" ]; then missing+=("$v")
  else case "$v" in SYNOLOGY_IP_*) [[ "$val" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}$ ]] || invalid+=("$v");; esac; fi   # the leg's client refuses a non-IPv4 host: availability must agree with it
done
if [ "${#missing[@]}" -eq 0 ] && [ "${#invalid[@]}" -eq 0 ]; then nas='{"leg":"nas_smb_readonly","state":"available","note":"credentials present in the gitignored env file (names only checked); the leg itself records its own result"}'
elif [ "${#missing[@]}" -eq 0 ]; then nas="$(printf '%s\n' "${invalid[@]}" | jq -R . | jq -sc '{leg:"nas_smb_readonly", state:"blocked-unavailable", reason:"credentials_invalid", variables:., blocked_on:"ODG-01"}')"
else nas="$(printf '%s\n' "${missing[@]}" | jq -R . | jq -sc '{leg:"nas_smb_readonly", state:"blocked-unavailable", reason:"credentials_absent", variables:., blocked_on:"ODG-01"}')"; fi
FB="${TI_NFS_FALLBACK:-$ROOT/specs/001-full-project-audit-remediation/evidence/wp10/nfs-fallback.json}"
FBREL="$(realpath -m --relative-to="$ROOT" -- "$FB" 2>/dev/null || echo "$FB")"   # the REAL input path, whatever was read (WF17 TI-E3)
FBSTATE="$(jq -r '.state // empty' "$FB" 2>/dev/null)"
# the record must be what nfs_fallback_state.sh writes: schema nfs-fallback/1; `not_needed` additionally needs the attempt record it was derived from (a pass, with a sha256) and fallback_run false;
# a bare {"state":"not_needed"} is not enough, and the free-text reason is only copied from a closed set
FBOK="$(jq -r 'if .schema != "nfs-fallback/1" then "bad_schema"
  elif .state == "blocked" then (if (.reason // "odg08_unanswered") == "odg08_unanswered" then "ok" else "bad_reason" end)
  elif .state == "not_needed" then (if (.attempt_record | type == "object") and .attempt_record.state == "pass" and ((.attempt_record.sha256 // "") | test("^[0-9a-f]{64}$")) and .fallback_run == false then "ok" else "not_needed_underevidenced" end)
  else "bad_state" end' "$FB" 2>/dev/null)"
[ "$FBOK" = ok ] || FBSTATE=""
case "$FBSTATE" in
  blocked) nfs="$(jq -c --arg sha "$(sha256sum "$FB" | cut -d' ' -f1)" --arg f "$FBREL" '{leg:"nfs_owner_host", state:"blocked-unavailable", reason:"odg08_unanswered", blocked_on:"ODG-08", unconfirmed:(.unconfirmed // "the application kernel NFS mount path"), derived_from:{file:$f, sha256:$sha}, note:"the owner NFS host that ODG-08 names; NFS is closed from this host on all seven Synology hosts"}' "$FB")";;
  not_needed) nfs="$(jq -c --arg sha "$(sha256sum "$FB" | cut -d' ' -f1)" --arg f "$FBREL" '{leg:"nfs_owner_host", state:"not_needed", derived_from:{file:$f, sha256:$sha}}' "$FB")";;
  *) nfs='{"leg":"nfs_owner_host","state":"blocked-unavailable","reason":"nfs_fallback_record_missing","blocked_on":"ODG-08","note":"the T134a record is absent, unreadable or not a valid nfs-fallback/1 record: the leg is not claimed closed"}';;
esac
legs="$(jq -nc --argjson nas "$nas" --argjson nfs "$nfs" '[$nas,
  {leg:"minio", state:"blocked-unavailable", reason:"image_unavailable", blocked_on:"owner choice of an S3 image", evidence:"wp12/minio-blocked.txt"},
  $nfs]')"
rec="$(jq -n --argjson legs "$legs" --arg at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" '{schema:"blocked-external/1", task:"T135", run_at:$at, legs:$legs}')"
if [ -n "$OUT" ]; then printf '%s\n' "$rec" >"$OUT" || { echo "blocked_external: cannot write $OUT" >&2; exit 1; }; else printf '%s\n' "$rec"; fi
