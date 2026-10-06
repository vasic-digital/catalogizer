#!/usr/bin/env bash
# test_roundtrip_redis.sh - T132: the redis round trip (see test_roundtrip.sh)
exec bash "$(dirname "${BASH_SOURCE[0]}")/test_roundtrip.sh" redis "$@"
