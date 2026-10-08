#!/usr/bin/env bash
# up.sh - T131. Start ONE test-infrastructure project `catalogizer-test-<build_id>` (docs/16 section 10.4): the per-project single-owner lease
# (11.4.119, registered long operation 11.4.232, scripts/longops), per-run credentials and random ports (gen_env.sh), the deterministic corpus
# (seed_corpus.sh through `TIC tooling unit`), `podman-compose up` of docker-compose.test-infra.yml, then a bounded wait until every started service
# answers its protocol-level probe (probe.sh). Rootless; images are digest-pinned and never pulled here.
# Usage:  up.sh --build-id <id> [--services postgres,redis,ftp,smb,webdav] [--seed STR] [--timeout S] [--ev-dir DIR]
#   (every valued option needs a non-empty value; --timeout is a positive integer without a leading zero; --seed is printable ASCII; --ev-dir is made absolute against the caller's cwd)
#   --services   subset of services to start (default: postgres,redis,ftp,smb,webdav); `nfs` adds docker-compose.test-infra.nfs.yml (the unprivileged user-space
#                NFS server of T134, image built by nfs_build.sh); minio is BLOCKED and refused
#   --seed       corpus seed (default catalogizer-wp13-seed-1); recorded in the stack's state
#   --timeout    seconds to wait for every probe to pass (default 150)
#   --ev-dir     copy the non-secret ports file (`<project>.ports.env`) and the corpus checksum there
# Lease: the exclusive resource is the project's namespace and data directory, so the purpose key is the project name. A second owner of the SAME
#   project is refused (exit 3 `lease_held`); a different project is admitted at the same time. A dead holder is never taken over silently (exit 4
#   `lease_stale`; down.sh --build-id <id> (it reaps the operation AND the claim) decides). The holder is a keeper process that lives until down.sh; the registered operation is `<project>-up-<UTC time>-<pid>`, named in the env file as TI_OP_ID.
# Containers run WITHOUT a podman pod (`podman-compose --in-pod false`): the pod podman-compose creates by default is unlabelled, so a label-scoped teardown never found it and every cycle leaked one (WF12 F2).
#   A published host port that another process takes between gen_env.sh choosing it and the bind (bind(0)+close is a time-of-check/time-of-use gap, WF12 F16) is retried: up to 3 attempts, each with fresh credentials and ports.
#   The seeded corpus cache is keyed on the seeder, the seed and the digest of the image that builds it, and is RE-VERIFIED on every start: its digest is recomputed from the files (WF12 F15).
# Exit:   0 up and every probe passing; 3 lease held, or REFUSED reason=foreign_owner (a resource of the project is not this checkout's: nothing is touched); 4 stale lease; 1 failure (the partial project is
#         torn down by its owner, the operation is recorded `failed`); 2 usage.
# Contract (WF17 round 5): the registered operation has an explicit no-progress budget (TI_OP_BUDGET_S, default 3600) and its keeper heartbeats only while a labelled container runs, so a live stack is never `hung`;
#   the project lock (one per user) is held from before the keeper file until the registration verdict, so a concurrent down.sh cannot tear a start down; an EXIT trap tears down what this start created on any
#   interrupt (INT/TERM/HUP) or failure; the op id is persisted at registration in <state>/op_id (mode 0600) and printed as `test-infra: registered op_id=<id>` on stderr; before registering, every non-terminal operation
#   of the same project whose owner is PROVEN dead is closed `reaped` (reap.sh --op-id), a live one is `lease_held`; the lease is verified (claim, holder pid, live keeper) before `exit 0`.
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$HERE/lib.sh"
BID=""; SVC="postgres,redis,ftp,smb,webdav"; SEED="catalogizer-wp13-seed-1"; TMO=150; EVDIR=""
while [ $# -gt 0 ]; do
  case "$1" in
    --build-id) ti_optval "$1" $# "${2:-}"; BID=$2; shift 2;; --services) ti_optval "$1" $# "${2:-}"; SVC=$2; shift 2;; --seed) ti_optval "$1" $# "${2:-}"; SEED=$2; shift 2;;
    --timeout) ti_optval "$1" $# "${2:-}"; TMO=$2; shift 2;; --ev-dir) ti_optval "$1" $# "${2:-}"; EVDIR=$2; shift 2;;
    *) ti_die "unknown argument '$1'" 2;;
  esac
done
ti_valid_id "$BID" || ti_die "--build-id must match ^[a-z0-9][a-z0-9-]{0,30}\$" 2
ti_uint "$TMO" 6 --timeout; TMO=$((10#$TMO))
ti_safe_word --seed "$SEED" 128
[ -z "$EVDIR" ] || EVDIR="$(ti_abs "$EVDIR")"
ti_uint0 "${TI_TIC_RETRIES:-400}" 4 TI_TIC_RETRIES
ti_need podman podman-compose jq python3 flock realpath
IFS=, read -r -a LIST <<<"$SVC"
for s in "${LIST[@]}"; do case "$s" in postgres|redis|ftp|smb|webdav|nfs) ;; minio) ti_refuse minio_blocked "no obtainable MinIO image (evidence/wp12/minio-blocked.txt)";; *) ti_die "unknown service '$s'" 2;; esac; done
P="$(ti_project "$BID")"; S="$(ti_state "$BID")"; OPID="$P-up-$(date -u +%Y%m%dT%H%M%S)-$$"   # one registered operation per start: a restarted project gets a fresh record
ti_safe_path "$S"
ti_ld_init; LD="$TI_LD_CACHE"; BUDGET="$(ti_op_budget)" || exit 2   # a bad TI_OP_BUDGET_S is a usage error (exit 2): the exit inside the command substitution ends only the subshell
# ---- project lock, then the lease (before anything else is touched) ----
ti_lock "$P"
S_NEW=0; [ -d "$S" ] || S_NEW=1
mkdir -p "$S" || ti_die "cannot create $S"
# a non-terminal operation of this project is either owned by a LIVE process (lease_held) or its owner is PROVEN dead (reap.sh --op-id: the operation becomes `reaped`, its claim goes with it): a dead
# operation left by a printed `reap.sh --purpose` or a reboot is closed here, never duplicated (WF17 TI-A3)
for f in "$LD"/ops/"$P"-up-*.json; do
  [ -e "$f" ] || continue
  o="$(jq -r 'select(.purpose_key=="'"$P"'" and (.state|IN("complete","failed","reaped","handoff","blocked-escape")|not))|.op_id' "$f" 2>/dev/null)"; [ -n "$o" ] || continue
  if DRY="$(ti_lo reap --op-id "$o" --dry-run 2>&1)" && printf '%s' "$DRY" | grep -q 'would reap dead'; then
    ti_lo reap --op-id "$o" >/dev/null 2>&1 && echo "test-infra: closed the operation $o of $P (its owner is proven dead) as reaped" >&2 || { [ "$S_NEW" = 0 ] || rmdir "$S" 2>/dev/null; ti_refuse lease_stale "the dead operation $o of $P could not be reaped; run down.sh --build-id $BID" 4; }
  else [ "$S_NEW" = 0 ] || rmdir "$S" 2>/dev/null; ti_refuse lease_held "$P is held by operation $o (a live owner, or a stack whose containers are gone: down.sh --build-id $BID --op-id <that operation> tears it down and records it)" 3; fi
done
# the keeper watches a file of THIS attempt: a refused second owner removes only its own file, never the live holder's
KEEP="$S/lease.keep.$OPID"; : >"$KEEP"
# until the registration verdict only the keeper file (and possibly a claim registered an instant before a signal) exists: any exit before the full teardown below is armed removes the keeper file, releases a
# claim/operation of THIS start only (compare-and-swap on its own run id: another owner's claim is never touched) and removes a state directory this start created and left empty
up_early_cleanup() {
  rm -f "$KEEP" "$S/lease.err.$OPID" "$S/op_id"
  ti_lo release --op-id "$OPID" --state failed --verdict interrupted_before_registration >/dev/null 2>&1 || ti_lo release --purpose "$P" --run-id "$OPID" >/dev/null 2>&1 || true
  [ "$S_NEW" = 0 ] || rmdir "$S" 2>/dev/null || true
}
ti_exit_on_signals
trap 'up_early_cleanup' EXIT
ti_keeper_start "$KEEP" "$OPID"
ti_lo register --purpose "$P" --owner test-infra-up --op-id "$OPID" --pid "$KPID" --container-label "$P" --no-progress-s "$BUDGET" >/dev/null 2>"$S/lease.err.$OPID"; RC=$?
if [ "$RC" -ne 0 ]; then
  rm -f "$KEEP"   # the keeper sees the file go and exits by itself: no signal is sent
  MSG="$(tr '\n' ' ' <"$S/lease.err.$OPID" | cut -c1-200)"; rm -f "$S/lease.err.$OPID"; [ "$S_NEW" = 0 ] || rmdir "$S" 2>/dev/null
  case "$RC" in
    3) ti_refuse lease_held "$P is held by another owner: $MSG" 3;;
    4) ti_refuse lease_stale "$P has a stale claim; down.sh --build-id $BID (it reaps the operation AND the claim) decides: $MSG" 4;;
    *) ti_die "lease registration failed rc=$RC: $MSG" 1;;
  esac
fi
rm -f "$S/lease.err.$OPID"
( umask 077; printf '%s\n' "$OPID" >"$S/op_id" ) || { rm -f "$KEEP"; ti_lo release --op-id "$OPID" --state failed --verdict op_id_not_persisted >/dev/null 2>&1; ti_die "cannot persist the operation id in $S/op_id" 1; }
echo "test-infra: registered op_id=$OPID" >&2
# from here on every exit tears down what this start owns; the success path disarms the trap immediately before `exit 0`
fail_down() { trap - EXIT; echo "test-infra: up failed (reason=$1): $2" >&2; bash "$HERE/down.sh" --build-id "$BID" --op-id "$OPID" --keep-logs --outcome failed --reason "$1" >/dev/null 2>&1; exit 1; }   # the owner (this start) tears its own project down
trap 'fail_down interrupted "interrupted or exited early (signal or unexpected exit)"' EXIT
# a resource of this project that is not this checkout's (another checkout, a partial-label container, another operation) is REFUSED and left alone: this start releases only its own lease
ti_scan "$P" || fail_down podman_query_failed "$TI_UNKNOWN failed: the state of $P is unknown, nothing was started"
if [ -n "$TI_FOREIGN" ]; then
  trap - EXIT; rm -f "$KEEP" "$S/op_id"; ti_lo release --op-id "$OPID" --state failed --verdict foreign_owner >/dev/null 2>&1; [ "$S_NEW" = 0 ] || rmdir "$S" 2>/dev/null
  ti_refuse foreign_owner "$TI_FOREIGN; this start released only its own lease and touched nothing" 3
fi
if [ "${#TI_OWN_C[@]}" -gt 0 ]; then fail_down project_not_clean "containers of $P already exist (${#TI_OWN_C[@]}); they are removed by this failed start, start again"; fi
ti_unlock   # the registration verdict is in: the long steps below hold no lock (down.sh takes it itself; the lease is the cross-process guard)
# ---- credentials and ports ----
GOUT="$(bash "$HERE/gen_env.sh" --build-id "$BID" --op-id "$OPID")" || fail_down gen_env_failed "gen_env.sh failed"
ENVF="$(printf '%s\n' "$GOUT" | sed -n 's/^env=//p')"; PORTSF="$(printf '%s\n' "$GOUT" | sed -n 's/^ports=//p')"
[ -r "$ENVF" ] && [ -r "$PORTSF" ] || fail_down gen_env_failed "gen_env.sh printed no readable env=/ports= path"
# ---- deterministic corpus (through TIC tooling unit), then one copy per server (each server needs its own writable root) ----
# The corpus is deterministic (tests/infra/test_seed_corpus.sh: two runs byte-identical), so it is built through TIC ONCE per (seeder file, seed) and cached; a start copies the cache.
# The cache key is the sha256 of the seeder script and the seed, so a changed seeder or seed rebuilds it. TIC is refused while any container of any stream trips the anti-mess sweep,
# hence the retry (TI_TIC_RETRIES times, default 400, 5 s apart) on a cache miss only.
TUDIGEST="$(python3 -I - "$TI_LOCK" <<'PY'
import sys, yaml
d = yaml.safe_load(open(sys.argv[1]))
print([i["digest"] for i in d["images"] if i["id"] == "IMG-TESTUTIL"][0])
PY
)" || fail_down lock_unreadable "cannot read the IMG-TESTUTIL digest from $TI_LOCK"
CKEY="$(printf '%s\n%s\n%s' "$(sha256sum "$HERE/seed_corpus.sh" | cut -d' ' -f1)" "$SEED" "$TUDIGEST" | sha256sum | cut -c1-16)"
CACHE="${TI_CORPUS_CACHE_DIR:-$TI_ROOT/.audit/out/test-infra-corpus-$CKEY}"   # TI_CORPUS_CACHE_DIR: a test relocates the cache to prove that a corrupted one is refused
if [ ! -s "$CACHE/corpus.sha256" ] || [ ! -d "$CACHE/corpus" ]; then
  SEEDOUT="$TI_ROOT/.audit/out/$P-seed"; rm -rf -- "${SEEDOUT:?}"; mkdir -p "$SEEDOUT"
  n=0
  while :; do
    bash "$TI_ROOT/scripts/test-in-container.sh" --out "$SEEDOUT" tooling unit -- bash /src/scripts/test-infra/seed_corpus.sh --seed "$SEED" --out /out/corpus --checksum-file /out/corpus.sha256 >"$S/seed.log" 2>&1; rc=$?
    if [ "$rc" -ne 0 ] && grep -q 'reason=\(anti_mess_drift\|limit_exceeds_envelope\)' "$S/seed.log" && [ "$n" -lt "$((10#${TI_TIC_RETRIES:-400}))" ]; then n=$((n+1)); rm -rf -- "${SEEDOUT:?}/corpus"; sleep 5; continue; fi
    break
  done
  [ "$rc" -eq 0 ] && [ -d "$SEEDOUT/corpus" ] || fail_down seeding_failed "seeding failed rc=$rc: $(tail -2 "$S/seed.log" | tr '\n' ' ' | cut -c1-200)"
  # two cache-miss starts may both reach this point: the install is ONE rename onto the cache name (mv -T fails on a non-empty existing directory), the loser removes its temporary tree (WF17 TI-C5)
  rm -rf -- "${CACHE:?}.tmp.$$"; mkdir -p "${CACHE:?}.tmp.$$" && mv "$SEEDOUT/corpus" "$SEEDOUT/corpus.sha256" "${CACHE:?}.tmp.$$/" && { [ -z "${TI_TEST_SLEEP_CACHE_INSTALL:-}" ] || sleep "$TI_TEST_SLEEP_CACHE_INSTALL"; mv -T -- "${CACHE:?}.tmp.$$" "$CACHE" 2>/dev/null || true; }; rm -rf -- "${CACHE:?}.tmp.$$" "${SEEDOUT:?}"
  [ -d "$CACHE/corpus" ] && [ -s "$CACHE/corpus.sha256" ] || fail_down cache_not_installed "the corpus cache $CACHE was not installed"
fi
SEEDOUT="$CACHE"
# the cache is trusted only after its digest is recomputed from the files (same definition as seed_corpus.sh: sha256 over the sorted `rel NUL kind NUL size NUL file-sha256` lines, names NFC)
CDIGEST="$(python3 -I - "$SEEDOUT/corpus" <<'PY'
import hashlib, os, sys, unicodedata
out = sys.argv[1]; entries = []
for d, ds, fs in os.walk(out):
    for n in ds + fs: entries.append(os.path.join(d, n))
lines = []
for p in entries:
    rel = unicodedata.normalize("NFC", os.path.relpath(p, out))
    lines.append((rel, "dir", 0, "") if os.path.isdir(p) else (rel, "file", os.path.getsize(p), hashlib.sha256(open(p, "rb").read()).hexdigest()))
lines.sort(); m = hashlib.sha256()
for r, k, sz, h in lines: m.update(("%s\0%s\0%d\0%s\n" % (r, k, sz, h)).encode("utf-8"))
print(m.hexdigest())
PY
)" || fail_down corpus_digest "cannot recompute the corpus digest"
[ "$CDIGEST" = "$(cut -d' ' -f1 "$SEEDOUT/corpus.sha256")" ] || fail_down corpus_cache_mismatch "the cached corpus does not match its recorded digest (recomputed ${CDIGEST:0:16}..., recorded $(cut -c1-16 "$SEEDOUT/corpus.sha256")...): remove $SEEDOUT and start again"
# the data files of an earlier --keep-state run are owned by the sub-uids of the container user namespace: only `podman unshare rm` can remove them (WF17 TI-C4); a leftover is never copied over
podman unshare rm -rf -- "${S:?}/data" 2>/dev/null || rm -rf -- "${S:?}/data" 2>/dev/null
[ ! -e "$S/data" ] || fail_down previous_state_not_removable "the data directory of an earlier run cannot be removed ($S/data)"
mkdir -p "$S/data"
for d in ftp smb dav; do cp -a "$SEEDOUT/corpus" "$S/data/$d" || fail_down corpus_copy "cannot copy the corpus to $d"; done
cp "$SEEDOUT/corpus.sha256" "$S/corpus.sha256"
( cd "$SEEDOUT/corpus" && find . -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum ) >"$S/manifest.sha256" || fail_down manifest "cannot write the corpus manifest"
# ---- the user-space NFS server image (T134) when `nfs` is requested ----
CF=(-f "$TI_COMPOSE_FILE")
case ",$SVC," in *,nfs,*)
  NB="$(bash "$HERE/nfs_build.sh" 2>"$S/nfs-build.log")" || fail_down nfs_build "nfs_build.sh failed: $(tail -2 "$S/nfs-build.log" | tr '\n' ' ' | cut -c1-200)"
  printf 'TI_NFS_IMAGE=%s\n' "$(printf '%s\n' "$NB" | sed -n 's/^image_ref=//p')" >>"$ENVF"
  CF+=(-f "$TI_ROOT/docker-compose.test-infra.nfs.yml");;
esac
# ---- compose up (images are local and digest-pinned; pull_policy never) ----
attempt=1
while :; do
  ti_compose --in-pod false -p "$P" "${CF[@]}" --env-file "$ENVF" up -d "${LIST[@]}" >"$S/compose-up.log" 2>&1 && break
  # a host port taken between the choice in gen_env.sh and the bind: remove what this attempt created, draw fresh ports and credentials, try again (bounded)
  if grep -qiE 'address already in use|port is already allocated|bind: .*in use' "$S/compose-up.log" && [ "$attempt" -lt 3 ]; then
    echo "test-infra: a published port was taken before the bind (attempt $attempt of 3): drawing new ports" >&2
    TI_SCAN_ONLY_OP="$OPID" ti_rm_resources "$P" || fail_down retry_cleanup "cannot remove the half-started project before the retry"   # only the resources of THIS operation (WF17 TI-B1 c)
    bash "$HERE/gen_env.sh" --build-id "$BID" --op-id "$OPID" >/dev/null || fail_down gen_env_failed "gen_env.sh failed on the retry"
    case ",$SVC," in *,nfs,*) printf 'TI_NFS_IMAGE=%s\n' "$(printf '%s\n' "$NB" | sed -n 's/^image_ref=//p')" >>"$ENVF";; esac
    attempt=$((attempt+1)); continue
  fi
  fail_down compose_up "podman-compose up failed: $(tail -3 "$S/compose-up.log" | tr '\n' ' ' | cut -c1-240)"
done
# ---- wait for protocol-level readiness ----
deadline=$(( $(date +%s) + TMO )); ready=0
while [ "$(date +%s)" -lt "$deadline" ]; do
  if bash "$HERE/probe.sh" --build-id "$BID" --services "$SVC" >"$S/ready.log" 2>&1; then ready=1; break; fi
  sleep 3
done
if [ "$ready" != 1 ]; then
  for c in $(podman ps -a -q --filter "label=catalogizer.test_project=$P" 2>/dev/null); do podman logs "$c" >"$S/log-$c.txt" 2>&1; done
  fail_down not_ready "services not ready within ${TMO}s: $(grep -E '^PROBE' "$S/ready.log" | grep -v PASS | head -3 | tr '\n' ' ' | cut -c1-240)"
fi
if [ -n "$EVDIR" ]; then { mkdir -p "$EVDIR" && cp "$PORTSF" "$EVDIR/$P.ports.env" && cp "$S/corpus.sha256" "$EVDIR/$P.corpus.sha256"; } || fail_down evidence_not_written "cannot write the requested evidence files to $EVDIR"; fi
# the lease is verified, not assumed: the claim names this operation, its holder is this start's live keeper (WF17 TI-B3)
ti_holder_ok "$P" "$OPID" "$KPID" || fail_down lease_lost "the lease of $P no longer names this start's live keeper"
trap - EXIT
echo "project=$P"; echo "op_id=$OPID"; grep -E '^TI_PORT_' "$ENVF"; echo "corpus_sha256=$CDIGEST"
exit 0
