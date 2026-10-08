#!/usr/bin/env bash
# t191_scratch_run.sh - T191 (tasks.md WP-22): run the sync driver with the repository's own config (.helix/reporting.yaml, zero configured trackers) and record each
# candidate tracker (GitHub issues, GitFlic, GitLab, GitVerse, Crashlytics, Sonar) as SKIPPED(not_configured) in reg_tracker_sync_log, written by the driver through
# scripts/register/locked.sh. HONEST SCOPE: docs/workable_items.db does NOT exist yet (the register go-live is T069), so T191 as worded ("the rows in reg_tracker_sync_log of the
# register") cannot be completed by this script; it runs against a SCRATCH register built as the tests build it (clib.sh: real engine, real DDL, real wrapper, three items) and the
# evidence says so in its "scratch_register" block. The scratch database is deleted with the scratch root, so the file hashes in the "db" block identify a file that no longer
# exists; the retained, checkable content is the text dump written next to the JSON (<out>.scratch-dump.sql, its sha256 is in SHA256SUMS). Re-run it against the real register the
# day the database exists: `scripts/register/sync_trackers.sh --json specs/001-full-project-audit-remediation/evidence/register/tracker_sync_0.json` (the path the task names).
# Usage: t191_scratch_run.sh <repo-relative output json under the config's evidence_dir>   (companion documentation: docs/scripts/sync_trackers.md, section "t191_scratch_run.sh")
# Output: the JSON, <out>.scratch-dump.sql (the scratch register dumped as text), exit 0 only when every candidate has its SKIPPED(not_configured) rows.
set -u
. "$(dirname "$0")/lib.sh"; . "$(dirname "$0")/clib.sh"; SYNC=${SYNC:-$REG_DIR/sync_trackers.sh}; . "$(dirname "$0")/sync_common.sh"
OUTJ=${1:?output json (repository-relative)}
R=$(mkroot t191) && cdb "$R" workable_items.db || { echo "setup failed" >&2; exit 1; }
for i in 1 2 3; do lk "$R" -- sh -c "$(mint_cmd workable_items.db)" >/dev/null 2>&1 || { echo "mint $i failed" >&2; exit 1; }; done
mkdir -p "$R/.helix"; cp "$ROOT/.helix/reporting.yaml" "$R/.helix/reporting.yaml"
tool "$R" "$SYNC" --json "$OUTJ" || exit $?
mkdir -p "$(dirname "$ROOT/$OUTJ")"
python3 -I - "$R/$OUTJ" "$ROOT/$OUTJ" "$R/docs/workable_items.db" "$(git -C "$ROOT" rev-parse HEAD 2>/dev/null)" "$ROOT/$OUTJ.scratch-dump.sql" <<'PY'
import json, sys, sqlite3, urllib.parse
j = json.load(open(sys.argv[1]))
c = sqlite3.connect("file:" + urllib.parse.quote(sys.argv[3]) + "?mode=ro", uri=True)
rows = [list(r) for r in c.execute("select tracker_id,status,skip_reason,count(*) from reg_tracker_sync_log group by 1,2,3 order by 1,2,3")]
j["identity"]["repository_git_head"] = sys.argv[4]
j["identity"]["git_head"] = sys.argv[4] or j["identity"].get("git_head")   # the scratch root is not a git checkout: the repository head the script ran from
j["scratch_register"] = {"note": "docs/workable_items.db does not exist yet (T069); this run used a scratch register with 3 scratch items, built by clib.sh (real engine, real DDL, real locked.sh). "
                                 "The 'db' block (path docs/workable_items.db, sha256_before/after) describes the SCRATCH file under that relative path; the file was deleted with the scratch root.",
                         "items": c.execute("select count(*) from items").fetchone()[0], "sync_log_by_tracker_status_reason": rows,
                         "minted_by": [r[0] for r in c.execute("select minted_by from reg_ids where minted_by like 'sync_trackers:%' order by 1")],
                         "checkable_content": "the scratch register's reg_trackers, reg_tracker_sync_log, reg_ids and items rows as text: " + sys.argv[5].rsplit("/", 1)[-1]}
with open(sys.argv[5], "w") as fh:
    for t in ("reg_trackers", "reg_tracker_sync_log", "reg_ids", "items"):
        fh.write("-- %s\n" % t)
        for r in c.execute("select * from %s order by 1" % t): fh.write(json.dumps(list(r), ensure_ascii=False) + "\n")
json.dump(j, open(sys.argv[2], "w"), indent=1, sort_keys=True); open(sys.argv[2], "a").write("\n")
cand = {"github_issues", "gitflic", "gitlab", "gitverse", "crashlytics", "sonar"}
got = {r[0] for r in rows if r[1] == "SKIPPED" and r[2] == "not_configured"}
if not cand <= got: sys.exit("candidate trackers without SKIPPED(not_configured) rows: %s" % sorted(cand - got))
PY
