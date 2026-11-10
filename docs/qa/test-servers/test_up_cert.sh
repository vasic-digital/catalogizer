#!/usr/bin/env bash
# Sandbox test (fake podman + fake ip, real openssl): up.sh takes TS_HOST from qa_lan_ip and (re)generates the FTPS cert when its SAN lacks the current IP.
# usage: test_up_cert.sh [UP_SH]   (UP_SH override lets a mutation run target a mutated copy living next to ports.env/lib; exit 0 = all pass)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; UP="${1:-$HERE/up.sh}"
R="$(mktemp -d)" || exit 1; [ -n "$R" ] && [ -d "$R" ] || exit 1; trap 'rm -rf -- "${R:?}"' EXIT
mkdir -p "$R/bin" "$R/root/corpus" "$R/root/tls"
cat > "$R/bin/podman" <<'S'
#!/usr/bin/env bash
echo "podman $*" >> "${PLOG:?}"; [ "$1 $2" = "image exists" ] && exit 0; exit 0
S
cat > "$R/bin/ip" <<'S'
#!/usr/bin/env bash
case "$*" in *"route get"*) [ -n "${FAKE_SRC-}" ] || exit 1; echo "1.1.1.1 via 10.0.0.1 dev eth0 src ${FAKE_SRC} uid 1000";;
  *"addr show"*) for a in ${FAKE_ADDRS-}; do echo "2: eth0    inet $a/24 scope global eth0"; done;; *) exit 1;; esac
S
chmod +x "$R/bin/"*
printf 'TS_USER=u\nTS_PASS=p\nTS_HOST=192.0.2.200\n' > "$R/env"; chmod 600 "$R/env"
PLOG="$R/plog"; export PLOG
P=0; F=0; ok() { if [ "$1" = 0 ]; then P=$((P+1)); echo "PASS $2"; else F=$((F+1)); echo "FAIL $2"; fi; }
mkcert() { openssl req -x509 -newkey rsa:2048 -nodes -days 2 -subj /CN=anton.local -addext "subjectAltName=DNS:anton.local,IP:$1" -keyout "$R/root/tls/key.pem" -out "$R/root/tls/cert.pem" >/dev/null 2>&1; }
san() { openssl x509 -in "$R/root/tls/cert.pem" -noout -ext subjectAltName 2>/dev/null | tr '\n' ' '; }
up() { : > "$PLOG"; ( unset TS_HOST QA_LAN_IP QA_BIND LAN_BIND; for kv in "$@"; do export "${kv?}"; done
    PATH="$R/bin:$PATH" TS_ROOT="$R/root" TS_ENV="$R/env" bash "$UP" >"$R/out" 2>&1 ); echo $? > "$R/rc"; }
# 1. stale cert (old IP) + route-derived current IP -> regenerated, SAN has the new IP, old pair kept
mkcert 192.0.2.115; up FAKE_SRC=10.0.0.5 FAKE_ADDRS=10.0.0.5
[ "$(cat "$R/rc")" = 0 ]; ok $? "up.sh rc 0 with route-derived address"
san | grep -q 'IP Address:10.0.0.5'; ok $? "stale cert regenerated: SAN has current IP"
san | grep -q 'DNS:anton.local'; ok $? "regenerated SAN keeps DNS:anton.local"
ls "$R/root/tls"/cert.pem.bak-* >/dev/null 2>&1 && ls "$R/root/tls"/key.pem.bak-* >/dev/null 2>&1; ok $? "old cert and key kept as .bak"
[ "$(stat -c %a "$R/root/tls/key.pem")" = 600 ]; ok $? "new key mode 600"
grep -q 'TS_HOST=10.0.0.5' "$PLOG"; ok $? "containers get TS_HOST=<detected> (stale env-file value 192.0.2.200 not used)"
grep -q 'TS_HOST=192.0.2.200' "$PLOG"; [ $? != 0 ]; ok $? "stale env-file TS_HOST never passed"
# 2. cert already has the IP -> untouched
h1="$(sha256sum "$R/root/tls/cert.pem")"; n1="$(ls "$R/root/tls" | wc -l)"; up FAKE_SRC=10.0.0.5 FAKE_ADDRS=10.0.0.5
[ "$h1" = "$(sha256sum "$R/root/tls/cert.pem")" ] && [ "$n1" = "$(ls "$R/root/tls" | wc -l)" ]; ok $? "cert with current IP left untouched (idempotent)"
# 3. a caller TS_HOST wins over detection
up FAKE_SRC=10.0.0.5 FAKE_ADDRS=10.0.0.5 TS_HOST=10.0.0.9
grep -q 'TS_HOST=10.0.0.9' "$PLOG" && san | grep -q 'IP Address:10.0.0.9'; ok $? "caller TS_HOST wins and the cert follows it"
# 4. address cannot be determined -> fail loudly, cert untouched
mkcert 192.0.2.115; h2="$(sha256sum "$R/root/tls/cert.pem")"; up FAKE_ADDRS=10.0.0.5
[ "$(cat "$R/rc")" != 0 ]; ok $? "undeterminable address -> non-zero"
[ "$h2" = "$(sha256sum "$R/root/tls/cert.pem")" ]; ok $? "undeterminable address -> cert untouched"
grep -q 'qa_lan_ip' "$R/out"; ok $? "undeterminable address -> message names qa_lan_ip"
[ ! -s "$PLOG" ] || ! grep -q 'podman run' "$PLOG"; ok $? "undeterminable address -> no container started"
echo "passed=$P failed=$F"; [ "$F" = 0 ]
