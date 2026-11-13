#!/usr/bin/env python3
"""pdf_table_cols.py - table layout for PDF rendering (used by render_md_to_pdf.sh): break opportunities, segment-based column minimums.

Purpose  Line breaks inside table cells may only happen at SEPARATORS, never inside an id or a digit run.
         1. Break opportunities: in every table cell a zero-width space (U+200B; WeasyPrint ignores <wbr>, measured) is inserted after '/', '_', '-' and after '.' when a letter follows (not between
            digits, so 2026-11-12 and 7.67 stay whole). A cell word is thereby a chain of SEGMENTS (e.g. 'catalog-api/' 'handlers/' 'collection_'
            'handler.' 'go:118'); ':' and digits never start a segment, so 'go:118' is one unbreakable segment.
         2. Column minimums: the minimum width of a column is the estimated rendered width of its longest SEGMENT (glyph widths per character class,
            segment capped at SEGMENT_CAP characters; only a longer segment may still break mid-segment) plus padding. The rest of the line is
            shared by mean cell length.
         3. If the segment minimums do not fit the line the table font shrinks in steps (7.5 .. 5.5pt); only when even that fails the minimums are
            scaled down together and the table is reported on stderr (words may then break; the coverage gate reports it).
Usage    pdf_table_cols.py PAGE.html [LINE_PT] [FONT_PT]   (rewrites in place; LINE_PT: usable width in points, default 780, portrait A4 is 527;
         FONT_PT: largest table font size, default 7.5; adds <colgroup> to every <table>, replacing any existing one, and U+200B to cell text)
Exit     0 ok, 2 usage / unreadable input.
"""
import html, re, sys

SEGMENT_CAP = 30     # a segment up to this many characters always fits its column; the coverage gate permits a mid-segment break only beyond it
PAD_PT = 7.0         # horizontal cell padding (2 x 3pt) plus border and rounding slack
FONT_STEPS = (7.5, 7.0, 6.5, 6.0, 5.5)


def break_points(w):
    """Indexes i such that a break is allowed between w[i-1] and w[i]."""
    out = []
    for i in range(1, len(w)):
        a, b = w[i - 1], w[i]
        if a in '/_': out.append(i)
        elif a == '-' and not (i >= 2 and w[i - 2].isdigit() and b.isdigit()) and b not in '-': out.append(i)
        elif a == '.' and b.isalpha(): out.append(i)
    return out


def segments(w):
    bp = [0] + break_points(w) + [len(w)]
    return [w[bp[k]:bp[k + 1]] for k in range(len(bp) - 1)]


def glyph_em(c):
    if c in 'il.,:;\'|!jt1 /': return 0.40
    if c.isupper() or c in 'mwMW@%&': return 0.82
    if c.isdigit(): return 0.66
    if ord(c) > 0x2E80: return 1.05          # CJK
    return 0.62


def word_pt(w, font):
    return sum(glyph_em(c) for c in w) * font


def cells(row):
    return [html.unescape(re.sub(r'<[^>]+>', '', c)).strip() for c in re.findall(r'<t[hd][^>]*>(.*?)</t[hd]>', row, flags=re.S)]


def add_wbr(cell_html):
    """Insert U+200B at break_points inside the text nodes of one cell (tags and entities untouched)."""
    def text(m):
        s = m.group(0)
        if not s.strip(): return s
        un = html.unescape(s)
        if '&' in s and len(un) != len(s):               # entities present: work on the unescaped text and re-escape
            pass
        out = []
        for part in re.split(r'(\s+)', un):
            if part.isspace() or not part: out.append(html.escape(part, quote=False)); continue
            bp = set(break_points(part))
            out.append(''.join(('\u200b' if i in bp else '') + html.escape(c, quote=False) for i, c in enumerate(part)))
        return ''.join(out)
    return re.sub(r'(?<=>)[^<]+|^[^<]+', text, cell_html)


def layout(table, line, maxfont):
    rows = [cells(r) for r in re.findall(r'<tr[^>]*>(.*?)</tr>', table, flags=re.S)]
    n = max((len(r) for r in rows), default=0)
    if n == 0: return None
    cols = []; wts = []
    for i in range(n):
        col = [r[i] for r in rows if i < len(r)]
        lens = [len(c) for c in col] or [1]
        segs = [s[:SEGMENT_CAP] for c in col for w in c.split() for s in segments(w)] or ['x']
        cols.append(segs)
        wts.append(max(sum(lens) / len(lens), 3.0))      # mean cell length: a column of long prose gets the room, a column of short ids keeps its segment minimum
    font = FONT_STEPS[-1]; fit = False
    for f in [s for s in FONT_STEPS if s <= maxfont] or [maxfont]:
        mins = [max(word_pt(s, f) for s in segs) + PAD_PT for segs in cols]
        font = f
        if sum(mins) <= line: fit = True; break
    if not fit:
        ms = sum(mins); mins = [m * line / ms for m in mins]
    rest = line - sum(mins)
    ws = sum(wts)
    vals = [m + rest * w / ws for m, w in zip(mins, wts)]
    tot = sum(vals)
    cg = '<colgroup>' + ''.join('<col style="width:%.2f%%">' % (100.0 * v / tot) for v in vals) + '</colgroup>'
    return cg, font, fit


def main(argv):
    if len(argv) not in (1, 2, 3): print(__doc__, file=sys.stderr); return 2
    line = float(argv[1]) if len(argv) >= 2 else 780.0
    font = float(argv[2]) if len(argv) == 3 else 7.5
    try: src = open(argv[0], encoding='utf-8').read()
    except OSError as e: print('pdf_table_cols: %s' % e, file=sys.stderr); return 2
    def f(m):
        t = re.sub(r'<colgroup>.*?</colgroup>', '', m.group(0), flags=re.S)
        lo = layout(t, line, font)
        if lo is None: return t
        cg, fs, fit = lo
        if not fit: print('pdf_table_cols: segment minimums exceed the line even at %.1fpt (words may break)' % fs, file=sys.stderr)
        t = re.sub(r'(<t[hd][^>]*>)(.*?)(</t[hd]>)', lambda x: x.group(1) + add_wbr(x.group(2)) + x.group(3), t, flags=re.S)
        t = re.sub(r'<table([^>]*)>', lambda x: '<table%s style="font-size:%.1fpt">%s' % (x.group(1), fs, cg), t, count=1)
        return t
    out = re.sub(r'<table\b.*?</table>', f, src, flags=re.S)
    open(argv[0], 'w', encoding='utf-8').write(out)
    return 0


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))
