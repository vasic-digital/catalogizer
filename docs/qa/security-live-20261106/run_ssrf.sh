#!/bin/bash
# Part 3: SSRF through real product endpoints (image-proxy public GET; POST /storage/roots; POST /sync/endpoints).
A=http://127.0.0.1:38082; LAN=169.254.1.2; PORT=${SSRF_PORT:-47931}
HL=$(mktemp); python3 "$(dirname "$0")/listener.py" 0.0.0.0 "$HL" $PORT & LP=$!
BF=$(mktemp); trap 'kill $LP 2>/dev/null; rm -f "$HL" "$BF"' EXIT; sleep 1
set -a; . /home/milosvasic/.cache/claude-tmp/catqa3/qa.env; set +a
ADM=$(python3 -c 'import json,os;print(json.dumps({"username":os.environ["ADMIN_USERNAME"],"password":os.environ["ADMIN_PASSWORD"]}))' | curl -s -X POST -H 'Content-Type: application/json' -d @- $A/api/v1/auth/login | python3 -c 'import sys,json;print(json.load(sys.stdin)["session_token"])')
norm(){ python3 -c 'import sys,json
t=sys.stdin.read()
try:
  d=json.loads(t[:t.rindex(" ")]); m=" | ".join(str(d[k]) for k in ("error","details","message") if k in d)
except Exception: m=t[:t.rindex(" ")][:120]
print(m[:150],t.rsplit(" ",1)[1].strip())'; }
authc(){ printf 'header = "Authorization: Bearer %s"\n' "$ADM" | curl -s -K - -m 20 "$@"; }
echo "control: listener reachable from API container netns: $(podman exec catqa3-api wget -q -T 5 -O - http://$LAN:$PORT/ctl 2>&1 | head -c 20) ; hits=$(grep -c 'HIT GET /ctl' $HL)"
: > $HL
echo "== image-proxy (public GET, expect 403 domain not allowed) =="
urls=("http://$LAN:$PORT/x" "https://$LAN:$PORT/x" "https://127.0.0.1/x" "https://localhost/x" "https://169.254.169.254/latest/meta-data/" "https://[::1]/" "https://10.0.0.5/" "https://image.tmdb.org@127.0.0.1/" "https://image.tmdb.org.127.0.0.1.nip.io/" "https://127.0.0.1.nip.io/" "https://169.254.169.254.nip.io/" "https://$LAN.nip.io/" "http://image.tmdb.org/t/p/w500/x.jpg" "https://image.tmdb.org:8443/x" "https://[::ffff:127.0.0.1]/" "https://0177.0.0.1/" "https://2130706433/" "file:///etc/passwd")
for u in "${urls[@]}"; do enc=$(python3 -c 'import sys,urllib.parse;print(urllib.parse.quote(sys.argv[1],safe=""))' "$u"); r=$(curl -s -m 15 -w ' %{http_code}' "$A/api/v1/image-proxy?url=$enc" | tr -d '\n' | tail -c 80); echo "image-proxy $u -> $r"; done
echo "control image-proxy allow-listed host (expect 200) -> $(curl -s -m 20 -o /dev/null -w '%{http_code}' "$A/api/v1/image-proxy?url=$(python3 -c 'import urllib.parse;print(urllib.parse.quote("https://image.tmdb.org/t/p/w500/none.jpg",safe=""))')")"
echo "control image-proxy missing url (expect 400) -> $(curl -s -m 10 -o /dev/null -w '%{http_code}' $A/api/v1/image-proxy)"
echo "== POST /storage/roots webdav (expect 400 url refused) =="
for u in "http://127.0.0.1:$PORT/dav" "http://localhost/dav" "http://169.254.169.254/latest/meta-data/" "http://[::1]/" "http://[::ffff:127.0.0.1]/" "http://0177.0.0.1/" "http://2130706433/" "http://127.1/" "http://127.0.0.1.nip.io/dav" "http://169.254.169.254.nip.io/dav" "http://localtest.me/dav" "http://100.100.100.200/" "http://[fd00:ec2::254]/" "http://0.0.0.0/" "ftp://example.invalid/"; do
  body=$(python3 -c 'import json,sys;print(json.dumps({"name":"ssrf-probe","protocol":"webdav","url":sys.argv[1]}))' "$u")
  r=$(echo "$body" > $BF; authc -X POST -H 'Content-Type: application/json' -d @$BF -w ' %{http_code}' $A/api/v1/storage/roots | norm); echo "root $u -> $r"
done
echo "control storage-root missing url -> $(echo '{"name":"c","protocol":"webdav"}' > $BF; authc -X POST -H 'Content-Type: application/json' -d @$BF -w ' %{http_code}' $A/api/v1/storage/roots | norm)"
echo "control storage-root non-admin n/a; unauth -> $(curl -s -m 10 -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' -d '{}' $A/api/v1/storage/roots)"
echo "roots count after probes: $(authc $A/api/v1/storage/roots | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["roots"]))')"
echo "== POST /sync/endpoints (expect 400) =="
for u in "http://127.0.0.1:$PORT/dav" "http://$LAN:$PORT/dav" "http://$LAN:$PORT/redir" "http://host.containers.internal:$PORT/dav" "http://169.254.169.254/latest/meta-data/" "http://[::1]/" "http://10.0.0.5/" "http://127.0.0.1.nip.io/" "http://169.254.1.2.nip.io:$PORT/" "http://169.254.169.254.nip.io/" "ftp://example.invalid/"; do
  body=$(python3 -c 'import json,sys;print(json.dumps({"name":"ssrf","type":"webdav","url":sys.argv[1],"sync_direction":"upload","local_path":"/app/data/secqa_sync","remote_path":"/"}))' "$u")
  r=$(echo "$body" > $BF; authc -X POST -H 'Content-Type: application/json' -d @$BF -w ' %{http_code}' $A/api/v1/sync/endpoints | norm); echo "sync $u -> $r"
done
# positive control: a public URL passes the URL check and is created, then removed (own data only)
body=$(python3 -c 'import json;print(json.dumps({"name":"secqa-ctl","type":"webdav","url":"https://example.com/dav","sync_direction":"upload","local_path":"/app/data/secqa_sync","remote_path":"/"}))'); echo "$body" > $BF
CR=$(authc -X POST -H 'Content-Type: application/json' -d @$BF -w ' %{http_code}' $A/api/v1/sync/endpoints); echo "control sync public url -> $(echo "$CR" | norm)"
CID=$(echo "$CR" | python3 -c 'import sys,json;t=sys.stdin.read();d=json.loads(t[:t.rindex(" ")]);x=d.get("data",d);x=x.get("endpoint",x) if isinstance(x,dict) else {};print(x.get("id",""))' 2>/dev/null)
[ -n "$CID" ] && echo "control cleanup delete own endpoint -> $(authc -o /dev/null -w '%{http_code}' -X DELETE $A/api/v1/sync/endpoints/$CID)"
echo "control sync endpoints list (expect 200) -> $(authc -o /dev/null -w '%{http_code}' $A/api/v1/sync/endpoints)"
sleep 1
echo "LISTENER HITS from product paths (expect 0): $(grep -c HIT $HL)"
