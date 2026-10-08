#!/usr/bin/env bash
# test_blocked_external.sh - T135, reworked by WF17 round 5 (TI-G6, TI-E3, TI-D11). Oracle for scripts/test-infra/blocked_external.sh: a round trip that needs an external provider credential or device records
# `blocked-unavailable` with `credentials_absent` and the variable NAMES (BLOCKED-ON ODG-01), never a pass. SPECIFIED by T135: env file with every variable -> `available`; env file absent -> blocked-unavailable /
# credentials_absent listing ALL names; env file lacking one variable -> blocked-unavailable listing exactly that name; a credential VALUE never appears in the record (sentinel check after EVERY run of the
# battery, not after one); the record never says `pass`; the nfs_owner_host leg is DERIVED from the T134a record and cites the REAL input path and its sha256 for `blocked` AND `not_needed`; a fallback record
# that is not a valid nfs-fallback/1 record (a bare {"state":"not_needed"}, a foreign schema, a reason outside the closed set such as `<script>`) never makes a leg `not_needed` and its free text is never copied.
# Paired mutations: available without reading the variables; the T134a record hard-coded; a value written into the record; the `available` note carrying a value (WF17 B1); the blocked leg dropping
# derived_from (WF17 BE3); the bare not_needed accepted; the free-text reason copied; identity (BE0, must SURVIVE). Usage: test_blocked_external.sh  (BLK_NO_MUTATIONS=1: tests only)
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
SUT_REL="${BLK_SUT:-scripts/test-infra/blocked_external.sh}"
if [ ! -f "$TI_REPO/$SUT_REL" ]; then bad "script absent: $SUT_REL"; ti_summary; exit 1; fi
export TI_ROOT="$TI_REPO"
FX="$TI_REPO/.audit/scratch/blk-fx.$$"; mkdir -p "$FX"; trap 'rm -rf -- "${FX:?}"; ti_cleanup' EXIT
SV="SENTINELVALUE$RANDOM$RANDOM"
printf 'SYNOLOGY_SMB_USER=%s\nSYNOLOGY_SMB_PASSWORD=%s\nSYNOLOGY_IP_3=192.0.2.3\nSYNOLOGY_IP_4=192.0.2.4\n' "$SV" "$SV" >"$FX/full.env"
printf 'SYNOLOGY_SMB_USER=%s\nSYNOLOGY_IP_3=192.0.2.3\nSYNOLOGY_IP_4=192.0.2.4\n' "$SV" >"$FX/nopw.env"
SHA64=$(printf 'x' | sha256sum | cut -d' ' -f1)
echo '{"schema":"nfs-fallback/1","state":"blocked","reason":"odg08_unanswered","unconfirmed":"the application kernel NFS mount path"}' >"$FX/fb-blocked.json"
echo '{"schema":"nfs-fallback/1","state":"not_needed","fallback_run":false,"attempt_record":{"file":"wp10/nfs-attempt.json","state":"pass","sha256":"'"$SHA64"'"}}' >"$FX/fb-notneeded.json"
echo '{"state":"not_needed"}' >"$FX/fb-bare-notneeded.json"
echo '{"schema":"something-else/9","state":"not_needed","fallback_run":false,"attempt_record":{"state":"pass","sha256":"'"$SHA64"'"}}' >"$FX/fb-foreign-schema.json"
echo '{"schema":"nfs-fallback/1","state":"blocked","reason":"<script>alert(1)</script>"}' >"$FX/fb-free-text-reason.json"
echo 'not json' >"$FX/fb-garbage.json"
LEAK=0
run() { local sut=$1 envf=$2; TI_ENV_FILE="$envf" TI_NFS_FALLBACK="${FB:-$FX/fb-blocked.json}" bash "$TI_REPO/$sut" --out "$FX/rec.json" >"$FX/out.txt" 2>"$FX/err.txt"; RC=$?
  # the sentinel is looked for after EVERY run (a leak in one leg of the battery must not hide behind the last record)
  if grep -qF "$SV" "$FX/rec.json" "$FX/out.txt" "$FX/err.txt" 2>/dev/null; then LEAK=$((LEAK+1)); fi; }
leg() { jq -r --arg l "$1" ".legs[] | select(.leg == \$l) | $2" "$FX/rec.json" 2>/dev/null; }
rel() { realpath -m --relative-to="$TI_REPO" -- "$1"; }
battery() { local sut=$1 n=0; LEAK=0
  run "$sut" "$FX/full.env"
  if [ "$RC" = 0 ] && [ "$(leg nas_smb_readonly .state)" = available ]; then :; else echo "FAIL a complete env file did not give available (rc=$RC state=$(leg nas_smb_readonly .state))"; n=$((n+1)); fi
  run "$sut" "$FX/absent.env"
  if [ "$(leg nas_smb_readonly .state)" = blocked-unavailable ] && [ "$(leg nas_smb_readonly .reason)" = credentials_absent ] && [ "$(leg nas_smb_readonly '.variables | sort | join(",")')" = "SYNOLOGY_IP_3,SYNOLOGY_IP_4,SYNOLOGY_SMB_PASSWORD,SYNOLOGY_SMB_USER" ] && [ "$(leg nas_smb_readonly .blocked_on)" = ODG-01 ]; then :; else echo "FAIL an absent env file was not blocked-unavailable/credentials_absent listing all four names"; n=$((n+1)); fi
  run "$sut" "$FX/nopw.env"
  if [ "$(leg nas_smb_readonly .state)" = blocked-unavailable ] && [ "$(leg nas_smb_readonly '.variables | join(",")')" = SYNOLOGY_SMB_PASSWORD ]; then :; else echo "FAIL a missing password was not reported by name only (vars=$(leg nas_smb_readonly '.variables | join(",")'))"; n=$((n+1)); fi
  if jq -e 'any(.legs[]; .state == "pass")' "$FX/rec.json" >/dev/null 2>&1; then echo "FAIL a record says pass"; n=$((n+1)); fi
  if [ "$(leg minio .state)" = blocked-unavailable ] && [ "$(leg minio .reason)" = image_unavailable ] && [ "$(leg nfs_owner_host .state)" = blocked-unavailable ] && [ "$(leg nfs_owner_host .blocked_on)" = ODG-08 ]; then :; else echo "FAIL the minio / nfs_owner_host legs are not recorded blocked-unavailable"; n=$((n+1)); fi
  # WF12 F7 + WF17 BE3: the nfs_owner_host leg is DERIVED from the T134a record in EVERY state, citing the real input path and its sha256
  FB="$FX/fb-notneeded.json"; run "$sut" "$FX/full.env"; unset FB
  if [ "$(leg nfs_owner_host .state)" = not_needed ] && [ "$(leg nfs_owner_host .derived_from.file)" = "$(rel "$FX/fb-notneeded.json")" ] && [ "$(leg nfs_owner_host .derived_from.sha256)" = "$(sha256sum "$FX/fb-notneeded.json" | cut -d' ' -f1)" ]; then :; else echo "FAIL a valid not_needed T134a record was not reflected with derived_from = its REAL path and sha256 (state=$(leg nfs_owner_host .state) file=$(leg nfs_owner_host .derived_from.file))"; n=$((n+1)); fi
  run "$sut" "$FX/full.env"
  if [ "$(leg nfs_owner_host .state)" = blocked-unavailable ] && [ "$(leg nfs_owner_host .reason)" = odg08_unanswered ] && [ "$(leg nfs_owner_host .unconfirmed)" = "the application kernel NFS mount path" ] && [ "$(leg nfs_owner_host .derived_from.file)" = "$(rel "$FX/fb-blocked.json")" ] && [ "$(leg nfs_owner_host .derived_from.sha256)" = "$(sha256sum "$FX/fb-blocked.json" | cut -d' ' -f1)" ]; then :; else echo "FAIL a blocked T134a record was not reflected with derived_from = its REAL path and sha256 (BE3: file=$(leg nfs_owner_host .derived_from.file) sha=$(leg nfs_owner_host .derived_from.sha256 | cut -c1-8))"; n=$((n+1)); fi
  # E3: records that are not valid nfs-fallback/1 records leave the leg blocked nfs_fallback_record_missing; their free text is never copied
  for f in fb-garbage fb-bare-notneeded fb-foreign-schema fb-free-text-reason absent-fb; do
    FB="$FX/$f.json"; run "$sut" "$FX/full.env"; unset FB
    if [ "$(leg nfs_owner_host .state)" = blocked-unavailable ] && [ "$(leg nfs_owner_host .reason)" = nfs_fallback_record_missing ]; then :; else echo "FAIL $f was accepted for the nfs_owner_host leg (state=$(leg nfs_owner_host .state) reason=$(leg nfs_owner_host .reason))"; n=$((n+1)); fi
  done
  if grep -q '<script>' "$FX/rec.json"; then echo "FAIL the free-text reason '<script>' was copied into the record"; n=$((n+1)); fi
  # the committed records agree: the leg of the committed blocked-external.json is the one the committed T134a record derives
  local real="$TI_REPO/specs/001-full-project-audit-remediation/evidence/wp12/blocked-external.json" fbj="$TI_REPO/specs/001-full-project-audit-remediation/evidence/wp10/nfs-fallback.json"
  case "$(jq -r .state "$fbj" 2>/dev/null)" in blocked) want=blocked-unavailable;; not_needed) want=not_needed;; *) want="unreadable:$fbj";; esac
  [ "$(jq -r '.legs[] | select(.leg=="nfs_owner_host") | .state' "$real" 2>/dev/null)" = "$want" ] || { echo "FAIL the committed blocked-external.json and nfs-fallback.json disagree about nfs_owner_host (want $want)"; n=$((n+1)); }
  [ "$LEAK" -eq 0 ] || { echo "FAIL a credential value appeared in a record or the output in $LEAK run(s) of the battery"; n=$((n+1)); }
  return "$n"; }
res=$(battery "$SUT_REL"); n=$?
[ "$n" -eq 0 ] && ok "all fixtures hold (available / credentials_absent with names / one missing name / no value leaked in ANY run / never pass / minio and nfs host blocked / derived_from real path+sha / invalid records refused)" || bad "$n fixture(s) violated: $(printf '%s' "$res" | tr '\n' ';' | cut -c1-400)"
if [ "${BLK_NO_MUTATIONS:-0}" != 1 ]; then
  MUTLOG="${BLK_MUTATION_RECORD:-$TI_SCRATCH/mutations.txt}"; : >"$MUTLOG"
  mut() { local name=$1 want=$2 d="$TI_REPO/.audit/scratch/blk-mut-$1" r k; shift 2   # mut <name> <caught|survive> <old> <new> [<old2> <new2>...]
    rm -rf -- "${d:?}"; mkdir -p "$d"; cp "$TI_REPO/scripts/test-infra/dotenv_get.py" "$d/"
    python3 -I - "$TI_REPO/$SUT_REL" "$d/blocked_external.sh" "$@" <<'PY' || { bad "mutation $name: anchor missing"; return; }
import sys
s = open(sys.argv[1]).read(); a = sys.argv[3:]
for i in range(0, len(a), 2):
    if s.count(a[i]) != 1: print("anchor count %d for %r" % (s.count(a[i]), a[i])); sys.exit(1)
    s = s.replace(a[i], a[i + 1])
open(sys.argv[2], "w").write(s)
PY
    r=$(battery ".audit/scratch/blk-mut-$name/blocked_external.sh"); k=$?
    if [ "$want" = survive ]; then [ "$k" -eq 0 ] && { ok "identity mutant $name SURVIVED (as required)"; echo "$name SURVIVED-AS-REQUIRED" >>"$MUTLOG"; } || { bad "identity mutant $name FAILED the battery ($(printf '%s' "$r" | head -1 | cut -c1-100))"; echo "$name FAILED-BUT-IDENTITY" >>"$MUTLOG"; }
    elif [ "$k" -gt 0 ]; then ok "mutation $name CAUGHT ($(printf '%s' "$r" | head -1 | cut -c1-100))"; echo "$name CAUGHT" >>"$MUTLOG"; else bad "mutation $name SURVIVED"; echo "$name SURVIVED" >>"$MUTLOG"; fi
    rm -rf -- "${d:?}"; }
  mut always_available caught '  if [ -z "$val" ]; then missing+=("$v")' '  if false; then missing+=("$v")'
  mut nfs_leg_hard_coded caught 'case "$FBSTATE" in' 'case "forced" in'
  mut value_leaked caught 'missing+=("$v")' 'missing+=("$v:$(envval SYNOLOGY_SMB_USER)")'
  # WF17-BE-B1 (reviewer B1): the user value written into the `available` note
  mut be_b1_value_in_available_note caught "nas='{\"leg\":\"nas_smb_readonly\",\"state\":\"available\",\"note\":\"credentials present in the gitignored env file (names only checked); the leg itself records its own result\"}'" "nas='{\"leg\":\"nas_smb_readonly\",\"state\":\"available\",\"note\":\"credentials present for '\"\$(envval SYNOLOGY_SMB_USER)\"'\"}'"
  # BE3 (proof lens): the blocked leg drops derived_from
  mut be3_blocked_leg_drops_derived_from caught 'unconfirmed:(.unconfirmed // "the application kernel NFS mount path"), derived_from:{file:$f, sha256:$sha}, note:' 'unconfirmed:(.unconfirmed // "the application kernel NFS mount path"), note:'
  mut bare_not_needed_accepted caught '[ "$FBOK" = ok ] || FBSTATE=""' 'FBSTATE="$(jq -r ".state // empty" "$FB" 2>/dev/null)"'
  mut free_text_reason_copied caught 'reason:"odg08_unanswered", blocked_on:"ODG-08", unconfirmed' 'reason:(.reason // "odg08_unanswered"), blocked_on:"ODG-08", unconfirmed' 'then (if (.reason // "odg08_unanswered") == "odg08_unanswered" then "ok" else "bad_reason" end)' 'then "ok"'
  mut be0_identity survive 'command -v jq >/dev/null 2>&1 || { echo "blocked_external: jq is required" >&2; exit 2; }' 'command -v jq >/dev/null 2>&1 || { echo "blocked_external: jq is required" >&2; exit 2; }; :'
  [ -z "${BLK_EV:-}" ] || cp "$MUTLOG" "$BLK_EV/blocked-external-mutations.txt"
fi
ti_summary
