#!/bin/sh
# Usage: check_smoke_rows.sh <convert.tsv>   -> prints FAIL rows, exits 1 if any.
# A row is "<label>\trc=N\t..."; every rc must be 0 except the pre-existing opus muxer quirk
# ("audio a.X->opus" with -ar 44100, rc=234 in the distro ffmpeg too; see README) and the rows that are not rc rows.
f=$1; bad=0
while IFS= read -r line; do
  case $line in *"rc="*) ;; *) continue;; esac
  rc=$(printf '%s' "$line" | sed -n 's/.*\brc=\([0-9]*\).*/\1/p' | head -1)
  [ "$rc" = 0 ] && continue
  case $line in "audio a."*"->opus"*) [ "$rc" = 234 ] && continue;; esac
  echo "FAIL: $line"; bad=$((bad+1))
done < "$f"
rows=$(grep -c 'rc=' "$f"); echo "rows=$rows failures=$bad"
[ "$bad" = 0 ] && echo "SMOKE PASS" || echo "SMOKE FAIL"
[ "$bad" = 0 ]
