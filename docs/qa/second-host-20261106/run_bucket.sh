#!/bin/bash
# ANNOUNCE: floods QA login (stack 3) with 35 bad logins per direction -> ~1 min lockout of the flooded source bucket.
# Source A = default (192.168.1.115), source B = 192.168.1.250 (secondary /32 on enp5s0). Not a physically distinct host (see README).
T=http://192.168.1.115:33080/api/v1/auth/login
post(){ local i=$1; shift; curl -s -o /dev/null -m 10 ${i:+--interface $i} -w '%{http_code}' -X POST -H 'Content-Type: application/json' "$@" -d '{"username":"qa_nonexistent_fd","password":"x"}' $T; }
dir(){ # flooder probe
  sleep 62
  ctl=$(post "$2"); echo "ctl probe($2) pre-flood=$ctl (must be 401)"
  for _ in $(seq 35); do post "$1" >/dev/null; done
  fl=$(post "$1"); pr=$(post "$2"); echo "flooder($1)=$fl (must be 429) other($2)=$pr (must be 401)"
  [ "$ctl" = 401 ] && [ "$fl" = 429 ] && [ "$pr" = 401 ]
}
echo "A-flood -> B-probe"; dir "" 192.168.1.250; r1=$?
echo "B-flood -> A-probe"; dir 192.168.1.250 ""; r2=$?
[ $r1 = 0 ] && [ $r2 = 0 ] && echo RESULT_PASS || echo RESULT_FAIL
