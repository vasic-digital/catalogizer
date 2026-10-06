#!/usr/bin/env bash
# test_roundtrip_ftp.sh - T132: the ftp round trip (see test_roundtrip.sh)
exec bash "$(dirname "${BASH_SOURCE[0]}")/test_roundtrip.sh" ftp "$@"
