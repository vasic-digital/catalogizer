#!/usr/bin/env bash
# test_roundtrip_smb.sh - T132: the smb round trip (see test_roundtrip.sh)
exec bash "$(dirname "${BASH_SOURCE[0]}")/test_roundtrip.sh" smb "$@"
