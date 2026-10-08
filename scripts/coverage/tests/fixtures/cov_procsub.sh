#!/usr/bin/env bash
n=0
while read -r l; do
  n=$((n+1))
done < <(echo a; echo b)
echo "$n"
