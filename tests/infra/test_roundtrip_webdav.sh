#!/usr/bin/env bash
# test_roundtrip_webdav.sh - T132: the webdav round trip (see test_roundtrip.sh)
exec bash "$(dirname "${BASH_SOURCE[0]}")/test_roundtrip.sh" webdav "$@"
