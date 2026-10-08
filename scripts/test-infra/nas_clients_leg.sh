#!/usr/bin/env bash
# nas_clients_leg.sh - WP-12. READ-ONLY validation of the NEW Go protocol clients of submodules/filesystem (pkg/ftp FTP+FTPS with certificate pinning, pkg/sftp with host-key pinning, pkg/nfs3,
# pkg/fabric Retrying/Limited/Confined, pkg/decorators ReadOnly) against the owner's seven real Synology hosts (docs/infrastructure/synology-hosts.md), one protocol at a time.
# The harness is scripts/test-infra/nas_clients/ (a NEW nested Go module, build tag nasleg, no package code touched). Per host x protocol it connects through the chain
# Confined > Retrying > Limited > ReadOnly > tripwire > client, lists the top-level shares, takes a bounded breadth-first sample (depth <= 3, <= 2000 entries, <= 80 listing requests per share),
# reads at most 1 MiB of ONE small file per share (hashed, then discarded), measures listing p50/p95 and throughput, cross-checks counts against the earlier SMB survey (evidence/wp12/nas-survey)
# and records shares that SMB denies but the protocol exposes as a security finding (listing sample only, no file read).
# Trust: the certificate / host-key fingerprints recorded by the earlier survey (evidence/wp12/nas-protocols) are pinned through the clients' Pin APIs. That is a SIMULATED owner confirmation and the
# evidence says so; a mismatch is reported as a refusal and never bypassed.
# Credentials: ONLY from the gitignored env file (default <repo>/.env: SYNOLOGY_SMB_USER, SYNOLOGY_SMB_PASSWORD, SYNOLOGY_IP_<n>); parsed with dotenv_get.py, never sourced. They go into a 0600 env file in
# a 0700 directory under /dev/shm that podman reads with --env-file (names only on the command line), deleted on every exit path. Before the evidence is published it is scanned for the user name, the
# password and every host address; the run is REFUSED if one is found. Evidence records host indexes (Synology<n>), share names, counts, sha256 of names, extension histograms, latencies only.
# Containers: rootless, scripts/containers/run_pinned.sh IMG-GO, ONE container at a time (protocols run sequentially; the hosts of one protocol run in parallel goroutines inside it, each host sequential at <= 1 request/second).
# Usage:  nas_clients_leg.sh [--hosts 1,2,3,4,5,6,7] [--protocols ftps,ftp,sftp,nfs] [--ev-dir DIR] [--assemble-only]
#   --assemble-only  run no container: scan and assemble the results already present under .audit/out/nas-clients/res (a protocol run refused for host memory can be repeated alone, then all are assembled)     Env: TI_ENV_FILE (default <repo>/.env)
# Exit:   0 finished (a refused/blocked protocol is a RESULT, recorded); 1 a leak scan, container run or assembly step failed; 2 usage. SKIP (exit 0) when the env file or a variable is absent.
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$HERE/lib.sh"
HOSTS="1,2,3,4,5,6,7"; PROTOS="ftps,ftp,sftp,nfs"; EVDIR=""; ASSEMBLE_ONLY=0; DIAG=0
while [ $# -gt 0 ]; do case "$1" in
  --hosts) ti_optval "$1" $# "${2:-}"; HOSTS=$2; shift 2;; --protocols) ti_optval "$1" $# "${2:-}"; PROTOS=$2; shift 2;;
  --ev-dir) ti_optval "$1" $# "${2:-}"; EVDIR=$2; shift 2;; --assemble-only) ASSEMBLE_ONLY=1; shift;; --diag) DIAG=1; shift;; *) ti_die "unknown argument '$1'" 2;; esac; done
[[ "$HOSTS" =~ ^[1-7](,[1-7])*$ ]] || ti_die "--hosts must be a comma list of 1..7" 2
[[ "$PROTOS" =~ ^(ftps|ftp|sftp|nfs)(,(ftps|ftp|sftp|nfs))*$ ]] || ti_die "--protocols must be a comma list of ftps,ftp,sftp,nfs" 2
[ -n "$EVDIR" ] || EVDIR="$TI_ROOT/specs/001-full-project-audit-remediation/evidence/wp12/nas-clients"
EVDIR="$(ti_abs "$EVDIR")"
ENVF="${TI_ENV_FILE:-$TI_ROOT/.env}"
if [ ! -r "$ENVF" ]; then echo "SKIP nas_clients reason=env_absent the gitignored env file is absent; the real-NAS leg is optional"; exit 0; fi
ti_need jq sha256sum python3 podman git
envval() { python3 -I "$HERE/dotenv_get.py" "$ENVF" "$1" 2>/dev/null; }
missing=""
[ -n "$(envval SYNOLOGY_SMB_USER)" ] || missing="$missing SYNOLOGY_SMB_USER"
[ -n "$(envval SYNOLOGY_SMB_PASSWORD)" ] || missing="$missing SYNOLOGY_SMB_PASSWORD"
IFS=, read -r -a IDX <<<"$HOSTS"
for i in "${IDX[@]}"; do [ -n "$(envval "SYNOLOGY_IP_$i")" ] || missing="$missing SYNOLOGY_IP_$i"; done
if [ -n "$missing" ]; then echo "SKIP nas_clients reason=credentials_absent variables:$missing"; exit 0; fi
SRC_EV="$TI_ROOT/specs/001-full-project-audit-remediation/evidence/wp12"
[ -d "$SRC_EV/nas-protocols" ] && [ -d "$SRC_EV/nas-survey" ] || ti_die "the reference evidence nas-protocols/ and nas-survey/ is missing"
SECDIR="$(mktemp -d /dev/shm/catalogizer-ncl.XXXXXX)" || ti_die "cannot create a directory under /dev/shm"
chmod 700 "$SECDIR"
mkdir -p "$TI_ROOT/.audit/scratch" && VIEW="$(mktemp -d "$TI_ROOT/.audit/scratch/ncl-view.XXXXXX")" || ti_die "cannot create the view"
STABLE="$TI_ROOT/.audit/out/nas-clients"; mkdir -p "$STABLE/gocache" "$STABLE/gomod" || ti_die "cannot create $STABLE"
cleanup() {
  case "$SECDIR" in /dev/shm/catalogizer-ncl.*) rm -rf -- "${SECDIR:?}";; esac
  case "$VIEW" in "$TI_ROOT"/.audit/scratch/ncl-view.*) rm -rf -- "${VIEW:?}";; esac
}
ti_exit_on_signals
trap cleanup EXIT
# ---- the view: the module under test (no .git), the harness, and the reference evidence (survey fingerprints and SMB counts); never the repository, never .env ----
mkdir -p "$VIEW/submodules" "$VIEW/scripts/test-infra" "$VIEW/ref" || ti_die "cannot build the view"
cp -a "$TI_ROOT/submodules/filesystem" "$VIEW/submodules/filesystem" && rm -rf -- "${VIEW:?}/submodules/filesystem/.git" || ti_die "cannot copy the module"
cp -a "$TI_ROOT/scripts/test-infra/nas_clients" "$VIEW/scripts/test-infra/nas_clients" || ti_die "cannot copy the harness"
for i in "${IDX[@]}"; do
  for f in "nas-protocols/ftps-$i.json" "nas-protocols/sftp-$i.json" "nas-survey/survey-$i.json"; do cp "$SRC_EV/$f" "$VIEW/ref/" 2>/dev/null || true; done
done
OUT="$STABLE"; mkdir -p "$OUT/res" || ti_die "cannot create $OUT/res"
# a run replaces only the results of the protocols it measures (so one refused protocol run can be repeated alone)
if [ "$ASSEMBLE_ONLY" = 0 ]; then for p in ${PROTOS//,/ }; do for i in "${IDX[@]}"; do rm -f -- "${OUT:?}/res/$p-$i.json"; done; done; fi
USER_V="$(envval SYNOLOGY_SMB_USER)"; PASS_V="$(envval SYNOLOGY_SMB_PASSWORD)"
RUNRE=TestNASClients; [ "$DIAG" = 0 ] || RUNRE=TestDiagFTPUnparsed
GOCMD='set -u; NCL_RUNRE=__RUNRE__; W=/tmp/w; mkdir -p $W/scripts/test-infra $W/submodules && cp -a /src/submodules/filesystem $W/submodules/ && cp -a /src/scripts/test-infra/nas_clients $W/scripts/test-infra/ && cd $W/scripts/test-infra/nas_clients && env GOTOOLCHAIN=local GOFLAGS=-mod=mod HOME=/out GOCACHE=/out/gocache GOMODCACHE=/out/gomod GOMAXPROCS=2 CGO_ENABLED=1 go test -tags nasleg -count=1 -run "$NCL_RUNRE" -timeout 45m -v . ; rc=$?; cp go.sum /out/go.sum.generated 2>/dev/null; exit $rc'
GOCMD="${GOCMD//__RUNRE__/$RUNRE}"
echo "nas_clients_leg: hosts=$HOSTS protocols=$PROTOS (read-only, <=1 request/s per host, one container at a time)"
RC=0
IFS=, read -r -a PL <<<"$PROTOS"
for p in "${PL[@]}"; do
  [ "$ASSEMBLE_ONLY" = 0 ] || continue
  ( umask 077; { printf 'NCL_USER=%s\nNCL_PW=%s\nNCL_PROTO=%s\nNCL_HOSTS=%s\nNCL_OUT=/out/res\nNCL_REF=/src/ref\n' "$USER_V" "$PASS_V" "$p" "$HOSTS"; [ "$DIAG" = 0 ] || printf 'NCL_DIAG=1\n'; for i in "${IDX[@]}"; do printf 'NCL_IP_%s=%s\n' "$i" "$(envval "SYNOLOGY_IP_$i")"; done; } >"$SECDIR/env" ) || ti_die "cannot write the env file"
  chmod 600 "$SECDIR/env"
  OPID="nasclients-$$-$p"
  ARGV=()
  while IFS= read -r line; do ARGV+=("$line"); done < <(cd "$VIEW" && TI_RUNP_PRINT=1 ti_runpinned --out "$OUT" --op-id "$OPID" IMG-GO -- sh -c "$GOCMD")
  [ "${#ARGV[@]}" -gt 5 ] && [ "${ARGV[0]}" = podman ] || ti_refuse run_pinned_failed "run_pinned.sh printed no podman argv"
  NEW=(); ins=0
  for a in "${ARGV[@]}"; do
    if [ "$ins" = 0 ] && [ "$a" = "--" ]; then NEW+=(--env-file "$SECDIR/env"); ins=1; fi
    NEW+=("$a")
  done
  [ "$ins" = 1 ] || ti_refuse argv_malformed "no -- separator before the image"
  DISK_HEADROOM_REPO_ROOT="$TI_ROOT" bash "$TI_ROOT/scripts/containers/disk_headroom.sh" --need 1500000000 --op-id "$OPID-run" >/dev/null 2>&1 || ti_refuse disk_headroom "go image run"
  echo "nas_clients_leg: protocol $p start $(date -u +%H:%M:%SZ)"
  "${NEW[@]}" 9>&- >"$OUT/log-$p.txt" 2>&1; lrc=$?
  rm -f -- "${SECDIR:?}/env"
  echo "nas_clients_leg: protocol $p done rc=$lrc $(date -u +%H:%M:%SZ)"
  [ "$lrc" = 0 ] || RC=1
done
# ---- leak scan: the user name, the password and every address, as fixed strings from a 0600 pattern file (never argv) ----
( umask 077; { printf '%s\n%s\n' "$USER_V" "$PASS_V"; for i in "${IDX[@]}"; do envval "SYNOLOGY_IP_$i"; done; } >"$SECDIR/pat" )
LEAK=0
if grep -rIlF -f "$SECDIR/pat" "$OUT/res" "$OUT"/log-*.txt >/dev/null 2>&1; then LEAK=1; fi
if grep -rIlE '(^|[^0-9.])[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}([^0-9.]|$)' "$OUT/res" "$OUT"/log-*.txt >/dev/null 2>&1; then LEAK=1; fi
# control needle: the scanner must find a planted address in a scratch file
printf 'x 10.20.30.40 y\n' >"$SECDIR/needle"
grep -IlE '(^|[^0-9.])[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}([^0-9.]|$)' "$SECDIR/needle" >/dev/null || ti_die "leak-scan control needle not found: the scanner is blind" 1
if [ "$LEAK" = 1 ]; then ti_refuse leak_found "a credential or an address is in the results or logs under $OUT; nothing is published" 1; fi
if [ "$DIAG" = 1 ]; then
  mkdir -p "$EVDIR" && cp "$OUT"/res/diag-*.json "$EVDIR/" 2>/dev/null || ti_die "no diag result produced" 1
  ( cd "$EVDIR" && sha256sum -- *.json go.sum.generated 2>/dev/null >SHA256SUMS && sha256sum -c --quiet SHA256SUMS ) || ti_die "SHA256SUMS does not verify" 1
  echo "nas_clients_leg: diagnostic result in $EVDIR"; exit "$RC"
fi
# ---- assemble the evidence ----
HEAD_C="$(git -C "$TI_ROOT" rev-parse HEAD 2>/dev/null)"; AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
FS_HEAD="$(git -C "$TI_ROOT/submodules/filesystem" rev-parse HEAD 2>/dev/null)"
FS_DIRTY="$(git -C "$TI_ROOT/submodules/filesystem" status --porcelain -- pkg 2>/dev/null | wc -l)"
IMG_DIGEST="$(python3 -I - "$TI_LOCK" <<'PY'
import sys, yaml
d = yaml.safe_load(open(sys.argv[1]))
print([i for i in d["images"] if i["id"] == "IMG-GO"][0]["digest"])
PY
)"
mkdir -p "$EVDIR" || ti_die "cannot create $EVDIR"
rm -f -- "${EVDIR:?}"/*.json "${EVDIR:?}"/SHA256SUMS
TOOLS="$(for f in scripts/test-infra/nas_clients_leg.sh scripts/test-infra/nas_clients/nas_clients_test.go scripts/test-infra/nas_clients/nas_diag_test.go scripts/test-infra/nas_clients/go.mod scripts/test-infra/nas_clients/go.sum; do printf '{"file":"%s","sha256":"%s"}\n' "$f" "$(sha256sum "$TI_ROOT/$f" | cut -d' ' -f1)"; done | jq -s .)"
IDENT="$(jq -n --arg head "$HEAD_C" --arg at "$AT" --arg fs "$FS_HEAD" --argjson dirty "$FS_DIRTY" --arg img "$IMG_DIGEST" --argjson tools "$TOOLS" \
  '{schema_owner:"wp12-nas-clients", head:$head, run_at:$at, filesystem_submodule_head:$fs, filesystem_pkg_uncommitted_files:$dirty, image:"IMG-GO", image_digest:$img, tools:$tools, rootless:true, one_container_at_a_time:true,
    trust:"certificate/host-key pins come from the wp12 survey fingerprints: SIMULATED owner confirmation", writes_performed:0, content_stored:false, names_recorded:false, host_addresses_recorded:false, credentials_recorded:false}')"
for f in "$OUT"/res/*.json; do
  [ -e "$f" ] || continue
  jq --argjson id "$IDENT" '. + {identity:$id}' "$f" >"$EVDIR/$(basename "$f")" || ti_die "cannot write evidence for $f"
done
cp "$OUT/go.sum.generated" "$EVDIR/go.sum.generated" 2>/dev/null || true
python3 -I - "$EVDIR" "$IDENT" "$HOSTS" "$PROTOS" <<'PY' || ti_die "cannot assemble summary.json" 1
import json, sys, os, statistics
ev, ident, hosts, protos = sys.argv[1], json.loads(sys.argv[2]), [int(x) for x in sys.argv[3].split(",")], sys.argv[4].split(",")
R = {}
for p in protos:
    for n in hosts:
        f = f"{ev}/{p}-{n}.json"
        R[(p, n)] = json.load(open(f)) if os.path.exists(f) else None
matrix, meas, defects, findings = [], [], [], []
for n in hosts:
    row = {"host": f"Synology{n}"}
    for p in protos:
        r = R[(p, n)]
        if r is None:
            row[p] = {"status": "no_result", "reason": "the harness wrote no result for this host/protocol"}
            defects.append({"host": n, "protocol": p, "defect": "no result file written (harness or container failure)"})
            continue
        row[p] = {"status": r["status"], "reason": r["reason"]}
        for ng in r.get("negative_tests", []) or []:
            if not ng.get("ok"):
                defects.append({"host": n, "protocol": p, "defect": "negative test failed: " + ng["name"], "expected": ng["expected"], "got": ng.get("got"), "message": ng.get("message")})
        for fi in r.get("findings", []) or []:
            (defects if fi.startswith("DEFECT") else findings).append({"host": n, "protocol": p, "finding": fi})
        if r["status"] == "works":
            reads = [s["read_sample"] for s in r.get("shares", []) if s.get("read_sample") and s["read_sample"].get("status") == "ok"]
            lat = r.get("listing_latency_all", {})
            meas.append({"host": n, "protocol": p, "connect_ms": r.get("connect_ms"), "listing_requests": lat.get("n"), "listing_p50_ms": lat.get("p50_ms"), "listing_p95_ms": lat.get("p95_ms"),
                         "read_mib_per_s_median_total": round(statistics.median([x["mib_per_s_total"] for x in reads]), 2) if reads else None,
                         "read_mib_per_s_median_after_ttfb": round(statistics.median([x["mib_per_s_after_first_byte"] for x in reads]), 2) if reads else None,
                         "shares_sampled": len([s for s in r["shares"] if s["status"] == "listed"]), "min_gap_between_client_requests_s": r.get("min_gap_between_client_requests_s"),
                         "tripwire_hits": (r.get("read_only_proof") or {}).get("tripwire_hits_below_ReadOnly")})
            for s in r["shares"]:
                sm = s.get("smb_crosscheck") or {}
                if sm.get("smb_status") == "listed" and sm.get("top_level_entries_match") is False:
                    defects.append({"host": n, "protocol": p, "share": s["share"], "defect": "top-level entry count differs from the SMB survey", "smb": sm.get("smb_top_level_entries"), "this": sm.get("this_top_level_entries")})
                if sm.get("sampled_entries_comparable") and sm.get("sampled_entries_match") is False:
                    defects.append({"host": n, "protocol": p, "share": s["share"], "defect": "sampled entry total differs from the SMB survey although neither side hit its bound", "smb": sm.get("smb_sampled_entries"), "this": sm.get("this_sampled_entries")})
                if s.get("list_errors"):
                    defects.append({"host": n, "protocol": p, "share": s["share"], "defect": "listing errors inside the sample", "errors": s["list_errors"][:3]})
                rs = s.get("read_sample") or {}
                if rs and rs.get("status") not in ("ok", "no_candidate_file"):
                    defects.append({"host": n, "protocol": p, "share": s["share"], "defect": "read sample failed", "error": rs.get("error")})
    matrix.append(row)
# cross-protocol consistency: the same share must give the same top-level count, name hash and (same selection) file hash over ftp, ftps, sftp
cons = []
for n in hosts:
    by = {p: {s["share"]: s for s in (R.get((p, n)) or {}).get("shares", []) if s["status"] == "listed"} for p in ("ftp", "ftps", "sftp")}
    for sh in sorted(set().union(*[set(v) for v in by.values()])):
        got = {p: by[p][sh] for p in by if sh in by[p]}
        if len(got) < 2:
            continue
        tl = {p: (s["top_level_entries"], s["top_level_names_sha256"]) for p, s in got.items()}
        rd = {p: (s["read_sample"]["sha256_of_bytes_read"], s["read_sample"]["bytes_read"]) for p, s in got.items() if s.get("read_sample") and s["read_sample"].get("status") == "ok"}
        ok_tl = len(set(tl.values())) == 1
        ok_rd = len(set(rd.values())) <= 1
        cons.append({"host": n, "share": sh, "protocols": sorted(got), "top_level_identical": ok_tl, "read_sample_identical": ok_rd})
        if not ok_tl:
            defects.append({"host": n, "share": sh, "defect": "top-level listing differs between protocols", "detail": {p: v[0] for p, v in tl.items()}})
        if not ok_rd:
            defects.append({"host": n, "share": sh, "defect": "the sampled file differs between protocols (sha256 or size)"})
        sm = {p: (s["sampled_entries"], s["sampled_files"], s["sampled_dirs"], s["listing_requests"]) for p, s in got.items()}
        if len(set(sm.values())) > 1:
            defects.append({"host": n, "share": sh, "defect": "the bounded sample (same deterministic walk, same bounds) differs between protocols: a listing failed or returned different entries on one", "detail": {p: {"entries": v[0], "files": v[1], "dirs": v[2], "listing_requests": v[3], "list_errors": got[p].get("list_error_count")} for p, v in sm.items()}})
diags = [json.load(open(f"{ev}/{f}")) for f in sorted(os.listdir(ev)) if f.startswith("diag-") and f.endswith(".json")]
for d in diags:
    d.pop("identity", None)
summary = {"schema": "wp12-nas-clients-summary/1", "diagnostics": diags, "identity": ident, "hosts": hosts, "protocols": protos, "matrix": matrix, "measurements": meas, "cross_protocol_consistency": cons,
           "defects_found_in_clients_or_harness": defects, "findings_and_security": findings}
json.dump(summary, open(f"{ev}/summary.json", "w"), indent=2)
print("summary: %d matrix rows, %d measurements, %d defect entries, %d findings" % (len(matrix), len(meas), len(defects), len(findings)))
PY
( cd "$EVDIR" && sha256sum -- *.json go.sum.generated 2>/dev/null >SHA256SUMS ) || ti_die "cannot write SHA256SUMS" 1
( cd "$EVDIR" && sha256sum -c --quiet SHA256SUMS ) || ti_die "SHA256SUMS does not verify" 1
echo "nas_clients_leg: evidence in $EVDIR"
exit "$RC"
