#!/usr/bin/env bash
# probe_redis.sh - T128: an authenticated PING answered PONG, and an unauthenticated PING is REFUSED (NOAUTH): the server really requires the generated password.
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
H="$(host_of REDIS redis)"
out="$(REDISCLI_AUTH="$TI_REDIS_PASSWORD" timeout 10 redis-cli -h "$H" -p 6379 PING 2>&1)"
[ "$out" = PONG ] || fail "redis authenticated PING answered '${out:0:80}'"
noauth="$(env -u REDISCLI_AUTH timeout 10 redis-cli -h "$H" -p 6379 PING 2>&1)"
case "$noauth" in *NOAUTH*|*"Authentication required"*) ;; *) fail "redis accepted an unauthenticated PING ('${noauth:0:80}')";; esac
pass "redis PING answered PONG and the unauthenticated PING was refused"
