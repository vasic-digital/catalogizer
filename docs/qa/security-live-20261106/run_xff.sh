#!/bin/bash
# Part 4: X-Forwarded-For spoof probe from the SAME host as the configured trusted proxy (TRUSTED_PROXIES=192.168.1.115/32).
# Unauthenticated /api/v1/auth/login is limited to 30 rpm per ClientIP(). We flood under a spoofed XFF value and watch
# which bucket fills: XFF honoured -> only the spoofed bucket 429s; XFF ignored -> the real-peer bucket 429s as well.
T=$1; XFA=$2; XFB=$3
post(){ curl -s -o /dev/null -m 10 -w '%{http_code}' -X POST -H 'Content-Type: application/json' "$@" -d '{"username":"qa_nonexistent_xff","password":"x"}' $T/api/v1/auth/login; }
sleep 62
first=0; n429=0
for i in $(seq 1 35); do c=$(post -H "X-Forwarded-For: $XFA"); if [ "$c" = 429 ]; then n429=$((n429+1)); [ $first = 0 ] && first=$i; fi; done
echo "target=$T flood XFF=$XFA first429_at=$first n429=$n429/35"
echo "after flood: no-XFF=$(post)  XFF-other=$(post -H "X-Forwarded-For: $XFB")  XFF-chain-spoof=$(post -H "X-Forwarded-For: $XFB, $XFA")"
