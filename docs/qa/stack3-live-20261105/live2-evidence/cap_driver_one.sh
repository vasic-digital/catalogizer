#!/bin/bash
# one.sh KIND IDX  (token in file)
kind=$1; i=$2; B=http://127.0.0.1:38082; U=$(cat /home/milosvasic/.cache/claude-tmp/stack3live2/.usertok)
rules='[{"field":"year","operator":"greater_than","value":1900}]'
if [ $kind = pl ]; then p=/api/v1/playlists; d="{\"name\":\"qa-cap-pl-$i\",\"is_smart\":true,\"smart_rules\":$rules}"; else p=/api/v1/collections; d="{\"name\":\"qa-cap-co-$i\",\"is_smart\":true,\"smart_rules\":$rules}"; fi
out=$(curl -s -m 60 -o /tmp/qa_one_$kind_$i.b -w '%{http_code}' -X POST $B$p -H "Authorization: Bearer $U" -H 'Content-Type: application/json' -d "$d")
echo "$kind $i $out $(date -u +%T.%N)"
