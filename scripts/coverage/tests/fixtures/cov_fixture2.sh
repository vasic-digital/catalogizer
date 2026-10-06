#!/usr/bin/env bash
# fixture 2: case arms, a here-document and a continued command.
x=1
case "$x" in
  1) echo one ;;
  2) echo two ;;
esac
cat <<EOF2
body $x
EOF2
echo a \
  b
