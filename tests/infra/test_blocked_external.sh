#!/usr/bin/env bash
# test_blocked_external.sh - T135 (RED first). Oracle for scripts/test-infra/blocked_external.sh: a round trip that needs an external provider credential or device records `blocked-unavailable`
# with `credentials_absent` and the variable NAMES (BLOCKED-ON ODG-01), never a pass. SPECIFIED by T135: env file with every variable -> `available`; env file absent -> blocked-unavailable /
# credentials_absent listing ALL names; env file lacking one variable -> blocked-unavailable listing exactly that name; a credential VALUE never appears in the record (sentinel check); the
# record never says `pass`. Paired mutations: a copy that reports available without reading the variables; a copy that writes the values. Usage: test_blocked_external.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
SUT_REL="${BLK_SUT:-scripts/test-infra/blocked_external.sh}"
if [ ! -f "$TI_REPO/$SUT_REL" ]; then bad "script absent: $SUT_REL"; ti_summary; exit 1; fi
FX="$TI_REPO/.audit/scratch/blk-fx.$$"; mkdir -p "$FX"; trap 'rm -rf -- "${FX:?}"; ti_cleanup' EXIT
SV="SENTINELVALUE$RANDOM$RANDOM"
printf 'SYNOLOGY_SMB_USER=%s\nSYNOLOGY_SMB_PASSWORD=%s\nSYNOLOGY_IP_3=%s\nSYNOLOGY_IP_4=%s\n' "$SV" "$SV" "$SV" "$SV" >"$FX/full.env"
printf 'SYNOLOGY_SMB_USER=%s\nSYNOLOGY_IP_3=%s\nSYNOLOGY_IP_4=%s\n' "$SV" "$SV" "$SV" >"$FX/nopw.env"
run() { local sut=$1 envf=$2; TI_ENV_FILE="$envf" bash "$TI_REPO/$sut" --out "$FX/rec.json" >"$FX/out.txt" 2>"$FX/err.txt"; RC=$?; }
leg() { jq -r --arg l "$1" ".legs[] | select(.leg == \$l) | $2" "$FX/rec.json" 2>/dev/null; }
battery() { local sut=$1 n=0
  run "$sut" "$FX/full.env"
  if [ "$RC" = 0 ] && [ "$(leg nas_smb_readonly .state)" = available ]; then :; else echo "FAIL a complete env file did not give available (rc=$RC state=$(leg nas_smb_readonly .state))"; n=$((n+1)); fi
  run "$sut" "$FX/absent.env"
  if [ "$(leg nas_smb_readonly .state)" = blocked-unavailable ] && [ "$(leg nas_smb_readonly .reason)" = credentials_absent ] && [ "$(leg nas_smb_readonly '.variables | sort | join(",")')" = "SYNOLOGY_IP_3,SYNOLOGY_IP_4,SYNOLOGY_SMB_PASSWORD,SYNOLOGY_SMB_USER" ] && [ "$(leg nas_smb_readonly .blocked_on)" = ODG-01 ]; then :; else echo "FAIL an absent env file did not give blocked-unavailable credentials_absent with all four names (state=$(leg nas_smb_readonly .state))"; n=$((n+1)); fi
  run "$sut" "$FX/nopw.env"
  if [ "$(leg nas_smb_readonly .state)" = blocked-unavailable ] && [ "$(leg nas_smb_readonly '.variables | join(",")')" = SYNOLOGY_SMB_PASSWORD ]; then :; else echo "FAIL a missing password was not reported by name only (vars=$(leg nas_smb_readonly '.variables | join(",")'))"; n=$((n+1)); fi
  if grep -qF "$SV" "$FX/rec.json" "$FX/out.txt" "$FX/err.txt"; then echo "FAIL a credential value appeared in the record or the output"; n=$((n+1)); fi
  if jq -e 'any(.legs[]; .state == "pass")' "$FX/rec.json" >/dev/null 2>&1; then echo "FAIL a record says pass"; n=$((n+1)); fi
  if [ "$(leg minio .state)" = blocked-unavailable ] && [ "$(leg minio .reason)" = image_unavailable ] && [ "$(leg nfs_owner_host .state)" = blocked-unavailable ] && [ "$(leg nfs_owner_host .blocked_on)" = ODG-08 ]; then :; else echo "FAIL the minio / nfs_owner_host legs are not recorded blocked-unavailable"; n=$((n+1)); fi
  return "$n"; }
res=$(battery "$SUT_REL"); n=$?
[ "$n" -eq 0 ] && ok "all fixtures hold (available / credentials_absent with names / one missing name / no value leaked / never pass / minio and nfs host blocked)" || bad "$n fixture(s) violated: $(printf '%s' "$res" | tr '\n' ';' | cut -c1-300)"
if [ "${BLK_NO_MUTATIONS:-0}" != 1 ]; then
  mut() { local name=$1 old=$2 new=$3 dst="$TI_REPO/.audit/scratch/blk-mut-$1.sh" r k
    python3 -I - "$TI_REPO/$SUT_REL" "$dst" "$old" "$new" <<'PY' || { bad "mutation $name: anchor missing"; return; }
import sys
s = open(sys.argv[1]).read()
if s.count(sys.argv[3]) != 1: print("anchor count %d for %r" % (s.count(sys.argv[3]), sys.argv[3])); sys.exit(1)
open(sys.argv[2], "w").write(s.replace(sys.argv[3], sys.argv[4]))
PY
    r=$(battery ".audit/scratch/blk-mut-$name.sh"); k=$?
    [ "$k" -gt 0 ] && ok "mutation $name CAUGHT ($(printf '%s' "$r" | head -1 | cut -c1-100))" || bad "mutation $name SURVIVED"
    rm -f -- "${dst:?}"; }
  mut always_available '  if [ -n "$(envval "$v")" ]; then' '  if true; then'
  mut value_leaked 'missing+=("$v")' 'missing+=("$v:$(envval SYNOLOGY_SMB_USER)")'
fi
ti_summary
