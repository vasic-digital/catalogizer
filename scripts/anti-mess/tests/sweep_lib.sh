#!/usr/bin/env bash
# sweep_lib.sh - the fixture helpers shared by test_sweep.sh and test_sweep_r5.sh (WP-08, T090; 11.4.276 round 5). Sourced after scripts/longops/tests/lib.sh. Real git repositories, real processes, real /proc, real flock;
# the only stand-ins are a unit-level `podman` shell script and a fake `cpa-host` that refuses (neither exists in the fixture).
SW=${SWEEP:-$TROOT/scripts/anti-mess/sweep.sh}
export LC_ALL=C GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=protocol.file.allow GIT_CONFIG_VALUE_0=always
mkrepo() { git init -q "$1" 2>/dev/null; git -C "$1" config user.email t@t; git -C "$1" config user.name t; git -C "$1" config commit.gpgsign false; }
cmt() { git -C "$1" add -A && git -C "$1" -c core.hooksPath=/dev/null commit -qm "${2:-c}" 2>/dev/null; }
swfx() {  # swfx <name>: a clean fixture repository with an ignored .audit, an empty exception list, a fake podman
  newfx "$1"; R=$FXN/root; rm -rf "$R"; mkrepo "$R"; echo 1 >"$R/a.txt"; printf '/.audit/\n' >"$R/.gitignore"; cmt "$R" init; mkdir -p "$R/.audit"
  printf '# path\tkind\tfile\twt\tblob\treason\n' >"$FXN/exc.tsv"
  cat >"$FXN/podman" <<'PE'
#!/usr/bin/env bash
echo "$*" >>"${PODLOG:-/dev/null}"
case "$1" in ps) case "$*" in *'{{.ID}}'*) ;; *) cat "${PODJSON:-/dev/null}" 2>/dev/null || echo '[]' ;; esac ;; *) ;; esac
exit 0
PE
  chmod +x "$FXN/podman"; echo '[]' >"$FXN/pods.json"
  export LONGOPS_REPO=$R LONGOPS_DIR=$R/.audit/longops LONGOPS_AUDIT=$R/.audit LONGOPS_PODMAN=$FXN/podman PODJSON=$FXN/pods.json PODLOG=$FXN/podlog
  export ANTIMESS_ROOT=$R ANTIMESS_OWNED_ORGS=fixorg AM_EXC=$FXN/exc.tsv ANTIMESS_LOCK_MIN_AGE=60 ANTIMESS_ORPHAN_AGE_S=300
  J=$FXN/out.json
  mkdir -p "$FXN/ap/scripts/repo"; printf 'resume_ttl=60\n' >"$FXN/ap/scripts/repo/commit_push.conf"; export CPA_APPROVED_DIR=$FXN/ap   # the approved copy of the CPA run the sweep serves
}
sw() { bash "$SW" --json "$J" "$@" >"$FXN/sw.out" 2>"$FXN/sw.err"; SWRC=$?; }
st() { jq -r --arg i "$1" '.invariants[]|select(.id==$i)|.status' "$J" 2>/dev/null; }
cls() { jq -r --arg i "$1" '.invariants[]|select(.id==$i)|.findings[]|select(.severity=="drift")|.class' "$J" 2>/dev/null | sort | tr '\n' ' ' | sed 's/ $//'; }
infocls() { jq -r --arg i "$1" '.invariants[]|select(.id==$i)|.findings[]|select(.severity=="info")|.class' "$J" 2>/dev/null | sort | tr '\n' ' ' | sed 's/ $//'; }
needle() { jq -r --arg i "$1" '.invariants[]|select(.id==$i)|.control_needle' "$J" 2>/dev/null; }
ALL=AM-R1,AM-R2,AM-R5,AM-G2,AM-P1,AM-P2,AM-P3,AM-P4,INV-9
tree_hash() { ( cd "$1" && find . -path ./.git -prune -o -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum | cut -c1-64; git -C "$1" status --porcelain | sha256sum | cut -c1-64 ); }
# a podman stand-in with STATE and label filtering (sweep side): $PODDIR/ps.json is the container list (podman JSON), `ps --filter label=K=V` keeps the containers whose Labels hold K=V (label=project=... lists all),
# `--format {{.ID}}` prints the 12-character ids, `stop <id>` removes the container from the list; every call is logged to $PODDIR/calls
cat >"$FX/podman-state" <<'PE'
#!/usr/bin/env bash
echo "$*" >>"$PODDIR/calls"
exec python3 -I - "$PODDIR" "$@" <<'PY'
import json, sys
d = sys.argv[1]; args = sys.argv[2:]
ps = json.load(open(d + "/ps.json"))
if args and args[0] == "ps":
    flt = None; fmt = ""
    for i, a in enumerate(args):
        if a.startswith("label="): flt = a[6:]
        if a == "--format" and i + 1 < len(args): fmt = args[i + 1]
    if flt and not flt.startswith("project="):
        k, _, v = flt.partition("=")
        ps = [c for c in ps if (c.get("Labels") or {}).get(k) == v]
    if "{{.ID}}" in fmt: print("\n".join(c["Id"][:12] for c in ps))
    else: print(json.dumps(ps))
elif args and args[0] == "stop":
    cid = args[-1]
    open(d + "/ps.json", "w").write(json.dumps([c for c in ps if c["Id"][:12] != cid]))
PY
PE
chmod +x "$FX/podman-state"
