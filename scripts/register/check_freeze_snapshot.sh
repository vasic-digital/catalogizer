#!/usr/bin/env bash
# check_freeze_snapshot.sh <snapshot dir> <manifest> - re-hash a frozen snapshot against its manifest (WP-20, T161 rule).
# STAND-IN NOTICE (UNCONFIRMED vs T161): T161 owns this file; written here by the WP-20 test worker so the
# consumers (enumerate_sources.py, T162/T164) can run. Refuses with `freeze_snapshot_moved` naming each path
# added, missing or changed (exit 20), with `freeze_manifest_invalid` for an unreadable or malformed manifest (exit 20); a special file in the
# snapshot is `freeze_special_file` (exit 20, from snapshot_manifest.py); exit 2 usage; 0 = the snapshot equals its manifest.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[ $# -eq 2 ] && [ -d "$1" ] && [ -f "$2" ] || { echo "usage: check_freeze_snapshot.sh <snapshot dir> <manifest>" >&2; exit 2; }
exec python3 - "$1" "$2" "$HERE/snapshot_manifest.py" <<'PY'
import importlib.util, json, sys
snap, mf, modp = sys.argv[1:4]
spec = importlib.util.spec_from_file_location("snapshot_manifest", modp)
m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
try:
    want = {e["path"]: e["sha256"] for e in json.load(open(mf, encoding="utf-8"))["entries"]}
except (OSError, ValueError, KeyError, TypeError) as e:     # a malformed manifest is a refusal, never a traceback (round-23 review F3)
    sys.stderr.write("check_freeze_snapshot: REFUSED reason=freeze_manifest_invalid %s: %s\n" % (mf, type(e).__name__))
    sys.exit(20)
have = {e["path"]: e["sha256"] for e in m.manifest(snap)["entries"]}
bad = [("added", p) for p in sorted(set(have) - set(want))] + [("missing", p) for p in sorted(set(want) - set(have))] \
    + [("changed", p) for p in sorted(p for p in set(want) & set(have) if want[p] != have[p])]
if bad:
    for k, p in bad:
        sys.stderr.write("check_freeze_snapshot: REFUSED reason=freeze_snapshot_moved %s %s\n" % (k, p))
    sys.exit(20)
PY
