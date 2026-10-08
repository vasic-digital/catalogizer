#!/usr/bin/env bash
# wp20_ident.sh - sourced by the WP-20 shell tests AFTER lib.sh: an identity header whose container line states what the run really was.
# lib.sh's ident_header ends with a fixed line claiming "RUNP/IMG-TESTUTIL absent: host-side run", which is false (and contradicted by the
# transcripts' own host=/path= lines) whenever the test runs through scripts/containers/run_pinned.sh (WF23 review E5). This keeps lib.sh
# (a WP-06 file) untouched and replaces only that line.
container_line() {
  # inside a run_pinned.sh container: env `container=podman` is set (/run/.containerenv exists but is empty); the pinned digest is the one the
  # lock file records for IMG-TESTUTIL (the run refuses an image that does not match it), read from the repository mount when it is visible
  if [ "${container:-}" = podman ] || [ -e /run/.containerenv ]; then
    local lock dig; lock=${RUNP_LOCK:-$ROOT/build/containers/images.lock.yaml}
    dig=$(awk '/^- id: IMG-TESTUTIL$/{f=1;next} f&&/^  digest:/{print $2;exit} f&&/^- id:/{exit}' "$lock" 2>/dev/null)
    echo "# container: in-container run (container=${container:-unset}) image=IMG-TESTUTIL lock_digest=${dig:-UNKNOWN} (the digest the lock pins; the running image is not readable from inside)"
  else
    echo "# container: host-side run (no container marker); a container leg is UNCONFIRMED for this transcript"
  fi
}
ident_header_wp20() { ident_header "$@" | sed '$d'; container_line; }
