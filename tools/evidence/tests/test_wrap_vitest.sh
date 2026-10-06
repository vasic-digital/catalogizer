#!/usr/bin/env bash
# T053 - failing-first parser test of tools/evidence/wrap-vitest.sh on RECORDED reports: fixtures/vt_*.json are real
# `vitest run --reporter=json --outputFile` reports of the seed fixtures/seeds/vitest (vitest 1.6.1, node 20.20.2, recorded in IMG-NODE,
# a scratch install of vitest: the project's own catalog-web toolchain is WP-11). vt_fail_truncated.json is a byte prefix of vt_fail.json.
# The live seeded-failure leg runs in IMG-NODE after WP-11: recorded blocked-unavailable below until the project toolchain exists.
set -u
. "$(dirname "${BASH_SOURCE[0]}")/wraplib.sh"; wsetup wrap-vitest.sh
wcheck "recorded pass: 1 test"                                      0 --from-report "$FX/vt_pass.json" :: 'runner=vitest' 'status=pass' 'tests=1' 'passed=1' 'failed=0'
wcheck "recorded seeded failure is reported through the parser"     1 --from-report "$FX/vt_fail.json" :: 'status=fail' 'tests=4' 'failed=1' 'failed .*seeded failure' 'skipped=1'
wcheck "hidden failure behind a TRUNCATED report is still found"    1 --from-report "$FX/vt_fail_truncated.json" :: 'status=fail' 'failed .*seeded failure' 'truncated'
wcheck "no test files found (stderr text, exit 1) is an error, not a failure" 126 --from-report "$FX/vt_none.err" --rc 1 :: 'status=error' 'reason=(no_tests|unparsable)'
python3 - "$FX/vt_pass.json" "$S/lie.json" <<'P'
import json,sys
d=json.load(open(sys.argv[1])); d["numFailedTests"]=0; d["success"]=True
d["testResults"][0]["assertionResults"][0]["status"]="failed"; d["testResults"][0]["assertionResults"][0]["failureMessages"]=["boom"]
json.dump(d,open(sys.argv[2],"w"))
P
wcheck "a report whose header says success but whose assertion failed: the assertion wins" 1 --from-report "$S/lie.json" :: 'status=fail' 'failed=1'
python3 - "$FX/vt_pass.json" "$S/hdr.json" <<'P'
import json,sys
d=json.load(open(sys.argv[1])); d["numFailedTests"]=2; d["success"]=False
json.dump(d,open(sys.argv[2],"w"))
P
wcheck "a header that counts failures the assertions do not show fails (hidden failure)" 1 --from-report "$S/hdr.json" :: 'status=fail' 'reason=header_failures'
python3 - "$FX/vt_pass.json" "$S/zero.json" <<'P'
import json,sys
d=json.load(open(sys.argv[1])); d["numTotalTests"]=0; d["testResults"]=[]; d["numPassedTests"]=0
json.dump(d,open(sys.argv[2],"w"))
P
wcheck "a report with zero tests is an error"                       126 --from-report "$S/zero.json" :: 'status=error' 'reason=no_tests'
: >"$S/e.json"; wcheck "an empty report is an error"               126 --from-report "$S/e.json" :: 'status=error' 'reason=empty_report'
wrefuse_r "no --run-token is refused (run_token_missing)" run_token_missing --from-report "$FX/vt_pass.json"
wrefuse_r "a malformed --run-token is refused (run_token_malformed)" run_token_malformed --run-token zz --from-report "$FX/vt_pass.json"
o=$("$W" --run-token "$TOK" --from-report "$FX/vt_pass.json" 2>/dev/null); printf '%s\n' "$o" | grep -q "$TOK" && bad "the wrapper printed the run token" || ok "the wrapper does not print the run token"
echo "blocked-unavailable: live seeded-failure leg (IMG-NODE with the catalog-web vitest toolchain exists after WP-11; the parser legs above are recorded real reports)"
wdone
