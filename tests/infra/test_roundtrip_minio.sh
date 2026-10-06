#!/usr/bin/env bash
# test_roundtrip_minio.sh - T132: the minio round trip (see test_roundtrip.sh)
exec bash "$(dirname "${BASH_SOURCE[0]}")/test_roundtrip.sh" minio "$@"
