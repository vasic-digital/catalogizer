#!/usr/bin/env bash
# import.sh - T168 (WP-20, doc04 section 9.3 `register-import` actor): the Stage 1/2 importer of the findings register.
# Usage: import.sh --db <register db> --freeze-json <freeze.json> --engine <workable-items binary>
#          [--kinds issue_file] [--category-map F] [--backup-record F] [--progress-log F] [--category-out F] [--report F]
#          [--busy-timeout-ms N] [--dry-run]
# The register is written only through locked.sh (single writer, doc04 12.2): in the real register this script is the ONE command of
#   scripts/register/locked.sh -- scripts/register/import.sh --db docs/workable_items.db ...   (inside IMG-TESTUTIL, after the pre-op
#   backup `scripts/register/backup_db.sh --record F`, whose record --backup-record F must name the unchanged database; the real
#   database is refused without it). A scratch run (tests, rehearsals) is called directly on a scratch DB.
# The work is import_tickets.py (documented there and in docs/scripts/import.md); this file is the stable entry point and adds nothing
# else, so that an `exec` cannot leave a second process holding the register open. Exit: 0 ok, 2 usage, 20 refusal, 21 resumable write failure, 1 other.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
command -v python3 >/dev/null 2>&1 || { echo "import: REFUSED reason=python3_missing" >&2; exit 20; }
exec python3 -I "$HERE/import_tickets.py" "$@"
