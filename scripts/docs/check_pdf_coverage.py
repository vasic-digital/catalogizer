#!/usr/bin/env python3
"""check_pdf_coverage.py - gate: a PDF twin must contain every distinct word-token of its Markdown source (constitution 11.4.65, 11.4.201).

Purpose  A PDF rendered with wide tables or long unbroken lines is silently CLIPPED at the page edge; the file opens fine but text is
         missing. This compares the distinct tokens (>= 6 chars) of FILE.md with the text pdftotext extracts from FILE.pdf.
Usage    check_pdf_coverage.py [--root DIR] [--allow FILE] [--min-len N] [--quiet] FILE.md [FILE.md ...]
         (the PDF is the sibling FILE.pdf)
Method   md text -> link targets, HTML comments, fence markers, image syntax removed -> NFKC -> tokens split on whitespace and markdown
         punctuation -> keep tokens >= min-len. PDF text -> NFKC, soft hyphens removed. Both sides are then compared with all whitespace
         and hyphens deleted (the PDF is read in two orders, default and -raw; a token counts if either carries it), so a token wrapped across lines or hyphen-broken still matches.
Allow    scripts/docs/pdf_coverage_allow.tsv (path<TAB>token<TAB>reason): tokens legitimately absent from a rendering. Default empty.
Control  Before the scan the needle control runs on the first PDF: a token known present (taken from the PDF itself) must be found and a
         fabricated token must not be; otherwise the instrument is BLIND (exit 2).
Exit     0 no token missing, 1 one or more missing (or a PDF absent/empty), 2 BLIND / usage / tool missing.
"""
import os, re, subprocess, sys, unicodedata

CTRL_FAKE = 'zqxjvkwfabricatedneedle'


def norm(s):
    s = unicodedata.normalize('NFKC', s).replace('­', '')
    return s


def squash(s):
    return re.sub(r'[\s\-‐-―]+', '', norm(s))


def md_tokens(text, minlen):
    text = re.sub(r'<!--.*?-->', ' ', text, flags=re.S)
    text = re.sub(r'!\[([^\]]*)\]\([^)]*\)', r'\1', text)
    text = re.sub(r'\[([^\]]*)\]\([^)]*\)', r'\1', text)
    text = re.sub(r'^\s*(```|~~~)\S*\s*$', ' ', text, flags=re.M)
    text = re.sub(r'<[^>\n]{1,200}>', ' ', text)  # inline html tags
    text = norm(text)
    toks = set()
    for t in re.split(r'[\s|`*_<>()\[\]{},;:"\'#=\\!?]+', text):
        t = t.strip('.-/~')
        if len(t) >= minlen and re.search(r'[A-Za-z0-9]', t):
            toks.add(t)
    return toks


def pdf_text(pdf):
    # two reading orders (content-stream and physical); a token counts as present if either reading carries it contiguously
    return '\n\x00\n'.join(subprocess.run(['pdftotext'] + m + [pdf, '-'], capture_output=True, text=True, check=True).stdout for m in ([], ['-raw']))


def load_allow(p):
    allow = set()
    if p and os.path.exists(p):
        for l in open(p, encoding='utf-8').read().split('\n'):
            if l.strip() and not l.startswith('#'):
                c = l.split('\t')
                if len(c) != 3 or not c[2].strip():
                    raise SystemExit('check_pdf_coverage: bad allow row (path, token, reason required): %r' % l)
                allow.add((c[0], squash(c[1])))
    return allow


def main(argv):
    root = os.getcwd(); allow_p = None; minlen = 6; quiet = False; files = []; i = 0
    while i < len(argv):
        a = argv[i]
        if a == '--root' and i + 1 < len(argv): root = argv[i + 1]; i += 2
        elif a == '--allow' and i + 1 < len(argv): allow_p = argv[i + 1]; i += 2
        elif a == '--min-len' and i + 1 < len(argv): minlen = int(argv[i + 1]); i += 2
        elif a == '--quiet': quiet = True; i += 1
        elif a.startswith('--'): print(__doc__, file=sys.stderr); return 2
        else: files.append(a); i += 1
    if not files: print(__doc__, file=sys.stderr); return 2
    try:
        subprocess.run(['pdftotext', '-v'], capture_output=True)
    except OSError:
        print('check_pdf_coverage: pdftotext not found', file=sys.stderr); return 2
    if allow_p is None: allow_p = os.path.join(root, 'scripts/docs/pdf_coverage_allow.tsv')
    allow = load_allow(allow_p)
    rc = 0; controlled = False
    for md in files:
        pdf = md[:-3] + '.pdf' if md.endswith('.md') else md + '.pdf'
        rel = os.path.relpath(os.path.abspath(md), os.path.abspath(root))
        if not os.path.isfile(pdf) or os.path.getsize(pdf) == 0:
            print('FAIL %s: PDF twin absent or empty' % rel); rc = max(rc, 1); continue
        try:
            raw = pdf_text(pdf)
        except subprocess.CalledProcessError:
            print('FAIL %s: pdftotext cannot read the PDF (truncated or corrupt)' % rel); rc = max(rc, 1); continue
        hay = '\x00'.join(squash(x) for x in raw.split('\x00'))
        if not controlled:
            words = [w for w in re.findall(r'[A-Za-z]{8,}', norm(raw))]
            if not words:
                print('BLIND %s: PDF yields no text to draw a control needle from' % rel); return 2
            if squash(words[0]) not in hay or squash(CTRL_FAKE) in hay:
                print('BLIND: needle control failed'); return 2
            controlled = True
        toks = md_tokens(open(md, encoding='utf-8', errors='replace').read(), minlen)
        missing = sorted(t for t in toks if squash(t) not in hay and (rel, squash(t)) not in allow)
        if not toks:
            print('BLIND %s: no tokens extracted from the Markdown' % rel); return 2
        if missing:
            rc = max(rc, 1)
            print('FAIL %s: %d/%d tokens missing' % (rel, len(missing), len(toks)))
            if not quiet:
                for t in missing[:15]: print('   missing: %s' % t)
        else:
            print('OK   %s: 0/%d tokens missing' % (rel, len(toks)))
    return rc


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))
