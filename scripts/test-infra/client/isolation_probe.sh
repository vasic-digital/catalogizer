#!/usr/bin/env bash
# isolation_probe.sh - WF12 F3 (inside IMG-INFRA-CLIENT): reports what the container can see of the repository, never any content. Usage: isolation_probe.sh <path under /src>...
# Output: `TOP <entries of /src>`, `NEEDLE present|absent` (the lib.sh next to this script: proves the probe can see what the view holds), `PATH <path> readable|unreadable|absent` per argument
# (existence and readability only: no byte of any file is read or printed) and `ENV <VAR> set|unset` for TI_REDIS_PASSWORD (the project's own credential reaches the client by --env-file).
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
echo "TOP $(ls -A /src | tr '\n' ' ')"
[ -f "$(dirname "${BASH_SOURCE[0]}")/lib.sh" ] && echo "NEEDLE present" || echo "NEEDLE absent"   # the client directory this very script runs from
for p in "$@"; do
  if [ ! -e "/src/$p" ]; then echo "PATH $p absent"; elif [ -r "/src/$p" ]; then echo "PATH $p readable"; else echo "PATH $p unreadable"; fi
done
[ -n "${TI_REDIS_PASSWORD:-}" ] && echo "ENV TI_REDIS_PASSWORD set" || echo "ENV TI_REDIS_PASSWORD unset"
