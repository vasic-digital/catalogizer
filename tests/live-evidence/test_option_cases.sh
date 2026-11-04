#!/usr/bin/env bash
# Frozen option cases (option_cases.json) driven through verify_live_evidence.sh as complete records (constitution 11.4.289(D)): gate-logic UNIT test.
# usage: test_option_cases.sh <verifier> [jq-select-on-case, default "true"]   env: LIVE_EVIDENCE_INTERP_OPTIONS (table under test), LIVE_EVIDENCE_DOUBLES
# Also: COVERAGE=1 checks that every member of the REAL table is pinned by at least one case (adding a member without a case fails).
set -u
HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT=$(cd "$HERE/../.." && pwd)
V=${1:?usage: test_option_cases.sh <verifier> [filter]}; FILTER=${2:-true}
export LIVE_EVIDENCE_DOUBLES=${LIVE_EVIDENCE_DOUBLES:-$ROOT/docs/qa/live-evidence.doubles.txt}
CASES=$HERE/option_cases.json
W=$(mktemp -d "${TMPDIR:-/tmp}/lev-oc.XXXXXX"); trap 'rm -rf "${W:?}"' EXIT
EX=1111111111111111111111111111111111111111111111111111111111111111; SC=2222222222222222222222222222222222222222222222222222222222222222
if [ "${COVERAGE:-0}" = 1 ]; then
  T=$ROOT/docs/qa/live-evidence.interpreter-options.json
  miss=$(jq -r --slurpfile c "$CASES" '[.families | to_entries[] | .key as $f | (.value.benign[] | "\($f):benign:\(.opt)"), (.value.dangerous[] | "\($f):dangerous:\(.)")] as $m
    | ($c[0] | map(.member)) as $have | $m[] | . as $n | select(($have | any(. == $n or startswith($n + ":"))) | not)' "$T")
  [ -z "$miss" ] || { echo "FAIL: table members without a frozen case:" >&2; echo "$miss" >&2; exit 1; }
  echo "coverage: every table member is pinned by a frozen case"
fi
# ONE jq call emits, per case, a record line and an expectation line ("class<TAB>reasons<TAB>member<TAB>args"); FAILFAST=1 stops at the first mismatch (mutant runs)
EXPECT='def exp($c): if ($c.member | endswith("control-char-token")) then ["INVALID","cmdline_malformed"]
  elif $c.want == "ok" then (if ($c.script == null or ($c.script | startswith("/") | not)) then ["UNIT","script_not_in_cmdline"] else ["LIVE","none"] end)
  elif $c.want == "codeload" then ["UNIT","interpreter_code_loading_option"]
  elif $c.want == "unknown" then ["UNIT","interpreter_option_unknown"]
  else ["UNIT","interpreter_family_unsupported"] end;'
n=0; bad=0
while IFS= read -r rec && IFS= read -r exp; do
  printf '%s\n' "$rec" > "$W/r.json"
  line=$("$V" "$W/r.json" 2>&1 | head -1); cls=$(sed -n 's/.* class=\([A-Z]*\) .*/\1/p' <<<"$line"); rs=$(sed -n 's/.* reasons=//p' <<<"$line")
  IFS=$'\t' read -r ec er member args <<<"$exp"
  n=$((n+1))
  if [ "$cls" != "$ec" ] || [ "$rs" != "$er" ]; then bad=$((bad+1)); echo "FAIL $member: $args got class=$cls reasons=$rs want class=$ec reasons=$er" >&2; [ "${FAILFAST:-0}" = 1 ] && break; fi
done < <(jq -r --arg ex "$EX" --arg sc "$SC" "$EXPECT"' [ .[] | select('"$FILTER"') ][] as $c
  | ($c.exe | split("/") | last) as $b
  | (if $c.script == null then "/srv/app/real" elif ($c.script | startswith("/")) then $c.script else "/srv/app/" + $c.script end) as $sp
  | ({schema:"catalogizer.live-evidence/1",test_id:"oc",class:"LIVE",verdict:"PASS",timestamp:"2026-11-04T10:00:00Z",command:"x",
       artifact:{fingerprint:("sha256:"+$sc),source:"target"},endpoint:{host:"127.0.0.1",port:8080,resolved_ip:"127.0.0.1"},
       answering:({kind:"process",pid:7,exe_path:$c.exe,exe_sha256:$ex,cmdline:($c.args | join(" ")),script_path:$sp,script_sha256:$sc,env_checked:true}
                  + (if ($b | test("^(python[0-9.]*|pypy[0-9.]*|node|nodejs|deno|bun|java|ruby[0-9.]*|perl[0-9.]*|php[0-9.]*|lua[0-9.]*|bash|sh|dash|zsh|ksh|fish)$")) then {} else {runtime_hint:"libpython3.12"} end)),
       expected:{exe_sha256:$ex,script_sha256:$sc},requests:{total:3},responses:{},runs:{n:3,identical:true,signature:"sha256:s"}} | tojson),
    (exp($c) as $e | [$e[0], $e[1], $c.member, ($c.args | tojson)] | join("\t"))' "$CASES")
echo "option cases via verifier: $n run, $bad failed"
[ "$n" -gt 0 ] && [ "$bad" = 0 ]
