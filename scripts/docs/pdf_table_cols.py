#!/usr/bin/env python3
"""pdf_table_cols.py - give every table of a pandoc HTML page explicit column widths for PDF rendering (used by render_md_to_pdf.sh).

Purpose  An auto-layout table whose cells hold long unbroken words (paths, identifiers) grows past the page edge (clipped text); a fixed
         layout with equal or dash-count widths leaves 1-2 character columns. This computes widths from the cell text in POINTS:
         the minimum width of a column is the estimated rendered width of its longest word (glyph widths per character class: capitals
         and digits are wider than lowercase; the word is capped at 24 characters, shrinking to 16 when the minimums do not fit) plus the cell padding and border, so a short id such as
         B100 or a file:line number never wraps; the rest of the line is shared by weight (0.5 * mean cell length + 0.5 * min(longest cell, 40)).
         When the minimums alone exceed the line they are scaled down together (words may then break; the coverage gate reports it).
Usage    pdf_table_cols.py PAGE.html [LINE_PT] [FONT_PT]   (rewrites in place; LINE_PT: usable width in points, default 780, portrait A4 is 527;
         FONT_PT: table font size, default 7.5; adds <colgroup> to every <table>, replacing any existing one)
Exit     0 ok, 2 usage / unreadable input.
"""
import html, re, sys


def cells(row):
    return [html.unescape(re.sub(r'<[^>]+>', '', c)).strip() for c in re.findall(r'<t[hd][^>]*>(.*?)</t[hd]>', row, flags=re.S)]


FLOOR = 16    # shortest word cap (characters); the coverage gate accepts a mid-word wrap only for words at least this long
PAD_PT = 7.0   # horizontal cell padding (2 x 2.25pt) plus border and rounding slack
# glyph width in em, deliberately on the wide side of Noto Serif / DejaVu Sans Mono at table size
def glyph_em(c):
    if c in 'il.,:;\'|!jt1 ': return 0.40
    if c.isupper() or c in 'mwMW@%&': return 0.82
    if c.isdigit(): return 0.66
    if ord(c) > 0x2E80: return 1.05          # CJK
    return 0.62


def word_pt(w, font):
    return sum(glyph_em(c) for c in w) * font


def colgroup(table, line, font):
    rows = [cells(r) for r in re.findall(r'<tr[^>]*>(.*?)</tr>', table, flags=re.S)]
    n = max((len(r) for r in rows), default=0)
    if n == 0: return None
    cols = []; wts = []
    for i in range(n):
        col = [r[i] for r in rows if i < len(r)]
        lens = [len(c) for c in col] or [1]
        cols.append([x for c in col for x in c.split()] or ['x'])
        wts.append(max(0.5 * sum(lens) / len(lens) + 0.5 * min(max(lens), 40), 3.0))
    # minimum = width of the widest word capped at CAP characters; when the minimums do not fit the cap shrinks (never below FLOOR),
    # so that short ids stay whole and only words longer than the cap break
    for cap in (24, 20, FLOOR):
        mins = [max(word_pt(x[:cap], font) for x in ws) + PAD_PT for ws in cols]
        if sum(mins) <= line: break
    ms = sum(mins)
    if ms > line: mins = [m * line / ms for m in mins]
    rest = line - sum(mins)
    ws = sum(wts)
    vals = [m + rest * w / ws for m, w in zip(mins, wts)]
    tot = sum(vals)
    return '<colgroup>' + ''.join('<col style="width:%.2f%%">' % (100.0 * v / tot) for v in vals) + '</colgroup>'


def main(argv):
    if len(argv) not in (1, 2, 3): print(__doc__, file=sys.stderr); return 2
    line = float(argv[1]) if len(argv) >= 2 else 780.0
    font = float(argv[2]) if len(argv) == 3 else 7.5
    try: src = open(argv[0], encoding='utf-8').read()
    except OSError as e: print('pdf_table_cols: %s' % e, file=sys.stderr); return 2
    def f(m):
        t = re.sub(r'<colgroup>.*?</colgroup>', '', m.group(0), flags=re.S)
        cg = colgroup(t, line, font)
        return t if cg is None else re.sub(r'(<table[^>]*>)', lambda x: x.group(1) + cg, t, count=1)
    out = re.sub(r'<table\b.*?</table>', f, src, flags=re.S)
    open(argv[0], 'w', encoding='utf-8').write(out)
    return 0


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))
