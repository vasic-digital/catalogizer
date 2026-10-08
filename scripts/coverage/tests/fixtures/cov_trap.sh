#!/usr/bin/env bash
d="$(cd "$(dirname "$0")" && pwd)"
trap 'true' USR1
trap 'echo err-trap >/dev/null' ERR
. "$d/lib_trap.sh"
work
fail_it
echo end
