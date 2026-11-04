#!/usr/bin/env bash
# webdav_live.sh - runs the LIVE WebDAV test (catalog-api/filesystem/live_webdav_test.go, build tag live) against a real WebDAV server and prints a
# machine-readable verdict. Constitution section 11.4.289: real server only; an absent service is an honest SKIP, never a stand-in.
# Environment (names only): LIVE_WEBDAV_URL, LIVE_WEBDAV_USER, LIVE_WEBDAV_PASSWORD, LIVE_WEBDAV_SANDBOX (share/folder), optional LIVE_WEBDAV_INSECURE=1.
# Output is redacted (user, password, host) before printing. Only the two named live tests are counted (the gate-logic unit tests are excluded).
# Exit: 0 VERDICT=PASS (both live tests PASSed, nothing skipped); 1 FAIL; 2 usage/env; 3 SKIPPED (no live test passed or one skipped = PARTIAL is also 3).
set -u -o pipefail
for v in LIVE_WEBDAV_URL LIVE_WEBDAV_USER LIVE_WEBDAV_PASSWORD LIVE_WEBDAV_SANDBOX; do
  [ -n "${!v:-}" ] || { echo "VERDICT=SKIPPED reason=credentials_absent missing=$v"; exit 3; }
done
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
host="${LIVE_WEBDAV_URL#*://}"; host="${host%%/*}"; host="${host%%:*}"; export LIVE_WEBDAV_HOSTNAME="$host"
# Redaction reads the secrets from the environment (awk ENVIRON): a credential on a command line (sed -e s/<password>/) is visible in /proc/<pid>/cmdline.
redact() { awk 'BEGIN{n=split(ENVIRON["LIVE_WEBDAV_PASSWORD"] "\n" ENVIRON["LIVE_WEBDAV_USER"] "\n" ENVIRON["LIVE_WEBDAV_HOSTNAME"],s,"\n")} {for(i=1;i<=n;i++) if(length(s[i])>0){while((j=index($0,s[i]))>0) $0=substr($0,1,j-1) "<redacted>" substr($0,j+length(s[i]))} print}'; }
out=$(cd "$ROOT/catalog-api" && go test -tags live -count=1 -v -run '^(TestLiveWebDAV_RoundTripAndFailurePaths|TestLiveWebDAV_BadCredentialRejected)$' ./filesystem/ 2>&1); rc=$?
printf '%s\n' "$out" | redact
ps=$(printf '%s\n' "$out" | grep -cE -- '^--- PASS: TestLiveWebDAV_(RoundTripAndFailurePaths|BadCredentialRejected) ')
sk=$(printf '%s\n' "$out" | grep -cE -- '^--- SKIP: TestLiveWebDAV_(RoundTripAndFailurePaths|BadCredentialRejected) ')
echo "GO_RC=$rc PASSED=$ps SKIPPED=$sk"
if [ "$rc" -ne 0 ]; then echo VERDICT=FAIL; exit 1
elif [ "$ps" -eq 2 ] && [ "$sk" -eq 0 ]; then echo VERDICT=PASS; exit 0
elif [ "$ps" -eq 0 ]; then echo "VERDICT=SKIPPED (no live leg ran)"; exit 3
else echo "VERDICT=PARTIAL (not all live legs ran)"; exit 3; fi
