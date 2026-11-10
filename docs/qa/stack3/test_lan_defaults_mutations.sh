#!/usr/bin/env bash
# Paired mutations for test_lan_defaults.sh: each mutated copy of sonarqube/{lib,up}.sh or s3/up.sh must make the test FAIL. Exit 0 = all killed.
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
W="$(mktemp -d)" || exit 1; [ -n "$W" ] && [ -d "$W" ] || exit 1; trap 'rm -rf -- "${W:?}"' EXIT
S=0; K=0
mut() { # name file(relative to stack3) sed-expression
  rm -rf -- "${W:?}/t"; mkdir -p "$W/t/stack3/sonarqube" "$W/t/stack3/s3" "$W/t/lib"
  cp -- "$HERE"/sonarqube/*.sh "$W/t/stack3/sonarqube/"; cp -- "$HERE/s3/up.sh" "$W/t/stack3/s3/"; cp -- "$HERE/../lib/lan_ip.sh" "$W/t/lib/"
  sed -E "$3" "$HERE/$2" > "$W/t/stack3/$2"
  if cmp -s "$HERE/$2" "$W/t/stack3/$2"; then echo "NOT-APPLIED $1"; S=$((S+1)); return; fi
  if bash "$HERE/test_lan_defaults.sh" "$W/t/stack3" >/dev/null 2>&1; then echo "SURVIVED $1"; S=$((S+1)); else echo "KILLED   $1"; K=$((K+1)); fi
}
mut sonar-undetectable-fatal sonarqube/lib.sh 's/SONAR_BIND=127.0.0.1; SONAR_BIND_UNRESOLVED=1; \}/exit 1; }/'
mut sonar-unresolved-flag-not-set sonarqube/lib.sh 's/SONAR_BIND_UNRESOLVED=1;/:;/'
mut sonar-wildcard-from-lan-bind sonarqube/lib.sh 's/SONAR_BIND="\$\(qa_lan_ip 2>\/dev\/null\)"/SONAR_BIND="${LAN_BIND:-$(qa_lan_ip 2>\/dev\/null)}"/'
mut sonar-detection-bypassed sonarqube/lib.sh 's/SONAR_BIND="\$\(qa_lan_ip 2>\/dev\/null\)"/SONAR_BIND=127.0.0.1; : "$(qa_lan_ip 2>\/dev\/null)"/'
mut sonar-explicit-bind-overridden sonarqube/lib.sh 's/if \[ -z "\$\{SONAR_BIND:-\}" \]; then/if true; then/'
mut sonar-up-unresolved-allowed sonarqube/up.sh 's/^\[ "\$\{SONAR_BIND_UNRESOLVED:-0\}" = 0 \] \|\| die.*$/:/'
mut s3-lan-keyword-ignored s3/up.sh 's/if \[ "\$S3_BIND" = lan \]/if false/'
mut s3-default-wildcard s3/up.sh 's/S3_BIND="\$\{S3_BIND:-127.0.0.1\}"/S3_BIND="${S3_BIND:-0.0.0.0}"/'
mut s3-lan-failure-ignored s3/up.sh 's/S3_BIND="\$\(qa_lan_ip\)" \|\| exit 1/S3_BIND="$(qa_lan_ip || echo 127.0.0.1)"/'
mut s3-lan-bind-honoured s3/up.sh 's/S3_BIND="\$\{S3_BIND:-127.0.0.1\}"/S3_BIND="${S3_BIND:-${LAN_BIND:-127.0.0.1}}"/'
echo "killed=$K survived_or_unapplied=$S"; [ "$S" = 0 ]
