#!/usr/bin/env bash
# test_containerfiles.sh - T106 (the build/containers/ tree, docs/16 DR-16-1). Static checks over every image directory:
#   C1 the tree holds every required image directory (go gotools node playwright docs rust android mut sigverify infra-client infra-postgres
#      infra-redis infra-ftp infra-smb infra-webdav; infra-minio is absent: no MinIO server image is obtainable from any official registry, T106 blocked item; plus the two T006 directories kcov and testutil);
#   C2 each directory has a non-empty Containerfile, README.md and digests.lock;
#   C3 the Containerfile passes scripts/containers/check_pins.sh (digest-pinned FROM, no pipe-to-shell; T105);
#   C4 every download line (curl or wget in a RUN) is followed, in the same RUN, by a SHA-256 check (sha256sum -c or shasum -a 256 -c);
#   C5 digests.lock names every FROM digest and every SHA-256 literal the Containerfile checks (the lock and the file cannot drift);
#   C6 the directory has a matching images.lock.yaml entry (rust and android are the two images whose entry T143 and T144 write), the
#      entry carries a `class` from {compile, interpreter, service, runtime, runtime-base} for the entries T106 writes, and when the
#      entry's reference is one of the Containerfile's FROM references the two digests are equal.
# The checker is the python block below, run on the real tree AND on deliberate-violation fixtures written to a temporary directory at run
# time (golden-bad: each rule must fire; golden-good: a compliant directory passes; negative control: a compliant directory next to a bad
# one is not blamed). Paired mutations: copies of THIS file with one rule weakened must FAIL on the fixtures (the real tree stays clean under
# them, so the fixtures are what catches a weakened checker). Mutation record: $CF_MUTATION_RECORD (default: scratch).
# Run: on the host (control plane) or through `scripts/containers/run_pinned.sh IMG-KCOV -- bash scripts/containers/tests/test_containerfiles.sh`.
# Env: CF_TEST_NO_MUTATIONS=1 skips the mutations; CF_TEST_MUTANT=1 marks a mutant run; CF_TEST_ROOT overrides the repository root (tests).
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="${CF_TEST_ROOT:-$(cd "$HERE/../../.." && pwd)}"
SELF="${CF_SELF:-${BASH_SOURCE[0]}}"
FAILS=0; PASSES=0
ok()  { PASSES=$((PASSES+1)); [ "${QUIET:-0}" = 1 ] || echo "PASS: $1"; }
bad() { FAILS=$((FAILS+1)); echo "FAIL: $1"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
command -v python3 >/dev/null 2>&1 || { echo "FAIL: python3 is required"; exit 2; }
T="$(mktemp -d "${TMPDIR:-/tmp}/cf-test.XXXXXX")"; trap 'rm -rf "$T"' EXIT

# the checker: prints one line per violation `<rule> <dir>: <message>`; exit 0 whatever it prints (the caller counts lines)
CHECKER="$T/checker.py"
cat >"$CHECKER" <<'PY'
import os, re, subprocess, sys
repo, tree, lockfile, pins = sys.argv[1:5]
required = sys.argv[5].split(",") if len(sys.argv) > 5 and sys.argv[5] else []
ID = {"go": "IMG-GO", "gotools": "IMG-GOTOOLS", "node": "IMG-NODE", "playwright": "IMG-PW", "docs": "IMG-DOCS", "rust": "IMG-RUST",
      "android": "IMG-ANDROID", "mut": "IMG-MUT", "sigverify": "IMG-SIGVERIFY", "infra-client": "IMG-INFRA-CLIENT",
      "infra-postgres": "IMG-INFRA-POSTGRES", "infra-redis": "IMG-INFRA-REDIS", "infra-minio": "IMG-INFRA-MINIO", "infra-ftp": "IMG-INFRA-FTP",
      "infra-smb": "IMG-INFRA-SMB", "infra-webdav": "IMG-INFRA-WEBDAV", "kcov": "IMG-KCOV", "testutil": "IMG-TESTUTIL"}
DEFERRED = {"rust", "android"}          # entries written by T143 and T144
NO_CLASS_CHECK = {"kcov", "testutil", "go"}   # entries written before the class field existed (T006 and T005d; T121b backfills them)
CLASSES = {"compile", "interpreter", "service", "runtime", "runtime-base"}
DIGEST = re.compile(r"@sha256:([0-9a-f]{64})(?![0-9A-Za-z])")
out = []
def v(rule, d, msg): out.append("%s %s: %s" % (rule, d, msg))

import yaml
entries = {}
if os.path.exists(lockfile):
    for e in (yaml.safe_load(open(lockfile)) or {}).get("images", []):
        entries.setdefault(e.get("id"), []).append(e)

for r in required:                                                   # C1
    if not os.path.isdir(os.path.join(tree, r)):
        v("C1-missing-directory", r, "required image directory is absent")

def logical(text):
    acc, cur = [], ""
    for raw in text.split("\n"):
        s = raw.rstrip()
        if not cur and s.lstrip().startswith("#"):
            continue
        if s.endswith("\\"):
            cur += " " + s[:-1].strip()
        else:
            cur += " " + s.strip(); acc.append(cur.strip()); cur = ""
    if cur: acc.append(cur.strip())
    return acc

for d in sorted(x for x in os.listdir(tree) if os.path.isdir(os.path.join(tree, x))):
    p = os.path.join(tree, d)
    cf = os.path.join(p, "Containerfile")
    if not os.path.isfile(cf) or os.path.getsize(cf) == 0:          # C2
        v("C2-containerfile", d, "Containerfile missing or empty"); continue
    for f in ("README.md", "digests.lock"):
        fp = os.path.join(p, f)
        if not os.path.isfile(fp) or os.path.getsize(fp) == 0:
            v("C2-" + f, d, f + " missing or empty")
    text = open(cf).read()
    r = subprocess.run(["bash", pins, "--root", tree, d], capture_output=True, text=True)   # C3
    if r.returncode != 0:
        for line in r.stdout.splitlines():
            if line.startswith("VIOLATION"): v("C3-check_pins", d, line)
        if r.returncode not in (0, 1): v("C3-check_pins", d, "check_pins exited %d" % r.returncode)
    shas = []
    for ln in logical(text):                                         # C4
        if re.match(r"^RUN\b", ln, re.I) and re.search(r"\b(curl|wget)\s+(-|https?://)", ln):
            m = re.search(r"\b(curl|wget)\b.*?(sha256sum\s+-c|shasum\s+-a\s+256\s+-c)", ln)   # MUT-ANCHOR c4-check
            if not m: v("C4-download-without-sha256", d, ln[:140])
            shas += re.findall(r"\b([0-9a-f]{64})\b", ln)
    froms = []
    args = {}
    for ln in logical(text):
        am = re.match(r"^ARG\s+(\w+)=(\S+)", ln, re.I)
        if am: args[am.group(1)] = am.group(2)
        fm = re.match(r"^FROM\s+(?:--\S+\s+)*(\S+)", ln, re.I)
        if fm:
            ref = fm.group(1)
            ref = re.sub(r"^\$\{?(\w+)\}?$", lambda mm: args.get(mm.group(1), ref), ref)
            if DIGEST.search(ref): froms.append(ref)
    if not froms and not any(re.match(r"^FROM\s+\$", ln, re.I) for ln in logical(text)):
        v("C2-from", d, "no FROM line")
    lockp = os.path.join(p, "digests.lock")
    locktxt = open(lockp).read() if os.path.isfile(lockp) else ""
    for ref in froms:                                                # C5
        if DIGEST.search(ref).group(1) not in locktxt:               # MUT-ANCHOR c5-from
            v("C5-digests.lock-from", d, "FROM digest of %s not in digests.lock" % ref.split("@")[0])
    for h in sorted(set(shas)):
        if h not in locktxt: v("C5-digests.lock-sha256", d, "sha256 %s... not in digests.lock" % h[:12])
    if d in ID:                                                      # C6
        es = entries.get(ID[d], [])
        if not es:
            if d not in DEFERRED: v("C6-lock-entry", d, "no images.lock.yaml entry %s" % ID[d])
        else:
            e = es[0]
            if d not in NO_CLASS_CHECK and e.get("class") not in CLASSES:     # MUT-ANCHOR c6-class
                v("C6-class", d, "entry %s class %r not in %s" % (ID[d], e.get("class"), sorted(CLASSES)))
            for ref in froms:
                base, dg = ref.split("@")
                if e.get("reference") == base and e.get("digest") != dg:
                    v("C6-digest-mismatch", d, "Containerfile FROM %s@%s but lock entry digest is %s" % (base, dg[:19], str(e.get("digest"))[:19]))
    else:
        v("C6-unknown-directory", d, "directory has no id mapping in the checker")
print("\n".join(out))
PY

# run_checker <tree> <lock> <required-list> -> OUT (lines)
run_checker() { OUT="$(python3 -I "$CHECKER" "$REPO" "$1" "$2" "$REPO/scripts/containers/check_pins.sh" "$3" 2>&1)"; }

# ---------------------------------------------------------------- the real tree
REQ="go,gotools,node,playwright,docs,rust,android,mut,sigverify,infra-client,infra-postgres,infra-redis,infra-ftp,infra-smb,infra-webdav,kcov,testutil"
if [ -d "$REPO/build/containers" ]; then run_checker "$REPO/build/containers" "$REPO/build/containers/images.lock.yaml" "$REQ"; else OUT="C1-missing-directory build/containers: the tree is absent"; fi
if [ -z "$OUT" ]; then ok "REAL build/containers tree: every required directory, file, pin, checksum and lock entry check passes"
else bad "REAL build/containers tree has $(printf '%s\n' "$OUT" | wc -l) violations: $(printf '%s' "$OUT" | head -6 | tr '\n' '|')"; fi

# ---------------------------------------------------------------- fixtures (golden-bad, golden-good, negative control)
D64="$(printf 'a%.0s' $(seq 64))"; D64B="$(printf 'b%.0s' $(seq 64))"; SH64="$(printf 'c%.0s' $(seq 64))"
mkfix() { # <name> : writes $T/fx-<name>/tree and $T/fx-<name>/lock.yaml; the caller adds directories
  rm -rf "$T/fx-$1"; mkdir -p "$T/fx-$1/tree"; printf 'schema: 1\nimages: []\n' >"$T/fx-$1/lock.yaml"; }
gooddir() { # <fxname> <dir> : a compliant directory
  local p="$T/fx-$1/tree/$2"; mkdir -p "$p"
  printf 'FROM docker.io/library/debian@sha256:%s\nRUN curl -fsSL -o /tmp/f https://example.invalid/f && echo "%s  /tmp/f" | sha256sum -c -\n' "$D64" "$SH64" >"$p/Containerfile"
  printf '# %s\n' "$2" >"$p/README.md"
  printf 'base docker.io/library/debian@sha256:%s\ndownload https://example.invalid/f sha256:%s\n' "$D64" "$SH64" >"$p/digests.lock"; }
addlock() { # <fxname> <id> <reference> <digest> [class]
  python3 -I - "$T/fx-$1/lock.yaml" "$2" "$3" "$4" "${5:-}" <<'PY'
import sys, yaml
f, i, ref, dg, cls = sys.argv[1:6]
d = yaml.safe_load(open(f)); e = {"id": i, "reference": ref, "digest": dg}
if cls: e["class"] = cls
d["images"].append(e); open(f, "w").write(yaml.safe_dump(d))
PY
}
fxcheck() { # <fxname> <required>  -> OUT
  OUT="$(python3 -I "$CHECKER" "$REPO" "$T/fx-$1/tree" "$T/fx-$1/lock.yaml" "$REPO/scripts/containers/check_pins.sh" "${2:-}" 2>&1)"; }
expect_rule() { # <label> <rule-prefix>
  if printf '%s\n' "$OUT" | grep -q "^$2"; then ok "$1 -> $2 fires"; else bad "$1: no '$2' line in: $(printf '%s' "$OUT" | head -4 | tr '\n' '|')"; fi; }

mkfix good; gooddir good go; addlock good IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck good go; check "GOLDEN-GOOD a compliant directory produces no violation" "$OUT" ""

mkfix tagfrom; gooddir tagfrom go; sed -i "1s#.*#FROM docker.io/library/debian:12#" "$T/fx-tagfrom/tree/go/Containerfile"; addlock tagfrom IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck tagfrom; expect_rule "GOLDEN-BAD tag-only FROM" C3-check_pins

mkfix pipe; gooddir pipe go; printf 'RUN curl -fsSL https://example.invalid/i.sh | sh\n' >>"$T/fx-pipe/tree/go/Containerfile"; addlock pipe IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck pipe; expect_rule "GOLDEN-BAD pipe-to-shell" C3-check_pins

mkfix nosha; gooddir nosha go; printf 'RUN curl -fsSL -o /tmp/g https://example.invalid/g && chmod +x /tmp/g\n' >>"$T/fx-nosha/tree/go/Containerfile"; addlock nosha IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck nosha; expect_rule "GOLDEN-BAD download without a SHA-256 check" C4-download-without-sha256

mkfix noreadme; gooddir noreadme go; rm "$T/fx-noreadme/tree/go/README.md"; addlock noreadme IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck noreadme; expect_rule "GOLDEN-BAD README.md missing" C2-README.md

mkfix nolock; gooddir nolock go; rm "$T/fx-nolock/tree/go/digests.lock"; addlock nolock IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck nolock; expect_rule "GOLDEN-BAD digests.lock missing" C2-digests.lock

mkfix drift; gooddir drift go; printf 'base docker.io/library/debian@sha256:%s\n' "$D64B" >"$T/fx-drift/tree/go/digests.lock"; addlock drift IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck drift; expect_rule "GOLDEN-BAD digests.lock does not name the FROM digest" C5-digests.lock-from
expect_rule "GOLDEN-BAD digests.lock does not name the checked SHA-256" C5-digests.lock-sha256

mkfix noentry; gooddir noentry go
fxcheck noentry; expect_rule "GOLDEN-BAD no images.lock.yaml entry" C6-lock-entry

mkfix noclass; gooddir noclass node; addlock noclass IMG-NODE docker.io/library/debian "sha256:$D64"
fxcheck noclass; expect_rule "GOLDEN-BAD lock entry without a class" C6-class

mkfix mismatch; gooddir mismatch go; addlock mismatch IMG-GO docker.io/library/debian "sha256:$D64B" compile
fxcheck mismatch; expect_rule "GOLDEN-BAD lock entry digest differs from the Containerfile FROM digest" C6-digest-mismatch

mkfix aptcurl; gooddir aptcurl go; printf 'RUN apt-get install -y --no-install-recommends curl=7.88.1 ca-certificates\n' >>"$T/fx-aptcurl/tree/go/Containerfile"; addlock aptcurl IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck aptcurl; check "GOLDEN-GOOD apt-get installing the curl package is not a download line" "$OUT" ""

mkfix deferred; gooddir deferred rust
fxcheck deferred; check "GOLDEN-GOOD rust needs no lock entry (T143 writes it)" "$OUT" ""

mkfix missing; gooddir missing go; addlock missing IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck missing "go,node"; expect_rule "GOLDEN-BAD required directory absent" C1-missing-directory

mkfix empty; mkdir -p "$T/fx-empty/tree/go"; : >"$T/fx-empty/tree/go/Containerfile"
fxcheck empty; expect_rule "GOLDEN-BAD empty Containerfile" C2-containerfile

# negative control: a compliant directory next to a bad one is not blamed
mkfix neg; gooddir neg go; gooddir neg node; sed -i "1s#.*#FROM docker.io/library/node:20#" "$T/fx-neg/tree/node/Containerfile"; addlock neg IMG-GO docker.io/library/debian "sha256:$D64" compile; addlock neg IMG-NODE docker.io/library/debian "sha256:$D64" compile
fxcheck neg; check "NEGCTL the compliant directory 'go' is never named" "$(printf '%s\n' "$OUT" | grep -c ' go:')" "0"
check "NEGCTL the bad directory 'node' is named" "$([ "$(printf '%s\n' "$OUT" | grep -c ' node:')" -ge 1 ] && echo yes || echo no)" "yes"

# ---------------------------------------------------------------- paired mutations of this file
if [ -z "${CF_TEST_MUTANT:-}" ] && [ -z "${CF_TEST_NO_MUTATIONS:-}" ]; then
  REC="${CF_MUTATION_RECORD:-$T/containerfiles-mutation.txt}"
  { echo "# containerfiles-mutation record: paired mutations of the checker in scripts/containers/tests/test_containerfiles.sh"; echo "# run_at: $(date -u +%Y-%m-%dT%H:%M:%SZ) host: $(hostname)"; echo "# self_sha256: $(sha256sum "$SELF" | cut -d' ' -f1)"; } >"$REC"
  nm=0
  while IFS=$'\t' read -r name old new; do
    case "$name" in ""|"#"*) continue;; esac
    nm=$((nm+1)); cp_="$T/mutant-$name.sh"
    if ! python3 -I - "$SELF" "$cp_" "$old" "$new" <<'PY'
import sys
s = open(sys.argv[1]).read(); old, new = sys.argv[3], sys.argv[4]
if old == new or s.count(old) != 1: print("anchor count %d" % s.count(old)); sys.exit(3)
open(sys.argv[2], "w").write(s.replace(old, new))
PY
    then bad "MUT $name: cannot be applied"; echo "MUTANT $name: NOT-APPLIED" >>"$REC"; continue; fi
    mo="$(CF_SELF="$cp_" CF_TEST_ROOT="$REPO" CF_TEST_MUTANT=1 QUIET=1 bash "$cp_" 2>&1)"; mrc=$?
    if [ "$mrc" -ne 0 ]; then ok "MUT $name: caught"; echo "MUTANT $name: CAUGHT first_fail=$(printf '%s' "$mo" | grep -m1 '^FAIL')" >>"$REC"
    else bad "MUT $name: SURVIVED"; echo "MUTANT $name: SURVIVED" >>"$REC"; fi
  done <"$HERE/containerfiles_mutations.tsv"
  echo "# mutations: $nm" >>"$REC"
fi

echo "test_containerfiles: $PASSES passed, $FAILS failed"
[ "$FAILS" -eq 0 ]
