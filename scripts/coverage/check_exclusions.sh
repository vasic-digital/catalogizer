#!/usr/bin/env bash
# check_exclusions.sh - T200. The coverage exclusion-fence gate (docs/05 7.3, constitution 11.4.224 E). A launcher for check_exclusions.py (python3 + PyYAML).
# Usage: check_exclusions.sh FILE [--application NAME] [--used FILE] [--root DIR] [--json OUT]
#   FILE            coverage/exclusions/<app>.yaml (schema coverage-exclusions/1)
#   --used FILE     the exclusion patterns the measuring tool really uses, one per line: a pattern not listed in FILE is an UNLISTED exclusion (FAIL)
#   --root DIR      the application root: each entry is matched against the files there; an entry matching none is reported `stale` (a note)
#   --json OUT      the verdict (schema exclusions-check/1): verdict, problems, entries, first_party_entries, sha256 of FILE
# Exit: 0 PASS; 1 FAIL (named problems on stderr); 2 usage. Full contract: docs/scripts/check_exclusions.md.
exec python3 -I "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")/check_exclusions.py" "$@"
