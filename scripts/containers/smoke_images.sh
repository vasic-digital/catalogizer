#!/usr/bin/env bash
# smoke_images.sh - T008 (docs/16 section 6.3). Image smoke tests for the four WP-09 images through run_pinned.sh and the tracked lock.
# Usage: smoke_images.sh [EV_WP09_DIR]   (default: specs/001-full-project-audit-remediation/evidence/wp09)
# Writes smoke-IMG-GO.json, smoke-IMG-SHELLCHECK.json, smoke-IMG-KCOV.json, smoke-IMG-TESTUTIL.json and tool-image-map.json.
# Exit 0 only when every (test, image) pair of the map has every tool it needs, in that image, and every control needle behaved
# (absent needle, errored needle, read-only write, cache write, container uid == host uid).
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; ROOT="$(cd "$HERE/../.." && pwd)"; cd "$ROOT" || exit 2
EVD="${1:-specs/001-full-project-audit-remediation/evidence/wp09}"; mkdir -p "$EVD/smoke-out"; EVABS="$(cd "$EVD" && pwd)"
RUNP="$HERE/run_pinned.sh"; LOCK="${RUNP_LOCK:-$ROOT/build/containers/images.lock.yaml}"
FAIL=0; note() { echo "smoke: $*"; }
lockdig() { python3 -c 'import yaml,sys;print(next(i["reference"]+"@"+i["digest"] for i in yaml.safe_load(open(sys.argv[1]))["images"] if i["id"]==sys.argv[2]))' "$LOCK" "$1"; }
for img in IMG-GO IMG-KCOV IMG-TESTUTIL; do
  out="$EVABS/smoke-out/$img"; rm -rf "$out" "$EVABS/smoke-$img.json"; mkdir -p "$out"   # a stale per-image file must never feed the map of a failed run
  ref="$(lockdig "$img")"; insp="$(podman image inspect --format '{{.Digest}} {{.Id}}' -- "$ref" 2>&1)"
  if ! "$RUNP" --out "$out" --op-id "smoke-$img" "$img" -- bash /src/scripts/containers/smoke_probe.sh "$img" >"$out/stdout.txt" 2>"$out/stderr.txt"; then note "$img run FAILED: $(head -3 "$out/stderr.txt")"; FAIL=1; continue; fi
  python3 - "$img" "$ref" "$insp" "$out/toolchain.json" >"$EVABS/smoke-$img.json" <<'PY'
import sys, json
img, ref, insp, tc = sys.argv[1:5]
print(json.dumps({"schema": 1, "image_id": img, "reference": ref, "inspect_digest_and_id": insp, "toolchain": json.load(open(tc))}, indent=2))
PY
done
# IMG-SHELLCHECK has no shell: probes run from the host through RUNP (entrypoint_override), with a defective and a clean script as control needles.
img=IMG-SHELLCHECK; out="$EVABS/smoke-out/$img"; rm -rf "$out" "$EVABS/smoke-$img.json"; mkdir -p "$out"; ref="$(lockdig "$img")"; insp="$(podman image inspect --format '{{.Digest}} {{.Id}}' -- "$ref" 2>&1)"
mkdir -p .audit/scratch; printf '#!/bin/sh\necho $x\n' >.audit/scratch/smoke-sc-bad.sh; printf '#!/bin/sh\necho "ok"\n' >.audit/scratch/smoke-sc-clean.sh
"$RUNP" --out "$out" --op-id smoke-sc-ver "$img" -- shellcheck --version >"$out/version.txt" 2>"$out/ver.err"; vrc=$?
"$RUNP" --out "$out" --op-id smoke-sc-bad "$img" -- shellcheck .audit/scratch/smoke-sc-bad.sh >"$out/bad.txt" 2>&1; brc=$?
"$RUNP" --out "$out" --op-id smoke-sc-clean "$img" -- shellcheck .audit/scratch/smoke-sc-clean.sh >"$out/clean.txt" 2>&1; crc=$?
rm -f .audit/scratch/smoke-sc-bad.sh .audit/scratch/smoke-sc-clean.sh
python3 - "$img" "$ref" "$insp" "$vrc" "$brc" "$crc" "$out/version.txt" >"$EVABS/smoke-$img.json" <<'PY'
import sys, json
img, ref, insp, vrc, brc, crc, vf = sys.argv[1:8]
v = open(vf).read().strip().splitlines()
json.dump({"schema": 1, "image_id": img, "reference": ref, "inspect_digest_and_id": insp,
  "toolchain": {"tools": {"shellcheck": {"status": "present" if vrc == "0" else "error", "version": " ".join(v[:3])}},
   "control_needle_defective_script_exit": int(brc), "control_needle_expected": "non-zero", "clean_script_exit": int(crc),
   "shell_probes": "not_applicable: built FROM scratch, no shell (entrypoint_override)", "cache_write_probe": "not_applicable_no_shell", "ro_mount_write_probe": "not_applicable_no_shell"}}, sys.stdout, indent=2)
PY
[ "$vrc" = 0 ] && [ "$brc" != 0 ] && [ "$crc" = 0 ] || { note "IMG-SHELLCHECK probe or control needle failed (ver=$vrc bad=$brc clean=$crc)"; FAIL=1; }
# the per-(test, image) pair map: every pair needs its tools in THAT image
python3 - "$EVABS" >"$EVABS/tool-image-map.json" <<'PY' || FAIL=1
import json, sys
ev = sys.argv[1]
import os
def load(i):
    try: return json.load(open(f"{ev}/smoke-{i}.json"))["toolchain"]
    except Exception: return {}   # a failed or missing run leaves no per-image file: every pair needing it is not ok
tc = {i: load(i) for i in ("IMG-GO", "IMG-SHELLCHECK", "IMG-KCOV", "IMG-TESTUTIL")}
def has(img, tool):
    t = tc[img].get("tools", {}).get(tool, {})
    return t.get("status") == "present"
def imp(img, key):
    return tc[img].get("python_imports", {}).get(key) == "present"
PAIRS = [
 ("T011,T016,T017,T018 (bash python3 jq, PyYAML)", "IMG-TESTUTIL", ["bash", "python3", "jq"], ["pytest_yaml_jsonschema"]),
 ("T033 (bash python3 jq, jsonschema)", "IMG-TESTUTIL", ["bash", "python3", "jq"], ["pytest_yaml_jsonschema"]),
 ("T031,T039,T044 (kcov git bash jq script realpath)", "IMG-KCOV", ["kcov", "git", "bash", "jq", "script", "realpath"], []),
 ("T071,T060-T063,T064+ locked.sh (sqlite3 bash git)", "IMG-TESTUTIL", ["sqlite3", "bash", "git"], []),
 ("T075 (python3)", "IMG-TESTUTIL", ["python3"], []),
 ("CPA S2/S3 T040,T040a secrets and hooks", "IMG-TESTUTIL", ["detect-secrets", "detect-private-key", "check-yaml", "bash", "python3"], ["cpa_tools_identify"]),
 ("CPA go_fmt", "IMG-GO", ["gofmt"], []),
 ("lint (shellcheck)", "IMG-SHELLCHECK", ["shellcheck"], []),
]
rows, ok = [], True
for test, img, tools, imps in PAIRS:
    miss = [t for t in tools if not has(img, t)] + [i for i in imps if not imp(img, i)]
    rows.append({"test": test, "image": img, "tools": tools, "imports": imps, "missing": miss, "ok": not miss}); ok = ok and not miss
# KCOV must also give the P0 interpreter tools of its base (python imports) - recorded as extra pair
for img in ("IMG-KCOV",):
    rows.append({"test": "T039 harness python imports in IMG-KCOV", "image": img, "imports": ["pytest_yaml_jsonschema"], "missing": [] if imp(img, "pytest_yaml_jsonschema") else ["pytest_yaml_jsonschema"], "ok": imp(img, "pytest_yaml_jsonschema")})
    ok = ok and imp(img, "pytest_yaml_jsonschema")
# control needles
for img in ("IMG-GO", "IMG-KCOV", "IMG-TESTUTIL"):
    t = tc[img]; tl = t.get("tools", {})
    c1 = tl.get("definitely-not-a-tool", {}).get("status") == "absent"; c2 = t.get("ro_mount_write_probe") == "failed"; c3 = t.get("cache_write_probe") == "ok"
    c4 = tl.get("errored-needle", {}).get("status") == "error"   # a tool that exits non-zero must be recorded as error, never present
    c5 = t.get("uid") == os.getuid()                              # the container ran as the mapped host user, not the image's root
    rows.append({"test": f"control needles {img}", "image": img, "absent_needle": c1, "ro_write_failed": c2, "cache_write_ok": c3, "errored_needle_is_error": c4, "uid_is_host_uid": c5, "host_uid": os.getuid(), "ok": c1 and c2 and c3 and c4 and c5}); ok = ok and c1 and c2 and c3 and c4 and c5
json.dump({"schema": 1, "all_ok": ok, "pairs": rows}, sys.stdout, indent=2)
sys.exit(0 if ok else 1)
PY
[ "$FAIL" = 0 ] && note "ALL OK" || note "FAILED"
exit "$FAIL"
