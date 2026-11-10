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
REAL_OPENSSL="$(command -v openssl)"; export REAL_OPENSSL
cat > "$R/bin/openssl" <<'S'
#!/usr/bin/env bash
# shim: FAKE_OPENSSL=fail -> `req` fails (after writing a partial file); wrongsan -> cert for another IP; badkey -> key from a different pair; else real
if [ "$1" = req ]; then
  case "${FAKE_OPENSSL-}" in
    fail) for a in "$@"; do case "$prev" in -keyout|-out) echo partial > "$a";; esac; prev="$a"; done; exit 1;;
    wrongsan) args=(); for a in "$@"; do args+=("${a//IP:*/IP:192.0.2.77}"); done; exec "$REAL_OPENSSL" "${args[@]}";;
    badkey) "$REAL_OPENSSL" "$@" || exit 1; kf=""; prev=""; for a in "$@"; do [ "$prev" = -keyout ] && kf="$a"; prev="$a"; done
            "$REAL_OPENSSL" genrsa -out "$kf" 2048 >/dev/null 2>&1; exit 0;;
  esac
fi
exec "$REAL_OPENSSL" "$@"
S
REAL_DATE="$(command -v date)"; export REAL_DATE
cat > "$R/bin/date" <<'S'
#!/usr/bin/env bash
# shim: a seconds-only timestamp format is frozen (forces same-second collisions); formats with %N (sub-second) stay real
case "$*" in *%N*) exec "$REAL_DATE" "$@";; *) echo 20260101T000000;; esac
S
chmod +x "$R/bin/"*
printf 'TS_USER=u\nTS_PASS=p\nTS_HOST=192.0.2.200\n' > "$R/env"; chmod 600 "$R/env"
PLOG="$R/plog"; export PLOG
P=0; F=0; ok() { if [ "$1" = 0 ]; then P=$((P+1)); echo "PASS $2"; else F=$((F+1)); echo "FAIL $2"; fi; }
mkcert() { rm -f -- "${R:?}"/root/tls/* "${R:?}"/root/tls/.[!.]* 2>/dev/null; openssl req -x509 -newkey rsa:2048 -nodes -days 2 -subj /CN=anton.local -addext "subjectAltName=DNS:anton.local,IP:$1" -keyout "$R/root/tls/key.pem" -out "$R/root/tls/cert.pem" >/dev/null 2>&1; }
san() { openssl x509 -in "$R/root/tls/cert.pem" -noout -ext subjectAltName 2>/dev/null | tr '\n' ' '; }
up() { : > "$PLOG"; ( unset TS_HOST QA_LAN_IP QA_BIND LAN_BIND FAKE_OPENSSL; for kv in "$@"; do export "${kv?}"; done
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
# 5. S1: openssl failure / wrong SAN / mismatched key -> old pair untouched, non-zero, no backup churn, no container, no leftovers
for mode in fail wrongsan badkey; do
  mkcert 192.0.2.115; hc="$(sha256sum "$R/root/tls/cert.pem" "$R/root/tls/key.pem")"
  up FAKE_SRC=10.0.0.5 FAKE_ADDRS=10.0.0.5 FAKE_OPENSSL=$mode
  [ "$(cat "$R/rc")" != 0 ]; ok $? "S1[$mode]: non-zero exit"
  case $mode in fail) m="cert generation failed";; wrongsan) m="lacks SAN IP";; badkey) m="does not match";; esac
  grep -q "$m" "$R/out"; ok $? "S1[$mode]: failure names its own check ('$m'), not a downstream one"
  [ "$hc" = "$(sha256sum "$R/root/tls/cert.pem" "$R/root/tls/key.pem")" ]; ok $? "S1[$mode]: live cert+key byte-identical (never left without a cert)"
  [ "$(ls -A "$R/root/tls" | wc -l)" = 2 ]; ok $? "S1[$mode]: no temp or .bak leftovers in tls dir (got: $(ls -A "$R/root/tls" | tr '\n' ' '))"
  ! grep -q 'podman run' "$PLOG"; ok $? "S1[$mode]: no container (re)started"
done
# 6. S1: unique backup names on two regenerations within the same second
mkcert 192.0.2.115; up FAKE_SRC=10.0.0.5 FAKE_ADDRS=10.0.0.5; mkcert_keep() { openssl req -x509 -newkey rsa:2048 -nodes -days 2 -subj /CN=anton.local -addext "subjectAltName=DNS:anton.local,IP:192.0.2.116" -keyout "$R/root/tls/key.pem" -out "$R/root/tls/cert.pem" >/dev/null 2>&1; }; mkcert_keep; up FAKE_SRC=10.0.0.5 FAKE_ADDRS=10.0.0.5
[ "$(cat "$R/rc")" = 0 ]; ok $? "S1: second regeneration in the same (frozen-seconds) timestamp still succeeds"
[ "$(ls "$R/root/tls"/cert.pem.bak-* 2>/dev/null | wc -l)" -ge 2 ]; ok $? "S1: backups never overwrite each other (>=2 distinct cert backups)"
[ "$(stat -c %a "$R/root/tls/key.pem")" = 600 ]; ok $? "S1: key 600 after in-place swap"
# 7. S2: TS_HOST injection into the SAN extension is refused before anything is touched
mkcert 192.0.2.115; hc="$(sha256sum "$R/root/tls/cert.pem")"
for bad in '10.0.0.5,DNS:evil.example' '10.0.0.5
junk' '10.0.0.5/32' 'anton.local' '01.2.3.4'; do
  up FAKE_SRC=10.0.0.5 FAKE_ADDRS=10.0.0.5 "TS_HOST=$bad"
  grep -q 'not an IPv4' "$R/out"; ok $? "S2: refused by the IPv4 guard itself ('$(printf %s "$bad" | tr '\n' '~')')"
  [ "$(cat "$R/rc")" != 0 ] && [ "$hc" = "$(sha256sum "$R/root/tls/cert.pem")" ] && ! grep -q 'podman run' "$PLOG"; ok $? "S2: TS_HOST '$(printf %s "$bad" | tr '\n' '~')' refused, cert untouched, no container"
done
# 8. M3: verify.sh validates a caller TS_HOST too (refused before any evidence dir / network use)
mkdir -p "$R/ev"
for bad in '10.0.0.5,DNS:evil.example' 'anton.local'; do
  ( unset QA_LAN_IP; PATH="$R/bin:$PATH" TS_ROOT="$R/root" TS_ENV="$R/env" TS_EVID="$R/ev" TS_HOST="$bad" bash "$HERE/verify.sh" >"$R/vout" 2>&1 ); vrc=$?
  [ "$vrc" != 0 ] && grep -q 'not an IPv4' "$R/vout" && [ -z "$(ls -A "$R/ev")" ]; ok $? "M3: verify.sh refuses TS_HOST '$bad' before doing anything (rc=$vrc)"
done
echo "passed=$P failed=$F"; [ "$F" = 0 ]
