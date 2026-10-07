#!/usr/bin/env bash
# multi-line array assignment fixture: bash traces the assignment ONCE, at its closing line (measured: Build/lib/hash.sh 31-42 traces line 42 only)
ARR=(
  one
  two
)
echo "${#ARR[@]}" >/dev/null
