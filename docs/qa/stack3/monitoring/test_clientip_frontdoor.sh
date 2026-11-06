#!/bin/bash
# CAT-FRONTDOOR-RATELIMIT-1 stack-level check, run against a LIVE stack 3 (does not restart anything).
# Usage: test_clientip_frontdoor.sh http://192.168.1.115:33080 [SECOND_HOST_SSH]
# Property A (single host, always run): a client-forged X-Forwarded-For must NOT yield a fresh rate-limit bucket (spoof-resistance).
# Property B (needs a second LAN host reachable by ssh, else SKIP honestly): a client on ANOTHER host keeps its own bucket, i.e. is not
#   locked out by a flood from this host. Host-local clients all share pasta's alias (169.254.1.2) by design; that is not property B.
# WARNING: the flood (35 bad logins) locks every client that shares this host's rate-limit bucket (host-local clients,
#   pasta alias 169.254.1.2) out of login for about a minute. Do not run it while someone is using the stack from this host.
# Waits 62 s first so the 30/min login window is empty. Exit 0 PASS / 1 FAIL.
set -u
T="${1:?front door URL}"; OTHER="${2:-}"
post(){ curl -s -o /dev/null -m 10 -w '%{http_code}' -X POST -H 'Content-Type: application/json' "$@" -d '{"username":"qa_nonexistent_fd","password":"x"}' "$T/api/v1/auth/login"; }
sleep 62
# Pre-flood control: the instrument must see an un-throttled 401 (a wrong URL / dead API would also look "429-free"),
# otherwise the later 429s prove nothing.
ctl=$(post)
if [ "$ctl" != 401 ]; then echo "CONTROL FAIL: pre-flood login returned $ctl (expected 401); instrument blind or stack unhealthy"; echo FAIL; exit 1; fi
for _ in $(seq 1 35); do post >/dev/null; done
no_xff=$(post); spoof=$(post -H 'X-Forwarded-For: 9.9.9.9'); chain=$(post -H 'X-Forwarded-For: 8.8.8.8, 9.9.9.9')
echo "A: after flood no-XFF=$no_xff spoof=$spoof chain-spoof=$chain (all must be 429)"
rc=0; [ "$no_xff$spoof$chain" = 429429429 ] || rc=1
if [ -n "$OTHER" ]; then
  other=$(ssh -o BatchMode=yes -o ConnectTimeout=5 "$OTHER" "curl -s -o /dev/null -m 10 -w '%{http_code}' -X POST -H 'Content-Type: application/json' -d '{\"username\":\"qa_nonexistent_fd\",\"password\":\"x\"}' $(printf '%q' "$T/api/v1/auth/login")")
  echo "B: second host $OTHER got $other (must be 401, not 429)"; [ "$other" = 401 ] || rc=1
else echo "B: SKIP second_host_not_supplied (UNCONFIRMED: remote-LAN bucket isolation)"; fi
if [ $rc != 0 ]; then echo FAIL; exit 1; fi
if [ -n "$OTHER" ]; then echo "PASS (A and B)"; else echo "PASS (A only; B SKIP - remote-LAN isolation NOT verified)"; fi
exit 0
