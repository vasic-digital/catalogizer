#!/usr/bin/env bash
f() {
  local a=(
    one
    two
  )
  declare -a b=(
    x
  )
  readonly -a c=(1
    2)
  typeset -a d=(
    p
  )
  echo "${#a[@]}${#b[@]}${#c[@]}${#d[@]}" >/dev/null
}
f
