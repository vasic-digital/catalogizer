#!/usr/bin/env bash
# import_plan.sh - T170 / T171 (WP-20): imports the plan-document seeds (S-26) and the innovation entries (S-27) into the findings register.
# Usage: import_plan.sh --class seeds|innovation --db <register db> --freeze-json <freeze.json> --engine <workable-items binary>
#          [--category-map F] [--backup-record F] [--progress-log F] [--report F] [--reconciliation F] [--busy-timeout-ms N] [--dry-run]
# The register is written only through locked.sh (single writer, doc04 12.2): in the real register this script is the ONE command of
#   scripts/register/locked.sh -- scripts/register/import_plan.sh --class seeds --db docs/workable_items.db ...   (inside IMG-TESTUTIL, after the pre-op backup
#   `scripts/register/backup_db.sh --record F`, whose record --backup-record F must name the unchanged database; the real database is refused without it).
# The work is import_plan.py (documented there and in docs/scripts/import_plan.md); this file is the stable entry point and adds nothing else.
# Exit: 0 ok, 2 usage, 20 refusal, 21 resumable write failure, 1 other.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
command -v python3 >/dev/null 2>&1 || { echo "import_plan: REFUSED reason=python3_missing" >&2; exit 20; }
exec python3 -I "$HERE/import_plan.py" "$@"
