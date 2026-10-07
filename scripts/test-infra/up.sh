#!/usr/bin/env bash
# up.sh - T131. Start ONE test-infrastructure project `catalogizer-test-<build_id>` (docs/16 section 10.4): the per-project single-owner lease
# (11.4.119, registered long operation 11.4.232, scripts/longops), per-run credentials and random ports (gen_env.sh), the deterministic corpus
# (seed_corpus.sh through `TIC tooling unit`), `podman-compose up` of docker-compose.test-infra.yml, then a bounded wait until every started service
# answers its protocol-level probe (probe.sh). Rootless; images are digest-pinned and never pulled here.
# Usage:  up.sh --build-id <id> [--services postgres,redis,ftp,smb,webdav] [--seed STR] [--timeout S] [--ev-dir DIR]
#   --services   subset of services to start (default: postgres,redis,ftp,smb,webdav); `nfs` adds docker-compose.test-infra.nfs.yml (the unprivileged user-space
#                NFS server of T134, image built by nfs_build.sh); minio is BLOCKED and refused
#   --seed       corpus seed (default catalogizer-wp13-seed-1); recorded in the stack's state
#   --timeout    seconds to wait for every probe to pass (default 150)
#   --ev-dir     copy the non-secret ports file (`<project>.ports.env`) and the corpus checksum there
# Lease: the exclusive resource is the project's namespace and data directory, so the purpose key is the project name. A second owner of the SAME
#   project is refused (exit 3 `lease_held`); a different project is admitted at the same time. A dead holder is never taken over silently (exit 4
#   `lease_stale`; scripts/longops/reap.sh decides). The holder is a keeper process that lives until down.sh; the registered operation is `<project>-up-<UTC time>-<pid>`, named in the env file as TI_OP_ID.
# Containers run WITHOUT a podman pod (`podman-compose --in-pod false`): the pod podman-compose creates by default is unlabelled, so a label-scoped teardown never found it and every cycle leaked one (WF12 F2).
#   A published host port that another process takes between gen_env.sh choosing it and the bind (bind(0)+close is a time-of-check/time-of-use gap, WF12 F16) is retried: up to 3 attempts, each with fresh credentials and ports.
#   The seeded corpus cache is keyed on the seeder, the seed and the digest of the image that builds it, and is RE-VERIFIED on every start: its digest is recomputed from the files (WF12 F15).
# Exit:   0 up and every probe passing; 3 lease held; 4 stale lease; 1 failure (the partial project is torn down); 2 usage.
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$HERE/lib.sh"
BID=""; SVC="postgres,redis,ftp,smb,webdav"; SEED="catalogizer-wp13-seed-1"; TMO=150; EVDIR=""
while [ $# -gt 0 ]; do
  case "$1" in
    --build-id) BID=${2:-}; shift 2;; --services) SVC=${2:-}; shift 2;; --seed) SEED=${2:-}; shift 2;; --timeout) TMO=${2:-}; shift 2;; --ev-dir) EVDIR=${2:-}; shift 2;;
    *) ti_die "unknown argument '$1'" 2;;
  esac
done
ti_valid_id "$BID" || ti_die "--build-id must match ^[a-z0-9][a-z0-9-]{0,30}\$" 2
[[ "$TMO" =~ ^[0-9]+$ ]] || ti_die "--timeout must be a number of seconds" 2
ti_need podman podman-compose jq python3
IFS=, read -r -a LIST <<<"$SVC"
for s in "${LIST[@]}"; do case "$s" in postgres|redis|ftp|smb|webdav|nfs) ;; minio) ti_refuse minio_blocked "no obtainable MinIO image (evidence/wp12/minio-blocked.txt)";; *) ti_die "unknown service '$s'" 2;; esac; done
P="$(ti_project "$BID")"; S="$(ti_state "$BID")"; OPID="$P-up-$(date -u +%Y%m%dT%H%M%S)-$$"   # one registered operation per start: a restarted project gets a fresh record
mkdir -p "$S" || ti_die "cannot create $S"
# ---- lease (before anything else is touched) ----
# the keeper watches a file of THIS attempt: a refused second owner removes only its own file, never the live holder's
KEEP="$S/lease.keep.$OPID"; : >"$KEEP"
setsid bash -c 'while [ -e "$1" ]; do sleep 1; done' ti-lease-keeper "$KEEP" </dev/null >/dev/null 2>&1 &
KPID=$!
for _ in 1 2 3 4 5 6 7 8 9 10; do [ -r "/proc/$KPID/stat" ] && break; sleep 0.1; done
ti_lo register --purpose "$P" --owner test-infra-up --op-id "$OPID" --pid "$KPID" --container-label "$P" >/dev/null 2>"$S/lease.err.$OPID"; RC=$?
if [ "$RC" -ne 0 ]; then
  rm -f "$KEEP"   # the keeper sees the file go and exits by itself: no signal is sent
  case "$RC" in
    3) ti_refuse lease_held "$P is held by another owner: $(tr '\n' ' ' <"$S/lease.err.$OPID" | cut -c1-200)" 3;;
    4) ti_refuse lease_stale "$P has a stale claim; scripts/longops/reap.sh --purpose $P decides: $(tr '\n' ' ' <"$S/lease.err.$OPID" | cut -c1-160)" 4;;
    *) ti_die "lease registration failed rc=$RC: $(tr '\n' ' ' <"$S/lease.err.$OPID" | cut -c1-200)" 1;;
  esac
fi
fail_down() { echo "test-infra: up failed: $1" >&2; bash "$HERE/down.sh" --build-id "$BID" --op-id "$OPID" --keep-logs >/dev/null 2>&1; exit 1; }   # the owner (this start) tears its own project down
if [ -n "$(podman ps -a -q --filter "label=catalogizer.test_project=$P" 2>/dev/null)" ]; then fail_down "containers of $P already exist"; fi
# ---- credentials and ports ----
bash "$HERE/gen_env.sh" --build-id "$BID" --op-id "$OPID" >/dev/null || fail_down "gen_env.sh failed"
ENVF="$S/env"
# ---- deterministic corpus (through TIC tooling unit), then one copy per server (each server needs its own writable root) ----
# The corpus is deterministic (tests/infra/test_seed_corpus.sh: two runs byte-identical), so it is built through TIC ONCE per (seeder file, seed) and cached; a start copies the cache.
# The cache key is the sha256 of the seeder script and the seed, so a changed seeder or seed rebuilds it. TIC is refused while any container of any stream trips the anti-mess sweep,
# hence the retry (TI_TIC_RETRIES times, default 400, 5 s apart) on a cache miss only.
TUDIGEST="$(python3 -I - "$TI_LOCK" <<'PY'
import sys, yaml
d = yaml.safe_load(open(sys.argv[1]))
print([i["digest"] for i in d["images"] if i["id"] == "IMG-TESTUTIL"][0])
PY
)" || fail_down "cannot read the IMG-TESTUTIL digest from $TI_LOCK"
CKEY="$(printf '%s\n%s\n%s' "$(sha256sum "$HERE/seed_corpus.sh" | cut -d' ' -f1)" "$SEED" "$TUDIGEST" | sha256sum | cut -c1-16)"
CACHE="${TI_CORPUS_CACHE_DIR:-$TI_ROOT/.audit/out/test-infra-corpus-$CKEY}"   # TI_CORPUS_CACHE_DIR: a test relocates the cache to prove that a corrupted one is refused
if [ ! -s "$CACHE/corpus.sha256" ] || [ ! -d "$CACHE/corpus" ]; then
  SEEDOUT="$TI_ROOT/.audit/out/$P-seed"; rm -rf -- "${SEEDOUT:?}"; mkdir -p "$SEEDOUT"
  n=0
  while :; do
    bash "$TI_ROOT/scripts/test-in-container.sh" --out "$SEEDOUT" tooling unit -- bash /src/scripts/test-infra/seed_corpus.sh --seed "$SEED" --out /out/corpus --checksum-file /out/corpus.sha256 >"$S/seed.log" 2>&1; rc=$?
    if [ "$rc" -ne 0 ] && grep -q 'reason=\(anti_mess_drift\|limit_exceeds_envelope\)' "$S/seed.log" && [ "$n" -lt "${TI_TIC_RETRIES:-400}" ]; then n=$((n+1)); rm -rf -- "${SEEDOUT:?}/corpus"; sleep 5; continue; fi
    break
  done
  [ "$rc" -eq 0 ] && [ -d "$SEEDOUT/corpus" ] || fail_down "seeding failed rc=$rc: $(tail -2 "$S/seed.log" | tr '\n' ' ' | cut -c1-200)"
  rm -rf -- "${CACHE:?}.tmp.$$"; mkdir -p "${CACHE:?}.tmp.$$" && mv "$SEEDOUT/corpus" "$SEEDOUT/corpus.sha256" "${CACHE:?}.tmp.$$/" && { [ -d "$CACHE" ] || mv "${CACHE:?}.tmp.$$" "$CACHE"; }; rm -rf -- "${CACHE:?}.tmp.$$" "${SEEDOUT:?}"
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
)" || fail_down "cannot recompute the corpus digest"
[ "$CDIGEST" = "$(cut -d' ' -f1 "$SEEDOUT/corpus.sha256")" ] || fail_down "the cached corpus does not match its recorded digest (recomputed ${CDIGEST:0:16}..., recorded $(cut -c1-16 "$SEEDOUT/corpus.sha256")...): remove $SEEDOUT and start again"
rm -rf -- "${S:?}/data"; mkdir -p "$S/data"
for d in ftp smb dav; do cp -a "$SEEDOUT/corpus" "$S/data/$d" || fail_down "cannot copy the corpus to $d"; done
cp "$SEEDOUT/corpus.sha256" "$S/corpus.sha256"
( cd "$SEEDOUT/corpus" && find . -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum ) >"$S/manifest.sha256" || fail_down "cannot write the corpus manifest"
# ---- the user-space NFS server image (T134) when `nfs` is requested ----
CF=(-f "$TI_COMPOSE_FILE")
case ",$SVC," in *,nfs,*)
  NB="$(bash "$HERE/nfs_build.sh" 2>"$S/nfs-build.log")" || fail_down "nfs_build.sh failed: $(tail -2 "$S/nfs-build.log" | tr '\n' ' ' | cut -c1-200)"
  printf 'TI_NFS_IMAGE=%s\n' "$(printf '%s\n' "$NB" | sed -n 's/^image_ref=//p')" >>"$ENVF"
  CF+=(-f "$TI_ROOT/docker-compose.test-infra.nfs.yml");;
esac
# ---- compose up (images are local and digest-pinned; pull_policy never) ----
attempt=1
while :; do
  podman-compose --in-pod false -p "$P" "${CF[@]}" --env-file "$ENVF" up -d "${LIST[@]}" >"$S/compose-up.log" 2>&1 && break
  # a host port taken between the choice in gen_env.sh and the bind: remove what this attempt created, draw fresh ports and credentials, try again (bounded)
  if grep -qiE 'address already in use|port is already allocated|bind: .*in use' "$S/compose-up.log" && [ "$attempt" -lt 3 ]; then
    echo "test-infra: a published port was taken before the bind (attempt $attempt of 3): drawing new ports" >&2
    ti_rm_resources "$P" || fail_down "cannot remove the half-started project before the retry"
    bash "$HERE/gen_env.sh" --build-id "$BID" --op-id "$OPID" >/dev/null || fail_down "gen_env.sh failed on the retry"
    case ",$SVC," in *,nfs,*) printf 'TI_NFS_IMAGE=%s\n' "$(printf '%s\n' "$NB" | sed -n 's/^image_ref=//p')" >>"$ENVF";; esac
    attempt=$((attempt+1)); continue
  fi
  fail_down "podman-compose up failed: $(tail -3 "$S/compose-up.log" | tr '\n' ' ' | cut -c1-240)"
done
# ---- wait for protocol-level readiness ----
deadline=$(( $(date +%s) + TMO )); ready=0
while [ "$(date +%s)" -lt "$deadline" ]; do
  if bash "$HERE/probe.sh" --build-id "$BID" --services "$SVC" >"$S/ready.log" 2>&1; then ready=1; break; fi
  sleep 3
done
if [ "$ready" != 1 ]; then
  for c in $(podman ps -a -q --filter "label=catalogizer.test_project=$P"); do podman logs "$c" >"$S/log-$c.txt" 2>&1; done
  fail_down "services not ready within ${TMO}s: $(grep -E '^PROBE' "$S/ready.log" | grep -v PASS | head -3 | tr '\n' ' ' | cut -c1-240)"
fi
if [ -n "$EVDIR" ]; then mkdir -p "$EVDIR" && cp "$S/ports.env" "$EVDIR/$P.ports.env" && cp "$S/corpus.sha256" "$EVDIR/$P.corpus.sha256"; fi
echo "project=$P"; echo "op_id=$OPID"; grep -E '^TI_PORT_' "$ENVF"; echo "corpus_sha256=$CDIGEST"
exit 0
