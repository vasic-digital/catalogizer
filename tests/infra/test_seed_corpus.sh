#!/usr/bin/env bash
# test_seed_corpus.sh - T130 (RED first). Oracle for scripts/test-infra/seed_corpus.sh, the deterministic corpus seeder.
# The seeder and this test's seeder runs go through `TIC tooling unit` (scripts/test-in-container.sh, IMG-TESTUTIL), never the bare host.
# Oracle: INVARIANT (two runs with the same seed are byte-identical: diff -r, file mtimes and the corpus digest) and SPECIFIED (a different seed
# changes the digest: the control needle; file magic numbers and the unicode / long-path names are the declared contract of the corpus).
# Paired mutations: copies of the seeder with ONE load-bearing line changed; the same checks run against each copy and every copy must FAIL a check.
# Usage: test_seed_corpus.sh              tests and mutations
#        SEED_NO_MUTATIONS=1 ...          tests only
# Env:   SEED_SUT=<file>  seeder under test (used by the mutation runs); SEED_EV=<dir> where corpus.sha256 is written (default scratch)
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
SUT_REL="${SEED_SUT:-scripts/test-infra/seed_corpus.sh}"
SUT="$TI_REPO/$SUT_REL"
TIC=(ti_tic)
SEED=${SEED_VALUE:-catalogizer-wp13-seed-1}

if [ ! -f "$SUT" ]; then bad "seeder absent: $SUT_REL"; ti_summary; exit 1; fi

seed_run() { # seed_run <seed> <outdir-name>   -> sets RC, stdout in $TI_SCRATCH/<name>.out
  local seed=$1 name=$2
  rm -rf -- "${TI_REPO:?}/.audit/out/ti-seed-${name:?}"; mkdir -p "$TI_REPO/.audit/out/ti-seed-$name"
  # the container sees the repository read-only at /src and the out dir at /out
  "${TIC[@]}" --out "$TI_REPO/.audit/out/ti-seed-$name" tooling unit -- bash "/src/$SUT_REL" --seed "$seed" --out /out/corpus --checksum-file /out/corpus.sha256 >"$TI_SCRATCH/$name.out" 2>"$TI_SCRATCH/$name.err"; RC=$?
}
dir_of() { echo "$TI_REPO/.audit/out/ti-seed-$1"; }
cleanup_out() { rm -rf -- "${TI_REPO:?}"/.audit/out/ti-seed-*; }
trap 'cleanup_out; ti_cleanup' EXIT

seed_run "$SEED" a; check "run a exits 0" "$RC" 0
seed_run "$SEED" b; check "run b exits 0" "$RC" 0
A="$(dir_of a)"; B="$(dir_of b)"
if diff -r "$A/corpus" "$B/corpus" >/dev/null 2>&1; then ok "two runs: trees byte-identical (diff -r)"; else bad "two runs: trees differ"; fi
ma=$(cd "$A/corpus" && find . -printf '%P %T@ %y\n' | sort | sha256sum | cut -d' ' -f1); mb=$(cd "$B/corpus" && find . -printf '%P %T@ %y\n' | sort | sha256sum | cut -d' ' -f1)
check "two runs: names, mtimes and types identical" "$ma" "$mb"
da=$(cut -d' ' -f1 "$A/corpus.sha256" 2>/dev/null); db=$(cut -d' ' -f1 "$B/corpus.sha256" 2>/dev/null)
if [[ "$da" =~ ^[0-9a-f]{64}$ ]]; then ok "checksum file holds a sha256 digest"; else bad "checksum file digest malformed ('$da')"; fi
check "two runs: corpus digest identical" "$da" "$db"
# the digest is not a constant: it must be the digest of THIS tree (independent recomputation by python, a second implementation)
re=$(python3 -I - "$A/corpus" <<'PY'
import hashlib, os, sys, unicodedata
root = sys.argv[1]; lines = []
for d, ds, fs in os.walk(root):
    for n in ds:
        p = os.path.join(d, n); lines.append((unicodedata.normalize("NFC", os.path.relpath(p, root)), "dir", 0, ""))
    for n in fs:
        p = os.path.join(d, n); h = hashlib.sha256(open(p, "rb").read()).hexdigest()
        lines.append((unicodedata.normalize("NFC", os.path.relpath(p, root)), "file", os.path.getsize(p), h))
lines.sort()
m = hashlib.sha256()
for r, k, s, h in lines: m.update(("%s\0%s\0%d\0%s\n" % (r, k, s, h)).encode("utf-8"))
print(m.hexdigest())
PY
)
check "digest equals an independent recomputation over the tree" "$da" "$re"
# control needle: another seed gives another digest and another tree
seed_run "${SEED}-other" c; check "run c (other seed) exits 0" "$RC" 0
dc=$(cut -d' ' -f1 "$(dir_of c)/corpus.sha256" 2>/dev/null)
if [ -n "$dc" ] && [ "$dc" != "$da" ]; then ok "control needle: a changed seed changes the digest"; else bad "control needle: changed seed left the digest unchanged ('$dc' vs '$da')"; fi
# contract of the corpus: real file types, unicode names, a long path, empty directories
nfiles=$(find "$A/corpus" -type f | wc -l)
if [ "$nfiles" -ge 20 ]; then ok "corpus has >= 20 files ($nfiles)"; else bad "corpus has only $nfiles files"; fi
magic=$(python3 -I - "$A/corpus" <<'PY'
import os, sys
root = sys.argv[1]; want = {
 ".png": b"\x89PNG\r\n\x1a\n", ".jpg": b"\xff\xd8\xff", ".mp3": b"ID3", ".flac": b"fLaC", ".pdf": b"%PDF-", ".zip": b"PK\x03\x04",
 ".mkv": b"\x1a\x45\xdf\xa3", ".mp4": None, ".gif": b"GIF89a"}
seen = {}
for d, ds, fs in os.walk(root):
    for n in fs:
        ext = os.path.splitext(n)[1].lower()
        if ext in want:
            b = open(os.path.join(d, n), "rb").read(16)
            ok = (b[4:8] == b"ftyp") if ext == ".mp4" else b.startswith(want[ext])
            seen.setdefault(ext, []).append(ok)
good = sorted(e for e, v in seen.items() if v and all(v))
print(",".join(good))
PY
)
for e in .png .jpg .mp3 .flac .pdf .zip .mkv .mp4 .gif; do case ",$magic," in *",$e,"*) ok "file type $e has its magic number";; *) bad "file type $e missing or wrong magic";; esac; done
uni=$(find "$A/corpus" -type f | LC_ALL=C grep -c -P '[^\x00-\x7F]' || true)
if [ "${uni:-0}" -ge 3 ]; then ok "unicode file names present ($uni)"; else bad "unicode file names missing ($uni)"; fi
longest=$(cd "$A/corpus" && find . -type f | awk '{ if (length($0) > m) m = length($0) } END { print m+0 }')
if [ "$longest" -ge 200 ]; then ok "long path present ($longest chars)"; else bad "no long path ($longest chars)"; fi
if [ -n "$(find "$A/corpus" -type d -empty | head -1)" ]; then ok "an empty directory is present"; else bad "no empty directory"; fi
# the recorded evidence: corpus.sha256 of this run
if [ -n "${SEED_EV:-}" ] && [ "${SEED_TEST_MUTANT:-0}" != 1 ]; then mkdir -p "$SEED_EV"; { ti_identity "T130 corpus checksum (seed $SEED)" "$SUT_REL"; cat "$A/corpus.sha256"; } >"$SEED_EV/corpus.sha256"; fi

# ---------------- paired mutations ----------------
if [ "${SEED_NO_MUTATIONS:-0}" != 1 ] && [ "${SEED_TEST_MUTANT:-0}" != 1 ]; then
  MUTLOG="${SEED_MUTATION_RECORD:-$TI_SCRATCH/mutations.txt}"; : >"$MUTLOG"
  mutate() { # mutate <name> <python-regex-or-literal old> <new>   one occurrence, written to .audit/scratch
    local name=$1 old=$2 new=$3 dst="$TI_REPO/.audit/scratch/seedmut-$1.sh"
    python3 -I - "$SUT" "$dst" "$old" "$new" <<'PY' || return 1
import sys
s = open(sys.argv[1]).read(); old, new = sys.argv[3], sys.argv[4]
if s.count(old) != 1: print("mutation anchor count %d for %r" % (s.count(old), old)); sys.exit(1)
open(sys.argv[2], "w").write(s.replace(old, new))
PY
  }
  run_mut() { # run_mut <name>: the body of this test against the copy, expect a non-zero exit
    local name=$1 out rc
    out=$(SEED_SUT=".audit/scratch/seedmut-$name.sh" SEED_TEST_MUTANT=1 SEED_NO_MUTATIONS=1 QUIET=1 bash "${BASH_SOURCE[0]}" 2>&1); rc=$?
    if [ "$rc" -ne 0 ]; then ok "mutation $name CAUGHT ($(printf '%s\n' "$out" | grep -m1 '^FAIL' | cut -c1-90))"; echo "$name CAUGHT" >>"$MUTLOG"; else bad "mutation $name SURVIVED"; echo "$name SURVIVED" >>"$MUTLOG"; fi
    rm -f "$TI_REPO/.audit/scratch/seedmut-$name.sh"
  }
  if mutate nondeterministic 'SEED_ENTROPY=""' 'SEED_ENTROPY="$(date +%s%N)"'; then run_mut nondeterministic; else bad "mutation nondeterministic: anchor missing in the seeder"; fi
  if mutate seed_ignored 'SEED_FOR_STREAM="$SEED"' 'SEED_FOR_STREAM="fixed"'; then run_mut seed_ignored; else bad "mutation seed_ignored: anchor missing in the seeder"; fi
  if mutate no_unicode 'UNICODE_NAMES=1' 'UNICODE_NAMES=0'; then run_mut no_unicode; else bad "mutation no_unicode: anchor missing in the seeder"; fi
  if mutate no_longpath 'LONG_PATH=1' 'LONG_PATH=0'; then run_mut no_longpath; else bad "mutation no_longpath: anchor missing in the seeder"; fi
  if mutate bad_magic 'PNG_MAGIC=1' 'PNG_MAGIC=0'; then run_mut bad_magic; else bad "mutation bad_magic: anchor missing in the seeder"; fi
  if mutate digest_constant 'DIGEST_REAL=1' 'DIGEST_REAL=0'; then run_mut digest_constant; else bad "mutation digest_constant: anchor missing in the seeder"; fi
  [ -z "${SEED_EV:-}" ] || cp "$MUTLOG" "$SEED_EV/seed-mutations.txt" 2>/dev/null
fi
ti_summary
