#!/usr/bin/env bash
shl() {
  local y=3
  x=$(( 1 << y ))
  echo "$x"
}
echo done
