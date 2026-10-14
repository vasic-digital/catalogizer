#!/usr/bin/env bash
# import_t172.sh - T172 (WP-20): doc12 WP-Q5 ticket specifics inside the import (QA-instrument cause items, platform inference, duplicate families).
# Usage: import_t172.sh --stage qa|families --db <register db> --freeze-json <freeze.json> --engine <workable-items binary> --out-dir <dir>
#          [--category-map F] [--backup-record F] [--progress-log F] [--report F] [--busy-timeout-ms N] [--dry-run]
# Order: enumerate_sources.py -> import_t172.sh --stage qa -> import.sh (T168) -> import_t172.sh --stage families. The register is written only through
#   locked.sh (single writer, doc04 12.2), inside IMG-TESTUTIL, after the pre-op backup `scripts/register/backup_db.sh --record F` (--backup-record F).
# The work is import_t172.py (documented there and in docs/scripts/import_t172.md); this file is the stable entry point and adds nothing else.
# Exit: 0 ok, 2 usage, 20 refusal, 21 resumable write failure, 1 other.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
command -v python3 >/dev/null 2>&1 || { echo "import_t172: REFUSED reason=python3_missing" >&2; exit 20; }
exec python3 -I "$HERE/import_t172.py" "$@"
