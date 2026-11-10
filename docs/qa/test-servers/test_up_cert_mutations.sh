#!/usr/bin/env bash
# Paired mutations for test_up_cert.sh: each mutated COPY of up.sh (placed next to ports.env) must make the test FAIL. Exit 0 = all killed.
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; S=0; K=0
mut() { # name sed-expression
  local f="$HERE/.up_mut_$1.sh"; sed -E "$2" "$HERE/up.sh" > "$f"
  if cmp -s "$HERE/up.sh" "$f"; then echo "NOT-APPLIED $1"; S=$((S+1)); rm -f -- "$f"; return; fi
  if bash "$HERE/test_up_cert.sh" "$f" >/dev/null 2>&1; then echo "SURVIVED $1"; S=$((S+1)); else echo "KILLED   $1"; K=$((K+1)); fi
  rm -f -- "$f"
}
mut cert-check-always-true 's/^if ! cert_has_ip; then/if false; then/'
mut env-file-host-wins 's/^if \[ -n "\$_ts_host_caller" \]; then TS_HOST="\$_ts_host_caller"; else TS_HOST="\$\(qa_lan_ip\)" \|\| exit 1; fi/TS_HOST="${TS_HOST:-}"/'
mut caller-host-ignored 's/^if \[ -n "\$_ts_host_caller" \]; then TS_HOST="\$_ts_host_caller"; else/if false; then :; else/'
mut detect-failure-ignored 's/TS_HOST="\$\(qa_lan_ip\)" \|\| exit 1/TS_HOST="$(qa_lan_ip || echo 10.0.0.5)"/'
mut containers-keep-stale-host 's/ -e "TS_HOST=\$TS_HOST"//'
mut san-without-ip 's/,IP:\$TS_HOST//'
mut old-cert-deleted-not-kept 's/mv -- "\$ROOT\/tls\/\$f" "\$ROOT\/tls\/\$f.bak-\$stamp"/rm -f -- "$ROOT\/tls\/$f"/'
mut key-mode-open 's/chmod 600 /chmod 644 /'
echo "killed=$K survived_or_unapplied=$S"; [ "$S" = 0 ]
