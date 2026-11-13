#!/usr/bin/env python3
"""check_pdf_coverage.py - gate: a PDF twin must contain every distinct word-token of its Markdown source (constitution 11.4.65, 11.4.201).

Purpose  A PDF rendered with wide tables or long unbroken lines is silently CLIPPED at the page edge, or a table column too narrow splits
         ids and numbers across two lines; the file opens fine but text is missing or broken. This compares the distinct tokens of
         FILE.md with the words pdftotext -bbox extracts from FILE.pdf.
Usage    check_pdf_coverage.py [--root DIR] [--allow FILE] [--min-len N] [--quiet] FILE.md [FILE.md ...]
         (the PDF is the sibling FILE.pdf; default --min-len 4; a token that contains a digit is checked from 2 characters up)
Method   md text -> comments, link targets, fence markers, image syntax removed, backticks dropped, NFKC -> tokens split on whitespace and
         markdown punctuation (Unicode letters and digits count as token characters; Cyrillic, CJK etc. included).
         Each token must appear in the PDF word stream (two reading orders: line by line, and content stream = cell by cell) on a word boundary. It must be whole inside one PDF word, or continue over a line
         wrap: a wrap (a space in the word stream) is accepted only (a) at a position the renderer may break: after '/', '_', '@', '=', '&', '?', after '-'
         (not between digits), after '.' before a letter, or inside one separator-delimited SEGMENT of 30 or more characters, and NEVER between
         two digits, whatever the length of the enclosing word, AND (b) geometrically: the next piece starts on a LATER line and not to the right of
         the previous piece's end, so two neighbouring cells ('Abcdefghijkl' | 'mnopqrstuvwxyz') cannot be glued into one long word.
Allow    scripts/docs/pdf_coverage_allow.tsv (path<TAB>token<TAB>reason): tokens legitimately absent from a rendering. Default empty.
Control  Per file, before judging (differential: the needle must be found in the real stream and NOT in a copy of it in which the needle is destroyed; a fabricated needle must not be found either): PDF side (a word taken from this PDF must be found, a fabricated one not) and Markdown side (the matcher
         must find a cleaned-Markdown token in the cleaned Markdown, a fabricated one not); otherwise BLIND (exit 2). A PDF with no text at
         all while the Markdown has tokens is a FAIL (blank render), never OK.
Exit     0 no token missing, 1 one or more missing (or a PDF absent/empty/blank), 2 BLIND / usage / tool missing.
"""
import html, os, re, subprocess, sys, unicodedata

CTRL_FAKE = 'zqxjvkwfabricatedneedle'
SEP = '-/._:@=&?'
TC = r'[^\W_]'                       # a token character: any Unicode letter or digit
BND = r'(?<!' + TC + r')%s(?!' + TC + r')'


def norm(s):
    return unicodedata.normalize('NFKC', s).replace('­', '').replace('\u200b', '')


def clean_md(text):
    text = re.sub(r'<!--.*?-->', ' ', text, flags=re.S)
    text = re.sub(r'!\[([^\]]*)\]\([^)]*\)', r'\1', text)
    text = re.sub(r'\[([^\]]*)\]\([^)]*\)', r'\1', text)
    text = re.sub(r'^\s*(```|~~~)\S*\s*$', ' ', text, flags=re.M)
    text = re.sub(r'<[^>\n]{1,200}>', ' ', text)
    return norm(text).replace('`', '')


def md_tokens(text, minlen):
    """dict token -> length of the longest whitespace-delimited Markdown word containing it (text: already cleaned)."""
    toks = {}
    for word in re.split(r'[\s|]+', text):
        for t in re.split(r'[*_<>()\[\]{},;:"\'#=\\!?]+', word):
            t = t.strip('.-/~')
            if re.search(TC, t) and (len(t) >= minlen or (re.search(r'\d', t) and len(t) >= 2)):
                toks[t] = max(toks.get(t, 0), len(word))
    return toks


class Words:
    """pdftotext -bbox words in reading order with a joined text and per-character word index."""
    def __init__(self, pdf, raw=False):
        out = subprocess.run(['pdftotext'] + (['-raw'] if raw else []) + ['-bbox', pdf, '-'], capture_output=True, text=True, check=True).stdout
        self.w = []
        for pg, body in enumerate(out.split('<page ')[1:], 1):
            for xa, ya, xb, yb, t in re.findall(r'<word xMin="([\d.]+)" yMin="([\d.]+)" xMax="([\d.]+)" yMax="([\d.]+)">(.*?)</word>', body, flags=re.S):
                self.w.append((pg, float(xa), float(ya), float(xb), float(yb), norm(html.unescape(t))))
        self._index()

    def _index(self):
        self.text = ' '.join(x[5] for x in self.w)
        self.idx = []
        for i, x in enumerate(self.w):
            self.idx.extend([i] * len(x[5])); self.idx.append(-1)

    def altered(self, needle):
        """A copy in which every occurrence of needle is destroyed (control: the matcher must stop finding it)."""
        c = Words.__new__(Words)
        c.w = [x[:5] + (x[5].replace(needle, 'q' + needle[::-1]),) for x in self.w]
        c._index()
        return c

    def wrap_ok(self, i, j):
        a, b = self.w[i], self.w[j]
        return a[0] == b[0] and b[2] > a[2] + 2 and b[1] < a[3] - 1       # same page, later line, not to the right of the previous piece's end

    def gaps_ok(self, m, t):
        """Every wrap inside the match must be geometrically a wrap AND at a permitted position of the token t:
        after a separator, or inside a separator-delimited segment of SEGMENT_CAP or more characters; never between two digits."""
        prev = None
        for k in range(m.start(), m.end()):
            if self.idx[k] == -1:
                prev = self.idx[k - 1]; nxt = self.idx[k + 1]
                if not self.wrap_ok(prev, nxt): return False
        return True


SEGMENT_CAP = 30


def gap_after(t, i):
    """May the PDF break the token t between t[i] and t[i+1]?"""
    a, b = t[i], t[i + 1]
    if a.isdigit() and b.isdigit(): return False
    if a in '/_@=&?': return True
    if a == '-': return not (i >= 1 and t[i - 1].isdigit() and b.isdigit())
    if a == '.': return b.isalpha()
    j = i
    while j >= 0 and t[j] not in SEP: j -= 1
    k = i + 1
    while k < len(t) and t[k] not in SEP: k += 1
    return k - j - 1 >= SEGMENT_CAP                      # inside one over-long segment


def present(tok, Ws):
    """Word-boundary match of tok in the word stream W (a Words, or a plain str for the Markdown-side control)."""
    return any(_present(tok, W) for W in (Ws if isinstance(Ws, (list, tuple)) else [Ws]))


def _present(tok, W):
    t = norm(tok)
    hay = W.text if isinstance(W, Words) else W
    if re.search(BND % re.escape(t), hay): return True
    parts = []
    for i, c in enumerate(t):
        parts.append(re.escape(c))
        if i < len(t) - 1 and gap_after(t, i): parts.append(' ?')
    for m in re.finditer(BND % ''.join(parts), hay):
        if not isinstance(W, Words) or W.gaps_ok(m, t): return True
    return False


def load_allow(p):
    allow = set()
    if p and os.path.exists(p):
        for l in open(p, encoding='utf-8').read().split('\n'):
            if l.strip() and not l.startswith('#'):
                c = l.split('\t')
                if len(c) != 3 or not c[2].strip():
                    raise SystemExit('check_pdf_coverage: bad allow row (path, token, reason required): %r' % l)
                allow.add((c[0], norm(c[1])))
    return allow


def main(argv):
    root = os.getcwd(); allow_p = None; minlen = 4; quiet = False; files = []; i = 0
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
    rc = 0
    for md in files:
        pdf = md[:-3] + '.pdf' if md.endswith('.md') else md + '.pdf'
        rel = os.path.relpath(os.path.abspath(md), os.path.abspath(root))
        if not os.path.isfile(pdf) or os.path.getsize(pdf) == 0:
            print('FAIL %s: PDF twin absent or empty' % rel); rc = max(rc, 1); continue
        try:
            Ws = [Words(pdf, False), Words(pdf, True)]   # physical (line by line) and content-stream (cell by cell) reading orders
            W = Ws[0]
        except subprocess.CalledProcessError:
            print('FAIL %s: pdftotext cannot read the PDF (truncated or corrupt)' % rel); rc = max(rc, 1); continue
        mdtext = clean_md(open(md, encoding='utf-8', errors='replace').read())
        toks = md_tokens(mdtext, minlen)
        if toks and not W.w:
            print('FAIL %s: PDF has no text (blank render), Markdown has %d tokens' % (rel, len(toks))); rc = max(rc, 1); continue
        if W.w:                                  # PDF-side control, per file
            cands = sorted({m for x in W.w for m in re.findall(r'(?<!%s)%s{3,}(?!%s)' % (TC, TC, TC), x[5])}, key=len, reverse=True)   # runs inside ONE pdf word
            cands = [t for t in cands if W.text.count(t) == 1][:1]   # a needle that occurs exactly once, so destroying it removes it from the stream
            alt = [w.altered(cands[0]) for w in Ws] if cands else []
            if not cands or not present(cands[0], Ws) or present(cands[0], alt) or present(CTRL_FAKE, Ws):
                print('BLIND %s: needle control failed (PDF side)' % rel); return 2
        if toks:                                 # Markdown-side control, per file, on the same cleaned text the tokens came from
            probe = max(toks, key=len)
            if not present(probe, mdtext) or present(probe, mdtext.replace(probe, 'q' + probe[::-1])) or present(CTRL_FAKE, mdtext):
                print('BLIND %s: needle control failed (Markdown side)' % rel); return 2
        else:
            print('OK   %s: no tokens of >= %d chars, nothing to compare' % (rel, minlen)); continue
        missing = sorted(t for t, el in toks.items() if not present(t, Ws) and (rel, norm(t)) not in allow)
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
