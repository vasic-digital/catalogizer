#!/usr/bin/env bash
# verify_chain_only.sh - TEST-ONLY wrapper: tools/evidence/verify --chain-only ARGS... Used by the recorder suites whose subject is the chain, the
# blobs, the schema and the run tokens; they build ledgers WITHOUT anchors, and `verify` (review WP-05 R1) answers exit 3 anchor_absent for those unless
# the caller opts in to the chain alone. The anchor behaviour itself is tested with the real `verify`: test_anchors.sh and test_anchors_r8.sh.
exec "$(dirname "$(readlink -f "$0")")/../verify" --chain-only "$@"
