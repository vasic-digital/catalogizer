#!/usr/bin/env python3
"""check_twin_presence.py - gate: every Markdown doc added or modified under docs/ since a ref has current .html and .pdf twins (constitution 11.4.65, 11.4.77).

Purpose  11.4.65 requires every doc to ship its exported siblings. A new doc committed without them (or with a stale html) is caught here.
Usage    check_twin_presence.py [--root DIR] [--since REF] [--no-html-drift] [--quiet]
         --since  git ref; default the newest tag (git describe --tags --abbrev=0). Scope: .md files ADDED or MODIFIED (git diff --diff-filter=AM REF..HEAD, plus staged,
                  unstaged and untracked-not-ignored working-tree files) under docs/ that have a twin of either kind (a modified doc that never had twins is not demanded; a
                  newly added one always is).
Checks   per scoped FILE.md: FILE.html exists and is non-empty; FILE.pdf exists, is non-empty, starts with %PDF and is a readable PDF whose
         Keywords field carries src-sha256:<sha256 of FILE.md> (so a stale PDF fails) and whose text covers the Markdown
         (check_pdf_coverage.py, 0 tokens missing; skipped with --no-pdf-coverage).
         Residual: the src-sha256 is metadata, so a forger can pair an old-content PDF with a new hash; the coverage run catches that
         whenever the new Markdown contains any token the old PDF lacks, not an edit that adds no new token; unless --no-html-drift,
         scripts/docs/render_md_to_html.sh --check FILE.md is clean (the html matches what would be generated).
Control  Before the scan, in a temp tree: a doc with both twins must be reported complete and a doc missing its pdf must be reported incomplete;
         an empty scope is BLIND, never a pass.
Output   MISSING <path> (<what>) per defect and a summary line.
Exit     0 all twins present and current, 1 one or more defects, 2 BLIND / usage / not a git repo.
"""
import hashlib, os, re, subprocess, sys, tempfile, shutil


def defects(root, md, html_drift=True, pdf_cov=True):
    base = os.path.join(root, md[:-3])
    out = []
    for ext, magic in (('.html', None), ('.pdf', b'%PDF')):
        p = base + ext
        if not os.path.isfile(p) or os.path.getsize(p) == 0: out.append(ext[1:] + ' twin absent or empty'); continue
        if magic and open(p, 'rb').read(4) != magic: out.append('pdf twin is not a PDF')
    pdf = base + '.pdf'
    if os.path.isfile(pdf) and os.path.getsize(pdf) and open(pdf, 'rb').read(4) == b'%PDF':
        info = subprocess.run(['pdfinfo', pdf], capture_output=True, text=True)
        if info.returncode != 0:
            out.append('pdf twin unreadable (pdfinfo failed)')
        else:
            h = hashlib.sha256(open(os.path.join(root, md), 'rb').read()).hexdigest()
            if 'src-sha256:' + h not in info.stdout: out.append('pdf twin is stale (no src-sha256 of the current Markdown in its Keywords)')
            elif pdf_cov:
                r = subprocess.run([sys.executable, os.path.join(os.path.dirname(os.path.abspath(__file__)), 'check_pdf_coverage.py'), '--quiet', '--root', root, os.path.join(root, md)], capture_output=True, text=True)
                if r.returncode != 0: out.append('pdf twin fails coverage: ' + (r.stdout.strip().split('\n')[0] if r.stdout.strip() else 'exit %d' % r.returncode))
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
        # presence only (the stub PDF is not a real one); the garbage-PDF and stale-PDF cases are covered by the tests
        return not [x for x in defects(d, 'docs/good.md', False, False) if 'absent' in x] and any('absent' in x for x in defects(d, 'docs/bad.md', False, False))
    finally:
        shutil.rmtree(d)


def main(argv):
    root = os.getcwd(); since = None; drift = True; pcov = True; quiet = False; i = 0
    while i < len(argv):
        a = argv[i]
        if a == '--root' and i + 1 < len(argv): root = argv[i + 1]; i += 2
        elif a == '--since' and i + 1 < len(argv): since = argv[i + 1]; i += 2
        elif a == '--no-html-drift': drift = False; i += 1
        elif a == '--no-pdf-coverage': pcov = False; i += 1
        elif a == '--quiet': quiet = True; i += 1
        else: print(__doc__, file=sys.stderr); return 2
    g = lambda *c: subprocess.run(['git', '-C', root] + list(c), capture_output=True, text=True)
    if since is None:
        r = g('describe', '--tags', '--abbrev=0')
        if r.returncode: print('BLIND: no tag to default --since to', file=sys.stderr); return 2
        since = r.stdout.strip()
    r = g('diff', '--name-only', '--diff-filter=AM', since + '..HEAD', '--', 'docs')
    if r.returncode: print('BLIND: git diff failed: ' + r.stderr.strip(), file=sys.stderr); return 2
    added = g('diff', '--name-only', '--diff-filter=A', since + '..HEAD', '--', 'docs').stdout.split('\n')
    r2 = g('diff', 'HEAD', '--name-only', '--diff-filter=AM', '--', 'docs')
    names = set(f for f in (r.stdout + r2.stdout).split('\n') if f.endswith('.md'))
    untracked = g('ls-files', '--others', '--exclude-standard', '--', 'docs').stdout.split('\n')   # new docs not yet git-added count as added
    names |= set(f for f in untracked if f.endswith('.md'))
    new = set(added) | set(g('diff', 'HEAD', '--name-only', '--diff-filter=A', '--', 'docs').stdout.split('\n')) | set(untracked)
    # modified docs are in scope only when they already have a twin (an older doc that never had twins is not demanded here)
    scope = sorted(f for f in names if f in new or os.path.isfile(os.path.join(root, f[:-3] + '.html')) or os.path.isfile(os.path.join(root, f[:-3] + '.pdf')))
    if not control(): print('BLIND: needle control failed'); return 2
    if not scope: print('BLIND: no added docs/*.md since ' + since); return 2
    bad = 0
    for md in scope:
        if not os.path.isfile(os.path.join(root, md)): continue
        for d in defects(root, md, drift, pcov): print('MISSING %s (%s)' % (md, d)); bad += 1
    print('%s: %d doc(s) added or modified since %s, %d defect(s)' % ('FAIL' if bad else 'OK', len(scope), since, bad))
    return 1 if bad else 0


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))
