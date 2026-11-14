#!/bin/sh
# Usage (inside the image): check_caps.sh required_caps.txt  -> PASS/FAIL per required capability, exit 1 on any FAIL.
bad=0; n=0
list() { ffmpeg -hide_banner "-$1" 2>/dev/null | awk 'NR>1 && NF>=2 {print $2}' | tr ',' '\n' ; }
for k in encoders decoders muxers demuxers filters; do list $k > /tmp/caps.$k; done
while IFS=: read -r cls name; do
  case $cls in ''|'#'*) continue;; esac
  n=$((n+1))
  if grep -qx -- "$name" /tmp/caps.${cls}s; then echo "PASS $cls:$name"; else echo "FAIL $cls:$name"; bad=$((bad+1)); fi
done < "$1"
# the demuxer/muxer lists print "a,b" alias pairs in one cell
echo "required=$n missing=$bad"; [ "$bad" = 0 ]
