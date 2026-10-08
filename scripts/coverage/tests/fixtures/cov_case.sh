#!/usr/bin/env bash
x="${1:-a}"
case "$x" in
  a)
    echo A
    ;;
  b)
    echo B
    ;;
esac
echo end
