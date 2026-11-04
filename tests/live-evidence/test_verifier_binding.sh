#!/usr/bin/env bash
# Verifier-only binding cases (constitution 11.4.289(D), gate-logic UNIT test): how the cmdline script token is bound to answering.script_path, java's
# positional_script:false, and entry shapes the shipped table does not exercise (synthetic table). usage: test_verifier_binding.sh <verifier>; exit 0 = all pass
set -u
HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd); ROOT=$(cd "$HERE/../.." && pwd)
V=${1:?usage: test_verifier_binding.sh <verifier>}
export LIVE_EVIDENCE_DOUBLES=${LIVE_EVIDENCE_DOUBLES:-$ROOT/docs/qa/live-evidence.doubles.txt}
REAL=$ROOT/docs/qa/live-evidence.interpreter-options.json
W=$(mktemp -d "${TMPDIR:-/tmp}/lev-vb.XXXXXX"); trap 'rm -rf "${W:?}"' EXIT
bad=0; n=0
# run <table> <exe> <cmdline> <script_path> <want-class> <want-reasons>
run() {
  jq --arg e "$2" --arg c "$3" --arg s "$4" '.answering.exe_path = $e | .answering.cmdline = $c | .answering.script_path = $s' "$HERE/fixtures/golden_node_plain.json" > "$W/r.json"
  line=$(LIVE_EVIDENCE_INTERP_OPTIONS=$1 "$V" "$W/r.json" 2>&1 | head -1)
  cls=$(sed -n 's/.* class=\([A-Z]*\) .*/\1/p' <<<"$line"); rs=$(sed -n 's/.* reasons=//p' <<<"$line"); n=$((n+1))
  if [ "$cls" != "$5" ] || [ "$rs" != "$6" ]; then bad=$((bad+1)); echo "FAIL [$3] script_path=$4 want $5/$6 got $cls/$rs" >&2; fi
}
N=/usr/bin/node
# script token vs script_path binding
run "$REAL" $N "node server.js"             /srv/app/server.js        LIVE none
run "$REAL" $N "node ./server.js"           /srv/app/server.js        LIVE none
run "$REAL" $N "node /srv/app/server.js"    /srv/app/server.js        LIVE none
run "$REAL" $N "node /app/server.js"        /srv/app/server.js        UNIT script_not_in_cmdline
run "$REAL" $N "node /srv/app/server.js"    /other/srv/app/server.js  UNIT script_not_in_cmdline
run "$REAL" $N "node other.js"              /srv/app/server.js        UNIT script_not_in_cmdline
run "$REAL" $N "node ../app/server.js"      /srv/app/server.js        LIVE none
run "$REAL" $N "node ../app/other.js"       /srv/app/server.js        UNIT script_not_in_cmdline
run "$REAL" $N "node app/server.js"         /srv/app/server.js        LIVE none
run "$REAL" $N "node pp/server.js"          /srv/app/server.js        UNIT script_not_in_cmdline
run "$REAL" $N "node"                       /srv/app/server.js        UNIT script_not_in_cmdline
# java: no positional script (positional_script:false) -> a class name is never a bound script
run "$REAL" /usr/bin/java "java -Dserver.port=8080 Main" /srv/app/Main             UNIT script_not_in_cmdline
run "$REAL" /usr/bin/java "java -jar app.jar" /srv/app/app.jar        LIVE none
# synthetic table: a next-valued long option with a regex, and a next-valued SCRIPT option, with the --name=value form
T=$W/t.json
jq '.families.node.benign += [{"opt":"--nx","val":"next","re":"^[a-z]+$","ex":"abc"},{"opt":"--sx","val":"next","script":true,"ex":"x.js"}]' "$REAL" > "$T"
run "$T" $N "node --nx=abc server.js"  /srv/app/server.js LIVE none
run "$T" $N "node --nx=ABC server.js"  /srv/app/server.js UNIT interpreter_option_unknown
run "$T" $N "node --nx abc server.js"  /srv/app/server.js LIVE none
run "$T" $N "node --nx ABC server.js"  /srv/app/server.js UNIT interpreter_option_unknown
run "$T" $N "node --nx -abc server.js" /srv/app/server.js UNIT interpreter_option_unknown
run "$T" $N "node --sx server.js"      /srv/app/server.js LIVE none
run "$T" $N "node --sx=server.js"      /srv/app/server.js UNIT interpreter_option_unknown
echo "verifier binding cases: $n run, $bad failed"
[ "$bad" = 0 ]
