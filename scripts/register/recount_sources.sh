#!/usr/bin/env bash
# recount_sources.sh - T166 (WP-20): independent recount of the Stage 0 source entries against reg_sources.scanned_entry_count.
# Usage: recount_sources.sh --freeze-json <freeze.json> --db <register db> --out <dir> [--explanations <json>] [--doc03 <file>]
# The work is recount_sources.py (documented there and in docs/scripts/recount_sources.md); this file is the stable entry point and adds nothing else.
# Exit: 0 every source matches or is explained, 1 a source differs without an explanation, 2 usage, 20 refusal (nothing written).
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
command -v python3 >/dev/null 2>&1 || { echo "recount: REFUSED reason=python3_missing" >&2; exit 20; }
command -v xargs >/dev/null 2>&1 && command -v grep >/dev/null 2>&1 && command -v awk >/dev/null 2>&1 || { echo "recount: REFUSED reason=instrument_missing (xargs, grep and awk are required)" >&2; exit 20; }
exec python3 -I "$HERE/recount_sources.py" "$@"
