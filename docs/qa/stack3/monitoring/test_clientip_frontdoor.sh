#!/bin/bash
# CAT-FRONTDOOR-RATELIMIT-1 stack-level check, run against a LIVE stack 3 (does not restart anything).
# Usage: test_clientip_frontdoor.sh http://192.168.1.115:33080 [SECOND_HOST_SSH]
# Property A (single host, always run): a client-forged X-Forwarded-For must NOT yield a fresh rate-limit bucket (spoof-resistance).
# Property B (needs a second LAN host reachable by ssh, else SKIP honestly): a client on ANOTHER host keeps its own bucket, i.e. is not
#   locked out by a flood from this host. Host-local clients all share pasta's alias (169.254.1.2) by design; that is not property B.
# Waits 62 s first so the 30/min login window is empty. Exit 0 PASS / 1 FAIL.
set -u
T="${1:?front door URL}"; OTHER="${2:-}"
post(){ curl -s -o /dev/null -m 10 -w '%{http_code}' -X POST -H 'Content-Type: application/json' "$@" -d '{"username":"qa_nonexistent_fd","password":"x"}' "$T/api/v1/auth/login"; }
sleep 62
for i in $(seq 1 35); do post >/dev/null; done
no_xff=$(post); spoof=$(post -H 'X-Forwarded-For: 9.9.9.9'); chain=$(post -H 'X-Forwarded-For: 8.8.8.8, 9.9.9.9')
echo "A: after flood no-XFF=$no_xff spoof=$spoof chain-spoof=$chain (all must be 429)"
rc=0; [ "$no_xff$spoof$chain" = 429429429 ] || rc=1
if [ -n "$OTHER" ]; then
  other=$(ssh -o BatchMode=yes -o ConnectTimeout=5 "$OTHER" "curl -s -o /dev/null -m 10 -w '%{http_code}' -X POST -H 'Content-Type: application/json' -d '{\"username\":\"qa_nonexistent_fd\",\"password\":\"x\"}' $T/api/v1/auth/login")
  echo "B: second host $OTHER got $other (must be 401, not 429)"; [ "$other" = 401 ] || rc=1
else echo "B: SKIP second_host_not_supplied (UNCONFIRMED: remote-LAN bucket isolation)"; fi
[ $rc = 0 ] && echo PASS || echo FAIL; exit $rc
