#!/usr/bin/env bash
# Paired mutations for lan_ip.sh: each mutated COPY must make test_lan_ip.sh FAIL (non-zero). Exit 0 = every mutant killed.
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
W="$(mktemp -d)" || exit 1
[ -n "$W" ] && [ -d "$W" ] || exit 1
trap 'rm -rf -- "${W:?}"' EXIT
LIB="$HERE/lan_ip.sh"; S=0; K=0
mut() { # name, sed-expression
  local f="$W/m_$1.sh"; sed -E "$2" "$LIB" > "$f"
  if cmp -s "$LIB" "$f"; then echo "NOT-APPLIED $1 (sed matched nothing)"; S=$((S+1)); return; fi
  if bash "$HERE/test_lan_ip.sh" "$f" >/dev/null 2>&1; then echo "SURVIVED $1"; S=$((S+1)); else echo "KILLED   $1"; K=$((K+1)); fi
}
mut ipv4-anchor-start 's/\^\$oct\\\./$oct\\./'
mut ipv4-anchor-end 's/\\\.\$oct\$/\\.$oct/'
mut assigned-prefix-match 's#\[\[:space:\]\]inet\[\[:space:\]\]\$\{ip//\./\\\\\.\}/#[[:space:]]inet[[:space:]]${ip//./\\\\.}#'
mut skip-assigned-check 's/^(  if ! )qa_ip_is_assigned "\$cand"/\1true/'
mut skip-ipv4-check 's/^(  if ! )qa_ip_is_ipv4 "\$cand"; then/\1true; then/'
mut env-override-ignored 's/if \[ -n "\$\{QA_LAN_IP-\}" \]/if false/'
mut qa-bind-honoured 's/\$\{QA_LAN_IP-\}/${QA_BIND-}/; s/cand="\$QA_LAN_IP"/cand="$QA_BIND"/'
mut lan-bind-honoured 's/\$\{QA_LAN_IP-\}/${LAN_BIND-}/; s/cand="\$QA_LAN_IP"/cand="$LAN_BIND"/'
mut route-never-fails 's/return 1; fi$/cand=127.0.0.1; fi/'
echo "killed=$K survived_or_unapplied=$S"; [ "$S" = 0 ]
