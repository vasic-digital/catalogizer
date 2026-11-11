#!/bin/bash
# Part 2: pprof + metrics authz. Tokens/passwords live only in shell variables, passed to curl via stdin config.
A=http://127.0.0.1:38082
set -a; . /home/milosvasic/.cache/claude-tmp/catqa3/qa.env; set +a
jget(){ python3 -c "import sys,json;print(json.load(sys.stdin)$1)"; }
hcode(){ local tok="$1" url="$2"; if [ -n "$tok" ]; then printf 'header = "Authorization: Bearer %s"\n' "$tok" | curl -s -K - -o /dev/null -m 90 -w '%{http_code}' "$url"; else curl -s -o /dev/null -m 90 -w '%{http_code}' "$url"; fi; }
login(){ python3 -c 'import json,sys;print(json.dumps({"username":sys.argv[1],"password":sys.argv[2]}))' "$1" "$2" | curl -s -m 10 -X POST -H 'Content-Type: application/json' -d @- $A/api/v1/auth/login; }
ADM=$(login "$ADMIN_USERNAME" "$ADMIN_PASSWORD" | jget "['session_token']")
echo "admin-login-token-len>0: $([ -n "$ADM" ] && echo yes || echo NO)"
echo "pprof anon /debug/pprof/ $(hcode '' $A/debug/pprof/)"
echo "pprof bad-token /debug/pprof/ $(hcode "bad.token.$RANDOM" $A/debug/pprof/)"
echo "pprof admin /debug/pprof/ $(hcode "$ADM" $A/debug/pprof/)"
echo "pprof admin /debug/pprof/heap $(hcode "$ADM" $A/debug/pprof/heap)"
# throwaway non-admin through registration + admin approval
U="qa_thrw_$$_$RANDOM"; PW="Qa-$(head -c 24 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 20)-9x!"
REG=$(python3 -c 'import json,sys;u=sys.argv[1];print(json.dumps({"username":u,"email":u+"@example.invalid","password":sys.argv[2],"first_name":"Qa","last_name":"Throwaway"}))' "$U" "$PW" | curl -s -m 10 -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' -d @- $A/api/v1/auth/register)
echo "register throwaway -> $REG"
pre=$(login "$U" "$PW" | python3 -c 'import sys,json
try:
  d=json.load(sys.stdin);print("token" if d.get("session_token") else "no-token:"+str(d.get("error",""))[:60])
except Exception as e: print("nonjson")')
echo "login before approval -> $pre"
# find id + approve (admin API, CSRF token from GET)
HDRS=$(mktemp); trap 'rm -f "$HDRS"' EXIT
LST=$(printf 'header = "Authorization: Bearer %s"\n' "$ADM" | curl -s -K - -D "$HDRS" -m 10 "$A/api/v1/admin/users?search=$U&limit=100")
CSRF=$(grep -i '^x-csrf-token:' "$HDRS" | awk '{print $2}' | tr -d '\r')
UID_=$(echo "$LST" | python3 -c 'import sys,json
d=json.load(sys.stdin)
us=d if isinstance(d,list) else (d.get("users") or d.get("data") or [])
us=us.get("users",[]) if isinstance(us,dict) else us
print(next((u["id"] for u in us if u.get("username")==sys.argv[1]),""))' "$U")
echo "admin found throwaway id: $([ -n "$UID_" ] && echo yes || echo NO) csrf-header: $([ -n "$CSRF" ] && echo yes || echo NO)"
if [ -n "$UID_" ]; then
  ap=$(printf 'header = "Authorization: Bearer %s"\nheader = "X-CSRF-Token: %s"\n' "$ADM" "$CSRF" | curl -s -K - -m 10 -o /dev/null -w '%{http_code}' -X PUT -H 'Content-Type: application/json' -b "$(grep -i '^set-cookie:' "$HDRS" | sed -E 's/^[^:]+: *([^;]*).*/\1/' | paste -sd';')" -d '{"status":"active"}' "$A/api/v1/admin/users/$UID_")
  echo "admin approve (PUT status=active) -> $ap"
fi
NA=$(login "$U" "$PW" | jget "['session_token']" 2>/dev/null)
echo "non-admin login after approval: $([ -n "$NA" ] && echo token || echo NO-TOKEN)"
if [ -n "$NA" ]; then
  echo "pprof non-admin /debug/pprof/ $(hcode "$NA" $A/debug/pprof/)"
  echo "pprof non-admin /debug/pprof/heap $(hcode "$NA" $A/debug/pprof/heap)"
  echo "control non-admin /api/v1/auth/me $(hcode "$NA" $A/api/v1/auth/me)"
  echo "control non-admin /api/v1/admin/users $(hcode "$NA" $A/api/v1/admin/users)"
fi
# sampling cap
for s in 120 61 0 abc 0.5; do echo "pprof admin /profile?seconds=$s $(hcode "$ADM" "$A/debug/pprof/profile?seconds=$s")"; done
echo "pprof admin /trace?seconds=120 $(hcode "$ADM" "$A/debug/pprof/trace?seconds=120")"
echo "pprof admin /heap?seconds=120 $(hcode "$ADM" "$A/debug/pprof/heap?seconds=120")"
echo "pprof admin /profile?seconds=2 (positive control) $(hcode "$ADM" "$A/debug/pprof/profile?seconds=2")"
# deactivate throwaway
if [ -n "$UID_" ]; then
  dc=$(printf 'header = "Authorization: Bearer %s"\nheader = "X-CSRF-Token: %s"\n' "$ADM" "$CSRF" | curl -s -K - -m 10 -o /dev/null -w '%{http_code}' -X PUT -H 'Content-Type: application/json' -b "$(grep -i '^set-cookie:' "$HDRS" | sed -E 's/^[^:]+: *([^;]*).*/\1/' | paste -sd';')" -d '{"status":"inactive"}' "$A/api/v1/admin/users/$UID_")
  echo "cleanup throwaway deactivate -> $dc"
fi
# metrics
echo "metrics anon $(hcode '' $A/metrics)"
echo "metrics bad-scrape-token $(hcode "wrong.$RANDOM" $A/metrics)"
echo "metrics scrape-token $(hcode "$METRICS_SCRAPE_TOKEN" $A/metrics)"
[ -n "$NA" ] && echo "metrics non-admin jwt $(hcode "$NA" $A/metrics)"
echo "metrics admin jwt $(hcode "$ADM" $A/metrics)"
