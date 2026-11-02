#!/usr/bin/env python3
"""gen_dir_indexes.py - regenerate the per-directory README.md indexes of the documentation tree (constitution 11.4.77).

Purpose   Each directory listed in scripts/docs/dir_indexes.tsv gets a README.md listing every tracked file below it
          (recursively, except its own README.md). A .md file is described by its first '# ' heading plus its first prose line
          (backticks and '**' stripped, cut at ~170 chars); any other file is listed by name and type only (not read).
          scripts/docs/index_description_overrides.tsv (path<TAB>description) replaces a description that reads poorly.
Usage     gen_dir_indexes.py [--root DIR] [--check]
          --check  writes nothing; exit 1 if any README.md differs from what would be generated (drift gate), 0 otherwise.
Input     scripts/docs/dir_indexes.tsv: dir<TAB>title<TAB>intro<TAB>relative-link-to-docs-README
Output    <dir>/README.md for each row (only files tracked in git are listed, so commit new docs before regenerating).
Exit      0 ok, 1 drift (--check), 2 usage/input error.
"""
import os, re, subprocess, sys

KINDS = {'.json': 'JSON data artefact', '.txt': 'plain-text artefact', '.html': 'HTML export', '.pdf': 'PDF export',
         '.svg': 'SVG image', '.mmd': 'Mermaid diagram source', '.db-shm': 'SQLite shared-memory artefact',
         '.db-wal': 'SQLite write-ahead-log artefact', '.status': 'status marker artefact', '.tsv': 'TSV data artefact',
         '.yaml': 'YAML data', '.yml': 'YAML data', '.png': 'PNG image', '.sh': 'shell script', '.py': 'Python script',
         '.docx': 'DOCX export'}


def clean(s):
    return re.sub(r'\s+', ' ', s.replace('`', '').replace('**', '')).replace('|', '\\|').strip()


def describe(root, path):
    try:
        lines = open(os.path.join(root, path), errors='ignore').read().split('\n')
    except OSError:
        return None, None
    title = None; d = None; fence = False
    for l in lines[:120]:
        s = l.strip()
        if s.startswith('```'):
            fence = not fence; continue
        if fence: continue
        if title is None:
            if s.startswith('# '): title = s[2:].strip()
            continue
        if not s or s[0] in '|#<-*=' or s.startswith(('---', 'Last', 'Version', 'Date', '[')): continue
        s = s.lstrip('> ').strip()
        if re.match(r'^\d+[.)]\s', s) or '](#' in s or s.startswith(('Table of', 'Contents')) or len(s) < 25: continue
        d = clean(s); break
    if d and len(d) > 170: d = d[:170].rsplit(' ', 1)[0] + '...'
    return (clean(title) if title else None), d


def load_tsv(p, n):
    rows = []
    if os.path.exists(p):
        for l in open(p).read().split('\n'):
            if l.strip() and not l.startswith('#'):
                c = l.split('\t')
                if len(c) != n: raise SystemExit('gen_dir_indexes: bad row in %s: %r' % (p, l))
                rows.append(c)
    return rows


def render(root, tracked, over, d, title, intro, up):
    base = d + '/'; rs = []
    for f in sorted(t for t in tracked if t.startswith(base)):
        rel = f[len(base):]
        if rel == 'README.md': continue
        if f in over: txt = over[f]
        elif f.endswith('.md'):
            t, ds = describe(root, f)
            txt = (t or os.path.basename(f)) + ((' - ' + ds) if ds else '')
        else:
            txt = '(' + KINDS.get(os.path.splitext(f)[1].lower(), 'file') + '; not read, listed by name only)'
        rs.append((rel, txt))
    out = '# %s\n\n%s\n\nBack to the documentation index: [docs/README.md](%s). Descriptions are copied mechanically from each file (its first heading and first prose line) unless overridden in scripts/docs/index_description_overrides.tsv; non-Markdown artefacts are listed by name and type without being read. %d entries.\n\n| File | Description |\n|---|---|\n' % (title, intro, up, len(rs))
    for rel, t in rs: out += '| [%s](%s) | %s |\n' % (rel, rel.replace(' ', '%20'), t)
    return out


def main(argv):
    root = None; check = False; i = 0
    while i < len(argv):
        if argv[i] == '--root' and i + 1 < len(argv): root = argv[i + 1]; i += 2
        elif argv[i] == '--check': check = True; i += 1
        else: print('usage: gen_dir_indexes.py [--root DIR] [--check]', file=sys.stderr); return 2
    if root is None: root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    if os.path.exists(os.path.join(root, '.git')):
        tracked = subprocess.check_output(['git', '-C', root, 'ls-files'], text=True).split('\n')
    else:
        tracked = sorted(os.path.relpath(os.path.join(d, f), root) for d, _, fs in os.walk(root) for f in fs)
    cfg = load_tsv(os.path.join(root, 'scripts/docs/dir_indexes.tsv'), 4)
    if not cfg: print('gen_dir_indexes: no rows in dir_indexes.tsv', file=sys.stderr); return 2
    over = dict(load_tsv(os.path.join(root, 'scripts/docs/index_description_overrides.tsv'), 2))
    drift = 0
    for d, title, intro, up in cfg:
        txt = render(root, tracked, over, d, title, intro, up)
        p = os.path.join(root, d, 'README.md')
        cur = open(p).read() if os.path.exists(p) else None
        if check:
            if cur != txt: drift += 1; print('DRIFT ' + d + '/README.md')
        else:
            open(p, 'w').write(txt); print('%s %d' % (d, txt.count('\n| [')))
    return 1 if drift else 0


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))
