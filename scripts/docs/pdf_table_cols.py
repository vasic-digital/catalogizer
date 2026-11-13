#!/usr/bin/env python3
"""pdf_table_cols.py - give every table of a pandoc HTML page explicit column widths for PDF rendering (used by render_md_to_pdf.sh).

Purpose  An auto-layout table whose cells hold long unbroken words (paths, identifiers) grows past the page edge (clipped text); a fixed
         layout with equal or dash-count widths leaves 1-2 character columns. This computes widths from the cell text instead:
         weight of a column = 0.5 * mean cell length + 0.5 * min(longest cell, 40), floored at min(longest word, 24) characters so an
         ordinary identifier fits its column unbroken; weights are scaled to 100% (floors win over weights when they fit).
Usage    pdf_table_cols.py PAGE.html [TOTAL_CHARS]   (TOTAL_CHARS: characters of table text that fit one line; default 200, use 135 for portrait; rewrites in place; adds <colgroup> to every <table>, replacing any existing one)
Exit     0 ok, 2 usage / unreadable input.
"""
import html, re, sys


def cells(row):
    return [html.unescape(re.sub(r'<[^>]+>', '', c)).strip() for c in re.findall(r'<t[hd][^>]*>(.*?)</t[hd]>', row, flags=re.S)]


def colgroup(table, total):
    rows = [cells(r) for r in re.findall(r'<tr[^>]*>(.*?)</tr>', table, flags=re.S)]
    n = max((len(r) for r in rows), default=0)
    if n == 0: return None
    w = []
    for i in range(n):
        col = [r[i] for r in rows if i < len(r)]
        lens = [len(c) for c in col] or [1]
        mean = sum(lens) / len(lens)
        longest_word = max((len(x) for c in col for x in c.split()), default=1)
        weight = 0.5 * mean + 0.5 * min(max(lens), 40)
        w.append((max(weight, 3.0), min(longest_word, 24) + 1.0))
    # character budget: every column first gets its floor (scaled down only when the floors alone exceed the line), the rest is shared by weight
    floors = [b for _, b in w]
    fs = sum(floors)
    if fs > total: floors = [f * total / fs for f in floors]
    rest = total - sum(floors)
    ws = sum(a for a, _ in w)
    vals = [f + rest * a / ws for f, (a, _) in zip(floors, w)]
    tot = sum(vals)
    pct = [100.0 * v / tot for v in vals]
    return '<colgroup>' + ''.join('<col style="width:%.2f%%">' % p for p in pct) + '</colgroup>'


def main(argv):
    if len(argv) not in (1, 2): print(__doc__, file=sys.stderr); return 2
    total = float(argv[1]) if len(argv) == 2 else 200.0
    try: src = open(argv[0], encoding='utf-8').read()
    except OSError as e: print('pdf_table_cols: %s' % e, file=sys.stderr); return 2
    def f(m):
        t = re.sub(r'<colgroup>.*?</colgroup>', '', m.group(0), flags=re.S)
        cg = colgroup(t, total)
        return t if cg is None else re.sub(r'(<table[^>]*>)', lambda x: x.group(1) + cg, t, count=1)
    out = re.sub(r'<table\b.*?</table>', f, src, flags=re.S)
    open(argv[0], 'w', encoding='utf-8').write(out)
    return 0


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))
