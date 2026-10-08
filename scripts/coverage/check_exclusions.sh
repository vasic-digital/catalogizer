#!/usr/bin/env bash
# check_exclusions.sh - T200. The coverage exclusion-fence gate (docs/05 7.3, constitution 11.4.224 E). A launcher for check_exclusions.py (python3 + PyYAML).
# Usage: check_exclusions.sh FILE --root DIR [--repo DIR] [--application NAME] [--tools FILE] [--lanes FILE] [--items-file FILE] [--used FILE] [--measured FILE] [--json OUT] [--schema-only]
#   FILE            coverage/exclusions/<app>.yaml (schema coverage-exclusions/1)
#   --root DIR      the application root: REQUIRED (exit 2 `root_required` without it, unless --schema-only). It must exist, enumerate at least one TRACKED file (never a walk
#                   inside a work tree, never an untracked file) and be the root the tools record binds to the application
#   --repo DIR      the repository (default: `git rev-parse --show-toplevel` of the root; it is NEVER derived from where FILE lives): it holds coverage/exclusions/tools.yaml,
#                   the tool configs, scripts/containers/lanes.tsv and build/containers/images.lock.yaml
#   --tools FILE    the tools record (default <repo>/coverage/exclusions/tools.yaml): a missing file, a missing row or a scalar row is a FAIL (tool_record_missing), never a skipped rule
#   --lanes FILE    the lanes table (default <repo>/scripts/containers/lanes.tsv); --images-lock FILE the image lock; --items-file FILE a register export of tracked item ids
#   --used FILE     a hand-kept list of the patterns the tool uses: it must equal the extraction from the tool's own config (a stale hand list FAILs)
#   --measured FILE the targets the collector actually measures, one per line (kind fence with a declared scope): an in-scope file that is neither measured nor named by an entry FAILs
#   --json OUT      the verdict (schema exclusions-check/1): verdict PASS|FAIL|SCHEMA_ONLY, problems, entries, first_party_entries, content_checked, applied_checked, the input digests
#   --schema-only   check the file alone (schema, classes, justifications, paths): the verdict is SCHEMA_ONLY and says content_checked=false applied_checked=false
# Exit: 0 PASS (or SCHEMA_ONLY); 1 FAIL (named problems on stderr); 2 usage; 3 the input could not be processed. Full contract: docs/scripts/check_exclusions.md.
exec python3 -I "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")/check_exclusions.py" "$@"
