#!/usr/bin/env bash
# test_check_doc_reachability.sh - executing test + paired mutation for scripts/docs/check_doc_reachability.py.
# Fixtures are generated under a mktemp root (no .git there, so the gate walks the directory). Exit 0 = all pass.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GATE="$HERE/../docs/check_doc_reachability.py"
T="$(mktemp -d)" || exit 2
: "${T:?}"
trap 'rm -rf -- "${T:?}"' EXIT
pass=0; fail=0
ok() { pass=$((pass+1)); }
no() { fail=$((fail+1)); echo "FAIL: $1"; }
mk() { # mk NAME  -> fresh fixture dir with README -> docs/README -> a.md
  local d="$T/$1"; mkdir -p "$d/docs/sub" "$d/docs/issues"
  printf '# Root\n[docs](docs/README.md)\n' >"$d/README.md"
  printf '# Docs\n[a](a.md)\n[sub](sub/)\n[tickets](issues/)\n' >"$d/docs/README.md"
  printf '# A\n' >"$d/docs/a.md"
  printf '# Sub\n[b](b.md)\n' >"$d/docs/sub/README.md"
  printf '# B\n' >"$d/docs/sub/b.md"
  printf '# T1\n' >"$d/docs/issues/T-1.md"
  echo "$d"
}
expect() { # expect LABEL WANT_EXIT gate args...
  local label="$1" want="$2" g="$3"; shift 3
  python3 "$g" "$@" >/dev/null 2>&1; local rc=$?
  [ "$rc" -eq "$want" ] && ok || no "$label (want $want got $rc)"
}
suite() { # suite GATE ; prints nothing, updates counters
  local g="$1" d
  d=$(mk good); expect "golden-good all reachable" 0 "$g" --root "$d"
  d=$(mk bad); printf '# Orphan\n' >"$d/docs/orphan.md"; : >"$T/empty.base"
  expect "golden-bad new orphan fails" 1 "$g" --root "$d" --baseline "$T/empty.base"
  printf 'docs/orphan.md\n' >"$T/orphan.base"
  expect "baselined orphan tolerated" 0 "$g" --root "$d" --baseline "$T/orphan.base"
  expect "strict ignores baseline" 1 "$g" --root "$d" --baseline "$T/orphan.base" --strict
  d=$(mk swap); printf '# O2\n' >"$d/docs/other.md"
  expect "swapped orphan (not in baseline) fails" 1 "$g" --root "$d" --baseline "$T/orphan.base"
  d=$(mk tracker); printf '# T2\n' >"$d/docs/issues/T-2.md"; expect "tracker root covers new ticket" 0 "$g" --root "$d"
  d=$(mk noreadme); rm -f "$d/README.md"; expect "BLIND no README" 2 "$g" --root "$d"
  d=$(mk nolinks); printf '# Root\nno links\n' >"$d/README.md"; expect "BLIND no links" 2 "$g" --root "$d"
  d=$(mk noscope); rm -rf "$d/docs"; expect "BLIND empty scope" 2 "$g" --root "$d"
  d=$(mk fence); printf '# Root\n```\n[docs](docs/README.md)\n```\n[x](docs/a.md)\n' >"$d/README.md"
  expect "link inside code fence ignored (README reaches only a.md, docs/README is extra root)" 0 "$g" --root "$d"
  d=$(mk entry); printf '# Root\n[x](docs/a.md)\n' >"$d/README.md"; expect "strict fails on missing entry link" 1 "$g" --root "$d" --strict
  d=$(mk hidden); printf '# Root\n[docs](docs/README.md)\n' >"$d/README.md"; printf '# O\n' >"$d/docs/h1.md"; printf '# O\n' >"$d/docs/h2.md"; printf '# O\n' >"$d/docs/h3.md"; printf '# O\n' >"$d/docs/h4.md"; printf '# O\n' >"$d/docs/h5.md"
  printf '# Docs\n[a](a.md)\n[sub](sub/)\n[tickets](issues/)\n<!-- [x](h1.md) -->\n`[x](h2.md)`\n~~~\n[x](h3.md)\n~~~\n\n    [x](h4.md)\n\n```\n[x](h5.md)\n```\n' >"$d/docs/README.md"
  : >"$T/empty.base"; expect "hidden links (comment/code/~~~/indent/fence) do not count" 1 "$g" --root "$d" --baseline "$T/empty.base"
  python3 "$g" --root "$d" --baseline "$T/empty.base" --list 2>/dev/null | grep -c '^NEW docs/h[1-5].md' | grep -qx 5 && ok || no "hidden: all five hidden-link docs must be orphans"
  d=$(mk refs); printf '# Docs\n[a](a.md)\n[sub](sub/)\n[tickets](issues/)\n[x][r]\n<a href="ah.md">z</a>\n\n[r]: refd.md\n' >"$d/docs/README.md"; printf '# R\n' >"$d/docs/refd.md"; printf '# A\n' >"$d/docs/ah.md"
  expect "reference-style and <a href> links count" 0 "$g" --root "$d"
  d=$(mk usage); expect "usage error" 2 "$g" --root "$d" --bogus
}

gitsuite() { # git-mode checks; GATE arg
  local g="$1" d
  d=$(mk git1); printf '# Orphan\n' >"$d/docs/orph.md"
  ( cd "$d" && git init -q && git add -A && git -c user.name=t -c user.email=t@t commit -qm init ) >/dev/null 2>&1
  printf '# Untracked\n' >"$d/docs/untracked.md"
  : >"$T/empty.base"
  python3 "$g" --root "$d" --baseline "$T/empty.base" --list 2>/dev/null | grep -q 'docs/untracked.md' && no "untracked file must not be in scope" || ok
  python3 "$g" --root "$d" --baseline "$T/empty.base" --list 2>/dev/null | grep -q 'NEW docs/orph.md' && ok || no "tracked orphan must be reported in git mode"
  expect "git mode: tracked orphan fails" 1 "$g" --root "$d" --baseline "$T/empty.base"
  # baseline committed empty, then grown by an uncommitted edit -> must FAIL even though the orphan is now 'tolerated'
  rm -f "$d/docs/untracked.md"; mkdir -p "$d/b"; printf '# header\n' >"$d/b/base.txt"
  ( cd "$d" && git add -A && git -c user.name=t -c user.email=t@t commit -qm base ) >/dev/null 2>&1
  printf 'docs/orph.md\n' >>"$d/b/base.txt"
  expect "baseline grown vs HEAD fails" 1 "$g" --root "$d" --baseline "$d/b/base.txt"
  # update-baseline refuses to add
  expect "update-baseline refuses to grow" 1 "$g" --root "$d" --baseline "$T/empty.base" --update-baseline
  # shrink is accepted and keeps header
  printf '# header\ndocs/orph.md\ndocs/gone.md\n' >"$d/b/base.txt"
  ( cd "$d" && git add -A && git -c user.name=t -c user.email=t@t commit -qm two ) >/dev/null 2>&1
  python3 "$g" --root "$d" --baseline "$d/b/base.txt" --update-baseline >"$T/upd.out" 2>&1; rc=$?
  { [ "$rc" -eq 0 ] && head -1 "$d/b/base.txt" | grep -qx '# header' && ! grep -q gone.md "$d/b/base.txt" && grep -q orph.md "$d/b/base.txt"; } && ok || no "update-baseline shrink keeps header (rc=$rc: $(tr "\n" " " <"$T/upd.out"))"
  expect "shrunk baseline accepted vs HEAD" 0 "$g" --root "$d" --baseline "$d/b/base.txt"
}
suite "$GATE"; gitsuite "$GATE"
base_fail=$fail
# --- paired mutations: each mutant of the gate MUST make the suite fail ---
mutant() { # mutant NAME 'python-replace-old' 'new'
  local m="$T/mut_$1.py"
  python3 - "$GATE" "$m" "$2" "$3" <<'PY'
import sys
s=open(sys.argv[1]).read()
assert sys.argv[3] in s, "mutation anchor missing: "+sys.argv[3]
open(sys.argv[2],'w').write(s.replace(sys.argv[3],sys.argv[4],1))
PY
  local save_pass=$pass save_fail=$fail
  fail=0; suite "$m" >/dev/null 2>&1; gitsuite "$m" >/dev/null 2>&1; local mf=$fail
  pass=$save_pass; fail=$save_fail
  if [ "$mf" -gt 0 ]; then ok; echo "mutation $1 killed ($mf suite checks failed against it)"; else no "mutation $1 survived"; fi
}
mutant orphan_detection 'orph = [f for f in scope if f not in seen]' 'orph = []'
mutant growth_check 'grown = sorted(base - read_entries(r.stdout))' 'grown = []'
mutant comment_strip 'text = re.sub(r"<!--.*?-->", "", text, flags=re.S)' 'pass'
mutant fence_tilde 'if m and m.group(1)[0] == fence[0]' 'if False'
mutant refuse_update 'if new and os.path.exists(baseline):' 'if False:'
mutant refs 'defs[d.group(1).strip().lower()] = d.group(2)' 'pass'
echo "test_check_doc_reachability: pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
