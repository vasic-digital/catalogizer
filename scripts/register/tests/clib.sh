#!/usr/bin/env bash
# clib.sh - shared helpers of the container-leg register tests (T064, T064a, T066, T067, T067a).
# Source AFTER lib.sh. Every register write of these tests goes through scripts/register/locked.sh, which composes the REAL
# scripts/containers/run_pinned.sh (RUNP) and the REAL podman and runs the image's own sqlite3 and engine binary (IMG-TESTUTIL).
# DEVIATION from the task text (recorded in every identity header): the task asks for a `podman` shim inside the container that runs the
# inner command directly; nested rootless podman is not available inside the container and the replacement would make the file mounts
# (/src, /out) unmappable, so the tests run on the HOST with a LOGGING shim: it records its argv and execs the real podman (real image,
# real mounts, real sqlite3 of IMG-TESTUTIL). The shim log gives the same "composed call" oracle.
# A scratch "root" stands in for the repository root (LOCKED_ROOT, honoured only with LOCKED_TEST_MODE=1): docs/, .audit/, a copy of the
# engine binary and of register_ext.sql. The real docs/workable_items.db is never touched.
# a private copy of the image lock (another agent or a lock refresh may rewrite the tracked file while a long run reads it: observed as a transient
# run_pinned lock_unreadable); RUNP_LOCK is the documented test input of RUNP and is read by backup_db.sh and export.sh for the image digest too
if [ -z "${RUNP_LOCK:-}" ]; then for _i in 1 2 3 4 5 6 7 8 9 10; do cp "$ROOT/build/containers/images.lock.yaml" "$T_SCR/images.lock.yaml" && python3 -I -c 'import sys,yaml;d=yaml.safe_load(open(sys.argv[1]));sys.exit(0 if isinstance(d,dict) and d.get("images") else 1)' "$T_SCR/images.lock.yaml" 2>/dev/null && break; sleep 1; done; export RUNP_LOCK="$T_SCR/images.lock.yaml"; fi
RUNP=${LOCKED_RUNP:-$ROOT/scripts/containers/run_pinned.sh}
LOCKED=${LOCKED:-$REG_DIR/locked.sh}
BACKUP=${BACKUP:-$REG_DIR/backup_db.sh}
SHIM_DIR="$T_SCR/shim"; SHIM_LOG="$T_SCR/shim.log"; mkdir -p "$SHIM_DIR"; : >"$SHIM_LOG"
cat >"$SHIM_DIR/podman" <<'SH'
#!/bin/sh
# logging shim: one line per call (argv joined by a space), then the real podman
printf '%s\n' "$*" >>"$SHIM_LOG"
exec /usr/bin/podman "$@"
SH
chmod +x "$SHIM_DIR/podman"
WI_IN=/src/submodules/constitution/scripts/workable-items/bin/workable-items-linux   # engine path INSIDE the container
mkroot() {  # mkroot NAME -> scratch root path (stdout)
  local r="$T_SCR/$1"; mkdir -p "$r/docs" "$r/.audit" "$r/submodules/constitution/scripts/workable-items/bin" "$r/scripts/register"
  cp "$ROOT/submodules/constitution/scripts/workable-items/bin/workable-items-linux" "$ROOT/submodules/constitution/scripts/workable-items/bin/.source.sha256" "$r/submodules/constitution/scripts/workable-items/bin/"
  cp "$REG_DIR/register_ext.sql" "$r/scripts/register/"; [ -f "${RECONCILE:-$REG_DIR/reconcile.sh}" ] && cp "${RECONCILE:-$REG_DIR/reconcile.sh}" "$r/scripts/register/reconcile.sh"; cp "$REG_DIR/gate.sh" "$REG_DIR/dump.sh" "$REG_DIR/export.sh" "$r/scripts/register/" 2>/dev/null; echo "$r"; }
# lk ROOT <locked.sh args...> : run the wrapper under test against a scratch root, shim first on PATH
lk() { local r=$1; shift; env LOCKED_TEST_MODE=1 LOCKED_ROOT="$r" LOCKED_RUNP="${LK_RUNP:-$RUNP}" LOCKED_BACKUP="${LK_BACKUP:-$BACKUP}" LOCKED_ROOT_SCRIPTS="$ROOT" SHIM_LOG="$SHIM_LOG" PATH="$SHIM_DIR:$PATH" bash "$LOCKED" "$@"; }
# cdb ROOT NAME : fresh register-shaped DB docs/NAME inside the scratch root, written through the wrapper
cdb() { lk "$1" -- sh -c "$WI_IN validate --db /src/docs/$2 >/dev/null && sqlite3 /src/docs/$2 '.read /src/scripts/register/register_ext.sql'" >/dev/null 2>"$T_SCR/cdb.err" || { echo "cdb failed: $(cat "$T_SCR/cdb.err")" >&2; return 1; }; }
MINT_SH='id=$(sqlite3 /src/docs/DBNAME "INSERT INTO reg_ids(minted_by,mint_basis) VALUES (\"t\",\"manual\"); SELECT atm_id FROM reg_ids ORDER BY seq DESC LIMIT 1;") || exit 3; '"$WI_IN"' add Task Low --db /src/docs/DBNAME --id "$id" --prefix CAT --title "scratch item" --description "scratch item for the container-leg register tests, long enough for the floor" >/dev/null || exit 4; echo "$id"'
mint_cmd() { printf '%s' "${MINT_SH//DBNAME/$1}"; }   # mint_cmd DBNAME -> shell text that mints one id and adds an item
evhead() {  # container-leg identity header (no credential, no env dump)
  echo "# identity: task=${1:-?} utc=$(date -u +%Y-%m-%dT%H:%M:%SZ) host=$(hostname -s) uid=$(id -u)"
  echo "# git_head=$(git -C "$ROOT" rev-parse HEAD 2>/dev/null) tree_dirty_files=$(git -C "$ROOT" status --porcelain 2>/dev/null | wc -l)"
  echo "# container leg: scripts/containers/run_pinned.sh IMG-TESTUTIL via the logging podman shim (deviation recorded in clib.sh); image: $(grep -A4 '^- id: IMG-TESTUTIL$' "${RUNP_LOCK:-$ROOT/build/containers/images.lock.yaml}" | grep -o 'sha256:[0-9a-f]*' | head -1)"
  echo "# sha256 locked=$(sha256sum "$LOCKED" 2>/dev/null | cut -c1-64) backup_db=$(sha256sum "$BACKUP" 2>/dev/null | cut -c1-64) test=$(sha256sum "${BASH_SOURCE[1]:-$0}" 2>/dev/null | cut -c1-64) clib=$(sha256sum "${BASH_SOURCE[0]}" | cut -c1-64)"
  echo "# engine=$WI sha256=$(sha256sum "$WI" 2>/dev/null | cut -d' ' -f1)"; }
fsha() { sha256sum "$1" 2>/dev/null | cut -d' ' -f1; }
DUMP=${DUMP:-$REG_DIR/dump.sh}; EXPORT=${EXPORT:-$REG_DIR/export.sh}; REPLAY=${REPLAY:-$REG_DIR/replay.sh}
# tool ROOT <script> args... : run one of the register tools against a scratch root (wrapper + RUNP are the real ones)
tool() { local r=$1 s=$2; shift 2; env LOCKED_TEST_MODE=1 LOCKED_ROOT="$r" LOCKED_RUNP="$RUNP" LOCKED="$LOCKED" SHIM_LOG="$SHIM_LOG" PATH="$SHIM_DIR:$PATH" bash "$s" "$@"; }
