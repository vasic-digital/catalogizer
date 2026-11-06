#!/bin/bash
# Part 1: Prometheus auth, Grafana datasource health, DB/Redis LAN exposure. Secrets read in-process, never printed.
IP=192.168.1.115; P=http://$IP:39090; G=http://$IP:33030
set -a; . /home/milosvasic/.cache/claude-tmp/catqa3/mon.env; set +a
code(){ curl -s -o /dev/null -m 10 -w '%{http_code}' "$@"; }
# authenticated calls: credentials via curl config on stdin (not argv)
acode(){ local u="$1" pw="$2"; shift 2; printf 'user = "%s:%s"\n' "$u" "$pw" | curl -s -K - -o /dev/null -m 10 -w '%{http_code}' "$@"; }
for path in /api/v1/targets '/api/v1/query?query=up' /-/ready; do
  echo "anon $path $(code "$P$path")"
done
echo "wrong-password qa_prom /api/v1/targets $(acode qa_prom "wrong-$RANDOM" $P/api/v1/targets)"
echo "wrong-user nobody /api/v1/targets $(acode nobody "$PROM_AUTH_PASSWORD" $P/api/v1/targets)"
echo "empty-password qa_prom /api/v1/targets $(acode qa_prom "" $P/api/v1/targets)"
for path in /api/v1/targets '/api/v1/query?query=up' /-/ready; do
  echo "right-creds $path $(acode qa_prom "$PROM_AUTH_PASSWORD" "$P$path")"
done
# targets all up? (body shape only)
printf 'user = "qa_prom:%s"\n' "$PROM_AUTH_PASSWORD" | curl -s -K - -m 10 $P/api/v1/targets | python3 -c 'import sys,json;d=json.load(sys.stdin);print("targets",sorted((t["labels"].get("job"),t["health"]) for t in d["data"]["activeTargets"]))'
# Grafana datasource health
L=$(printf 'user = "%s:%s"\n' "$GF_SECURITY_ADMIN_USER" "$GF_SECURITY_ADMIN_PASSWORD" | curl -s -K - -m 10 $G/api/datasources)
echo "$L" | python3 -c 'import sys,json;[print("datasource",d["name"],d["type"],d["uid"]) for d in json.load(sys.stdin)]'
for uid in $(echo "$L" | python3 -c 'import sys,json;[print(d["uid"]) for d in json.load(sys.stdin)]'); do
  printf 'user = "%s:%s"\n' "$GF_SECURITY_ADMIN_USER" "$GF_SECURITY_ADMIN_PASSWORD" | curl -s -K - -m 20 $G/api/datasources/uid/$uid/health | python3 -c 'import sys,json;d=json.load(sys.stdin);print("ds-health",d.get("status"),d.get("message"))'
done
echo "grafana-anon-datasources $(code $G/api/datasources)"
# LAN exposure
for pt in 35434 36381; do
  r=$(timeout 3 bash -c "</dev/tcp/$IP/$pt" 2>&1 && echo OPEN || echo CLOSED); echo "lan $IP:$pt $(echo "$r"|tail -1|grep -o 'OPEN\|refused\|timed out\|CLOSED'|head -1)"
done
for pt in 35434 36381; do (timeout 3 bash -c "</dev/tcp/127.0.0.1/$pt" && echo "control-loopback 127.0.0.1:$pt OPEN") 2>&1|tail -1; done
