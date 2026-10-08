#!/usr/bin/env bash
x="first
second"
printf '%s\n' "a
b
c" >/dev/null
echo end
if [ "${1:-}" = skip ]; then
  y="never
  seen"
fi
