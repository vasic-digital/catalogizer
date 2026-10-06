#!/usr/bin/env bash
set -euo pipefail
A='§11.4 End-user quality guarantee — forensic anchor'
H="$(grep -E '^#{1,6}[[:space:]]' "$1" || true)"
a=0;b=0
for i in $(seq 1 100); do if grep -qF "$A" <<<"$H"; then a=$((a+1)); fi; done
for i in $(seq 1 100); do if printf '%s\n' "$H" | grep -qF "$A"; then b=$((b+1)); fi; done
# control needle: absent string must NOT match either way
c=0; if grep -qF 'ZZZ-not-present-ZZZ' <<<"$H"; then c=1; fi
echo "here-string-pass=$a/100 printf-pipe-pass=$b/100 absent-needle-matched=$c"
