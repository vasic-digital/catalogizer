#!/usr/bin/env bash
# Paired mutations for test_up_cert.sh: each mutated COPY of up.sh must make the test FAIL. Exit 0 = all killed.
# Mutants live in a mktemp tree that mirrors test-servers/ + lib/ (up.sh locates ports.env and ../lib relative to itself); cleaned by trap.
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
W="$(mktemp -d)" || exit 1; [ -n "$W" ] && [ -d "$W" ] || exit 1; trap 'rm -rf -- "${W:?}"' EXIT
mkdir -p "$W/test-servers" "$W/lib"; cp -- "$HERE"/ports.env "$HERE"/test_up_cert.sh "$W/test-servers/"; cp -- "$HERE/../lib/lan_ip.sh" "$W/lib/"
cp -- "$HERE/up.sh" "$HERE/verify.sh" "$W/test-servers/"
S=0; K=0
mutv() { # name sed-expression : mutate verify.sh (the test script in the mutant tree finds it next to itself)
  local f="$W/test-servers/verify.sh"; sed -E "$2" "$HERE/verify.sh" > "$f"
  if cmp -s "$HERE/verify.sh" "$f"; then echo "NOT-APPLIED $1"; S=$((S+1)); else
    if bash "$W/test-servers/test_up_cert.sh" "$W/test-servers/up.sh" >/dev/null 2>&1; then echo "SURVIVED $1"; S=$((S+1)); else echo "KILLED   $1"; K=$((K+1)); fi; fi
  cp -- "$HERE/verify.sh" "$f"
}
mut() { # name sed-expression
  local f="$W/test-servers/up.sh"; cp -- "$HERE/up.sh" "$f"; sed -E "$2" "$HERE/up.sh" > "$f"
  if cmp -s "$HERE/up.sh" "$f"; then echo "NOT-APPLIED $1"; S=$((S+1)); return; fi
  if bash "$W/test-servers/test_up_cert.sh" "$f" >/dev/null 2>&1; then echo "SURVIVED $1"; S=$((S+1)); else echo "KILLED   $1"; K=$((K+1)); fi
}
mut cert-check-always-true 's/^if ! cert_has_ip; then/if false; then/'
mut env-file-host-wins 's/^if \[ -n "\$_ts_host_caller" \]; then TS_HOST="\$_ts_host_caller"; else TS_HOST="\$\(qa_lan_ip\)" \|\| exit 1; fi/TS_HOST="${TS_HOST:-}"/'
mut caller-host-ignored 's/^if \[ -n "\$_ts_host_caller" \]; then TS_HOST="\$_ts_host_caller"; else/if false; then :; else/'
mut detect-failure-ignored 's/TS_HOST="\$\(qa_lan_ip\)" \|\| exit 1/TS_HOST="$(qa_lan_ip || echo 10.0.0.5)"/'
mut containers-keep-stale-host 's/ -e "TS_HOST=\$TS_HOST"//'
mut san-without-ip 's/,IP:\$TS_HOST//'
mut key-mode-open 's/chmod 600 "\$tmpk"/chmod 644 "$tmpk"/'
mut ipv4-guard-removed 's/^qa_ip_is_ipv4 "\$TS_HOST" \|\| \{.*$/:/'
mut generated-san-not-verified 's/^  cert_has_ip "\$tmpc" \|\| cert_fail.*$/  :/'
mut key-cert-match-not-verified 's/ \&\& \[ "\$pk_c" = "\$pk_k" \] \|\| cert_fail/ || cert_fail/'
mut openssl-failure-ignored 's/-out "\$tmpc" >\/dev\/null 2>&1 \|\| cert_fail "cert generation failed"/-out "$tmpc" >\/dev\/null 2>\&1 || true/'
mut backup-name-second-resolution 's/stamp="\$\(date \+%Y%m%dT%H%M%S\.%N\)-\$\$"/stamp="$(date +%Y%m%dT%H%M%S)"/'
mut backup-not-kept 's/ln -- "\$ROOT\/tls\/\$f" "\$ROOT\/tls\/\$f.bak-\$stamp" \|\| cert_fail "cannot back up \$f"/rm -f -- "$ROOT\/tls\/$f"/'
mut live-pair-removed-before-generation 's/^  tmpk="" tmpc=""$/  tmpk="" tmpc=""; rm -f -- "$ROOT\/tls\/cert.pem" "$ROOT\/tls\/key.pem"/'
mutv verify-host-unvalidated 's/^qa_ip_is_ipv4 "\$TS_HOST" \|\| \{.*$/:/'
echo "killed=$K survived_or_unapplied=$S"; [ "$S" = 0 ]
