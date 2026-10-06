#!/usr/bin/env bash
# nas_readonly_leg.sh - T132. The clearly separated READ-ONLY leg against the owner's real Synology hosts (docs/infrastructure/synology-hosts.md; owner decision ODG-08/ODG-01).
# NOT part of the deterministic suite (the in-container Samba of IMG-INFRA-SMB is). It lists the root of one DATA share per host (depth 1) and reads at most ONE small file (<= 64 KiB),
# at most 2 requests per second, from IMG-INFRA-CLIENT through run_pinned.sh. It never writes, deletes, renames, creates or changes anything on a NAS.
# Credentials: ONLY from the gitignored env file (default <repo>/.env; variables SYNOLOGY_SMB_USER, SYNOLOGY_SMB_PASSWORD, SYNOLOGY_IP_<n>). The file is parsed, never sourced; the
# values are never printed, logged, put in argv or in the environment of any process: they go to an auth file (mode 0600, inside the run's out directory) that the client reads
# and this script deletes on every exit path. Entry NAMES and file CONTENT are never recorded: only counts, the sha256 of the sorted names, and the size and sha256 of the one small file.
# The leg SKIPs (exit 0, `SKIP nas_readonly reason=...`) when the env file is absent (env_absent) or lacks a variable (credentials_absent, variable NAMES listed), never fails for that.
# Usage:  nas_readonly_leg.sh [--hosts 3,4] [--json-out <file>]      Env: TI_ENV_FILE (default <repo>/.env)
# Exit:   0 PASS or SKIP; 1 a host leg failed; 2 usage.
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$HERE/lib.sh"
HOSTS="3,4"; JSON=""
while [ $# -gt 0 ]; do case "$1" in --hosts) HOSTS=${2:-}; shift 2;; --json-out) JSON=${2:-}; shift 2;; *) ti_die "unknown argument '$1'" 2;; esac; done
[[ "$HOSTS" =~ ^[1-7](,[1-7])*$ ]] || ti_die "--hosts must be a comma list of 1..7" 2
ENVF="${TI_ENV_FILE:-$TI_ROOT/.env}"
if [ ! -r "$ENVF" ]; then echo "SKIP nas_readonly reason=env_absent the gitignored env file is absent; the real-NAS leg is optional"; exit 0; fi
ti_need jq sha256sum
envval() { sed -n "s/^$1=//p" "$ENVF" | head -1 | sed -e 's/^"\(.*\)"$/\1/' -e "s/^'\(.*\)'$/\1/"; }
missing=""
[ -n "$(envval SYNOLOGY_SMB_USER)" ] || missing="$missing SYNOLOGY_SMB_USER"
[ -n "$(envval SYNOLOGY_SMB_PASSWORD)" ] || missing="$missing SYNOLOGY_SMB_PASSWORD"
IFS=, read -r -a IDX <<<"$HOSTS"
for i in "${IDX[@]}"; do [ -n "$(envval "SYNOLOGY_IP_$i")" ] || missing="$missing SYNOLOGY_IP_$i"; done
if [ -n "$missing" ]; then echo "SKIP nas_readonly reason=credentials_absent variables:$missing"; exit 0; fi
OUT="$TI_ROOT/.audit/out/nas-ro-$$"; mkdir -p "$OUT" && chmod 700 "$OUT" || ti_die "cannot create $OUT"
cleanup() { rm -f -- "${OUT:?}/auth"; }
trap cleanup EXIT
( umask 077; printf 'username = %s\npassword = %s\n' "$(envval SYNOLOGY_SMB_USER)" "$(envval SYNOLOGY_SMB_PASSWORD)" >"$OUT/auth" ) || ti_die "cannot write the auth file"
chmod 600 "$OUT/auth"
res="[]"; rc=0
for i in "${IDX[@]}"; do
  IP="$(envval "SYNOLOGY_IP_$i")"
  (cd "$TI_ROOT" && bash scripts/containers/run_pinned.sh --out "$OUT" --op-id "nas-ro-$$-$i" IMG-INFRA-CLIENT -- bash /src/scripts/test-infra/client/nas_smb_ro.sh "$IP" "$i") >"$OUT/leg-$i.log" 2>&1; lrc=$?
  if [ "$lrc" = 0 ] && [ -s "$OUT/nas-$i.tsv" ]; then
    one="$(jq -Rn --arg alias "Synology$i" --argjson ok true '[inputs | split("\t") | {(.[0]): .[1]}] | add + {alias:$alias, ok:$ok}' <"$OUT/nas-$i.tsv")"
  else rc=1; one="$(jq -n --arg alias "Synology$i" --argjson rc "$lrc" '{alias:$alias, ok:false, exit_code:$rc}')"; echo "FAIL nas_readonly host=Synology$i rc=$lrc"; fi
  res="$(jq -c --argjson o "$one" '. + [$o]' <<<"$res")"
done
doc="$(jq -n --argjson hosts "$res" --arg at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" --arg head "$(git -C "$TI_ROOT" rev-parse HEAD 2>/dev/null)" '{schema:"wp12-nas-readonly/1", task:"T132", run_at:$at, head:$head, protocol:"SMB3 read-only", requests_per_second_max:2, writes_performed:0, names_recorded:false, content_recorded:false, hosts:$hosts}')"
if [ -n "$JSON" ]; then printf '%s\n' "$doc" >"$JSON"; fi
echo "$doc" | jq -r '.hosts[] | "NAS " + .alias + " ok=" + (.ok|tostring) + " share=" + (.share // "-") + " entries=" + (.entries // "-") + " read_size=" + (.read_size // "-") + " writes=" + (.writes_performed // "-")'
exit "$rc"
