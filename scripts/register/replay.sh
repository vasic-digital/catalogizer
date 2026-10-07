#!/usr/bin/env bash
# replay.sh - T067a (docs/04 section 12.2 "Single writer across clones"): the register replay for a store conflict.
# Usage: replay.sh --onto <remote side database> --since <sha256> --out <absolute dir> [--journal <file>]
# Run in a scratch clone checked out at the remote tip (the plan owner has been asked first, section 11.4.66). It
#   1. copies the remote side's database (--onto, which must be docs/workable_items.db of THIS checkout) into <dir>/replay.db with
#      scripts/register/backup_db.sh (the verified online backup, T064a); <dir> must not exist or be empty;
#   2. replays, in order, through scripts/register/locked.sh against that copy (`locked.sh --out <dir>`, the database argument /src/docs/workable_items.db
#      rewritten to /out/replay.db, each recorded input re-bound to /out/inputs/<sha256>) every row of the local journal (--journal, default
#      .audit/register/journal.jsonl, its inputs in the sibling inputs/ directory; T064) AFTER the row whose db_sha_after equals --since (the sha256
#      of the database at the merge base; or all rows when the first row's db_sha_before equals it), with the inputs that row recorded and every minted id kept;
#   3. runs scripts/register/gate.sh on the copy (must print GATE OK), checks that it holds every reg_ids id of both sides, and writes the dump
#      (dump.sh, T066) and the exports (export.sh, T067) of the copy into <dir> (register.sql, export/);
#   4. writes <dir>/replay-report.json (rows replayed, rows skipped and why, ids, gate, sha256 of every output).
# Base row (WF10 F2): only rows of mode "register" on docs/workable_items.db are base candidates, and a row is the base only when its db_sha_after equals --since AND
# the -wal file was empty after it (wal_bytes_after = 0): the main-file hash alone does not identify a state while committed pages sit in the -wal file. A journal whose
# rows carry seq must be strictly increasing (else 20 journal_order_invalid); a corrupt line is 20 journal_corrupt; a pending marker next to the journal (a write that
# never reached it) is 20 replay_pending_ops.
# Ids (WF10 F1): a planned register row whose ids_snapshot is not "ok" (unavailable under a pending -wal, or failed) is REFUSED replay_ids_unknown: its minted ids are not
# known, so replaying it blind could give its item the next free id of the remote side (a renumbering). The tool never renumbers and never guesses.
# Rows skipped and listed: a row that is not a register write (not_a_register_write: --out and scratch rows), the regeneration rows of dump.sh/export.sh (regenerated_by_replay:
# a bash -c argument that STARTS with "# register-regenerate:"; step 3 regenerates them), `export.sh --install` rows (install_replay_owed_T175a: export.sh is a program argument; the
# T175a installer does not exist yet; UNCONFIRMED), a row that left the database unchanged (no_database_change, or command_failed when it also failed: hash equal, no ids minted, no
# -wal bytes). A row that exited non-zero but CHANGED the register (hash, minted ids or -wal bytes) is REFUSED replay_failed_row_changed_register (WF13 N1): skipping it would drop its
# committed part with an OK verdict, replaying it would repeat the failure; the plan owner decides.
# A local mint whose id the remote side holds for another item is REFUSED before anything is replayed (20 replay_id_collision, the id named): the plan
# owner decides, the tool never renumbers. A --since that matches no row is refused (20 since_not_found). Any refusal removes <dir>.
# Exit: 0 ok; 2 usage; 20 refused. Test hooks (LOCKED_TEST_MODE=1): LOCKED_ROOT, LOCKED_RUNP, LOCKED, BACKUP, DUMP, EXPORT.
set -u
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REAL_ROOT="$(cd "$SELF_DIR/../.." && pwd)"
usage() { echo "usage: replay.sh --onto <remote side database> --since <sha256> --out <absolute dir> [--journal <file>]" >&2; exit 2; }
ONTO=""; SINCE=""; OUT=""; JOURNAL=""
while [ $# -gt 0 ]; do case "$1" in
  --onto) [ $# -ge 2 ] || usage; ONTO="$2"; shift 2;;
  --since) [ $# -ge 2 ] || usage; SINCE="$2"; shift 2;;
  --out) [ $# -ge 2 ] || usage; OUT="$2"; shift 2;;
  --journal) [ $# -ge 2 ] || usage; JOURNAL="$2"; shift 2;;
  *) usage;; esac; done
[ -n "$ONTO" ] && [ -n "$SINCE" ] && [ -n "$OUT" ] || usage
case "$OUT" in /*) ;; *) usage;; esac
if [ "${LOCKED_TEST_MODE:-}" = 1 ]; then ROOT="${LOCKED_ROOT:-$REAL_ROOT}"; else ROOT="$REAL_ROOT"; unset LOCKED BACKUP DUMP EXPORT; fi
export REPLAY_ROOT="$ROOT" REPLAY_ONTO="$ONTO" REPLAY_SINCE="$SINCE" REPLAY_OUT="$OUT" REPLAY_JOURNAL="${JOURNAL:-$ROOT/.audit/register/journal.jsonl}"
export REPLAY_LOCKED="${LOCKED:-$SELF_DIR/locked.sh}" REPLAY_BACKUP="${BACKUP:-$SELF_DIR/backup_db.sh}" REPLAY_DUMP="${DUMP:-$SELF_DIR/dump.sh}" REPLAY_EXPORT="${EXPORT:-$SELF_DIR/export.sh}"
exec python3 -I - <<'PY'
import hashlib, json, os, re, shutil, subprocess, sys
E = os.environ; ROOT = E["REPLAY_ROOT"]; OUT = E["REPLAY_OUT"]; SINCE = E["REPLAY_SINCE"]
LOCKED, BACKUP, DUMP, EXPORT = E["REPLAY_LOCKED"], E["REPLAY_BACKUP"], E["REPLAY_DUMP"], E["REPLAY_EXPORT"]
WI = "/src/submodules/constitution/scripts/workable-items/bin/workable-items-linux"
created = False
def refuse(reason, detail=""):
    if created: shutil.rmtree(OUT, ignore_errors=True)
    print("replay: REFUSED reason=%s %s" % (reason, detail), file=sys.stderr); sys.exit(20)
def sh(cmd, check=True):
    p = subprocess.run(cmd, capture_output=True, text=True, stdin=subprocess.DEVNULL)
    if check and p.returncode != 0: refuse("tool_failed", "%s exited %d: %s" % (os.path.basename(cmd[0]), p.returncode, (p.stderr or p.stdout)[-300:].replace("\n", " ")))
    return p
def sha_file(p):
    h = hashlib.sha256()
    with open(p, "rb") as f:
        for b in iter(lambda: f.read(1 << 20), b""): h.update(b)
    return h.hexdigest()
if not re.fullmatch(r"[0-9a-f]{64}", SINCE): refuse("since_malformed", "--since must be 64 lowercase hex digits")
reg = os.path.realpath(os.path.join(ROOT, "docs/workable_items.db"))
if os.path.realpath(E["REPLAY_ONTO"] if os.path.isabs(E["REPLAY_ONTO"]) else os.path.join(ROOT, E["REPLAY_ONTO"])) != reg or not os.path.isfile(reg):
    refuse("onto_not_register_database", "--onto must be docs/workable_items.db of this checkout")
jr = E["REPLAY_JOURNAL"]
if not os.path.isfile(jr): refuse("journal_missing", jr)
rows = []
for ln, l in enumerate(open(jr), 1):
    if not l.strip(): continue
    try: r = json.loads(l)
    except ValueError: refuse("journal_corrupt", "line %d of %s is not JSON" % (ln, jr))
    if not isinstance(r, dict): refuse("journal_corrupt", "line %d of %s is not a journal row" % (ln, jr))
    rows.append(r)
pend = os.path.join(os.path.dirname(os.path.abspath(jr)), "pending")
if os.path.isdir(pend) and os.listdir(pend): refuse("replay_pending_ops", "%s holds %s: a register write started but its journal row was never written" % (pend, " ".join(sorted(os.listdir(pend))[:5])))
last = None
for r in rows:
    if "seq" in r:
        if not isinstance(r["seq"], int) or (last is not None and r["seq"] <= last): refuse("journal_order_invalid", "row %s has seq %r after seq %r" % (r.get("op_id"), r["seq"], last))
        last = r["seq"]
REGDB = "docs/workable_items.db"
def is_reg(r): return r.get("mode") == "register" and r.get("db", REGDB) == REGDB
# the rows to consider: those after the base row (a register row on the register database whose state is the file hash with an EMPTY -wal)
idx = None
for i, r in enumerate(rows):
    if is_reg(r) and r.get("db_sha_after") == SINCE and not r.get("wal_bytes_after"): idx = i   # MUT:base-row  # the LAST row that produced the base state
if idx is None:
    first = next((i for i, r in enumerate(rows) if is_reg(r)), None)
    if first is not None and rows[first].get("db_sha_before") == SINCE and not rows[first].get("wal_bytes_before"): idx = first - 1
    else: refuse("since_not_found", "no journal row has db_sha_after equal to %s" % SINCE)
todo = rows[idx + 1:]
if os.path.exists(OUT) and os.listdir(OUT): refuse("out_dir_not_empty", OUT)
os.makedirs(OUT, exist_ok=True); created = True
# 1. the remote side's database, by the verified online backup
rec = os.path.join(OUT, "backup.json")
env = dict(os.environ)
p = subprocess.run([BACKUP, "--record", rec], capture_output=True, text=True, stdin=subprocess.DEVNULL, env=env, cwd=ROOT)
if p.returncode != 0: refuse("backup_failed", (p.stderr or p.stdout)[-300:].replace("\n", " "))
brec = json.load(open(rec)); bak = os.path.join(ROOT, brec["backup_path"])
shutil.copyfile(bak, os.path.join(OUT, "replay.db"))
if sha_file(os.path.join(OUT, "replay.db")) != brec["backup_sha256"]: refuse("copy_mismatch", "the copy differs from the verified backup")
onto_sha = brec["source_sha256"]
def locked(opid, argv):
    return subprocess.run([LOCKED, "--out", OUT, "--op-id", opid, "--"] + argv, capture_output=True, text=True, stdin=subprocess.DEVNULL, env=env, cwd=ROOT)
def ids():
    p = locked("replay-ids", ["sqlite3", "-readonly", "file:/out/replay.db?immutable=1", "select atm_id from reg_ids order by seq"])
    if p.returncode != 0: refuse("tool_failed", "reading reg_ids: " + p.stderr[-200:].replace("\n", " "))
    return p.stdout.split()
remote_ids = set(ids())
# 2. plan: skipped rows, collisions (all checked BEFORE anything is replayed)
skipped = []; plan = []
def changed(r):  # did this row change (or may it have changed) the register database? hash, minted ids or bytes pending in the -wal file say so
    return not (r.get("db_sha_before") == r.get("db_sha_after") and not r.get("ids_minted") and not r.get("wal_bytes_after") and not r.get("wal_bytes_before"))
def is_regenerate(argv):  # the dump.sh / export.sh script: a bash -c argument that STARTS with the marker (a SQL text that merely mentions it is a real write)
    return any(isinstance(a, str) and a.startswith("# register-regenerate:") for a in argv)
def is_install(argv):  # `export.sh --install`: export.sh is the PROGRAM argument, --install an argument
    return "--install" in argv and any(isinstance(a, str) and os.path.basename(a) == "export.sh" for a in argv)
for r in todo:
    argv = r.get("argv", [])
    if r.get("mode") != "register": skipped.append({"op_id": r["op_id"], "reason": "not_a_register_write", "mode": r.get("mode")}); continue
    if is_regenerate(argv): skipped.append({"op_id": r["op_id"], "reason": "regenerated_by_replay"}); continue
    if is_install(argv): skipped.append({"op_id": r["op_id"], "reason": "install_replay_owed_T175a"}); continue
    if False: continue   # MUT:skip-mints
    if not changed(r):
        skipped.append({"op_id": r["op_id"], "reason": "command_failed" if r.get("exit") != 0 else "no_database_change", **({"exit": r.get("exit")} if r.get("exit") != 0 else {})}); continue
    if r.get("exit") != 0: refuse("replay_failed_row_changed_register", "row %s exited %s but CHANGED the register (hash, minted ids or -wal bytes): its committed part would be lost if it were skipped and cannot be replayed as it failed; the plan owner decides, nothing was written" % (r["op_id"], r.get("exit")))   # MUT:failed-row
    if r.get("ids_snapshot", "ok") != "ok": refuse("replay_ids_unknown", "row %s changed the register but its id snapshot is %r: the ids it minted are unknown, so it is not replayed blind (never a renumbering); the plan owner decides, nothing was written" % (r["op_id"], r.get("ids_snapshot")))   # MUT:ids-unknown
    plan.append(r)
local_ids = []
for r in plan:
    for i in r.get("ids_minted", []):
        if i in remote_ids: refuse("replay_id_collision", "local id %s (row %s) is held by the remote side for another item; the plan owner decides, nothing was written" % (i, r["op_id"]))
        local_ids.append(i)
# 3. replay in order; every minted id kept
indir = os.path.join(os.path.dirname(os.path.abspath(jr)), "inputs")
for n, r in enumerate(plan, 1):
    argv = [a.replace("/src/docs/workable_items.db", "/out/replay.db") for a in r["argv"]]
    for k, h in r.get("input_args", {}).items():
        src = os.path.join(indir, h)
        if not os.path.isfile(src) or sha_file(src) != h: refuse("input_missing", "recorded input %s of row %s is missing or corrupt" % (h, r["op_id"]))
        os.makedirs(os.path.join(OUT, "inputs"), exist_ok=True); shutil.copyfile(src, os.path.join(OUT, "inputs", h))
        a = argv[int(k)]; argv[int(k)] = (".read " if a.startswith(".read ") else "") + "/out/inputs/" + h
    p = locked("replay-%03d" % n, argv)
    if p.returncode != 0: refuse("replay_row_failed", "row %s exited %d: %s" % (r["op_id"], p.returncode, (p.stderr or p.stdout)[-200:].replace("\n", " ")))
final = set(ids())
missing = sorted((remote_ids | set(local_ids)) - final)
if missing: refuse("replay_id_lost", "ids not in the replayed database: " + " ".join(missing))
# 4. the gate on the copy, the exports, then the dump (the export adds reg_export_* rows, so the dump comes last)
g = locked("replay-gate", ["env", "WI=" + WI, "REG_ROOT=/src", "bash", "/src/scripts/register/gate.sh", "--db", "/out/replay.db"])
if g.returncode != 0 or not any(l.startswith("GATE OK") for l in g.stdout.splitlines()): refuse("replay_gate_failed", g.stdout[-300:].replace("\n", " "))
x = subprocess.run([EXPORT, "--out-mode", OUT, "--db-file", "replay.db"], capture_output=True, text=True, stdin=subprocess.DEVNULL, env=env, cwd=ROOT)
if x.returncode != 0: refuse("export_failed", (x.stderr or x.stdout)[-200:].replace("\n", " "))
d = subprocess.run([DUMP, "--out-dir", OUT, "--db-file", "replay.db", "--out", "register.sql"], capture_output=True, text=True, stdin=subprocess.DEVNULL, env=env, cwd=ROOT)
if d.returncode != 0: refuse("dump_failed", (d.stderr or d.stdout)[-200:].replace("\n", " "))
files = {}
for base, _, names in os.walk(OUT):
    for nme in sorted(names):
        pth = os.path.join(base, nme)
        if nme == "replay-report.json" or "/inputs" in base: continue
        files[os.path.relpath(pth, OUT)] = sha_file(pth)
report = {"since": SINCE, "onto_sha256": onto_sha, "journal": jr, "rows_replayed": [r["op_id"] for r in plan], "rows_skipped": skipped,
          "ids_remote": sorted(remote_ids), "ids_local_kept": sorted(local_ids), "gate": "GATE OK", "outputs_sha256": files}
json.dump(report, open(os.path.join(OUT, "replay-report.json"), "w"), indent=1, sort_keys=True)
print("replay: OK replayed=%d skipped=%d ids_kept=%s gate=GATE OK out=%s" % (len(plan), len(skipped), " ".join(sorted(local_ids)) or "-", OUT))
PY
