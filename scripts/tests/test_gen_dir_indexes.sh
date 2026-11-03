#!/usr/bin/env bash
# test_gen_dir_indexes.sh - executing test + paired mutation for scripts/docs/gen_dir_indexes.py. Exit 0 = all pass.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GEN="$HERE/../docs/gen_dir_indexes.py"
T="$(mktemp -d)" || exit 2
: "${T:?}"
trap 'rm -rf -- "${T:?}"' EXIT
pass=0; fail=0
ok() { pass=$((pass+1)); }
no() { fail=$((fail+1)); echo "FAIL: $1"; }
R="$T/r"; mkdir -p "$R/docs/m/sub" "$R/scripts/docs"
printf '# Guide One\n\nThis is the first real prose line of the guide, long enough.\n' >"$R/docs/m/a.md"
printf '# Bad **Title\n\nPoorly `formed unclosed **bold and tick prose here.\n' >"$R/docs/m/b.md"
printf '{}' >"$R/docs/m/sub/data.json"
printf '# Hand curated\n' >"$R/docs/m/README.md"
printf 'docs/m\tM index\tIntro text.\t../README.md\n' >"$R/scripts/docs/dir_indexes.tsv"
printf 'docs/m/b.md\tOverridden description\n' >"$R/scripts/docs/index_description_overrides.tsv"
python3 "$GEN" --root "$R" >/dev/null 2>&1 && ok || no "generator runs"
O="$R/docs/m/README.md"
grep -q '| \[a.md\](a.md) | Guide One - This is the first real prose line' "$O" && ok || no "md description = heading + first prose line"
grep -q '| \[b.md\](b.md) | Overridden description |' "$O" && ok || no "override applied"
grep -q 'sub/data.json.*JSON data artefact; not read' "$O" && ok || no "non-md listed by type, not read"
grep -q 'Hand curated' "$O" && no "old README content must be replaced" || ok
grep -q '\*\*' "$O" && no "no unclosed ** may leak" || ok
grep -q ' 3 entries' "$O" && ok || no "entry count excludes README itself"
cp "$O" "$T/first"; python3 "$GEN" --root "$R" >/dev/null 2>&1; cmp -s "$O" "$T/first" && ok || no "idempotent"
python3 "$GEN" --root "$R" --check >/dev/null 2>&1 && ok || no "--check clean after generate"
printf '# C\n\nAnother prose line that is long enough.\n' >"$R/docs/m/c.md"
python3 "$GEN" --root "$R" --check >/dev/null 2>&1; [ $? -eq 1 ] && ok || no "--check detects drift when a file is added"
# paired mutation: drop override support -> test must fail
python3 - "$GEN" "$T/mut.py" <<'PY'
import sys
s=open(sys.argv[1]).read(); a="if f in over: txt = over[f]"; assert a in s
open(sys.argv[2],'w').write(s.replace(a,"if False: txt = ''"))
PY
python3 "$T/mut.py" --root "$R" >/dev/null 2>&1
grep -q 'Overridden description' "$O" && no "mutation survived (override ignored but still asserted)" || ok
echo "test_gen_dir_indexes: pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
