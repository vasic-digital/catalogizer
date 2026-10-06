#!/usr/bin/env bash
# T040/T040b helper: validate_cheap.sh - CPA stage S3 (docs/16 section 12.2), the cheap checks of a declared change set.
#
# Usage   validate_cheap.sh --root <repo> --files-from <list> [--code-root <dir>] [--registry <tsv>] [--approved-registry <tsv>]
#                           [--run-declared <check>]... [--held-from <tsv>] [--out <dir>] [--trusted-tables <dir> | --adopt-working-tables]
#   --root              repository whose work tree is judged (never written)
#   --files-from        the declared change set, one path per line, relative to the root (a deleted path is allowed)
#   --code-root         where the commands of the registry live (default: the root); each command path is relative to it
#   --registry          check registry (default <code-root>/scripts/repo/validate_checks.tsv); columns in its header
#   --approved-registry the approved copy of the registry (CENTRAL C1); a row that differs from it, or that it lacks, is NOT run and is
#                       reported `check_pending_release` (a missing key runs only when named by --run-declared, i.e. when this run's
#                       change set declares the row held on a G-GATE verdict); without the option every row is approved
#   --held-from         rows `path<TAB>verdict path`: the declared paths held on that review verdict (held-table rule below)
#   --out               directory that receives report.tsv (the only thing written)
#   --trusted-tables    directory that holds the approved copy of the three class tables (the source of the helper's own tables, see HEAD tables)
#   --adopt-working-tables  the explicit, owner-approved adoption form: the helper's own working-tree tables are the reviewed tables
# Registry command forms: `<script> args...` (the script must be an executable file) or `<interpreter> <script> args...` (interpreter bash or
#         python3, the script only has to be readable: how a script runs is the registry's statement, not its file mode; WF2 review I2).
# Paths   every declared path (and every held-table row) is a LITERAL path (no `.`, `./` prefix, `/./`, trailing `/`, `* ? [`, leading `:`;
#         `unsafe_path`), a declared directory is `declared_directory` (a gitlink is allowed), git runs with GIT_LITERAL_PATHSPECS=1 (B1).
# HEAD tables  a HEAD that holds no class table is judged with the helper's own reviewed tables, which come from a TRUSTED source, never from the
#         helper's own working tree when it runs in place (WF3 review I-2): `--trusted-tables <dir>` (an approved copy); else, when the helper lives in
#         a git work tree, the code root's committed HEAD (`git show HEAD:<path>`; an untracked table or one that differs from it is refused 20
#         `class_table_unreviewed`); else (a released snapshot that is no repository) the tables next to the script; `--adopt-working-tables` is
#         the explicit owner-approved form for the adoption run, before the tables are committed.
# Checks  named rows of the registry, each given only the declared files whose class applies it (check_class.sh, T040b) after the
#         check's own filter (suffix / text-only, UNCONFIRMED against the hook manifests), check-only: the source is never written.
#         builtins: shell_parse merge_conflict trailing_whitespace end_of_file check_yaml check_json large_file; scripts: see the registry.
# Table rules (T040b)  the class tables scripts/repo/{check_classes.tsv,check_exemptions.tsv,fixture_roots.txt} are judged from the HEAD
#         the change set starts from; a declared change of one is used only under the review verdict its `# review: <path>` header
#         names: GO in HEAD or declared with GO in the change set => every path is judged with the declared tables; the verdict
#         declared (not GO) or named by a --held-from row => only the paths held on it use the declared tables, every other path
#         the HEAD tables, and an unheld path that fails a check under the HEAD tables which the declared tables would not apply or
#         would pass is refused 20 `table_admits_unheld_path`; otherwise HEAD tables and `class_table_unreviewed` (10) for the change.
#         `legacy_row_not_dropped` (10): a declared legacy root report (exact-path row of class legacy-collection) whose content
#         differs from HEAD while the tables in force keep its row.
# Output  stdout, one tab-separated line per finding:  fail <check> <path> | deferred <check> | check_pending_release <check> |
#         left_out <check> <path> | size_alarm <path> <size> <bound> | class_table_unreviewed <table> | legacy_row_not_dropped <path> |
#         table_admits_unheld_path <path> <check> | symlink_not_judged <path> (a declared symlink of a non-evidence class: reported, never skipped silently)
#         A declared symlink in class evidence or evidence-ledger, or a regular file whose HEAD entry is a symlink there, is `fail symlink <path>` (10):
#         it is judged by its link type, never followed (WF3 review B-1).
# Exits   0 all ran checks pass; 10 a check failed or a table rule refused; 14 only pending rows (deferral class 14); 20 refusal
#         (unsafe value, registry invalid, command absent, check_unknown, ratchet_not_implemented, table_admits_unheld_path).
#         Precedence: 20 over 10 over 14 (a failing check and a pending row exit 10, never 14; CENTRAL C1 (c)).
# NOT done in this slice: ratchet baselines and `--measure`/`--check` catch-up modes, the pinned-container run, hook-filters.json.
# Never   writes any file of the work tree; every path operand follows validation (safe_relpath) and `HEAD:` or a fixed prefix.
set -u
D="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec python3 - "$D" "$@" <<'PY'
import sys, os, re, subprocess, tempfile, json, shutil, atexit
os.environ['GIT_LITERAL_PATHSPECS'] = '1'
D = sys.argv[1]; a = sys.argv[2:]
TABLES = ['check_classes.tsv', 'check_exemptions.tsv', 'fixture_roots.txt']
TREL = 'scripts/repo'
INTERPRETERS = ('bash', 'python3')
out = []; refusals = []
def emit(*f): out.append('\t'.join(str(x) for x in f))
def die(reason, msg=''):
    sys.stderr.write(f"validate_cheap: {reason}: {msg}\n"); sys.exit(20)
def safe_rel(p):
    return bool(p) and not re.search(r'[\x00-\x1f\x7f]', p) and not p.startswith(('/', '-')) and '\\' not in p \
        and '//' not in p and not any(c == '..' for c in p.split('/'))
def safe_decl(p):   # a declared path is a LITERAL path (lib_safe.sh safe_declpath): no `.`, `./` prefix, `/./`, trailing `/`, glob characters, leading `:`
    return safe_rel(p) and p != '.' and not p.startswith(('./', ':')) and '/./' not in p and not p.endswith(('/.', '/')) and not any(c in p for c in '*?[')
def safe_dir(p): return bool(p) and not re.search(r'[\x00-\x1f\x7f]', p) and not p.startswith('-')
root = code = reg = approved = held_from = outdir = lst = trusted = None; declared_rows = []; adopt = False
i = 0
while i < len(a):
    k = a[i]
    if k == '--adopt-working-tables': adopt = True; i += 1; continue
    if k in ('--root', '--files-from', '--code-root', '--registry', '--approved-registry', '--run-declared', '--held-from', '--out', '--trusted-tables'):
        if i + 1 >= len(a): die('usage', f'{k} needs a value')
        v = a[i + 1]; i += 2
        if k == '--run-declared': declared_rows.append(v)
        elif k == '--files-from': lst = v
        elif k == '--root':
            if not safe_dir(v): die('unsafe_root', repr(v))
            root = v
        elif k == '--code-root':
            if not safe_dir(v): die('unsafe_root', repr(v))
            code = v
        elif k == '--registry': reg = v
        elif k == '--approved-registry': approved = v
        elif k == '--held-from': held_from = v
        elif k == '--out': outdir = v
        elif k == '--trusted-tables':
            if not safe_dir(v): die('unsafe_trusted_tables', repr(v))
            trusted = v
    else: die('usage', f'unknown argument {k!r}')
if root is None or lst is None: die('usage', '--root and --files-from are required')
try: top = subprocess.run(['git', '-C', root, 'rev-parse', '--show-toplevel'], capture_output=True, text=True, check=True).stdout.strip()
except Exception: die('not_a_repository', repr(root))
root = top; code = os.path.abspath(code) if code else root
reg = reg or os.path.join(code, TREL, 'validate_checks.tsv')
try: declared = [l.rstrip('\n') for l in open(lst) if l.strip()]
except OSError: die('list_unreadable', repr(lst))
for p in declared:
    if not safe_decl(p): die('unsafe_path', repr(p))
    fp = os.path.join(root, p)
    if os.path.isdir(fp) and not os.path.islink(fp):   # a directory is no declared path; a gitlink (a pin move) is the one directory allowed
        # only a gitlink (a pin move) may be declared as a directory: the index entry whose path EQUALS p decides, never the first entry below it (m1)
        ents = subprocess.run(['git', '-C', root, 'ls-files', '-s', '-z', '--', p], capture_output=True).stdout.split(b'\0')
        if not any(e.startswith(b'160000 ') and e.partition(b'\t')[2] == p.encode() for e in ents if e): die('declared_directory', f'{p!r} is a directory: declare its files')
for r in declared_rows:
    if not re.fullmatch(r'[a-z][a-z0-9_]*', r): die('unsafe_check_name', repr(r))
def git_blob(path):  # content of HEAD:path or None
    r = subprocess.run(['git', '-C', root, 'cat-file', 'blob', 'HEAD:' + path], capture_output=True)
    return r.stdout if r.returncode == 0 else None
def read_tsv(f):
    try: lines = open(f, encoding='utf-8').read().split('\n')
    except OSError: die('registry_unreadable', repr(f))
    return [l for l in lines if l.strip() and not l.startswith('#')]
# ---- registry ------------------------------------------------------------------------------------------------------------------
rows = {}
for l in read_tsv(reg):
    c = l.split('\t')
    if c[0] == 'check': continue
    if len(c) < 6: die('registry_invalid', f'row with fewer than 6 columns: {l[:60]!r}')
    name, cmd, image, mode, baseline, scope = c[:6]
    if not re.fullmatch(r'[a-z][a-z0-9_]*', name): die('registry_invalid', f'check name {name!r}')
    if mode not in ('plain', 'ratchet', 'deferred'): die('registry_invalid', f'{name}: mode {mode!r}')
    if scope not in ('files', 'changeset'): die('registry_invalid', f'{name}: scope {scope!r}')
    rows[name] = {'line': l.rstrip(), 'cmd': cmd, 'mode': mode, 'scope': scope}
appr = None
if approved is not None:
    appr = {}
    for l in read_tsv(approved):
        c = l.split('\t')
        if c[0] != 'check': appr[c[0]] = l.rstrip()
running = []; pending = []
for name, r in rows.items():
    if appr is None: running.append(name); continue
    if name in appr: (running if appr[name] == r['line'] else pending).append(name)
    else: (running if name in declared_rows else pending).append(name)
for name in pending: emit('check_pending_release', name)
for name in running:
    r = rows[name]
    if r['mode'] == 'ratchet': die('ratchet_not_implemented', f'{name}: ratchet baselines are not built in this slice (T040 owed)')
    if r['mode'] == 'deferred': emit('deferred', name)
# the class loader validates the rows that run: every files-scope, non-deferred row needs a class row for every class
regdir = tempfile.mkdtemp(prefix='vc_reg.'); atexit.register(shutil.rmtree, regdir, True)
runreg = os.path.join(regdir, 'reg.tsv')
open(runreg, 'w').write('check\tcommand\timage\tmode\tbaseline\tscope\n' + ''.join(rows[n]['line'] + '\n' for n in running if rows[n]['mode'] != 'deferred'))
# ---- class tables: HEAD and declared, per table ----------------------------------------------------------------------------------
work = tempfile.mkdtemp(prefix='vc_tbl.'); atexit.register(shutil.rmtree, work, True)
def helper_table(t):
    # The helper's own reviewed copy of class table `t`, used only when the root's HEAD holds none. It never comes from the helper's own working
    # tree when that tree is a git work tree (WF3 review I-2): an untracked or edited table there is an undeclared, unreviewed table.
    wt = os.path.join(D, t)
    cur = open(wt, 'rb').read() if os.path.exists(wt) else None
    if trusted is not None:
        p = os.path.join(trusted, t)
        return open(p, 'rb').read() if os.path.exists(p) else b''
    if adopt: return cur if cur is not None else b''
    rd = os.path.realpath(D)
    top = subprocess.run(['git', '-C', rd, 'rev-parse', '--show-toplevel'], capture_output=True, text=True)
    if top.returncode != 0: return cur if cur is not None else b''     # a released snapshot that is no repository: trusted as it is
    rel = os.path.relpath(os.path.join(rd, t), top.stdout.strip())
    h = subprocess.run(['git', '-C', rd, 'cat-file', 'blob', 'HEAD:' + rel], capture_output=True)
    if h.returncode != 0:
        die('class_table_unreviewed', f"{t}: the helper runs in place and this table is not committed in the code root's HEAD (give --trusted-tables <approved copy>, or --adopt-working-tables for the owner-approved adoption run)")
    if cur is not None and cur != h.stdout:
        die('class_table_unreviewed', f"{t}: the working-tree table differs from the code root's committed HEAD (give --trusted-tables <approved copy>, or --adopt-working-tables for the owner-approved adoption run)")
    return h.stdout
def tdir(name): d = os.path.join(work, name); os.makedirs(d, exist_ok=True); return d
headdir = tdir('head'); wtdir = tdir('wt')
for t in TABLES:
    b = git_blob(f'{TREL}/{t}')
    wtp = os.path.join(root, TREL, t)
    # A HEAD that holds no table is judged with the helper's own (reviewed) tables, NEVER with the working-tree copy: an untracked or
    # undeclared working-tree table is no HEAD table (WF2 review I1). A declared change of it is a table change judged by the review rule.
    if b is None: b = helper_table(t)
    open(os.path.join(headdir, t), 'wb').write(b)
    if os.path.exists(wtp): shutil.copy(wtp, os.path.join(wtdir, t))
    else: open(os.path.join(wtdir, t), 'wb').write(b)
changed = []
for t in TABLES:
    rel = f'{TREL}/{t}'
    if rel in declared and open(os.path.join(headdir, t), 'rb').read() != open(os.path.join(wtdir, t), 'rb').read(): changed.append(t)
held = {}   # path -> verdict
if held_from:
    for l in read_tsv(held_from):
        c = l.split('\t')
        if len(c) < 2 or not safe_decl(c[0]) or not safe_decl(c[1]): die('held_table_invalid', l[:60])
        if c[0] in held and held[c[0]] != c[1]: die('held_row_duplicate', f'{c[0]!r} is held on two different verdicts')
        if c[0] not in declared: die('held_row_unmatched', f'{c[0]!r} is a held row but not a declared path')
        held[c[0]] = c[1]
def verdict_go(vp):
    if vp in declared and os.path.isfile(os.path.join(root, vp)): data = open(os.path.join(root, vp), 'rb').read()
    else: data = git_blob(vp)
    if data is None: return False
    try: j = json.loads(data)
    except Exception: return False
    return j.get('verdict') == 'GO' and j.get('blocking_findings') == 0
tstatus = {}   # table -> ('go'|'held'|'unreviewed', verdict path)
for t in changed:
    m = None
    for ln in open(os.path.join(wtdir, t), encoding='utf-8', errors='replace').read().split('\n')[:10]:
        m = re.match(r'#\s*review:\s*(\S+)', ln)
        if m: break
    vp = m.group(1) if m and safe_rel(m.group(1)) else None
    if vp and verdict_go(vp): tstatus[t] = ('go', vp)
    elif vp and (vp in declared or vp in held.values()): tstatus[t] = ('held', vp)
    else: tstatus[t] = ('unreviewed', vp); emit('class_table_unreviewed', f'{TREL}/{t}')
combos = {}
def combo(choice):  # choice: tuple of 'head'/'wt' per table -> directory
    key = ''.join('h' if c == 'head' else 'w' for c in choice)
    if key not in combos:
        d = tdir('c_' + key)
        for t, c in zip(TABLES, choice): shutil.copy(os.path.join(headdir if c == 'head' else wtdir, t), os.path.join(d, t))
        combos[key] = d
    return combos[key]
def tableset(path, allow_held=True):
    ch = []
    for t in TABLES:
        st = tstatus.get(t)
        if st is None: ch.append('head')
        elif st[0] == 'go': ch.append('wt')
        elif st[0] == 'held' and allow_held and held.get(path) == st[1]: ch.append('wt')
        else: ch.append('head')
    return combo(tuple(ch))
# loader validation of the running registry once, with the table set of the main path
def loader(path, tdirp, extra=None):
    cmd = [os.path.join(D, 'check_class.sh'), '--exemptions', os.path.join(tdirp, 'check_exemptions.tsv'), '--classes', os.path.join(tdirp, 'check_classes.tsv'),
           '--fixture-roots', os.path.join(tdirp, 'fixture_roots.txt')] + (extra or []) + [path]
    r = subprocess.run(cmd, cwd=root, capture_output=True, text=True)
    return r
r0 = loader('x', tableset(''), ['--checks-registry', runreg])
if r0.returncode != 0:
    die(r0.stderr.split(' ')[0] or 'class_loader_failed', r0.stderr.strip())
for name in running:
    r = rows[name]
    if r['mode'] == 'deferred': continue
    toks = r['cmd'].split(' ')
    if r['cmd'].startswith('builtin:'):
        if r['cmd'][8:] not in ('shell_parse', 'merge_conflict', 'trailing_whitespace', 'end_of_file', 'check_yaml', 'check_json', 'large_file'):
            die('check_command_absent', f'{name}: unknown builtin {r["cmd"]!r}')
        r['argv'] = None
    else:
        # command form: `<script> args...` (the script must be executable) or `<interpreter> <script> args...` with the interpreter from a
        # closed set (the script then only has to be readable: the registry says HOW it runs, the file mode is not the gate; WF2 review I2)
        interp = None
        if toks[0] in INTERPRETERS:
            interp = toks[0]; toks = toks[1:]
            if not toks: die('registry_invalid', f'{name}: interpreter {interp!r} without a script')
        r['interp'] = interp
        if not safe_rel(toks[0]): die('registry_invalid', f'{name}: command {toks[0]!r} is not a safe path relative to the code root')
        for t in toks[1:]:
            if t not in ('{root}', '{files}', '{paths}') and not re.fullmatch(r'--[a-z][a-z-]*', t): die('registry_invalid', f'{name}: argument {t!r}')
        script = os.path.join(code, toks[0])
        if interp is None and not (os.path.isfile(script) and os.access(script, os.X_OK)): die('check_command_absent', f'{name}: {toks[0]} is not an executable file under the code root (name an interpreter in the registry row to run a non-executable script)')
        if interp is not None and not (os.path.isfile(script) and os.access(script, os.R_OK)): die('check_command_absent', f'{name}: {toks[0]} is not a readable file under the code root')
        r['argv'] = toks
cache = {}
def verdicts(path, tdirp):
    k = (path, tdirp)
    if k not in cache:
        r = loader(path, tdirp)
        if r.returncode != 0: die(r.stderr.split(' ')[0] or 'class_loader_failed', f'{path}: {r.stderr.strip()}')
        v = {}; cls = None
        for ln in r.stdout.splitlines():
            n, _, x = ln.partition(' ')
            if n == 'class': cls = x
            else: v[n] = x
        cache[k] = (cls, v)
    return cache[k]
# ---- the checks ---------------------------------------------------------------------------------------------------------------------
TEXTLESS = {'large_file'}
def suffix_ok(name, p):
    if name == 'shell_parse': return p.endswith(('.sh', '.bash'))
    if name == 'check_yaml': return p.endswith(('.yaml', '.yml'))
    if name == 'check_json': return p.endswith('.json')
    if name == 'revision_header': return p.endswith('.md')
    if name == 'no_false_positive_log': return p.endswith('_test.go')
    return True
def is_binary(path):
    try:
        with open(os.path.join(root, path), 'rb') as f: return b'\0' in f.read(8192)
    except OSError: return True
def run_builtin(name, path, bound=None):
    f = os.path.join(root, path)
    if name == 'large_file':
        sz = os.path.getsize(f)
        if sz > bound: return 'fail', f'{sz} B > {bound} B'
        if sz > bound * 0.75: emit('size_alarm', path, sz, bound)
        return 'pass', ''
    data = open(f, 'rb').read()
    if name == 'shell_parse':
        r = subprocess.run(['bash', '-n', '--', f], capture_output=True, text=True)
        return ('pass', '') if r.returncode == 0 else ('fail', r.stderr.strip()[:200])
    if name == 'merge_conflict':
        for n, ln in enumerate(data.split(b'\n'), 1):
            if ln.startswith((b'<<<<<<< ', b'>>>>>>> ', b'======= ')) or ln.rstrip(b'\r') == b'=======':
                return 'fail', f'line {n}'
        return 'pass', ''
    if name == 'trailing_whitespace':
        for n, ln in enumerate(data.split(b'\n'), 1):
            if ln.rstrip(b'\r') != ln.rstrip(b'\r').rstrip(b' \t'): return 'fail', f'line {n}'
        return 'pass', ''
    if name == 'end_of_file':
        if data and (not data.endswith(b'\n') or data.endswith(b'\n\n')): return 'fail', 'final newline'
        return 'pass', ''
    if name == 'check_yaml':
        try:
            import yaml; yaml.safe_load(data)
        except ImportError: die('tool_absent', 'PyYAML is required for check_yaml')
        except Exception as e: return 'fail', str(e).splitlines()[0][:200]
        return 'pass', ''
    if name == 'check_json':
        def hook(pairs):
            seen = set()
            for k, _ in pairs:
                if k in seen: raise ValueError(f'duplicate key {k}')
                seen.add(k)
            return dict(pairs)
        try: json.loads(data, object_pairs_hook=hook)
        except Exception as e: return 'fail', str(e)[:200]
        return 'pass', ''
    die('check_command_absent', name)
def run_script(name, files):
    r = rows[name]; argv = []
    tmpl = r['argv']
    lf = None
    for t in tmpl[1:]:
        if t == '{root}': argv.append(root)
        elif t == '{files}':
            lf = os.path.join(work, f'{name}.lst'); open(lf, 'w').write(''.join(x + '\n' for x in files)); argv.append(lf)
        elif t == '{paths}': argv.extend(files)
        else: argv.append(t)
    try: p = subprocess.run(([r['interp']] if r.get('interp') else []) + [os.path.join(code, tmpl[0])] + argv, cwd=root, capture_output=True, text=True, timeout=120)
    except subprocess.TimeoutExpired: die('check_timeout', name)
    if p.returncode == 20: die('check_refused', f'{name}: {p.stderr.strip()[:200]}')
    return p.returncode, (p.stdout + p.stderr).strip()
files_checks = [n for n in running if rows[n]['mode'] == 'plain' and rows[n]['scope'] == 'files']
cs_checks = [n for n in running if rows[n]['mode'] == 'plain' and rows[n]['scope'] == 'changeset']
failed = False
script_files = {n: [] for n in files_checks if rows[n]['argv']}
unheld_findings = []
anyheld = any(s[0] == 'held' for s in tstatus.values())
EVIDENCE_CLASSES = ('evidence', 'evidence-ledger')
def head_is_link(path):
    r = subprocess.run(['git', '-C', root, 'ls-tree', 'HEAD', '--', path], capture_output=True, text=True)
    return r.returncode == 0 and r.stdout.split(' ')[0] == '120000'
for path in declared:
    fp = os.path.join(root, path)
    if not os.path.lexists(fp): continue   # a deletion: not judged
    if os.path.islink(fp):                 # judged by its link type, never followed and never skipped silently (WF3 review B-1)
        lcls, _lv = verdicts(path, tableset(path))
        if lcls in EVIDENCE_CLASSES: emit('fail', 'symlink', path, f'a declared symlink in class {lcls} is judged by its link type'); failed = True
        else: emit('symlink_not_judged', path)
        continue
    if not os.path.isfile(fp): continue
    td = tableset(path); cls, v = verdicts(path, td)
    if cls in EVIDENCE_CLASSES and head_is_link(path):
        emit('fail', 'symlink', path, f'type change: HEAD holds a symlink in class {cls}, the work tree a regular file'); failed = True; continue
    # legacy_row_not_dropped: exact-path row of class legacy-collection kept, content edited
    if cls == 'legacy-collection':
        for l in read_tsv(os.path.join(td, 'check_exemptions.tsv')):
            c = l.split('\t')
            if c[0] == path and c[1] == 'legacy-collection' and not re.search(r'[*?\[]', path):
                head = git_blob(path)
                if head is not None and head != open(os.path.join(root, path), 'rb').read():
                    emit('legacy_row_not_dropped', path); failed = True
    held_here = held.get(path)
    alt = None
    if anyheld and td == tableset(path, allow_held=False) and any(s[0] == 'held' for s in tstatus.values()) and held_here is None:
        alt = verdicts(path, combo(tuple('wt' if t in tstatus and tstatus[t][0] in ('go', 'held') else 'head' for t in TABLES)))
    for name in files_checks:
        if not suffix_ok(name, path): continue
        ver = v.get(name)
        if ver is None: die('table_invalid', f'class {cls} has no verdict for {name}')
        res = 'skip'; detail = ''
        if ver != 'skip':
            if name not in TEXTLESS and is_binary(path): emit('left_out', name, path); continue
            if rows[name]['argv']: script_files[name].append(path); res = 'script'
            else:
                res, detail = run_builtin(name, path, int(ver) if ver.isdigit() else None)
        if res == 'fail':
            if alt is not None:
                av = alt[1].get(name); admits = av == 'skip'
                if av and av.isdigit() and name == 'large_file' and os.path.getsize(os.path.join(root, path)) <= int(av): admits = True
                if admits: unheld_findings.append((path, name)); continue
            emit('fail', name, path, detail); failed = True
# script rows run once over their filtered files
for name, fl in script_files.items():
    if not fl: continue
    rc, msg = run_script(name, fl)
    if rc != 0:
        failing = [f for f in fl if f in msg] or fl[:5]   # the files the check names, never merely the first five it was given (WF3 review m8)
        emit('fail', name, ','.join(failing), msg.replace('\n', ' | ')[:300]); failed = True
for name in cs_checks:
    if rows[name]['argv'] is None: continue
    rc, msg = run_script(name, [])
    if rc != 0: emit('fail', name, '-', msg.replace('\n', ' | ')[:300]); failed = True
for path, name in unheld_findings:
    emit('table_admits_unheld_path', path, name)
text = '\n'.join(out) + ('\n' if out else '')
sys.stdout.write(text)
if outdir:
    if not safe_dir(outdir): die('unsafe_out', repr(outdir))
    os.makedirs(outdir, exist_ok=True); open(os.path.join(outdir, 'report.tsv'), 'w').write(text)
if unheld_findings:
    sys.stderr.write('validate_cheap: table_admits_unheld_path: declare the path held on the table\'s verdict\n'); sys.exit(20)
if failed or any(l.startswith('class_table_unreviewed') for l in out): sys.exit(10)
if pending: sys.exit(14)
sys.exit(0)
PY
