#!/usr/bin/env python3
"""check_twin_presence.py - gate: every Markdown doc added under docs/ since a ref has current .html and .pdf twins (constitution 11.4.65, 11.4.77).

Purpose  11.4.65 requires every doc to ship its exported siblings. A new doc committed without them (or with a stale html) is caught here.
Usage    check_twin_presence.py [--root DIR] [--since REF] [--no-html-drift] [--quiet]
         --since  git ref; default the newest tag (git describe --tags --abbrev=0). Scope: files ADDED (git diff --diff-filter=A REF..HEAD, plus
                  untracked-but-present added files in the index) under docs/ that end in .md.
Checks   per scoped FILE.md: FILE.html exists and is non-empty; FILE.pdf exists and is non-empty and starts with %PDF; unless --no-html-drift,
         scripts/docs/render_md_to_html.sh --check FILE.md is clean (the html matches what would be generated).
Control  Before the scan, in a temp tree: a doc with both twins must be reported complete and a doc missing its pdf must be reported incomplete;
         an empty scope is BLIND, never a pass.
Output   MISSING <path> (<what>) per defect and a summary line.
Exit     0 all twins present and current, 1 one or more defects, 2 BLIND / usage / not a git repo.
"""
import os, subprocess, sys, tempfile, shutil


def defects(root, md, html_drift=True):
    base = os.path.join(root, md[:-3])
    out = []
    for ext, magic in (('.html', None), ('.pdf', b'%PDF')):
        p = base + ext
        if not os.path.isfile(p) or os.path.getsize(p) == 0: out.append(ext[1:] + ' twin absent or empty'); continue
        if magic and open(p, 'rb').read(4) != magic: out.append('pdf twin is not a PDF')
    if html_drift and os.path.isfile(base + '.html'):
        r = subprocess.run([os.path.join(root, 'scripts/docs/render_md_to_html.sh'), '--check', os.path.join(root, md)], capture_output=True, text=True)
        if r.returncode != 0: out.append('html twin drifts from render_md_to_html.sh output')
    return out


def control():
    d = tempfile.mkdtemp(prefix='twin-ctl.')
    try:
        os.makedirs(os.path.join(d, 'docs'))
        for n, pdf in (('good', True), ('bad', False)):
            open(os.path.join(d, 'docs', n + '.md'), 'w').write('# x\n')
            open(os.path.join(d, 'docs', n + '.html'), 'w').write('<html></html>')
            if pdf: open(os.path.join(d, 'docs', n + '.pdf'), 'wb').write(b'%PDF-1.4 x')
        return not defects(d, 'docs/good.md', False) and bool(defects(d, 'docs/bad.md', False))
    finally:
        shutil.rmtree(d)


def main(argv):
    root = os.getcwd(); since = None; drift = True; quiet = False; i = 0
    while i < len(argv):
        a = argv[i]
        if a == '--root' and i + 1 < len(argv): root = argv[i + 1]; i += 2
        elif a == '--since' and i + 1 < len(argv): since = argv[i + 1]; i += 2
        elif a == '--no-html-drift': drift = False; i += 1
        elif a == '--quiet': quiet = True; i += 1
        else: print(__doc__, file=sys.stderr); return 2
    g = lambda *c: subprocess.run(['git', '-C', root] + list(c), capture_output=True, text=True)
    if since is None:
        r = g('describe', '--tags', '--abbrev=0')
        if r.returncode: print('BLIND: no tag to default --since to', file=sys.stderr); return 2
        since = r.stdout.strip()
    r = g('diff', '--name-only', '--diff-filter=A', since + '..HEAD', '--', 'docs')
    if r.returncode: print('BLIND: git diff failed: ' + r.stderr.strip(), file=sys.stderr); return 2
    r2 = g('diff', '--cached', '--name-only', '--diff-filter=A', '--', 'docs')
    scope = sorted(set(f for f in (r.stdout + r2.stdout).split('\n') if f.endswith('.md')))
    if not control(): print('BLIND: needle control failed'); return 2
    if not scope: print('BLIND: no added docs/*.md since ' + since); return 2
    bad = 0
    for md in scope:
        if not os.path.isfile(os.path.join(root, md)): continue
        for d in defects(root, md, drift): print('MISSING %s (%s)' % (md, d)); bad += 1
    print('%s: %d doc(s) added since %s, %d defect(s)' % ('FAIL' if bad else 'OK', len(scope), since, bad))
    return 1 if bad else 0


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))
