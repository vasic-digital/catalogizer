#!/usr/bin/env bash
# T040b loader: check_class.sh - resolves a path to its check class and says which checks apply (plan owner's rule (V)).
#
# Usage   check_class.sh [--exemptions F] [--classes F] [--fixture-roots F] [--checks-registry F] <path>
#         check_class.sh ... --check class <path>        prints the class
#         check_class.sh ... --check <check> <path>      prints `apply`, `skip` or (large_file) the byte bound
#         Without --check: `class <c>` then one `<check> <apply|skip|bound>` line per check of the closed set.
#         Defaults: the tables next to this script (check_exemptions.tsv, check_classes.tsv, fixture_roots.txt).
# Resolve a path under a root of fixture_roots.txt is class `fixtures` (the root's row lists the checks it skips); else a row
#         naming the exact path (no glob character); else the glob rows (git-wildmatch: `*` inside one directory, `**/` zero
#         or more directories); a path no row matches is `source`, the strictest. Glob rows of two different classes that
#         match one path: exit 20 `class_ambiguous`, never decided by row order.
# Refuse  exit 20 `check_unknown <name>` when --checks-registry holds a row with scope `files`, a mode other than `deferred`,
#         whose check has no row for every class (never a default). Scope `changeset` rows are outside the class table.
#         Two exact-path rows of different classes for one path are `class_ambiguous` too (never decided by row order); a fixture-root row
#         needs the four columns fixture_roots.sh needs (`table_invalid` otherwise); an option without its value is `usage`.
# Exits   0; 20 on a refusal or an unreadable/invalid table (first stderr word names the reason).
set -u
D="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec python3 - "$D" "$@" <<'PY'
import sys, re, os
D = sys.argv[1]; a = sys.argv[2:]
ex = os.path.join(D, 'check_exemptions.tsv'); cl = os.path.join(D, 'check_classes.tsv'); fr = os.path.join(D, 'fixture_roots.txt')
reg = None; check = None; paths = []
CLOSED = "secret_fold private_key merge_conflict trailing_whitespace end_of_file check_yaml check_json large_file shell_parse revision_header anti_bluff check_pins bank_validator go_fmt go_vet go_imports no_false_positive_log eslint prettier".split()
def die(reason, msg=''):
    sys.stderr.write(f"{reason} {msg}\n"); sys.exit(20)
i = 0
while i < len(a):
    k = a[i]
    if k in ('--exemptions', '--classes', '--fixture-roots', '--checks-registry', '--check'):
        if i + 1 >= len(a): die('usage', f'{k} needs a value')
        v = a[i+1]; i += 2
        if k == '--exemptions': ex = v
        elif k == '--classes': cl = v
        elif k == '--fixture-roots': fr = v
        elif k == '--checks-registry': reg = v
        else: check = v
    elif k.startswith('--'): die('usage', k)
    else: paths.append(k); i += 1
if len(paths) != 1: die('usage', 'exactly one path is required')
def rows(f, n):
    out = []
    try: fh = open(f)
    except OSError as e: die('table_unreadable', f)
    for ln in fh:
        ln = ln.rstrip('\n')
        if not ln.strip() or ln.startswith('#'): continue
        c = ln.split('\t')
        if len(c) < n: die('table_invalid', f"{f}: {ln[:60]}")
        out.append(c)
    return out
def glob_re(g):
    r = ''; i = 0
    while i < len(g):
        if g.startswith('**/', i): r += '(?:.*/)?'; i += 3
        elif g.startswith('/**', i) and i + 3 == len(g): r += '/.*'; i += 3
        elif g.startswith('**', i): r += '.*'; i += 2
        elif g[i] == '*': r += '[^/]*'; i += 1
        elif g[i] == '?': r += '[^/]'; i += 1
        else: r += re.escape(g[i]); i += 1
    return re.compile('^' + r + '$')
path = os.path.normpath(paths[0]) if not paths[0].startswith('/') else paths[0]
exrows = rows(ex, 4); clrows = rows(cl, 4)
table = {}
for c in clrows: table[(c[0], c[1])] = c[2]
classes = sorted({c[0] for c in clrows})
# registry: every scope-files, non-deferred check needs a row for every class
if reg:
    for r in rows(reg, 6):
        if r[0] == 'check': continue
        name, mode, scope = r[0], r[3], r[5]
        if scope == 'files' and mode != 'deferred':
            miss = [k for k in classes if (k, name) not in table]
            if miss: die('check_unknown', f"{name} has no class row for: {','.join(miss)}")
cls = None; skipped = set()
for r in (rows(fr, 4) if os.path.exists(fr) and os.path.getsize(fr) else []):
    root = r[0].rstrip('/')
    if not root or root.startswith('/') or '..' in root.split('/') or any(ch in root for ch in '*?['): die('fixture_root_invalid', r[0])
    names = [x for x in r[1].split(',') if x]
    for n in names:
        if n not in CLOSED: die('check_unknown', f"fixture root {root} names {n}")
    if path == root or path.startswith(root + '/'): cls = 'fixtures'; skipped = set(names)
if cls is None:
    exact = [r for r in exrows if not any(ch in r[0] for ch in '*?[') and r[0] == path]
    if exact:
        ec = {r[1] for r in exact}
        if len(ec) > 1: die('class_ambiguous', f"{path} has exact rows of classes {','.join(sorted(ec))}")
        cls = exact[0][1]
    else:
        m = {r[1] for r in exrows if any(ch in r[0] for ch in '*?[') and glob_re(r[0]).match(path)}
        if len(m) > 1: die('class_ambiguous', f"{path} matches classes {','.join(sorted(m))}")
        cls = m.pop() if m else 'source'
def verdict(c):
    v = table.get((cls, c))
    if v is None: die('table_invalid', f"no row for class {cls} check {c}")
    if c in skipped and c != 'large_file': return 'skip'
    return 'apply' if v == 'yes' else 'skip' if v == 'no' else v
if check == 'class': print(cls)
elif check:
    if check not in CLOSED: die('check_unknown', check)
    print(verdict(check))
else:
    print('class', cls)
    for c in CLOSED: print(c, verdict(c))
PY
