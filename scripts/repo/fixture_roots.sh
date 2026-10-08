#!/bin/bash -p
# T040a reader: fixture_roots.sh - reads scripts/repo/fixture_roots.txt, the deliberate-violation fixture roots.
#
# File    rows `root<TAB>exempt checks (comma list, closed check set)<TAB>reason<TAB>adding task`; `#` lines are comments.
# Usage   fixture_roots.sh [--file F] exempt <path> <check>   exit 0 + `exempt`, or exit 1 + `not_exempt`
#         fixture_roots.sh [--file F] filter <check>          paths on stdin -> the paths the check is still given (stdout)
# Rule    a path under a listed root is exempt ONLY from the checks that root's row names; a file outside the root, a sibling
#         that merely shares the root's name prefix, and a check the row does not name are never exempt. A malformed row, a
#         check outside the closed set, a glob or `..` or absolute root, a missing reason: exit 20 (never skipped).
# Note    class resolution (fixture roots as class `fixtures`) lives in check_class.sh (T040b); this reader owns the root list.
set -u
D="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PYSRC="$(cat <<'PY'
import sys, os
D = sys.argv[1]; a = sys.argv[2:]
CLOSED = set("secret_fold private_key merge_conflict trailing_whitespace end_of_file check_yaml check_json large_file shell_parse revision_header anti_bluff check_pins bank_validator go_fmt go_vet go_imports no_false_positive_log eslint prettier".split())
def die(reason, msg=''):
    sys.stderr.write(f"{reason} {msg}\n"); sys.exit(20)
f = os.path.join(D, 'fixture_roots.txt')
if a[:1] == ['--file']: f = a[1]; a = a[2:]
if not a or a[0] not in ('exempt', 'filter'): die('usage', 'exempt <path> <check> | filter <check>')
cmd, args = a[0], a[1:]
if len(args) != (2 if cmd == 'exempt' else 1): die('usage', cmd)
check = args[-1]
if check not in CLOSED: die('check_unknown', check)
roots = []
try: fh = open(f)
except OSError: die('file_unreadable', f)
for ln in fh:
    ln = ln.rstrip('\n')
    if not ln.strip() or ln.startswith('#'): continue
    c = ln.split('\t')
    if len(c) < 4 or not c[0].strip() or not c[1].strip() or not c[2].strip() or not c[3].strip(): die('row_invalid', ln[:60])
    root = c[0].strip().rstrip('/')
    if not root or root.startswith('/') or '..' in root.split('/') or any(ch in root for ch in '*?['): die('root_invalid', c[0])
    names = [x.strip() for x in c[1].split(',') if x.strip()]
    if not names: die('row_invalid', 'no check named: ' + root)
    for n in names:
        if n not in CLOSED: die('check_unknown', f"root {root} names {n}")
    roots.append((root, set(names)))
def exempt(p):
    p = os.path.normpath(p)
    return any((p == r or p.startswith(r + '/')) and check in names for r, names in roots)
if cmd == 'exempt':
    e = exempt(args[0]); print('exempt' if e else 'not_exempt'); sys.exit(0 if e else 1)
for ln in sys.stdin:
    p = ln.rstrip('\n')
    if p and not exempt(p): print(p)
PY
)"
exec python3 -I -c "$PYSRC" "$D" "$@"
