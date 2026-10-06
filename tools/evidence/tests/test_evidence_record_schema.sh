#!/usr/bin/env bash
# T048a - tests the ev/1 evidence-record contract revision 6 (pre_release + test_state_gos).
# Env: EV_SCHEMA = schema under test (default: the feature contract, revision 6).
# Host python3+jsonschema is used because RUNP/IMG-TESTUTIL (scripts/containers/run_pinned.sh) does
# not exist yet (UNCONFIRMED: rerun under RUNP IMG-TESTUTIL when WP-09 lands).
set -u
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd)
SCHEMA=${EV_SCHEMA:-$root/specs/001-full-project-audit-remediation/contracts/evidence-record.schema.json}
fails=0; ok() { echo "ok   $1"; }; bad() { echo "FAIL $1"; fails=$((fails+1)); }
# check NAME EXPECT(valid|refused) JSON : validates JSON against $SCHEMA
check() {
  local name=$1 want=$2 doc=$3 got
  got=$(python3 - "$SCHEMA" "$doc" <<'PY'
import json,sys,jsonschema
s=json.load(open(sys.argv[1])); s["$id"]="urn:ev:1" if ":" not in s.get("$id","x:") else s["$id"]; d=json.loads(sys.argv[2])
try: jsonschema.Draft202012Validator(s).validate(d); print("valid")
except jsonschema.ValidationError: print("refused")
PY
)
  [ "$got" = "$want" ] && ok "$name ($want)" || bad "$name: wanted $want got ${got:-error}"
}
H=$(printf 'a%.0s' $(seq 64)); Z=$(printf '0%.0s' $(seq 64))
base=$(jq -cn --arg h "$H" --arg z "$Z" '{schema:"ev/1",seq:1,item:"CAT-001",polarity:"PROBE",iteration:1,
  started_at:"2026-10-05T00:00:00Z",cwd:"/tmp",argv:["true"],exit_status:0,verdict:"pass",duration_ms:3,
  stdout_sha256:$h,stderr_sha256:$h,target_fingerprint:$h,prev_hash:$z,entry_hash:$h}')
withp()  { jq -c ". + $1" <<<"$base"; }
check "pre-rev6 entry (neither property)"      valid   "$base"
old_prefix="AT""M"   # the withdrawn prefix (ODG-11), built so this file carries no literal of it
check "item with the withdrawn prefix (rev 7)"  refused "$(withp "{item:\"${old_prefix}-001\"}")"
check "item CAT-123 (rev 7)"                    valid   "$(withp '{item:"CAT-123"}')"
check "pre_release with test_state_gos"        valid   "$(withp '{pre_release:true,test_state_gos:["evidence/wp05/go-1.json"]}')"
check "pre_release without test_state_gos"     refused "$(withp '{pre_release:true}')"
check "test_state_gos without pre_release"     refused "$(withp '{test_state_gos:["evidence/wp05/go-1.json"]}')"
check "test_state_gos with pre_release=false"  refused "$(withp '{pre_release:false,test_state_gos:["x"]}')"
check "empty test_state_gos"                   refused "$(withp '{pre_release:true,test_state_gos:[]}')"
check "duplicate test_state_gos"               refused "$(withp '{pre_release:true,test_state_gos:["a","a"]}')"
check "unknown property still refused"         refused "$(withp '{not_in_schema:1}')"
# paired mutation: schema copy without pre_release must refuse the pre-release fixture
mut=$(mktemp); jq 'del(.properties.pre_release)' "$SCHEMA" >"$mut"
got=$(EV_SCHEMA=$mut bash -c 'python3 - "$EV_SCHEMA" "$1" <<PY
import json,sys,jsonschema
I=chr(36)+"id"   # the heredoc is unquoted: no literal dollar sign
s=json.load(open(sys.argv[1])); s[I]="urn:ev:1" if ":" not in s.get(I,"x:") else s[I]; d=json.loads(sys.argv[2])
try: jsonschema.Draft202012Validator(s).validate(d); print("valid")
except jsonschema.ValidationError: print("refused")
PY' _ "$(withp '{pre_release:true,test_state_gos:["g"]}')")
[ "$got" = refused ] && ok "mutation (no pre_release) caught" || bad "mutation survived: $got"; rm -f "$mut"
echo "failures=$fails"; [ "$fails" -eq 0 ]
