#!/usr/bin/env bash
# test_roundtrip_nfs.sh - T134: the NFS round trip (see test_roundtrip.sh)
exec bash "$(dirname "${BASH_SOURCE[0]}")/test_roundtrip.sh" nfs "$@"
