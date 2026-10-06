#!/usr/bin/env bash
# roundtrip_redis.sh - T132: SET/GET/INCR/EXPIRE/DEL with SPECIFIED answers over an authenticated connection; the unauthenticated command is refused.
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
H="$(host_of REDIS redis)"; N="$(nonce)"; export REDISCLI_AUTH="$TI_REDIS_PASSWORD"
r() { timeout 10 redis-cli -h "$H" -p 6379 "$@"; }
r_is() { local want=$1; shift; expect_eq "$(r "$@")" "$want"; }
incr3() { r INCR "ti:cnt:$N" >/dev/null; r INCR "ti:cnt:$N" >/dev/null; r_is 3 INCR "ti:cnt:$N"; }
noauth() { local o; o="$(env -u REDISCLI_AUTH timeout 10 redis-cli -h "$H" -p 6379 GET "ti:rt:$N" 2>&1)"; case "$o" in *NOAUTH*|*"Authentication required"*) return 0;; *) echo "$o"; return 1;; esac; }
step set_key r_is OK SET "ti:rt:$N" "v-$N"
step get_key r_is "v-$N" GET "ti:rt:$N"
step incr_three incr3
step expire_set r_is 1 EXPIRE "ti:rt:$N" 60
step delete_keys r_is 2 DEL "ti:rt:$N" "ti:cnt:$N"
step key_gone r_is 0 EXISTS "ti:rt:$N"
step unauthenticated_refused noauth
finish redis
